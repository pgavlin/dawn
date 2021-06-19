package main

import "github.com/pgavlin/dawn/label"

var replIndexOnly bool

var replCmd = newTargetCommand(&targetCommand{
	Use:   "repl",
	Short: "Launch the REPL",
	Run: func(label *label.Label, args []string) error {
		if err := work.loadProject(args, replIndexOnly, false); err != nil {
			return err
		}
		return work.repl(label)
	},
})

func init() {
	replCmd.Flags().BoolVar(&replIndexOnly, "index-only", false, "only load the index; not the complete project")
}
