package lsp

import (
	"fmt"
	"strings"

	"github.com/pgavlin/starlark-go/resolve"
	"github.com/pgavlin/starlark-go/starlark"
	"github.com/pgavlin/starlark-go/syntax"
)

const (
	completionKindText     = 1
	completionKindMethod   = 2
	completionKindFunction = 3
	completionKindVariable = 6
	completionKindModule   = 9
	completionKindKeyword  = 14
	completionKindConstant = 21
)

// completion returns completion items for the given position.
func (s *Server) completion(params *CompletionParams) *CompletionList {
	s.mu.RLock()
	doc, ok := s.documents[params.TextDocument.URI]
	s.mu.RUnlock()
	if !ok {
		return &CompletionList{Items: []CompletionItem{}}
	}

	// Determine context from position
	line := params.Position.Line
	char := params.Position.Character

	// Get the line text
	lineText := getLineText(doc.Content, line)
	if lineText == "" && doc.Content != "" {
		return &CompletionList{Items: s.allCompletions(doc)}
	}

	prefix := ""
	if char <= len(lineText) {
		prefix = lineText[:char]
	}

	// Dot completion: e.g. "sh." or "os.path."
	if strings.HasSuffix(prefix, ".") || containsDot(prefix) {
		if items := s.dotCompletion(doc, prefix); len(items) > 0 {
			return &CompletionList{Items: items}
		}
	}

	// Inside a load() string
	if isInsideLoadString(prefix) {
		return &CompletionList{Items: s.loadPathCompletion(doc)}
	}

	// Decorator: after @
	if strings.HasSuffix(strings.TrimSpace(prefix), "@") {
		return &CompletionList{Items: []CompletionItem{
			{Label: "target", Kind: completionKindFunction, Detail: "Dawn target decorator"},
		}}
	}

	// Keyword argument completion inside function calls
	if items := s.keywordArgCompletion(doc, params.Position); len(items) > 0 {
		return &CompletionList{Items: append(items, s.allCompletions(doc)...)}
	}

	// General identifier completion
	return &CompletionList{Items: s.allCompletions(doc)}
}

// dotCompletion provides completion for dot access (e.g. sh.exec, os.path.join).
func (s *Server) dotCompletion(doc *Document, prefix string) []CompletionItem {
	// Find the identifier before the dot
	parts := splitDotPrefix(prefix)
	if len(parts) == 0 {
		return nil
	}

	rootName := parts[0]

	if len(parts) == 1 {
		// Direct member access: sh., os., json., host.
		members := membersForName(rootName)
		return membersToCompletionItems(rootName, members)
	}

	if len(parts) == 2 {
		// Nested: os.path.
		members := membersForName(rootName)
		for _, m := range members {
			if m.Name == parts[1] && len(m.Members) > 0 {
				return membersToCompletionItems(rootName+"."+parts[1], m.Members)
			}
		}
	}

	return nil
}

func membersToCompletionItems(prefix string, members []BuiltinMember) []CompletionItem {
	items := make([]CompletionItem, 0, len(members))
	for _, m := range members {
		kind := completionKindFunction
		if m.Signature == "" {
			if len(m.Members) > 0 {
				kind = completionKindModule
			} else {
				kind = completionKindVariable
			}
		}
		items = append(items, CompletionItem{
			Label:  m.Name,
			Kind:   kind,
			Detail: m.Signature,
			Documentation: func() *Markup {
				if m.Doc == "" {
					return nil
				}
				return &Markup{Kind: "plaintext", Value: m.Doc}
			}(),
		})
	}
	return items
}

