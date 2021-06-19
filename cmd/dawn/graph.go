package main

import (
	"path/filepath"

	"github.com/pgavlin/dawn"
	"github.com/pgavlin/dawn/label"
)

type node struct {
	label label.Label

	dependencies []*node
	dependents   []*node
}

func (n *node) depends(visited map[string]struct{}, acc *[]string) {
	if _, ok := visited[n.label.String()]; ok {
		return
	}
	visited[n.label.String()] = struct{}{}

	for _, d := range n.dependencies {
		d.depends(visited, acc)
		*acc = append(*acc, d.label.String())
	}
}

func (n *node) whatDepends(visited map[string]struct{}, acc *[]string) {
	if _, ok := visited[n.label.String()]; ok {
		return
	}
	visited[n.label.String()] = struct{}{}

	for _, d := range n.dependents {
		d.whatDepends(visited, acc)
		*acc = append(*acc, d.label.String())
	}
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

	var paths []string
	for _, d := range n.dependencies {
		if dawn.IsSource(&d.label) {
			components := label.Split(d.label.Package)[1:]
			paths = append(paths, filepath.Join(root, filepath.Join(components...), d.label.Target))
		}
	}
	return paths, nil
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
