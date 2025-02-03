package main

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/mitchellh/go-homedir"
	"github.com/pgavlin/dawn/internal/mvs"
	"github.com/pgavlin/dawn/internal/project"
	"github.com/spf13/cobra"
)

var tidyCmd = &cobra.Command{
	Use:          "tidy",
	Short:        "Tidy the project file",
	Args:         cobra.NoArgs,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		projectFilePath := filepath.Join(work.root, work.configFile)

		config, err := project.LoadConfigFile(projectFilePath)
		if err != nil {
			return fmt.Errorf("loading config file: %w", err)
		}

		home, err := homedir.Dir()
		if err != nil {
			return fmt.Errorf("getting home directory: %w", err)
		}
		moduleCache := filepath.Join(home, ".dawn", "modules", "cache")

		renderer, err := newRenderer(work.verbose, work.diff, func() {})
		if err != nil {
			return err
		}
		defer renderer.Close()

		resolver := mvs.NewResolver(moduleCache, mvs.DefaultDialer, resolveEvents{events: renderer})

		newReqs, err := mvs.Tidy(context.TODO(), config, resolver)
		if err != nil {
			return err
		}

		config.Requirements = newReqs
		return project.WriteConfigFile(projectFilePath, config)
	},
}