// allCompletions returns completions for all in-scope names.
func (s *Server) allCompletions(doc *Document) []CompletionItem {
	items := make([]CompletionItem, 0, len(starlark.Universe)+len(builtinRegistry)+20)

	// Starlark universals (len, range, str, etc.)
	for name := range starlark.Universe {
		items = append(items, CompletionItem{
			Label:  name,
			Kind:   completionKindFunction,
			Detail: "(builtin)",
		})
	}

	// Dawn predeclared builtins (with docs from registry)
	for name, info := range builtinRegistry {
		kind := completionKindFunction
		if info.Signature == "" {
			if len(info.Members) > 0 {
				kind = completionKindModule
			} else {
				kind = completionKindVariable
			}
		}
		items = append(items, CompletionItem{
			Label:  name,
			Kind:   kind,
			Detail: info.Signature,
			Documentation: func() *Markup {
				if info.Doc == "" {
					return nil
				}
				return &Markup{Kind: "plaintext", Value: info.Doc}
			}(),
		})
	}

	// Additional predeclared names from env (not already in builtinRegistry)
	if s.env != nil {
		for name := range s.env.Predeclared {
			if _, ok := builtinRegistry[name]; ok {
				continue
			}
			if starlark.Universe[name] != nil {
				continue
			}
			items = append(items, CompletionItem{
				Label:  name,
				Kind:   completionKindVariable,
				Detail: "(predeclared)",
			})
		}
	}

	// Globals and loaded names from the current file
	if doc.analysis != nil {
		for _, fn := range doc.analysis.Functions {
			detail := fmt.Sprintf("def %s(...)", fn.Name)
			if fn.IsTarget {
				detail = "@target " + detail
			}
			items = append(items, CompletionItem{
				Label:  fn.Name,
				Kind:   completionKindFunction,
				Detail: detail,
			})
		}

		for _, load := range doc.analysis.Loads {
			for _, name := range load.Names {
				items = append(items, CompletionItem{
					Label:  name,
					Kind:   completionKindVariable,
					Detail: "from " + load.ModulePath,
				})
			}
		}

		// Global assignments
		if doc.file != nil {
			for _, stmt := range doc.file.Stmts {
				if assign, ok := stmt.(*syntax.AssignStmt); ok {
					if ident, ok := assign.LHS.(*syntax.Ident); ok {
						items = append(items, CompletionItem{
							Label:  ident.Name,
							Kind:   completionKindVariable,
							Detail: "(global variable)",
						})
					}
				}
			}
		}
	}

	// Local variables at cursor position - check bindings
	if doc.analysis != nil {
		seen := map[string]bool{}
		for _, id := range doc.analysis.Idents {
			b, ok := id.Binding.(*resolve.Binding)
			if !ok || seen[id.Name] {
				continue
			}
			if b.Scope == resolve.Local || b.Scope == resolve.Free || b.Scope == resolve.Cell {
				seen[id.Name] = true
				items = append(items, CompletionItem{
					Label:  id.Name,
					Kind:   completionKindVariable,
					Detail: "(local variable)",
				})
			}
		}
	}

	// Starlark keywords
	keywords := []string{
		"def", "if", "elif", "else", "for", "in", "return",
		"load", "pass", "break", "continue", "and", "or", "not",
		"True", "False", "None", "lambda",
	}
	for _, kw := range keywords {
		items = append(items, CompletionItem{
			Label: kw,
			Kind:  completionKindKeyword,
		})
	}

	return deduplicateCompletions(items)
}

// loadPathCompletion provides completions for load() paths.
func (s *Server) loadPathCompletion(doc *Document) []CompletionItem {
	if s.project == nil {
		return nil
	}

	items := make([]CompletionItem, 0, len(s.project.BuildFiles)+len(s.project.Requirements))

	// Suggest known BUILD.dawn package paths
	for pkg := range s.project.BuildFiles {
		items = append(items, CompletionItem{
			Label:  pkg + ":BUILD.dawn",
			Kind:   completionKindModule,
			Detail: "BUILD.dawn module",
		})
	}

	// Suggest aliases
	for alias := range s.project.Requirements {
		items = append(items, CompletionItem{
			Label:  alias + "//",
			Kind:   completionKindModule,
			Detail: "project alias",
		})
	}

	return items
}

func getLineText(content string, line int) string {
	lines := strings.Split(content, "\n")
	if line < 0 || line >= len(lines) {
		return ""
	}
	return lines[line]
}

func containsDot(s string) bool {
	// Check if we're in the middle of a dotted name
	// e.g. "  sh.ex" or "os.path.jo"
	for i := len(s) - 1; i >= 0; i-- {
		c := s[i]
		if c == '.' {
			return true
		}
		if !isIdentChar(c) {
			return false
		}
	}
	return false
}

func isIdentChar(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_'
}

