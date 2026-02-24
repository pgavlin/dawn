package main

import (
	"os"

	starlark_os "github.com/pgavlin/dawn/lib/os"
	starlark_sh "github.com/pgavlin/dawn/lib/sh"
	"github.com/pgavlin/dawn/lsp"
	starlark_json "github.com/pgavlin/starlark-go/lib/json"
	"github.com/pgavlin/starlark-go/starlark"
	"github.com/spf13/cobra"
)

var lspCmd = &cobra.Command{
	Use:   "lsp",
	Short: "Start the Dawn LSP server.",
	RunE: func(cmd *cobra.Command, args []string) error {
		server := lsp.NewServer(os.Stdin, os.Stdout, starlark.StringDict{
			"json": starlark_json.Module,
			"os":   starlark_os.Module,
			"sh":   starlark_sh.Module,
		})
		return server.Run()
	},
}
