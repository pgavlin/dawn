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

var textRegex = regexp.MustCompile(`^s*(?:(?://\s?)|(?:/\*+))?s?(.*?)(?:s*\*+/)?s*$`)

func getCommentText(comment *ast.Comment) (string, bool) {
	// Remove any annotations.
	if strings.HasPrefix(comment.Text, "//starlark:builtin") {
		return "", false
	}

	// Trim spaces and remove any leading or trailing comment markers.
	// Remove any leading or trailing comment markers.
	return textRegex.FindStringSubmatch(comment.Text)[1], true
}

func parseDecl(name string, doc *ast.CommentGroup) (*syntax.DefStmt, error) {
	if doc == nil {
		return nil, fmt.Errorf("function %v is missing a Starlark declaration", name)
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

	// Parse the result as a starlark file.
	f, err := syntax.Parse(name+".star", text.String(), syntax.RetainComments)
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

func getBuiltinAnnotation(comments *ast.CommentGroup) (*ast.Comment, bool) {
	if comments != nil {
		for _, comment := range comments.List {
			if strings.HasPrefix(comment.Text, "//starlark:builtin") {
				return comment, true
			}
		}
	}
	return nil, false
}

func gatherFile(file *ast.File) ([]*function, error) {
	var functions []*function
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			if comment, ok := getBuiltinAnnotation(fn.Doc); ok {
				def, err := parseDecl(fn.Name.Name, fn.Doc)
				if err != nil {
					return nil, err
				}
				functions = append(functions, &function{
					def:         def,
					decl:        fn,
					factoryName: getFactoryName(comment, fn.Name.Name),
				})
			}
		}
	}
	return functions, nil
}

func gatherPackage(pkg *packages.Package) ([]*function, error) {
	var functions []*function
	for _, f := range pkg.Syntax {
		fns, err := gatherFile(f)
		if err != nil {
			return nil, err
		}
		functions = append(functions, fns...)
	}
	return functions, nil
}
