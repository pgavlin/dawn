package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pgavlin/dawn"
	"github.com/pgavlin/dawn/label"
	starlark_os "github.com/pgavlin/dawn/lib/os"
	starlark_sh "github.com/pgavlin/dawn/lib/sh"
	starlark_sha256 "github.com/pgavlin/dawn/lib/sha256"
	"github.com/spf13/cobra"
	starlark_json "go.starlark.net/lib/json"
	"go.starlark.net/repl"
	"go.starlark.net/starlark"
)

type workspace struct {
	root     string
	package_ string
	reindex  bool

	project  *dawn.Project
	graph    graph
	renderer renderer
}

func (w *workspace) init() error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	rootDir := wd
	for {
		if _, err := os.Stat(filepath.Join(rootDir, ".dawnconfig")); err == nil {
			break
		}
		if rootDir == "" {
			return errors.New("could not find .dawnconfig")
		}
		rootDir = filepath.Dir(rootDir)
	}
	w.root = rootDir

	pkg, err := filepath.Rel(w.root, wd)
	if err != nil {
		return err
	}
	if pkg == "." {
		pkg = pkg[1:]
	}
	w.package_ = "//" + pkg

	return nil
}

func (w *workspace) labelArg(args []string) (*label.Label, []string, error) {
	rawlabel := ":default"
	switch len(args) {
	case 0:
		// OK
	default:
		if !strings.HasPrefix(args[0], "--") {
			rawlabel, args = args[0], args[1:]
		}
	}

	label, err := label.Parse(rawlabel)
	if err != nil {
		return nil, nil, err
	}
	label, err = label.RelativeTo(w.package_)
	if err != nil {
		return nil, nil, err
	}
	return label, args, nil
}

func (w *workspace) validLabels(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	targets, err := dawn.Targets(w.root)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	labels := make([]string, 0, len(targets))
	for _, t := range targets {
		if dawn.IsTarget(t.Label) {
			labels = append(labels, fmt.Sprintf("%v\t%s", t.Label, t.Summary))
		}
	}
	return labels, cobra.ShellCompDirectiveDefault
}

func (w *workspace) loadProject(args []string, index, quiet bool) error {
	rendered := make(chan bool)
	firstLoad := true

	renderer, err := newRenderer(func() {
		// We only care about the first onload event.
		if firstLoad {
			close(rendered)
			firstLoad = false
		}
	})
	if err != nil {
		return err
	}
	w.renderer = renderer

	events := dawn.Events(w.renderer)
	if quiet {
		events = dawn.DiscardEvents
	}

	options := &dawn.LoadOptions{
		Args:   args,
		Events: events,
		Builtins: starlark.StringDict{
			"json":   starlark_json.Module,
			"os":     starlark_os.Module,
			"sh":     starlark_sh.Module,
			"sha256": starlark_sha256.Module,
		},
		PreferIndex: !w.reindex && index,
	}
	project, err := dawn.Load(w.root, options)
	if err != nil {
		return err
	}
	w.project = project
	w.graph = buildGraph(w.project)

	<-rendered
	return nil
}

func (w *workspace) target(label *label.Label) (dawn.Target, error) {
	return w.project.Target(label)
}

func (w *workspace) flag(name string) (*dawn.Flag, error) {
	return w.project.Flag(name)
}

func (w *workspace) flags() []*dawn.Flag {
	return w.project.Flags()
}

func (w *workspace) depends(label *label.Label) ([]string, error) {
	t, err := w.target(label)
	if err != nil {
		return nil, err
	}
	return w.graph.depends(t)
}

func (w *workspace) whatDepends(label *label.Label) ([]string, error) {
	t, err := w.target(label)
	if err != nil {
		return nil, err
	}
	return w.graph.whatDepends(t)
}

func (w *workspace) sources(label *label.Label) ([]string, error) {
	t, err := w.target(label)
	if err != nil {
		return nil, err
	}
	return w.graph.sources(t, w.root)
}

func (w *workspace) repl(label *label.Label) error {
	repl.REPL(w.project.REPLEnv(os.Stdout, label))
	return nil
}

func (w *workspace) labelOrNearestDefault(l *label.Label) *label.Label {
	original := l
	if l.Target != "default" {
		return l
	}

	for l.Package != "//" {
		if _, err := w.project.Target(l); err == nil {
			return l
		}
		l.Package = label.Parent(l.Package)
	}
	return original
}

func (w *workspace) run(label *label.Label, opts dawn.RunOptions) error {
	err := w.project.Run(w.labelOrNearestDefault(label), &opts)
	w.renderer.Close()
	return err
}

func (w *workspace) watch(label *label.Label) error {
	err := w.project.Watch(w.labelOrNearestDefault(label))
	w.renderer.Close()
	return err
}
