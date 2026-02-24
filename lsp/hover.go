package lsp

import (
	"fmt"
	"strings"

	"github.com/pgavlin/starlark-go/resolve"
	"github.com/pgavlin/starlark-go/syntax"
)

// hover returns hover information for the identifier at the given position.
func (s *Server) hover(params *TextDocumentPositionParams) *Hover {
	s.mu.RLock()
	doc, ok := s.documents[params.TextDocument.URI]
	s.mu.RUnlock()
	if !ok || doc.analysis == nil {
		return nil
	}

	// Check if cursor is on a dot expression member (e.g. sh.exec)
	if hover := s.hoverDotExpr(doc, params.Position); hover != nil {
		return hover
	}

	ident := doc.identAtPosition(params.Position)
	if ident == nil {
		return nil
	}

	binding, _ := ident.Binding.(*resolve.Binding)

	// Predeclared/Universal builtins
	if binding != nil && (binding.Scope == resolve.Predeclared || binding.Scope == resolve.Universal) {
		if info, ok := builtinRegistry[ident.Name]; ok {
			return &Hover{
				Contents: builtinHoverMarkup(info),
				Range:    rangePtr(identToRange(ident)),
			}
		}
		return &Hover{
			Contents: Markup{Kind: "markdown", Value: fmt.Sprintf("(builtin) **%s**", ident.Name)},
			Range:    rangePtr(identToRange(ident)),
		}
	}

	// User-defined functions
	if binding != nil && binding.First != nil {
		def := findDefStmtForName(doc, binding.First)
		if def != nil {
			return &Hover{
				Contents: defStmtHoverMarkup(def),
				Range:    rangePtr(identToRange(ident)),
			}
		}

		// Variable
		return &Hover{
			Contents: Markup{
				Kind:  "markdown",
				Value: fmt.Sprintf("(variable) **%s**", ident.Name),
			},
			Range: rangePtr(identToRange(ident)),
		}
	}

	return nil
}

// hoverDotExpr checks if the cursor is on a dot expression and provides hover for the member.
func (s *Server) hoverDotExpr(doc *Document, pos Position) *Hover {
	if doc.file == nil {
		return nil
	}

	line := pos.Line + 1
	col := pos.Character + 1

	var result *Hover
	syntax.Walk(doc.file, func(n syntax.Node) bool {
		if result != nil {
			return false
		}
		dot, ok := n.(*syntax.DotExpr)
		if !ok {
			return true
		}
		nameStart, nameEnd := dot.Name.Span()
		if int(nameStart.Line) == line && int(nameStart.Col) <= col && col <= int(nameEnd.Col) {
			// Cursor is on the member name
			if xIdent, ok := dot.X.(*syntax.Ident); ok {
				members := membersForName(xIdent.Name)
				for _, m := range members {
					if m.Name == dot.Name.Name {
						var value string
						if m.Signature != "" {
							value = fmt.Sprintf("```python\n%s\n```\n%s", m.Signature, m.Doc)
						} else {
							value = fmt.Sprintf("**%s.%s**\n\n%s", xIdent.Name, m.Name, m.Doc)
						}
						result = &Hover{
							Contents: Markup{Kind: "markdown", Value: value},
							Range:    rangePtr(identToRange(dot.Name)),
						}
						return false
					}
				}
			}
			// Check nested dot (e.g. os.path.join)
			if innerDot, ok := dot.X.(*syntax.DotExpr); ok {
				if xIdent, ok := innerDot.X.(*syntax.Ident); ok {
					members := membersForName(xIdent.Name)
					for _, m := range members {
						if m.Name == innerDot.Name.Name {
							for _, sm := range m.Members {
								if sm.Name == dot.Name.Name {
									var value string
									if sm.Signature != "" {
										value = fmt.Sprintf("```python\n%s\n```\n%s", sm.Signature, sm.Doc)
									} else {
										value = fmt.Sprintf("**%s.%s.%s**\n\n%s", xIdent.Name, innerDot.Name.Name, sm.Name, sm.Doc)
									}
									result = &Hover{
										Contents: Markup{Kind: "markdown", Value: value},
										Range:    rangePtr(identToRange(dot.Name)),
									}
									return false
								}
							}
						}
					}
				}
			}
		}
		return true
	})
	return result
}

func findDefStmtForName(doc *Document, ident *syntax.Ident) *syntax.DefStmt {
	if doc.file == nil {
		return nil
	}
	for _, stmt := range doc.file.Stmts {
		if def, ok := stmt.(*syntax.DefStmt); ok {
			if def.Name == ident {
				return def
			}
		}
	}
	return nil
}

func builtinHoverMarkup(info *BuiltinInfo) Markup {
	var b strings.Builder
	if info.Signature != "" {
		fmt.Fprintf(&b, "```python\n%s\n```\n", info.Signature)
	} else {
		fmt.Fprintf(&b, "**%s**\n\n", info.Name)
	}
	if info.Doc != "" {
		b.WriteString(info.Doc)
	}
	return Markup{Kind: "markdown", Value: b.String()}
}

func defStmtHoverMarkup(def *syntax.DefStmt) Markup {
	var b strings.Builder
	fmt.Fprintf(&b, "```python\ndef %s(", def.Name.Name)
	for i, p := range def.Params {
		if i > 0 {
			b.WriteString(", ")
		}
		writeParam(&b, p)
	}
	b.WriteString(")\n```")

	// Extract docstring
	if doc := extractDocstring(def); doc != "" {
		fmt.Fprintf(&b, "\n\n%s", doc)
	}

	return Markup{Kind: "markdown", Value: b.String()}
}

func writeParam(b *strings.Builder, p syntax.Expr) {
	switch p := p.(type) {
	case *syntax.Ident:
		b.WriteString(p.Name)
	case *syntax.BinaryExpr:
		if p.Op == syntax.EQ {
			if ident, ok := p.X.(*syntax.Ident); ok {
				fmt.Fprintf(b, "%s=", ident.Name)
				if lit, ok := p.Y.(*syntax.Literal); ok {
					b.WriteString(lit.Raw)
				} else {
					b.WriteString("...")
				}
			}
		}
	case *syntax.UnaryExpr:
		switch p.Op { //nolint:exhaustive // only STAR and STARSTAR are valid for parameters
		case syntax.STAR:
			if p.X != nil {
				if ident, ok := p.X.(*syntax.Ident); ok {
					fmt.Fprintf(b, "*%s", ident.Name)
				}
			} else {
				b.WriteString("*")
			}
		case syntax.STARSTAR:
			if ident, ok := p.X.(*syntax.Ident); ok {
				fmt.Fprintf(b, "**%s", ident.Name)
			}
		}
	}
}

func extractDocstring(def *syntax.DefStmt) string {
	if len(def.Body) == 0 {
		return ""
	}
	exprStmt, ok := def.Body[0].(*syntax.ExprStmt)
	if !ok {
		return ""
	}
	lit, ok := exprStmt.X.(*syntax.Literal)
	if !ok || lit.Token != syntax.STRING {
		return ""
	}
	s, ok := lit.Value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

func rangePtr(r Range) *Range {
	return &r
}
