package lsp

import (
	"github.com/pgavlin/starlark-go/resolve"
)

// references returns all locations that reference the same binding as the identifier at the given position.
func (s *Server) references(params *ReferenceParams) []Location {
	s.mu.RLock()
	doc, ok := s.documents[params.TextDocument.URI]
	s.mu.RUnlock()
	if !ok || doc.analysis == nil {
		return []Location{}
	}

	ident := doc.identAtPosition(params.Position)
	if ident == nil {
		return []Location{}
	}

	binding, ok := ident.Binding.(*resolve.Binding)
	if !ok || binding == nil {
		return []Location{}
	}

	// For local/global/cell/free bindings, find all idents with the same binding.
	switch binding.Scope {
	case resolve.Local, resolve.Free, resolve.Cell, resolve.Global:
		locs := []Location{}
		for _, id := range doc.analysis.Idents {
			b, ok := id.Binding.(*resolve.Binding)
			if !ok {
				continue
			}
			if b.First == binding.First {
				locs = append(locs, Location{
					URI:   doc.URI,
					Range: identToRange(id),
				})
			}
		}
		return locs

	case resolve.Predeclared, resolve.Universal:
		// Find all uses of this predeclared/universal name.
		locs := []Location{}
		for _, id := range doc.analysis.Idents {
			b, ok := id.Binding.(*resolve.Binding)
			if !ok {
				continue
			}
			if b.Scope == binding.Scope && id.Name == ident.Name {
				locs = append(locs, Location{
					URI:   doc.URI,
					Range: identToRange(id),
				})
			}
		}
		return locs

	case resolve.Undefined:
		return []Location{}
	}

	return []Location{}
}
