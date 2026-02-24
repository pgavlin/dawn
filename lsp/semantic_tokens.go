package lsp

import (
	"sort"

	"github.com/pgavlin/starlark-go/resolve"
	"github.com/pgavlin/starlark-go/syntax"
)

// Semantic token types and modifiers indices (must match legend in capabilities).
const (
	tokenTypeNamespace = iota
	tokenTypeFunction
	tokenTypeVariable
	tokenTypeParameter
	tokenTypeProperty
	tokenTypeString
	tokenTypeNumber
	tokenTypeKeyword
	tokenTypeComment
	tokenTypeDecorator
	tokenTypeMacro
)

const (
	tokenModDeclaration = 1 << iota
	tokenModDefinition
	tokenModDefaultLibrary
)

// SemanticTokenTypes is the list of token types in the legend.
var SemanticTokenTypes = []string{
	"namespace",
	"function",
	"variable",
	"parameter",
	"property",
	"string",
	"number",
	"keyword",
	"comment",
	"decorator",
	"macro",
}

// SemanticTokenModifiers is the list of token modifiers in the legend.
var SemanticTokenModifiers = []string{
	"declaration",
	"definition",
	"defaultLibrary",
}

type semanticToken struct {
	line      uint32
	col       uint32
	length    uint32
	tokenType uint32
	modifiers uint32
}

// semanticTokens computes semantic tokens for the given document.
func (s *Server) semanticTokens(params *SemanticTokensParams) *SemanticTokensResult {
	s.mu.RLock()
	doc, ok := s.documents[params.TextDocument.URI]
	s.mu.RUnlock()
	if !ok || doc.file == nil || doc.analysis == nil {
		return &SemanticTokensResult{Data: []uint32{}}
	}

	tokens := make([]semanticToken, 0, len(doc.analysis.Idents))

	// Classify identifiers using resolver bindings.
	for _, ident := range doc.analysis.Idents {
		b, _ := ident.Binding.(*resolve.Binding)
		start, _ := ident.Span()
		if !start.IsValid() || start.Line < 1 || start.Col < 1 {
			continue
		}

		tok := semanticToken{
			line:   uint32(start.Line - 1),  //nolint:gosec // line numbers are always small positive values
			col:    uint32(start.Col - 1),   //nolint:gosec // col numbers are always small positive values
			length: uint32(len(ident.Name)), //nolint:gosec // identifier names have bounded length
		}

		if b == nil {
			continue
		}

		switch b.Scope {
		case resolve.Predeclared, resolve.Universal:
			tok.tokenType = tokenTypeMacro
			tok.modifiers = tokenModDefaultLibrary
		case resolve.Global:
			if b.First == ident {
				// Check if this is a function definition.
				if isDefName(doc, ident) {
					tok.tokenType = tokenTypeFunction
					tok.modifiers = tokenModDefinition
				} else {
					tok.tokenType = tokenTypeVariable
					tok.modifiers = tokenModDeclaration
				}
			} else {
				if isDefName(doc, b.First) {
					tok.tokenType = tokenTypeFunction
				} else {
					tok.tokenType = tokenTypeVariable
				}
			}
		case resolve.Local, resolve.Cell, resolve.Free:
			tok.tokenType = tokenTypeVariable
			if b.First == ident {
				tok.modifiers = tokenModDeclaration
			}
		case resolve.Undefined:
			continue
		}

		tokens = append(tokens, tok)
	}

	// Add decorator tokens.
	for _, fn := range doc.analysis.Functions {
		for _, dec := range fn.DefStmt.Decorators {
			start := dec.At
			if start.IsValid() && start.Line >= 1 && start.Col >= 1 {
				tokens = append(tokens, semanticToken{
					line:      uint32(start.Line - 1), //nolint:gosec // line numbers are always small positive values
					col:       uint32(start.Col - 1),  //nolint:gosec // col numbers are always small positive values
					length:    1,                      // just the @
					tokenType: tokenTypeDecorator,
				})
			}
		}
	}

	// Add load module strings as namespace tokens.
	for _, load := range doc.analysis.Loads {
		start, end := load.Stmt.Module.Span()
		if start.IsValid() && start.Line >= 1 && start.Col >= 1 && end.Col > start.Col {
			tokens = append(tokens, semanticToken{
				line:      uint32(start.Line - 1),      //nolint:gosec // line numbers are always small positive values
				col:       uint32(start.Col - 1),       //nolint:gosec // col numbers are always small positive values
				length:    uint32(end.Col - start.Col), //nolint:gosec // col difference is always non-negative
				tokenType: tokenTypeNamespace,
			})
		}
	}

	// Sort tokens by position.
	sort.Slice(tokens, func(i, j int) bool {
		if tokens[i].line != tokens[j].line {
			return tokens[i].line < tokens[j].line
		}
		return tokens[i].col < tokens[j].col
	})

	// Encode as LSP delta format.
	data := make([]uint32, 0, len(tokens)*5)
	var prevLine, prevCol uint32
	for _, tok := range tokens {
		deltaLine := tok.line - prevLine
		deltaCol := tok.col
		if deltaLine == 0 {
			deltaCol = tok.col - prevCol
		}
		data = append(data,
			deltaLine,
			deltaCol,
			tok.length,
			tok.tokenType,
			tok.modifiers,
		)
		prevLine = tok.line
		prevCol = tok.col
	}

	return &SemanticTokensResult{Data: data}
}

func isDefName(doc *Document, ident *syntax.Ident) bool {
	if doc.file == nil {
		return false
	}
	for _, stmt := range doc.file.Stmts {
		if def, ok := stmt.(*syntax.DefStmt); ok {
			if def.Name == ident {
				return true
			}
		}
	}
	return false
}
