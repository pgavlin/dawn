package main

import (
	"fmt"
	"os"

	"github.com/pgavlin/dawn/check"
	"github.com/spf13/cobra"
)

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Type-check all BUILD.dawn files in the project.",
	RunE: func(cmd *cobra.Command, args []string) error {
		results, err := check.CheckProject(work.root)
		if err != nil {
			return err
		}
		if len(results) == 0 {
			return nil
		}
		for _, f := range results {
			for _, e := range f.Errors {
				if e.Pos.IsValid() {
					fmt.Fprintf(os.Stderr, "%s: %s\n", e.Pos, e.Msg)
				} else {
					fmt.Fprintf(os.Stderr, "%s: %s\n", f.Path, e.Msg)
				}
			}
		}
		return fmt.Errorf("type-check failed")
	},
}
