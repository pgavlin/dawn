package main

import (
	"github.com/pgavlin/dawn/label"
	"github.com/spf13/cobra"
)

type targetCommand struct {
	Use               string
	Short             string
	Long              string
	Run               func(label *label.Label, args []string) error
	ValidArgsFunction func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective)
}

func newTargetCommand(cmd *targetCommand) *cobra.Command {
	run := cmd.Run
	cobraCmd := &cobra.Command{
		Use:          cmd.Use,
		Short:        cmd.Short,
		Long:         cmd.Long,
		Args:         cobra.ArbitraryArgs,
		SilenceUsage: true,
		RunE: func(_ *cobra.Command, args []string) error {
			label, args, err := work.labelArg(args)
			if err != nil {
				return err
			}
			return run(label, args)
		},
		ValidArgsFunction: cmd.ValidArgsFunction,
	}
	if cobraCmd.ValidArgsFunction == nil {
		cobraCmd.ValidArgsFunction = work.validLabels
	}

	cobraCmd.PersistentFlags().SetInterspersed(false)
	cobraCmd.Flags().SetInterspersed(false)

	return cobraCmd
}
