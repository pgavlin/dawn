package main

import (
	"fmt"
	"os"

	"github.com/pgavlin/dawn"
	"github.com/spf13/cobra"
)

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Type-check all BUILD.dawn files in the project.",
	RunE: func(cmd *cobra.Command, args []string) error {
		proj, err := dawn.Open(work.context, work.root, nil)
		if err != nil {
			return err
		}
		results := proj.CheckResults()
		if len(results) == 0 {
			return nil
		}
		for _, r := range results {
			for _, e := range r.Errors {
				if e.Pos.IsValid() {
					fmt.Fprintf(os.Stderr, "%s: %s\n", e.Pos, e.Msg)
				} else {
					fmt.Fprintf(os.Stderr, "%s: %s\n", r.Path, e.Msg)
				}
			}
		}
		return fmt.Errorf("type-check failed")
	},
}
