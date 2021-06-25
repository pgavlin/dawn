package main

import (
	_ "embed"
	"fmt"
	"go/ast"
	"go/types"
	"io"
	"sort"
	"strings"
	"text/template"

	"go.starlark.net/syntax"
	"golang.org/x/tools/go/packages"
)

func getDocstring(def *syntax.DefStmt) (string, bool) {
	body := def.Body
	if len(body) == 0 {
		return "", false
	}
	expr, ok := body[0].(*syntax.ExprStmt)
	if !ok {
		return "", false
	}
	lit, ok := expr.X.(*syntax.Literal)
	if !ok {
		return "", false
	}
	if lit.Token != syntax.STRING {
		return "", false
	}
	return lit.Value.(string), true
}

func getSigil(decl *ast.Field) string {
	if decl.Doc != nil {
		for _, comment := range decl.Doc.List {
			if strings.HasPrefix(comment.Text, "//starlark:") {
				return comment.Text[len("//starlark:"):]
			}
		}
	}
	return ""
}

func typeStringImpl(w io.Writer, imports importSet, pkg *packages.Package, x ast.Expr) {
	if x == nil {
		return
	}

	switch x := x.(type) {
	case *ast.Ident:
		fmt.Fprint(w, x.Name)
	case *ast.SelectorExpr:
		if t, ok := pkg.TypesInfo.Types[x].Type.(*types.Named); ok && imports != nil {
			pkg := t.Obj().Pkg()
			imports[pkg.Path()] = packageImport{
				Name: pkg.Name(),
				Path: pkg.Path(),
			}
		}
		typeStringImpl(w, imports, pkg, x.X)
		fmt.Fprint(w, "."+x.Sel.Name)
	case *ast.ArrayType:
		fmt.Fprint(w, "[")
		typeStringImpl(w, imports, pkg, x.Len)
		fmt.Fprint(w, "]")
		typeStringImpl(w, imports, pkg, x.Elt)
	case *ast.MapType:
		fmt.Fprint(w, "map[")
		typeStringImpl(w, imports, pkg, x.Key)
		fmt.Fprintf(w, "]")
		typeStringImpl(w, imports, pkg, x.Value)
	case *ast.StarExpr:
		fmt.Fprint(w, "*")
		typeStringImpl(w, imports, pkg, x.X)
	default:
		panic(fmt.Errorf("parameter types must be identifiers, selectors, arrays, slices, or pointers"))
	}
}

func typeString(imports importSet, pkg *packages.Package, x ast.Expr) (type_ string, err error) {
	defer func() {
		if x := recover(); x != nil {
			if e, ok := x.(error); ok {
				err = e
				return
			}
			panic(x)
		}
	}()

	var buf strings.Builder
	typeStringImpl(&buf, imports, pkg, x)
	return buf.String(), nil
}

//go:embed function_wrappers.tmpl
var functionWrappersTemplateText string
var functionWrappersTemplate = template.Must(template.New("FunctionWrappers").Parse(functionWrappersTemplateText))

type packageImport struct {
	Name string
	Path string
}

type importSet map[string]packageImport

type functionReceiver struct {
	Name string
	Type string
}

type functionParam struct {
	Name string
	Def  string
	Type string
}

type functionData struct {
	Name        string
	FactoryName string
	Def         string
	Docstring   string
	Receiver    *functionReceiver
	Params      []functionParam
}

func genFunctionWrapper(imports importSet, pkg *packages.Package, f *function) (*functionData, error) {
	docstring, _ := getDocstring(f.def)
	data := functionData{
		Name:        f.decl.Name.Name,
		FactoryName: f.factoryName,
		Def:         f.def.Name.Name,
		Docstring:   docstring,
	}

	if f.decl.Recv != nil && len(f.decl.Recv.List) != 0 {
		r := f.decl.Recv.List[0]
		if len(r.Names) != 1 {
			return nil, fmt.Errorf("function %v must have a named receiver", data.Name)
		}
		name := r.Names[0].Name
		type_, err := typeString(imports, pkg, r.Type)
		if err != nil {
			return nil, err
		}
		data.Receiver = &functionReceiver{
			Name: name,
			Type: type_,
		}
	}

	paramList := f.decl.Type.Params.List
	if len(paramList) < 2 {
		return nil, fmt.Errorf("function %v must have a signature of the form func(*starlark.Thread, *starlark.Builtin, ...)", data.Name)
	}

	for _, p := range paramList[2:] {
		if len(p.Names) == 0 {
			return nil, fmt.Errorf("all parameters to function %v must be named", data.Name)
		}
		type_, err := typeString(imports, pkg, p.Type)
		if err != nil {
			return nil, err
		}

		for _, id := range p.Names {
			data.Params = append(data.Params, functionParam{
				Name: id.Name,
				Type: type_,
			})
		}
	}

	if len(f.def.Params) != len(data.Params) {
		return nil, fmt.Errorf("definition and declaration of %v have different parameter counts", data.Name)
	}
	for i, p := range f.def.Params {
		name, sigil := "", ""
		switch p := p.(type) {
		case *syntax.Ident:
			name = p.Name
		case *syntax.BinaryExpr:
			name = p.X.(*syntax.Ident).Name
			value, ok := p.Y.(*syntax.Ident)
			if !ok || value.Name != "None" {
				return nil, fmt.Errorf("default value for parameter %v in function %v must be None", name, data.Name)
			}
			sigil = "??"
		}
		data.Params[i].Def = name + sigil
	}

	return &data, nil
}

func genFunctionWrappers(w io.Writer, pkg *packages.Package, fns []*function) error {
	var data struct {
		Package   string
		Imports   []packageImport
		Functions []*functionData
	}

	imports := importSet{}
	imports["go.starlark.net/starlark"] = packageImport{Path: "go.starlark.net/starlark"}

	for _, fn := range fns {
		fnData, err := genFunctionWrapper(imports, pkg, fn)
		if err != nil {
			return err
		}
		data.Functions = append(data.Functions, fnData)
	}

	data.Imports = make([]packageImport, 0, len(imports))
	for _, imp := range imports {
		data.Imports = append(data.Imports, imp)
	}
	sort.Slice(data.Imports, func(i, j int) bool {
		return data.Imports[i].Path < data.Imports[j].Path
	})

	data.Package = pkg.Types.Name()
	return functionWrappersTemplate.Execute(w, data)
}

//go:embed object_docs.tmpl
var objectDocsTemplateText string
var objectDocsTemplate = template.Must(template.New("Object").Parse(objectDocsTemplateText))

func genModuleDocs(w io.Writer, m *object) error {
	return objectDocsTemplate.Execute(w, m)
}
