package main

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/pgavlin/dawn"
	"github.com/pgavlin/dawn/label"
	starlark_os "github.com/pgavlin/dawn/lib/os"
	starlark_sh "github.com/pgavlin/dawn/lib/sh"
	fxs "github.com/pgavlin/fx/v2/slices"
	"github.com/spf13/cobra"
	starlark_json "go.starlark.net/lib/json"
	"go.starlark.net/repl"
	"go.starlark.net/starlark"
)

type workspace struct {
	root       string
	configFile string
	package_   string
	reindex    bool
	verbose    bool
	diff       bool

	project  *dawn.Project
	graph    graph
	renderer renderer
}

func (w *workspace) init() error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	rootDir, file := wd, ""
	for {
		if _, err := os.Stat(filepath.Join(rootDir, "dawn.toml")); err == nil {
			file = "dawn.toml"
			break
		}
		if _, err := os.Stat(filepath.Join(rootDir, ".dawnconfig")); err == nil {
			file = ".dawnconfig"
			break
		}
		if rootDir == "/" || rootDir == "." {
			return errors.New("could not find dawn.toml or .dawnconfig")
		}
		rootDir = filepath.Dir(rootDir)
	}
	w.root, w.configFile = rootDir, file

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

			// Unless it is obviously a label, see if the arg is interpretable as a path to a source file.
			if !strings.HasPrefix(rawlabel, "//") && !strings.ContainsRune(rawlabel, ':') {
				p, err := filepath.Abs(rawlabel)
				if err != nil {
					return nil, nil, fmt.Errorf("computing path: %w", err)
				}
				stat, err := os.Lstat(p)
				if !os.IsNotExist(err) {
					if err != nil {
						return nil, nil, fmt.Errorf("stating file: %w", err)
					}
					if !stat.IsDir() {
						modulePath, sourceFile := path.Split(filepath.ToSlash(rawlabel))
						rawlabel = fmt.Sprintf("source:%v:%v", modulePath, sourceFile)
					}
				}
			}
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

	labels := slices.Collect(fxs.FMap(targets, func(t dawn.TargetSummary) (string, bool) {
		if dawn.IsTarget(t.Label) {
			return fmt.Sprintf("%v\t%s", t.Label, t.Summary), true
		}
		return "", false
	}))
	return labels, cobra.ShellCompDirectiveDefault
}

func (w *workspace) loadProject(args []string, index, quiet bool) error {
	rendered := make(chan bool)
	firstLoad := true

	renderer, err := newRenderer(w.verbose, w.diff, func() {
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
		close(rendered)
		events = dawn.DiscardEvents
	}

	options := &dawn.LoadOptions{
		Args:   args,
		Events: events,
		Builtins: starlark.StringDict{
			"json": starlark_json.Module,
			"os":   starlark_os.Module,
			"sh":   starlark_sh.Module,
		},
		PreferIndex: !w.reindex && index,
	}
	project, err := dawn.Load(w.root, options)
	if err != nil {
		renderer.Close()
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
	thread, globals := w.project.REPLEnv(os.Stdout, label)
	globals["depends"] = w.newBuiltin_depends()
	globals["what_depends"] = w.newBuiltin_whatDepends()

	repl.REPL(thread, globals)
	return nil
}

func (w *workspace) labelOrNearestDefault(l *label.Label) *label.Label {
	original := l
	if l.Name != "default" {
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
