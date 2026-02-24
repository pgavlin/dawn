package main

import (
	"os"

	"github.com/pgavlin/dawn/lsp"
	"github.com/spf13/cobra"
)

var lspCmd = &cobra.Command{
	Use:   "lsp",
	Short: "Start the Dawn LSP server.",
	RunE: func(cmd *cobra.Command, args []string) error {
		server := lsp.NewServer(os.Stdin, os.Stdout)
		return server.Run()
	},
}
