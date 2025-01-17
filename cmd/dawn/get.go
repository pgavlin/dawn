package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/mitchellh/go-homedir"
	"github.com/pgavlin/dawn/internal/mvs"
	"github.com/pgavlin/dawn/internal/project"
	"github.com/spf13/cobra"
)

func newGetCommand() *cobra.Command {
	var updateAll bool

	cmd := &cobra.Command{
		Use:          "get",
		Short:        "Manage project dependencies",
		Args:         cobra.RangeArgs(0, 1),
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

			var newReqs map[string]project.RequirementConfig
			if updateAll {
				if len(args) != 0 {
					return errors.New("cannot use -u with arguments")
				}
				newReqs, err = mvs.UpgradeAll(context.TODO(), config, resolver)
			} else {
				if len(args) != 1 {
					return errors.New("get requires an argument")
				}
				newReqs, err = mvs.Get(context.TODO(), config, resolver, args[0])
			}
			if err != nil {
				return err
			}

			config.Requirements = newReqs
			return project.WriteConfigFile(projectFilePath, config)
		},
	}

	cmd.Flags().BoolVarP(&updateAll, "update", "u", false, "update all dependencies to newer minor or patch versions")

	return cmd
}
