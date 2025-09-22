package main

import (
	"github.com/pgavlin/dawn"
	"github.com/pgavlin/dawn/label"
)

var (
	buildJSON    string
	buildDOT     string
	buildOptions dawn.RunOptions
)

var buildCmd = newTargetCommand(&targetCommand{
	Use:   "build",
	Short: "Build a target",
	Run: func(label *label.Label, args []string) error {
		if err := work.loadProject(args, false, false); err != nil {
			return err
		}
		return work.run(label, buildOptions)
	},
})

func init() {
	buildCmd.Flags().BoolVarP(&buildOptions.Always, "always", "B", false, "consider all targets out-of-date")
	buildCmd.Flags().BoolVarP(&buildOptions.DryRun, "dry-run", "n", false, "print the targets that would be built, but do not build them")
	buildCmd.Flags().StringVar(&buildJSON, "json", "", "write JSON build events to the given path")
	buildCmd.Flags().StringVar(&buildDOT, "dot", "", "write a DOT graph of out-of-date targets to the given path")
}
