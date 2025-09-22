package main

import (
	"os"

	"github.com/pgavlin/dawn/cmd/dawn/internal/term"
	"github.com/pgavlin/dawn/util"
	"github.com/spf13/cobra"
)

var (
	prof      = &profiler{}
	work      = &workspace{}
	version   = "development"
	termWidth int
)

var rootCmd = &cobra.Command{
	Version:       version,
	Use:           "dawn",
	Short:         "dawn is a pragmatic polyglot build system.",
	Long:          `A pragmatic polyglot build system.`,
	Args:          cobra.ArbitraryArgs,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		termWidth, _, _ = term.GetSize(os.Stdout)

		if cmd.Use != "init" {
			if err := work.init(); err != nil {
				return err
			}
		}
		return prof.start()
	},
	RunE: buildCmd.RunE,
	PersistentPostRunE: func(_ *cobra.Command, _ []string) error {
		return prof.stop()
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&prof.cpuPath, "cpuprofile", "", "write a CPU profile to the given path")
	rootCmd.PersistentFlags().StringVar(&prof.memPath, "memprofile", "", "write a memory profile to the given path")
	rootCmd.PersistentFlags().StringVar(&prof.starPath, "profile", "", "write an execution profile to the given path")
	rootCmd.PersistentFlags().StringVar(&prof.tracePath, "trace", "", "write a runtime trace to the given path")

	util.Must(rootCmd.PersistentFlags().MarkHidden("cpuprofile"))
	util.Must(rootCmd.PersistentFlags().MarkHidden("memprofile"))
	util.Must(rootCmd.PersistentFlags().MarkHidden("trace"))

	rootCmd.PersistentFlags().BoolVarP(&work.reindex, "reindex", "r", false, "refresh the project's index")
	rootCmd.PersistentFlags().BoolVarP(&work.verbose, "verbose", "V", false, "print verbose build output (incl. target stdout)")
	rootCmd.PersistentFlags().BoolVarP(&work.diff, "diff", "d", false, "print the reasons that targets are built")

	rootCmd.Flags().BoolVarP(&buildOptions.Always, "always", "B", false, "consider all targets out-of-date")
	rootCmd.Flags().BoolVarP(&buildOptions.DryRun, "dry-run", "n", false, "print the targets that would be built, but do not build them")
	rootCmd.Flags().StringVar(&buildDOT, "dot", "", "write a DOT graph of out-of-date targets to the given path")
	rootCmd.Flags().StringVar(&buildJSON, "json", "", "write JSON build events to the given path")

	rootCmd.PersistentFlags().SetInterspersed(false)
	rootCmd.Flags().SetInterspersed(false)

	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(buildCmd)
	rootCmd.AddCommand(watchCmd)
	rootCmd.AddCommand(replCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(gcCmd)
	rootCmd.AddCommand(completionCmd)
	rootCmd.AddCommand(graphCmd)
	rootCmd.AddCommand(newGetCommand())
	rootCmd.AddCommand(tidyCmd)

	rootCmd.SetHelpCommand(helpCmd)
}