func splitDotPrefix(prefix string) []string {
	// Extract the dotted expression ending at cursor.
	// "  sh." -> ["sh"]
	// "  os.path." -> ["os", "path"]
	// "  os.path.jo" -> ["os", "path"]
	end := len(prefix)

	// If the last char is a dot, we want completion after it
	trailingDot := end > 0 && prefix[end-1] == '.'
	if trailingDot {
		end--
	}

	// Walk backward to find the start of the dotted expression
	start := end
	for start > 0 {
		c := prefix[start-1]
		if c == '.' || isIdentChar(c) {
			start--
		} else {
			break
		}
	}

	expr := prefix[start:end]
	if expr == "" {
		return nil
	}

	parts := strings.Split(expr, ".")
	// Filter empty parts
	result := parts[:0]
	for _, p := range parts {
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func isInsideLoadString(prefix string) bool {
	// Simple heuristic: check if we're inside load("...")
	trimmed := strings.TrimSpace(prefix)
	return strings.HasPrefix(trimmed, "load(\"") || strings.HasPrefix(trimmed, "load('")
}

// keywordArgCompletion provides completion for keyword arguments inside function calls.
func (s *Server) keywordArgCompletion(doc *Document, pos Position) []CompletionItem {
	if doc.file == nil {
		return nil
	}

	line := pos.Line + 1
	col := pos.Character + 1

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

	// Get parameter names for the called function.
	var paramNames []string
	switch fn := call.Fn.(type) {
	case *syntax.Ident:
		paramNames = s.paramNamesForFunc(doc, fn.Name)
	case *syntax.DotExpr:
		if xIdent, ok := fn.X.(*syntax.Ident); ok {
			paramNames = s.paramNamesForDotAccess(xIdent.Name, fn.Name.Name)
		}
	}
	if len(paramNames) == 0 {
		return nil
	}

	// Filter out already-used keyword args.
	used := map[string]bool{}
	for _, arg := range call.Args {
		if bin, ok := arg.(*syntax.BinaryExpr); ok && bin.Op == syntax.EQ {
			if ident, ok := bin.X.(*syntax.Ident); ok {
				used[ident.Name] = true
			}
		}
	}

	var items []CompletionItem
	for _, p := range paramNames {
		if !used[p] && p != "" && p[0] != '*' {
			items = append(items, CompletionItem{
				Label:      p,
				Kind:       completionKindVariable,
				Detail:     "parameter",
				InsertText: p + "=",
			})
		}
	}
	return items
}

// paramNamesForFunc returns parameter names for a function by name.
func (s *Server) paramNamesForFunc(doc *Document, name string) []string {
	// Check builtins.
	if info, ok := builtinRegistry[name]; ok && info.Signature != "" {
		return extractParamNamesFromSignature(info.Signature)
	}

	// Check user-defined functions.
	if doc.analysis != nil {
		for _, fn := range doc.analysis.Functions {
			if fn.Name == name {
				return extractParamNamesFromDef(fn.DefStmt)
			}
		}

		// Check loaded functions.
		if s.project != nil {
			for _, load := range doc.analysis.Loads {
				for i, to := range load.Stmt.To {
					if to.Name == name {
						fromName := load.Stmt.From[i].Name
						resolvedPath := s.project.ResolveLoadPath(doc.Path, load.ModulePath)
						if resolvedPath != "" {
							if names := s.paramNamesFromFile(resolvedPath, fromName); len(names) > 0 {
								return names
							}
						}
					}
				}
			}
		}
	}

	return nil
}

// paramNamesForDotAccess returns parameter names for a member function.
func (s *Server) paramNamesForDotAccess(obj, member string) []string {
	members := membersForName(obj)
	for _, m := range members {
		if m.Name == member && m.Signature != "" {
			return extractParamNamesFromSignature(m.Signature)
		}
	}
	return nil
}

// paramNamesFromFile extracts parameter names for a function in a file.
func (s *Server) paramNamesFromFile(path, name string) []string {
	uri := pathToURI(path)
	s.mu.RLock()
	doc, ok := s.documents[uri]
	s.mu.RUnlock()
	if ok && doc.analysis != nil {
		for _, fn := range doc.analysis.Functions {
			if fn.Name == name {
				return extractParamNamesFromDef(fn.DefStmt)
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
			return extractParamNamesFromDef(fn.DefStmt)
		}
	}
	return nil
}

func extractParamNamesFromSignature(sig string) []string {
	lparen := strings.Index(sig, "(")
	if lparen < 0 {
		return nil
	}
	rparen := strings.LastIndex(sig, ")")
	if rparen < 0 {
		return nil
	}
	params := splitParams(sig[lparen+1 : rparen])
	names := make([]string, 0, len(params))
	for _, p := range params {
		p = strings.TrimSpace(p)
		// Extract just the name (before = or *)
		if eqIdx := strings.Index(p, "="); eqIdx >= 0 {
			p = strings.TrimSpace(p[:eqIdx])
		}
		if p != "" {
			names = append(names, p)
		}
	}
	return names
}

func extractParamNamesFromDef(def *syntax.DefStmt) []string {
	names := make([]string, 0, len(def.Params))
	for _, p := range def.Params {
		switch p := p.(type) {
		case *syntax.Ident:
			names = append(names, p.Name)
		case *syntax.BinaryExpr:
			if ident, ok := p.X.(*syntax.Ident); ok {
				names = append(names, ident.Name)
			}
		case *syntax.UnaryExpr:
			if p.X != nil {
				if ident, ok := p.X.(*syntax.Ident); ok {
					if p.Op == syntax.STAR {
						names = append(names, "*"+ident.Name)
					} else {
						names = append(names, "**"+ident.Name)
					}
				}
			}
		}
	}
	return names
}

func deduplicateCompletions(items []CompletionItem) []CompletionItem {
	seen := map[string]bool{}
	result := items[:0]
	for _, item := range items {
		if !seen[item.Label] {
			seen[item.Label] = true
			result = append(result, item)
		}
	}
	return result
}
