package main

import "github.com/spf13/cobra"

var gcCmd = &cobra.Command{
	Use:          "gc",
	Short:        "Collect unreferenced target data",
	Args:         cobra.NoArgs,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := work.loadProject(args, true, false); err != nil {
			return err
		}
		return work.project.GC()
	},
}
