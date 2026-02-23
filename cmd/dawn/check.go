package main

import (
	"errors"

	"github.com/pgavlin/dawn"
	starlark_os "github.com/pgavlin/dawn/lib/os"
	starlark_sh "github.com/pgavlin/dawn/lib/sh"
	starlark_json "github.com/pgavlin/starlark-go/lib/json"
	"github.com/pgavlin/starlark-go/starlark"
	"github.com/spf13/cobra"
)

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Type-check all BUILD.dawn files in the project.",
	RunE: func(cmd *cobra.Command, args []string) error {
		rendered := make(chan bool)
		renderer, err := newRenderer(work.verbose, work.diff, func() {
			close(rendered)
		}, func() {})
		if err != nil {
			return err
		}
		work.renderer = renderer

		events := dawn.Events(work.renderer)
		_, err = dawn.Open(work.context, work.root, &dawn.OpenOptions{
			Args:   args,
			Events: events,
			Builtins: starlark.StringDict{
				"json": starlark_json.Module,
				"os":   starlark_os.Module,
				"sh":   starlark_sh.Module,
			},
		})
		if err != nil {
			return errors.Join(renderer.Close(), err)
		}

		<-rendered
		return nil
	},
}
