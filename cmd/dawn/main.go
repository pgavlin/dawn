package main

import (
	"fmt"
	"os"

	"go.starlark.net/starlark"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		if serr, ok := err.(*starlark.EvalError); ok {
			fmt.Fprintf(os.Stderr, serr.Backtrace())
		} else {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}
}
