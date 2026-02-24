package lsp

import (
	"github.com/pgavlin/starlark-go/resolve"
	"github.com/pgavlin/starlark-go/syntax"
)

// definition returns the location of the definition of the identifier at the given position.
func (s *Server) definition(params *TextDocumentPositionParams) *Location {
	s.mu.RLock()
	doc, ok := s.documents[params.TextDocument.URI]
	s.mu.RUnlock()
	if !ok || doc.analysis == nil {
		return nil
	}

	ident := doc.identAtPosition(params.Position)
	if ident == nil {
		return nil
	}

	binding, ok := ident.Binding.(*resolve.Binding)
	if !ok || binding == nil {
		return nil
	}

	switch binding.Scope {
	case resolve.Local, resolve.Free, resolve.Cell, resolve.Global:
		if binding.First != nil {
			// Check if this comes from a load() statement.
			if loc := s.definitionForLoadedGlobal(doc, binding.First); loc != nil {
				return loc
			}
			return &Location{
				URI:   doc.URI,
				Range: identToRange(binding.First),
			}
		}

	case resolve.Predeclared, resolve.Universal, resolve.Undefined:
		// Builtins and undefined names don't have source locations
		return nil
	}

	return nil
}

// definitionForLoadedGlobal checks if the given ident is from a load() statement
// and follows through to the definition in the loaded file.
func (s *Server) definitionForLoadedGlobal(doc *Document, first *syntax.Ident) *Location {
	if doc.analysis == nil || s.project == nil {
		return nil
	}
	for _, load := range doc.analysis.Loads {
		for i, to := range load.Stmt.To {
			if to == first {
				fromName := load.Stmt.From[i].Name
				resolvedPath := s.project.ResolveLoadPath(doc.Path, load.ModulePath)
				if resolvedPath != "" {
					return s.findGlobalInFile(resolvedPath, fromName)
				}
			}
		}
	}
	return nil
}

// findGlobalInFile parses a file on disk and finds a global definition.
func (s *Server) findGlobalInFile(path, name string) *Location {
	// First check if we have this file open.
	uri := pathToURI(path)
	s.mu.RLock()
	doc, ok := s.documents[uri]
	s.mu.RUnlock()
	if ok && doc.analysis != nil {
		return findDefInAnalysis(uri, doc.analysis, name)
	}

	// Parse the file on disk.
	f, err := syntax.Parse(path, nil, 0)
	if err != nil {
		return nil
	}

	analysis := extractAnalysis(f)
	return findDefInAnalysis(uri, analysis, name)
}

func findDefInAnalysis(uri string, analysis *FileAnalysis, name string) *Location {
	for _, fn := range analysis.Functions {
		if fn.Name == name {
			return &Location{
				URI:   uri,
				Range: identToRange(fn.DefStmt.Name),
			}
		}
	}
	for _, g := range analysis.Globals {
		if g.Name == name {
			return &Location{
				URI:   uri,
				Range: identToRange(g.Ident),
			}
		}
	}
	return nil
}

func identToRange(ident *syntax.Ident) Range {
	start, end := ident.Span()
	return Range{
		Start: Position{
			Line:      int(start.Line) - 1,
			Character: int(start.Col) - 1,
		},
		End: Position{
			Line:      int(end.Line) - 1,
			Character: int(end.Col) - 1,
		},
	}
}
