package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:          "init",
	Short:        "Create a new project",
	Args:         cobra.NoArgs,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}

		f, err := os.OpenFile("dawn.toml", os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			if os.IsExist(err) {
				return fmt.Errorf("%v already contains a dawn project", wd)
			}
			return err
		}
		return f.Close()
	},
}
