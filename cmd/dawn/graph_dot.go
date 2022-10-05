package main

import (
	"fmt"
	"io"
	"strings"
)

func (g graph) dot(w io.Writer, filter func(n *node) bool) error {
	builder := &dotBuilder{w}

	// Begin constructing DOT by adding a title and legend.
	builder.start()
	defer builder.finish()

	// Add nodes to DOT builder.
	nodeIDMap := make(map[*node]int, len(g))
	for _, n := range g {
		if filter(n) {
			id := len(nodeIDMap) + 1
			builder.addNode(n, id)
			nodeIDMap[n] = id
		}
	}

	// Add edges to DOT builder.
	for _, n := range g {
		for _, d := range n.dependencies {
			if filter(n) && filter(d) {
				builder.addEdge(nodeIDMap[n], nodeIDMap[d])
			}
		}
	}

	return nil
}

// builder wraps an io.Writer and understands how to compose DOT formatted elements.
type dotBuilder struct {
	io.Writer
}

// start generates a title and initial node in DOT format.
func (b *dotBuilder) start() {
	fmt.Fprintln(b, "digraph \"project\" {")
	fmt.Fprintln(b, `node [style=filled fillcolor="#f8f8f8"]`)
}

// finish closes the opening curly bracket in the constructed DOT buffer.
func (b *dotBuilder) finish() {
	fmt.Fprintln(b, "}")
}

// addNode generates a graph node in DOT format.
func (b *dotBuilder) addNode(node *node, nodeID int) {
	fmt.Fprintf(b, "N%d [label=\"%s\", id=\"node%d\", shape=\"box\"]\n", nodeID, escapeForDot(node.label.String()), nodeID)
}

// addEdge generates a graph edge in DOT format.
func (b *dotBuilder) addEdge(from, to int) {
	fmt.Fprintf(b, "N%d -> N%d\n", from, to)
}

// escapeForDot escapes double quotes and backslashes, and replaces Graphviz's
// "center" character (\n) with a left-justified character.
// See https://graphviz.org/docs/attr-types/escString/ for more info.
func escapeForDot(str string) string {
	return strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(str, `\`, `\\`), `"`, `\"`), "\n", `\l`)
}
