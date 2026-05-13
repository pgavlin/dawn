package lsp

import (
	"github.com/pgavlin/dawn"
	"github.com/pgavlin/starlark-go/resolve"
	"github.com/pgavlin/starlark-go/starlark"
	"github.com/pgavlin/starlark-go/syntax"
	"github.com/pgavlin/starlark-go/typecheck"
)

// FileAnalysis holds extracted information from a parsed and resolved file.
type FileAnalysis struct {
	Targets   []TargetDecl
	Loads     []LoadInfo
	Functions []FuncDecl
	Globals   []GlobalDecl
	Idents    []*syntax.Ident
}

// GlobalDecl represents a top-level variable assignment.
type GlobalDecl struct {
	Name  string
	Ident *syntax.Ident
}

// analyze parses, resolves, type-checks, and extracts information from the document.
func (d *Document) analyze(env *typecheck.Env) {
	d.file = nil
	d.parseErr = nil
	d.resolveErr = nil
	d.checkInfo = nil
	d.checkErrs = nil
	d.analysis = nil

	// Parse
	f, err := syntax.Parse(d.Path, d.Content, syntax.RetainComments)
	if err != nil {
		d.parseErr = err
		// Try to get a partial AST even on parse error
		if f == nil {
			return
		}
	}
	d.file = f

	// Resolve and type-check
	if env != nil {
		resolveErr := resolve.File(f, dawn.IsPredeclared(env), dawn.IsUniversal)
		if resolveErr != nil {
			d.resolveErr = resolveErr
		}

		info := &typecheck.Info{
			Defs:  make(map[*syntax.Ident]*typecheck.Binding),
			Uses:  make(map[*syntax.Ident]*typecheck.UseBinding),
			Types: make(map[syntax.Expr]typecheck.TypeAndValue),
		}
		d.checkErrs = typecheck.Check(f, env, info)
		d.checkInfo = info
	} else {
		// No env available — resolve with builtinRegistry names as predeclared.
		resolveErr := resolve.File(f, func(name string) bool {
			_, ok := builtinRegistry[name]
			return ok
		}, starlark.Universe.Has)
		if resolveErr != nil {
			d.resolveErr = resolveErr
		}
	}

	// Extract analysis info
	d.analysis = extractAnalysis(f)
}

// extractAnalysis walks the AST to collect targets, loads, functions, and identifiers.
func extractAnalysis(f *syntax.File) *FileAnalysis {
	a := &FileAnalysis{}

	for _, stmt := range f.Stmts {
		switch s := stmt.(type) {
		case *syntax.DefStmt:
			isTarget := false
			for _, dec := range s.Decorators {
				if ident, ok := dec.Expr.(*syntax.Ident); ok && ident.Name == "target" {
					isTarget = true
				}
				if call, ok := dec.Expr.(*syntax.CallExpr); ok {
					if ident, ok := call.Fn.(*syntax.Ident); ok && ident.Name == "target" {
						isTarget = true
					}
				}
			}
			fd := FuncDecl{
				Name:     s.Name.Name,
				DefStmt:  s,
				IsTarget: isTarget,
			}
			a.Functions = append(a.Functions, fd)

			if isTarget {
				a.Targets = append(a.Targets, TargetDecl{
					Name:     s.Name.Name,
					DefStmt:  s,
					Position: s.Def,
				})
			}

		case *syntax.AssignStmt:
			if ident, ok := s.LHS.(*syntax.Ident); ok {
				a.Globals = append(a.Globals, GlobalDecl{
					Name:  ident.Name,
					Ident: ident,
				})
			}

		case *syntax.LoadStmt:
			info := LoadInfo{
				Stmt:       s,
				ModulePath: s.ModuleName(),
			}
			for _, to := range s.To {
				info.Names = append(info.Names, to.Name)
			}
			a.Loads = append(a.Loads, info)
		}
	}

	// Collect all identifiers via AST walk.
	syntax.Walk(f, func(n syntax.Node) bool {
		if ident, ok := n.(*syntax.Ident); ok {
			a.Idents = append(a.Idents, ident)
		}
		return true
	})

	return a
}

// identAtPosition finds the identifier at the given LSP position.
func (d *Document) identAtPosition(pos Position) *syntax.Ident {
	if d.analysis == nil {
		return nil
	}

	line := pos.Line + 1     // LSP is 0-based, starlark is 1-based
	col := pos.Character + 1 // same

	for _, ident := range d.analysis.Idents {
		start, end := ident.Span()
		if int(start.Line) == line && int(start.Col) <= col && col <= int(end.Col) {
			return ident
		}
	}
	return nil
}

// nodeAtPosition finds the innermost node at the given LSP position.
func (d *Document) nodeAtPosition(pos Position) syntax.Node {
	if d.file == nil {
		return nil
	}

	line := pos.Line + 1
	col := pos.Character + 1

	var best syntax.Node
	syntax.Walk(d.file, func(n syntax.Node) bool {
		if n == nil {
			return false
		}
		start, end := n.Span()
		if !start.IsValid() {
			return true
		}
		sl, sc, el, ec := int(start.Line), int(start.Col), int(end.Line), int(end.Col)
		if (sl < line || (sl == line && sc <= col)) &&
			(el > line || (el == line && ec >= col)) {
			best = n
			return true
		}
		return true
	})
	return best
}

// defStmtForIdent finds the DefStmt that contains the given identifier, if any.
func (d *Document) defStmtForIdent(ident *syntax.Ident) *syntax.DefStmt {
	if d.file == nil {
		return nil
	}
	var result *syntax.DefStmt
	syntax.Walk(d.file, func(n syntax.Node) bool {
		if n == nil {
			return false
		}
		if def, ok := n.(*syntax.DefStmt); ok {
			if def.Name == ident {
				result = def
				return false
			}
			for _, p := range def.Params {
				if containsIdent(p, ident) {
					result = def
					return false
				}
			}
			for _, s := range def.Body {
				if containsNode(s, ident) {
					result = def
					return false
				}
			}
		}
		return true
	})
	return result
}

func containsIdent(n syntax.Node, target *syntax.Ident) bool {
	found := false
	syntax.Walk(n, func(node syntax.Node) bool {
		if found {
			return false
		}
		if node == target {
			found = true
			return false
		}
		return true
	})
	return found
}

func containsNode(n syntax.Node, target syntax.Node) bool {
	found := false
	syntax.Walk(n, func(node syntax.Node) bool {
		if found {
			return false
		}
		if node == target {
			found = true
			return false
		}
		return true
	})
	return found
}
