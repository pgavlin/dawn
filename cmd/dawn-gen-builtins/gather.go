package main

import (
	"fmt"
	"go/ast"
	"regexp"
	"strings"

	"go.starlark.net/syntax"
	"golang.org/x/tools/go/packages"
)

type function struct {
	def         *syntax.DefStmt
	decl        *ast.FuncDecl
	factoryName string
}

type object struct {
	Name       string
	Kind       string
	Docstring  string
	Children   []*object
	Attributes []*attribute
	Methods    []*method
}

type attribute struct {
	Name      string
	Docstring string
}

type method struct {
	Name      string
	Signature string
	Docstring string
}

type objectDecl struct {
	name       string
	kind       string
	doc        string
	children   []objectDecl
	attributes []*attribute
	methods    []string
}

var textRegex = regexp.MustCompile(`^\s*(?:(?://\s?)|(?:/\*+))?\s?(.*?)(?:\s*\*+/)?\s*$`)

func getCommentText(comment *ast.Comment) (string, bool) {
	// Remove any annotations.
	if strings.HasPrefix(comment.Text, "//starlark:") {
		return "", false
	}

	// Trim spaces and remove any leading or trailing comment markers.
	// Remove any leading or trailing comment markers.
	return textRegex.FindStringSubmatch(comment.Text)[1], true
}

func getDocText(doc *ast.CommentGroup) string {
	if doc == nil {
		return ""
	}

	var text strings.Builder

	// Remove leading blank lines.
	comments := doc.List
	for ; len(comments) > 0; comments = comments[1:] {
		line, ok := getCommentText(comments[0])
		if !ok || line == "" {
			continue
		}
		break
	}

	// Add each block of blank lines followed by text. This will remove any trailing blanks.
	blanks := 0
	for ; len(comments) > 0; comments = comments[1:] {
		line, ok := getCommentText(comments[0])
		switch {
		case !ok:
			continue
		case line == "":
			blanks++
		default:
			for ; blanks > 0; blanks-- {
				text.WriteRune('\n')
			}
			text.WriteString(line)
			text.WriteRune('\n')
		}
	}

	return text.String()
}

func parseDecl(name string, doc *ast.CommentGroup) (*syntax.DefStmt, error) {
	if doc == nil {
		return nil, fmt.Errorf("function %v is missing a Starlark declaration", name)
	}

	// Parse the doc comment as a starlark file.
	f, err := syntax.Parse(name+".star", getDocText(doc), syntax.RetainComments)
	if err != nil {
		return nil, fmt.Errorf("parsing declaration for %v: %w", name, err)
	}

	if len(f.Stmts) != 1 {
		return nil, fmt.Errorf("declaration for %v must be of the form `def fn(): ...`", name)
	}

	def, ok := f.Stmts[0].(*syntax.DefStmt)
	if !ok {
		return nil, fmt.Errorf("declaration for %v must be of the form `def fn(): ...`", name)
	}

	return def, nil
}

func getFactoryName(comment *ast.Comment, funcName string) string {
	options := strings.Split(comment.Text[len("//starlark:builtin"):], ",")
	for _, option := range options {
		equals := strings.IndexByte(option, '=')
		if equals == -1 {
			continue
		}
		name, value := option[:equals], option[equals+1:]
		if name == "factory" {
			return value
		}
	}
	return "new" + pascalCase(funcName)
}

func getStarlarkAnnotation(comments *ast.CommentGroup) (*ast.Comment, string, bool) {
	if comments != nil {
		for _, comment := range comments.List {
			if strings.HasPrefix(comment.Text, "//starlark:") {
				return comment, comment.Text[len("//starlark:"):], true
			}
		}
	}
	return nil, "", false
}

func parseObjectDecl(def *syntax.DefStmt) (*objectDecl, error) {
	body := def.Body

	docstring, ok := getDocstring(def)
	if ok {
		body = body[1:]
	}

	var children []objectDecl
	var attributes []*attribute
	var methods []string
	for _, s := range body {
		def, ok := s.(*syntax.DefStmt)
		if !ok {
			return nil, fmt.Errorf("module declarations must only contain def statements")
		}

		if len(def.Decorators) != 1 {
			return nil, fmt.Errorf("module members must be decorated as either constructors, attributes, functions, or methods")
		}

		kind, args := "", ([]syntax.Expr)(nil)
		switch decorator := def.Decorators[0].Expr.(type) {
		case *syntax.CallExpr:
			kind, args = decorator.Fn.(*syntax.Ident).Name, decorator.Args
		case *syntax.Ident:
			kind = decorator.Name
		}

		switch kind {
		case "constructor":
			if len(args) != 0 {
				return nil, fmt.Errorf("constructor decorator expects no arguments")
			}
			class, err := parseObjectDecl(def)
			if err != nil {
				return nil, err
			}
			class.kind = "class"
			children = append(children, *class)
		case "attribute":
			if len(args) != 0 {
				return nil, fmt.Errorf("attribute decorator expects no arguments")
			}
			docstring, _ := getDocstring(def)
			attributes = append(attributes, &attribute{
				Name:      def.Name.Name,
				Docstring: docstring,
			})
		case "function", "method":
			if len(args) != 1 {
				return nil, fmt.Errorf("function decorator expects a single string literal argument")
			}
			lit, ok := args[0].(*syntax.Literal)
			if !ok {
				return nil, fmt.Errorf("function decorator expects a single string literal argument")
			}
			str, ok := lit.Value.(string)
			if !ok {
				return nil, fmt.Errorf("function decorator expects a single string literal argument")
			}
			methods = append(methods, str)
		}
	}

	return &objectDecl{
		name:       def.Name.Name,
		doc:        docstring,
		children:   children,
		attributes: attributes,
		methods:    methods,
	}, nil
}

