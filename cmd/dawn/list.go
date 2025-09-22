package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"sort"
	"text/tabwriter"

	"github.com/pgavlin/dawn"
	"github.com/pgavlin/dawn/label"
	fxs "github.com/pgavlin/fx/v2/slices"
	"github.com/spf13/cobra"
)

var listJSON bool

type flagDescription struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Help string `json:"help"`
}

func printFlagList(list []*dawn.Flag) error {
	if !listJSON {
		w := tabwriter.NewWriter(os.Stdout, 0, 2, 0, ' ', 0)
		for _, flag := range list {
			defaultValue := ""
			if flag.Default != "" {
				defaultValue = fmt.Sprintf("(%s)", flag.Default)
			}
			fmt.Fprintf(w, "--%s %s\t %s\t %s\n", flag.Name, flag.FlagType, defaultValue, flag.Help)
		}
		return w.Flush()
	}

	descriptions := slices.Collect(fxs.Map(list, func(arg *dawn.Flag) flagDescription {
		return flagDescription{
			Name: arg.Name,
			Help: arg.Help,
		}
	}))
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "    ")
	return enc.Encode(descriptions)
}

type targetDescription struct {
	Label string `json:"label"`
	Doc   string `json:"doc"`
}

func printTargetList(list []dawn.Target) error {
	if !listJSON {
		w := tabwriter.NewWriter(os.Stdout, 0, 2, 0, ' ', 0)
		for _, t := range list {
			fmt.Fprintf(w, "%v\t %s\n", t.Label(), dawn.DocSummary(t))
		}
		return w.Flush()
	}

	descriptions := slices.Collect(fxs.Map(list, func(t dawn.Target) targetDescription {
		return targetDescription{
			Label: t.Label().String(),
			Doc:   t.Doc(),
		}
	}))

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "    ")
	return enc.Encode(descriptions)
}

func printStringList(list []string) error {
	sort.Strings(list)
	if !listJSON {
		for _, l := range list {
			fmt.Println(l)
		}
		return nil
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "    ")
	return enc.Encode(list)
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List information about a project or targets",
}

var flagsCmd = &cobra.Command{
	Use:   "flags",
	Short: "List available flags",
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := work.loadProject(args, true, listJSON); err != nil {
			return err
		}
		return errors.Join(work.renderer.Close(), printFlagList(work.project.Flags()))
	},
}

var targetsCmd = &cobra.Command{
	Use:   "targets",
	Short: "List available targets",
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := work.loadProject(args, true, listJSON); err != nil {
			return err
		}
		return errors.Join(work.renderer.Close(), printTargetList(work.project.Targets()))
	},
}

var dependsCmd = newTargetCommand(&targetCommand{
	Use:   "depends",
	Short: "List a target's transitive dependencies",
	Run: func(label *label.Label, args []string) error {
		if err := work.loadProject(args, true, listJSON); err != nil {
			return err
		}
		if err := work.renderer.Close(); err != nil {
			return err
		}
		labels, err := work.depends(label)
		if err != nil {
			return err
		}
		return printStringList(labels)
	},
})

var whatDependsCmd = newTargetCommand(&targetCommand{
	Use:   "what-depends",
	Short: "List a target's transitive dependents",
	Run: func(label *label.Label, args []string) error {
		if err := work.loadProject(args, true, listJSON); err != nil {
			return err
		}
		if err := work.renderer.Close(); err != nil {
			return err
		}
		labels, err := work.whatDepends(label)
		if err != nil {
			return err
		}
		return printStringList(labels)
	},
})

var sourcesCmd = newTargetCommand(&targetCommand{
	Use:   "sources",
	Short: "List a target's sources",
	Run: func(label *label.Label, args []string) error {
		if err := work.loadProject(args, true, listJSON); err != nil {
			return err
		}
		if err := work.renderer.Close(); err != nil {
			return err
		}
		paths, err := work.sources(label)
		if err != nil {
			return err
		}
		return printStringList(paths)
	},
})

func init() {
	listCmd.PersistentFlags().BoolVar(&listJSON, "json", false, "write JSON output")

	listCmd.AddCommand(flagsCmd)
	listCmd.AddCommand(targetsCmd)
	listCmd.AddCommand(dependsCmd)
	listCmd.AddCommand(whatDependsCmd)
	listCmd.AddCommand(sourcesCmd)
}
