package lsp

import (
	"fmt"
	"strings"

	"github.com/pgavlin/starlark-go/syntax"
)

// signatureHelp returns signature help for the function call at the given position.
func (s *Server) signatureHelp(params *SignatureHelpParams) *SignatureHelp {
	s.mu.RLock()
	doc, ok := s.documents[params.TextDocument.URI]
	s.mu.RUnlock()
	if !ok || doc.file == nil {
		return nil
	}

	line := params.Position.Line + 1
	col := params.Position.Character + 1

	// Find the enclosing CallExpr.
	var call *syntax.CallExpr
	syntax.Walk(doc.file, func(n syntax.Node) bool {
		if call != nil {
			return false
		}
		c, ok := n.(*syntax.CallExpr)
		if !ok {
			return true
		}
		// Check if cursor is between Lparen and Rparen.
		lpl, lpc := int(c.Lparen.Line), int(c.Lparen.Col)
		rpl, rpc := int(c.Rparen.Line), int(c.Rparen.Col)
		if (lpl < line || (lpl == line && lpc <= col)) &&
			(rpl > line || (rpl == line && rpc >= col)) {
			call = c
		}
		return true
	})
	if call == nil {
		return nil
	}

	// Count commas before cursor to determine active parameter.
	activeParam := 0
	for _, arg := range call.Args {
		argStart, _ := arg.Span()
		asl, asc := int(argStart.Line), int(argStart.Col)
		if asl > line || (asl == line && asc > col) {
			break
		}
		activeParam++
	}
	if activeParam > 0 {
		activeParam--
	}

	// Determine function name and look up signature.
	var sigInfo *SignatureInformation

	switch fn := call.Fn.(type) {
	case *syntax.Ident:
		sigInfo = s.signatureForName(doc, fn.Name)
	case *syntax.DotExpr:
		if xIdent, ok := fn.X.(*syntax.Ident); ok {
			sigInfo = s.signatureForDotAccess(xIdent.Name, fn.Name.Name)
		}
	}

	if sigInfo == nil {
		return nil
	}

	return &SignatureHelp{
		Signatures:      []SignatureInformation{*sigInfo},
		ActiveSignature: 0,
		ActiveParameter: activeParam,
	}
}

func (s *Server) signatureForName(doc *Document, name string) *SignatureInformation {
	// Check builtins.
	if info, ok := builtinRegistry[name]; ok && info.Signature != "" {
		return buildSignatureInfo(info.Signature, info.Doc)
	}

	// Check user-defined functions in the file.
	if doc.analysis != nil {
		for _, fn := range doc.analysis.Functions {
			if fn.Name == name {
				return defStmtSignatureInfo(fn.DefStmt)
			}
		}

		// Check loaded functions (cross-file).
		if s.project != nil {
			if sig := s.signatureForLoadedName(doc, name); sig != nil {
				return sig
			}
		}
	}

	return nil
}

// signatureForLoadedName resolves signatures for names imported via load().
func (s *Server) signatureForLoadedName(doc *Document, name string) *SignatureInformation {
	for _, load := range doc.analysis.Loads {
		for i, to := range load.Stmt.To {
			if to.Name == name {
				fromName := load.Stmt.From[i].Name
				resolvedPath := s.project.ResolveLoadPath(doc.Path, load.ModulePath)
				if resolvedPath != "" {
					return s.signatureFromFile(resolvedPath, fromName)
				}
			}
		}
	}
	return nil
}

// signatureFromFile finds a function signature in a file (open or on disk).
func (s *Server) signatureFromFile(path, name string) *SignatureInformation {
	uri := pathToURI(path)
	s.mu.RLock()
	doc, ok := s.documents[uri]
	s.mu.RUnlock()
	if ok && doc.analysis != nil {
		for _, fn := range doc.analysis.Functions {
			if fn.Name == name {
				return defStmtSignatureInfo(fn.DefStmt)
			}
		}
		return nil
	}

	f, err := syntax.Parse(path, nil, 0)
	if err != nil {
		return nil
	}
	analysis := extractAnalysis(f)
	for _, fn := range analysis.Functions {
		if fn.Name == name {
			return defStmtSignatureInfo(fn.DefStmt)
		}
	}
	return nil
}

func (s *Server) signatureForDotAccess(obj, member string) *SignatureInformation {
	members := membersForName(obj)
	for _, m := range members {
		if m.Name == member && m.Signature != "" {
			return buildSignatureInfo(m.Signature, m.Doc)
		}
	}
	return nil
}

func buildSignatureInfo(sig, doc string) *SignatureInformation {
	info := &SignatureInformation{
		Label: sig,
	}
	if doc != "" {
		info.Documentation = &Markup{Kind: "plaintext", Value: doc}
	}

	// Parse parameters from signature string.
	if lparen := strings.Index(sig, "("); lparen != -1 {
		if rparen := strings.LastIndex(sig, ")"); rparen != -1 {
			paramsStr := sig[lparen+1 : rparen]
			params := splitParams(paramsStr)
			for _, p := range params {
				p = strings.TrimSpace(p)
				// Find the offset of this parameter in the label.
				idx := strings.Index(sig, p)
				if idx >= 0 {
					info.Parameters = append(info.Parameters, ParameterInformation{
						Label: [2]int{idx, idx + len(p)},
					})
				}
			}
		}
	}

	return info
}

func defStmtSignatureInfo(def *syntax.DefStmt) *SignatureInformation {
	var b strings.Builder
	fmt.Fprintf(&b, "%s(", def.Name.Name)
	for i, p := range def.Params {
		if i > 0 {
			b.WriteString(", ")
		}
		writeParam(&b, p)
	}
	b.WriteString(")")

	label := b.String()
	info := &SignatureInformation{Label: label}

	// Extract docstring.
	if doc := extractDocstring(def); doc != "" {
		info.Documentation = &Markup{Kind: "plaintext", Value: doc}
	}

	// Build parameter info.
	for _, p := range def.Params {
		var paramName string
		switch p := p.(type) {
		case *syntax.Ident:
			paramName = p.Name
		case *syntax.BinaryExpr:
			if ident, ok := p.X.(*syntax.Ident); ok {
				paramName = ident.Name
			}
		case *syntax.UnaryExpr:
			if p.X != nil {
				if ident, ok := p.X.(*syntax.Ident); ok {
					if p.Op == syntax.STAR {
						paramName = "*" + ident.Name
					} else {
						paramName = "**" + ident.Name
					}
				}
			}
		}
		if paramName != "" {
			idx := strings.Index(label, paramName)
			if idx >= 0 {
				info.Parameters = append(info.Parameters, ParameterInformation{
					Label: [2]int{idx, idx + len(paramName)},
				})
			}
		}
	}

	return info
}

func splitParams(s string) []string {
	var params []string
	depth := 0
	start := 0
	for i, c := range s {
		switch c {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case ',':
			if depth == 0 {
				params = append(params, s[start:i])
				start = i + 1
			}
		}
	}
	if start < len(s) {
		params = append(params, s[start:])
	}
	return params
}
