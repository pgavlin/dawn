package main

import "github.com/pgavlin/dawn/label"

var watchCmd = newTargetCommand(&targetCommand{
	Use:   "watch",
	Short: "Watch for changes and rebuild a target as necessary",
	Run: func(label *label.Label, args []string) error {
		if err := work.loadProject(args, false, false); err != nil {
			return err
		}
		return work.watch(label)
	},
})

func init() {
	watchCmd.Flags().StringVar(&buildJSON, "json", "", "write JSON build events to the given path")
}
