package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var helpCmd = &cobra.Command{
	Use:   "help",
	Short: "Help about commands, build targets, and build arguments",
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return rootCmd.Help()
		}

		for _, c := range rootCmd.Commands() {
			if c.Name() == args[0] || c.CalledAs() == args[0] {
				return c.Help()
			}
		}

		label, args, err := work.labelArg(args)
		if err != nil {
			return err
		}
		if err := work.loadProject(args, true, false); err != nil {
			return err
		}

		target, err := work.target(label)
		if err != nil {
			return err
		}

		doc := target.Doc()
		if doc == "" {
			fmt.Printf("No help for %v\n", label)
			return nil
		}

		fmt.Println(target.Label())
		fmt.Println(target.Doc())
		return nil
	},
	ValidArgsFunction: func(cmd *cobra.Command, args []string, completing string) ([]string, cobra.ShellCompDirective) {
		topics := []string{"usage"}
		for _, c := range rootCmd.Commands() {
			if !c.Hidden {
				topics = append(topics, c.Name(), c.CalledAs())
			}
		}
		labels, _ := work.validLabels(cmd, args, completing)
		topics = append(topics, labels...)
		return topics, cobra.ShellCompDirectiveDefault
	},
}
