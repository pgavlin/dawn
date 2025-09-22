package main

import (
	"fmt"
	"strings"

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

		fmt.Print(target.Label())
		if pos := work.targetRelPos(target); pos != "" {
			fmt.Printf(" (defined at %v)", pos)
		}
		fmt.Println()
		fmt.Println()

		doc := target.Doc()
		if doc == "" {
			fmt.Println("    No help available.")
			fmt.Println()
			return nil
		}

		for l := range strings.Lines(strings.TrimSpace(doc)) {
			if l != "" && !strings.HasPrefix(l, "    ") {
				fmt.Print("    ")
			}
			fmt.Println(l)
		}
		fmt.Println()
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
