package lsp

import (
	"github.com/pgavlin/starlark-go/syntax"
)

const (
	symbolKindFile     = 1
	symbolKindModule   = 2
	symbolKindClass    = 5
	symbolKindFunction = 12
	symbolKindVariable = 13
)

// documentSymbols returns the document symbols for the given document.
func (s *Server) documentSymbols(params *DocumentSymbolParams) []DocumentSymbol {
	s.mu.RLock()
	doc, ok := s.documents[params.TextDocument.URI]
	s.mu.RUnlock()
	if !ok || doc.file == nil {
		return []DocumentSymbol{}
	}

	symbols := []DocumentSymbol{}

	for _, stmt := range doc.file.Stmts {
		switch st := stmt.(type) {
		case *syntax.DefStmt:
			kind := symbolKindFunction
			if doc.analysis != nil {
				for _, t := range doc.analysis.Targets {
					if t.DefStmt == st {
						kind = symbolKindClass
						break
					}
				}
			}
			symbols = append(symbols, DocumentSymbol{
				Name:           st.Name.Name,
				Kind:           kind,
				Range:          nodeToRange(st),
				SelectionRange: identToRange(st.Name),
			})

		case *syntax.AssignStmt:
			if ident, ok := st.LHS.(*syntax.Ident); ok {
				symbols = append(symbols, DocumentSymbol{
					Name:           ident.Name,
					Kind:           symbolKindVariable,
					Range:          nodeToRange(st),
					SelectionRange: identToRange(ident),
				})
			}

		case *syntax.LoadStmt:
			symbols = append(symbols, DocumentSymbol{
				Name:           st.ModuleName(),
				Kind:           symbolKindModule,
				Range:          nodeToRange(st),
				SelectionRange: nodeToRange(st.Module),
			})
		}
	}

	return symbols
}