// def module():
//     @constructor
//     def class():
//         """
//         Class docs
//         """
//
//     @attribute
//     def attr():
//         """
//         Attribute docs
//         """
//
//     @function("foo.bar")
//     def fn():
//
func parseModuleDecl(text string) (*objectDecl, error) {
	// Parse the text as a starlark file.
	f, err := syntax.Parse("module.star", text, syntax.RetainComments)
	if err != nil {
		return nil, fmt.Errorf("parsing module declaration: %w", err)
	}

	if len(f.Stmts) != 1 {
		return nil, fmt.Errorf("module declaration must be of the form `def module(): ...`")
	}

	def, ok := f.Stmts[0].(*syntax.DefStmt)
	if !ok {
		return nil, fmt.Errorf("module declaration must be of the form `def module(): ...`")
	}

	module, err := parseObjectDecl(def)
	if err != nil {
		return nil, err
	}
	module.kind = "module"
	return module, nil
}

func methodFunction(f *function) *method {
	var sig strings.Builder
	sig.WriteRune('(')
	for i, p := range f.def.Params {
		if i > 0 {
			sig.WriteString(", ")
		}
		switch p := p.(type) {
		case *syntax.Ident:
			sig.WriteString(p.Name)
		case *syntax.BinaryExpr:
			name := p.X.(*syntax.Ident).Name
			if value, ok := p.Y.(*syntax.Ident); ok {
				sig.WriteString(name)
				sig.WriteRune('=')
				sig.WriteString(value.Name)
			}
		}
	}
	sig.WriteRune(')')

	docstring, _ := getDocstring(f.def)
	return &method{
		Name:      f.def.Name.Name,
		Signature: sig.String(),
		Docstring: docstring,
	}
}

func gatherFile(file *ast.File) ([]objectDecl, []*function, error) {
	var modules []objectDecl
	var functions []*function
	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.FuncDecl:
			if comment, kind, ok := getStarlarkAnnotation(decl.Doc); ok && kind == "builtin" {
				def, err := parseDecl(decl.Name.Name, decl.Doc)
				if err != nil {
					return nil, nil, err
				}
				functions = append(functions, &function{
					def:         def,
					decl:        decl,
					factoryName: getFactoryName(comment, decl.Name.Name),
				})
			}
		case *ast.GenDecl:
			_, kind, ok := getStarlarkAnnotation(decl.Doc)
			if ok && kind == "module" {
				module, err := parseModuleDecl(getDocText(decl.Doc))
				if err != nil {
					return nil, nil, err
				}
				modules = append(modules, *module)
			}
		}
	}
	return modules, functions, nil
}

func gatherPackage(pkg *packages.Package) ([]*object, []*function, error) {
	var moduleDecls []objectDecl
	var functions []*function
	for _, f := range pkg.Syntax {
		ms, fs, err := gatherFile(f)
		if err != nil {
			return nil, nil, err
		}
		moduleDecls = append(moduleDecls, ms...)
		functions = append(functions, fs...)
	}

	// link
	funcMap := map[string]*function{}
	for _, f := range functions {
		name := f.decl.Name.Name
		if f.decl.Recv != nil {
			r := f.decl.Recv.List[0]
			type_, err := typeString(nil, pkg, r.Type)
			if err != nil {
				return nil, nil, err
			}
			name = type_ + "." + name
		}
		funcMap[name] = f
	}

	modules := make([]*object, len(moduleDecls))
	for i, mod := range moduleDecls {
		children := make([]*object, len(mod.children))
		for j, c := range mod.children {
			methods := make([]*method, len(c.methods))
			for k, m := range c.methods {
				f, ok := funcMap[m]
				if !ok {
					return nil, nil, fmt.Errorf("unknown function %v in %v %v", m, c.kind, c.name)
				}
				methods[k] = methodFunction(f)
			}
			children[j] = &object{
				Name:       c.name,
				Kind:       c.kind,
				Docstring:  c.doc,
				Attributes: c.attributes,
				Methods:    methods,
			}
		}

		methods := make([]*method, len(mod.methods))
		for j, fn := range mod.methods {
			f, ok := funcMap[fn]
			if !ok {
				return nil, nil, fmt.Errorf("unknown method %v in module %v", fn, mod.name)
			}
			methods[j] = methodFunction(f)
		}

		modules[i] = &object{
			Name:       mod.name,
			Kind:       "module",
			Docstring:  mod.doc,
			Children:   children,
			Attributes: mod.attributes,
			Methods:    methods,
		}
	}

	return modules, functions, nil
}
