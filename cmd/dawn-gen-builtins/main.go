package main

import (
	"fmt"
	"os"

	"golang.org/x/tools/go/packages"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "usage: %v <import path> <output path>\n", os.Args[0])
		os.Exit(-1)
	}

	importPath, outputPath := os.Args[1], os.Args[2]

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

	f, err := os.Create(outputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "creating output file: %v\n", err)
		os.Exit(-1)
	}
	defer f.Close()

	// TODO: imports

	functions, err := gatherPackage(pkgs[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(-1)
	}
	if err = genFunctionWrappers(f, pkgs[0], functions); err != nil {
		fmt.Fprintf(os.Stderr, "generating function wrappers: %v\n", err)
		os.Exit(-1)
	}
}
