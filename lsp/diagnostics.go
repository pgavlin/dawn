package lsp

import (
	"github.com/pgavlin/starlark-go/resolve"
	"github.com/pgavlin/starlark-go/syntax"
)

// collectDiagnostics gathers parse, resolve, and type-check errors as LSP diagnostics.
func (d *Document) collectDiagnostics() []Diagnostic {
	diags := make([]Diagnostic, 0, len(d.checkErrs))

	// Parse errors
	if d.parseErr != nil {
		diags = append(diags, errToDiagnostics(d.parseErr)...)
	}

	// Resolve errors
	if d.resolveErr != nil {
		diags = append(diags, errToDiagnostics(d.resolveErr)...)
	}

	// Type-check errors
	for _, e := range d.checkErrs {
		diags = append(diags, Diagnostic{
			Range:    posToRange(e.Pos),
			Severity: 1, // Error
			Source:   "dawn",
			Message:  e.Msg,
		})
	}

	return diags
}

func errToDiagnostics(err error) []Diagnostic {
	switch e := err.(type) {
	case resolve.ErrorList:
		diags := make([]Diagnostic, 0, len(e))
		for _, re := range e {
			diags = append(diags, Diagnostic{
				Range:    posToRange(re.Pos),
				Severity: 1, // Error
				Source:   "dawn",
				Message:  re.Msg,
			})
		}
		return diags
	case syntax.Error:
		return []Diagnostic{{
			Range:    posToRange(e.Pos),
			Severity: 1,
			Source:   "dawn",
			Message:  e.Msg,
		}}
	default:
		// Starlark errors may be an error list
		if el, ok := err.(*syntax.Error); ok {
			return []Diagnostic{{
				Range:    posToRange(el.Pos),
				Severity: 1,
				Source:   "dawn",
				Message:  el.Msg,
			}}
		}
		return []Diagnostic{{
			Range: Range{
				Start: Position{Line: 0, Character: 0},
				End:   Position{Line: 0, Character: 0},
			},
			Severity: 1,
			Source:   "dawn",
			Message:  err.Error(),
		}}
	}
}

func posToRange(pos syntax.Position) Range {
	line := int(pos.Line) - 1
	col := int(pos.Col) - 1
	if line < 0 {
		line = 0
	}
	if col < 0 {
		col = 0
	}
	return Range{
		Start: Position{Line: line, Character: col},
		End:   Position{Line: line, Character: col},
	}
}

func nodeToRange(n syntax.Node) Range {
	start, end := n.Span()
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
