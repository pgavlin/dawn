package main

import (
	"os"
	"path/filepath"
	"slices"

	"github.com/pgavlin/dawn"
	"github.com/pgavlin/dawn/diff"
	"github.com/pgavlin/dawn/label"
	fxs "github.com/pgavlin/fx/v2/slices"
	"github.com/spf13/cobra"
)

type node struct {
	label label.Label

	dependencies []*node
	dependents   []*node

	status string
	reason string
	diff   diff.ValueDiff
}

func (n *node) depends(visited map[string]struct{}, acc *[]string) bool {
	if _, ok := visited[n.label.String()]; ok {
		return false
	}
	visited[n.label.String()] = struct{}{}

	*acc = slices.AppendSeq(*acc, fxs.FMap(n.dependencies, func(d *node) (string, bool) {
		return d.label.String(), d.depends(visited, acc)
	}))
	return true
}

func (n *node) whatDepends(visited map[string]struct{}, acc *[]string) bool {
	if _, ok := visited[n.label.String()]; ok {
		return false
	}
	visited[n.label.String()] = struct{}{}

	*acc = slices.AppendSeq(*acc, fxs.FMap(n.dependents, func(d *node) (string, bool) {
		return d.label.String(), d.whatDepends(visited, acc)
	}))
	return true
}

type graph map[string]*node

func (g graph) depends(t dawn.Target) ([]string, error) {
	n := g[t.Label().String()]

	var labels []string
	n.depends(map[string]struct{}{}, &labels)
	return labels, nil
}

func (g graph) whatDepends(t dawn.Target) ([]string, error) {
	n := g[t.Label().String()]

	var labels []string
	n.whatDepends(map[string]struct{}{}, &labels)
	return labels, nil
}

func (g graph) sources(t dawn.Target, root string) ([]string, error) {
	n := g[t.Label().String()]

	return slices.Collect(fxs.FMap(n.dependencies, func(d *node) (string, bool) {
		if dawn.IsSource(&d.label) {
			components := label.Split(d.label.Package)[1:]
			return filepath.Join(root, filepath.Join(components...), d.label.Name), true
		}
		return "", false
	})), nil
}

func (g graph) getOrAddNode(label *label.Label) *node {
	if n, ok := g[label.String()]; ok {
		return n
	}
	n := &node{label: *label}
	g[label.String()] = n
	return n
}

func buildGraph(proj *dawn.Project) graph {
	g := graph{}

	targets := proj.Targets()
	for _, t := range targets {
		n := g.getOrAddNode(t.Label())
		for _, l := range t.Dependencies() {
			dep := g.getOrAddNode(l)

			n.dependencies = append(n.dependencies, dep)
			dep.dependents = append(dep.dependents, n)
		}
	}

	return g
}

var graphCmd = &cobra.Command{
	Use:          "graph",
	Short:        "Write the project's dependency graph to stdout in DOT format",
	Args:         cobra.NoArgs,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := work.loadProject(args, true, true); err != nil {
			return err
		}
		work.renderer.Close()

		return work.graph.dot(os.Stdout, func(_ *node) bool { return true })
	},
}
