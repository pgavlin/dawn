package lsp

import (
	"github.com/pgavlin/dawn"
	"github.com/pgavlin/starlark-go/syntax"
	"github.com/pgavlin/starlark-go/typecheck"
)

// Document represents an open text document in the editor.
type Document struct {
	URI     string
	Path    string
	Version int32
	Content string

	// Analysis state — from project module or re-parsed from buffer.
	file       *syntax.File
	checkInfo  *typecheck.Info
	checkErrs  []typecheck.Error
	parseErr   error
	resolveErr error
	analysis   *FileAnalysis
}

// TargetDecl represents a target declaration found in the AST.
type TargetDecl struct {
	Name     string
	DefStmt  *syntax.DefStmt
	Position syntax.Position
}

// LoadInfo represents a load() statement.
type LoadInfo struct {
	Stmt       *syntax.LoadStmt
	ModulePath string   // the raw module string
	Names      []string // imported names (the To identifiers)
}

// FuncDecl represents a top-level function definition.
type FuncDecl struct {
	Name     string
	DefStmt  *syntax.DefStmt
	IsTarget bool
}

// newDocument creates a new document and analyzes it from buffer content.
func newDocument(uri, path string, version int32, content string, env *typecheck.Env) *Document {
	doc := &Document{
		URI:     uri,
		Path:    path,
		Version: version,
		Content: content,
	}
	doc.analyze(env)
	return doc
}

// initFromModule initializes the document from pre-computed project module data.
func (d *Document) initFromModule(info dawn.ModuleInfo) {
	d.file = info.File
	d.checkInfo = info.Info
	d.checkErrs = append(d.checkErrs[:0], info.Errors...)
	d.parseErr = nil
	d.resolveErr = nil
	if d.file != nil {
		d.analysis = extractAnalysis(d.file)
	}
}

// update replaces the document content, re-parses, resolves, and type-checks.
func (d *Document) update(version int32, content string, env *typecheck.Env) {
	d.Version = version
	d.Content = content
	d.analyze(env)
}
