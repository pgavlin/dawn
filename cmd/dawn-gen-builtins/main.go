package main

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/tools/go/packages"
)

//nolint:gosec
func main() {
	if len(os.Args) != 4 {
		fmt.Fprintf(os.Stderr, "usage: %v <import path> <function wrappers path> <docs output directory path>\n", os.Args[0])
		os.Exit(-1)
	}

	importPath, wrappersPath, docsPath := os.Args[1], os.Args[2], os.Args[3]

	pkgs, err := packages.Load(&packages.Config{
		Mode: packages.NeedSyntax | packages.NeedTypesInfo | packages.NeedTypes | packages.NeedCompiledGoFiles,
	}, importPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load package %v: %v\n", importPath, err)
		os.Exit(-1)
	}

	if len(pkgs) != 1 {
		fmt.Fprintf(os.Stderr, "at most one package may be specified (found %v)\n", len(pkgs))
		os.Exit(-1)
	}

	f, err := os.Create(wrappersPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "creating %v: %v\n", wrappersPath, err)
		os.Exit(-1)
	}
	defer f.Close()

	modules, functions, err := gatherPackage(pkgs[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(-1)
	}
	if err = genFunctionWrappers(f, pkgs[0], functions); err != nil {
		fmt.Fprintf(os.Stderr, "generating function wrappers: %v\n", err)
		os.Exit(-1)
	}

	writeModuleDocs := func(m *object) error {
		path := filepath.Join(docsPath, m.Name+".rst")
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		defer f.Close()

		return genModuleDocs(f, m)
	}

	for _, m := range modules {
		if err := writeModuleDocs(m); err != nil {
			fmt.Fprintf(os.Stderr, "generating module docs for %v: %v\n", m.Name, err)
			os.Exit(-1)
		}
	}
}
