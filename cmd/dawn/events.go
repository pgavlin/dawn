package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/pgavlin/dawn"
	"github.com/pgavlin/dawn/cmd/dawn/internal/term"
	"github.com/pgavlin/dawn/diff"
	"github.com/pgavlin/dawn/internal/project"
	"github.com/pgavlin/dawn/label"
	"github.com/pgavlin/dawn/runner"
	starlarkjson "github.com/pgavlin/starlark-go/lib/json"
	starlark "github.com/pgavlin/starlark-go/starlark"
)

type renderer interface {
	io.Closer
	dawn.Events
}

type discardRendererT struct {
	dawn.Events
}

func (discardRendererT) Close() error {
	return nil
}

var discardRenderer = discardRendererT{dawn.DiscardEvents}

// simple renderer
type lineRenderer struct {
	m        sync.Mutex
	stdout   io.Writer
	stderr   io.Writer
	onLoaded func()
}

func (e *lineRenderer) Close() error {
	return nil
}

func (e *lineRenderer) Print(label *label.Label, line string) {
	e.print(label, line)
}

func (e *lineRenderer) RequirementLoading(label *label.Label, version string) {
	e.print(label, "loading")
}

func (e *lineRenderer) RequirementLoaded(label *label.Label, version string) {
	e.print(label, "loaded")
}

func (e *lineRenderer) RequirementLoadFailed(label *label.Label, version string, err error) {
	e.printe(label, fmt.Sprintf("failed: %v", errMessage(err)))
}

func (e *lineRenderer) ModuleLoading(label *label.Label) {
	e.print(label, "loading")
}

func (e *lineRenderer) ModuleLoaded(label *label.Label) {
	e.print(label, "loaded")
}

func (e *lineRenderer) ModuleLoadFailed(label *label.Label, err error) {
	e.printe(label, fmt.Sprintf("failed: %v", errMessage(err)))
}

func (e *lineRenderer) LoadDone(err error) {
	e.m.Lock()
	defer e.m.Unlock()

	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load project: %v", errMessage(err))
	} else {
		fmt.Fprintf(os.Stdout, "project loaded")
	}

	if e.onLoaded != nil {
		e.onLoaded()
	}
}

func (e *lineRenderer) TargetUpToDate(label *label.Label) {
	e.print(label, "up-to-date")
}

func (e *lineRenderer) TargetEvaluating(label *label.Label, reason string, diff diff.ValueDiff) {
	e.print(label, "evaluating...")
}

func (e *lineRenderer) TargetFailed(label *label.Label, err error) {
	e.printe(label, fmt.Sprintf("failed: %v", errMessage(err)))
}

func (e *lineRenderer) TargetSucceeded(label *label.Label, changed bool) {
	e.print(label, "done")
}

func (e *lineRenderer) RunDone(err error) {
	e.m.Lock()
	defer e.m.Unlock()

	if err != nil {
		fmt.Fprintf(os.Stderr, "build failed: %v", errMessage(err))
	} else {
		fmt.Fprintf(os.Stdout, "build succeeded")
	}
}

func (e *lineRenderer) FileChanged(label *label.Label) {
	e.print(label, "changed")
}

func (e *lineRenderer) print(label *label.Label, message string) {
	e.fprint(e.stdout, label, message)
}

func (e *lineRenderer) printe(label *label.Label, message string) {
	e.fprint(e.stderr, label, message)
}

func (e *lineRenderer) fprint(w io.Writer, label *label.Label, message string) {
	e.m.Lock()
	defer e.m.Unlock()

	fmt.Fprintf(w, "[%v] %v\n", label, message)
}

// DOT renderer
type dotRenderer struct {
	m    sync.Mutex
	next renderer
	dest io.WriteCloser
	work *workspace
}

func newDOTRenderer(dest io.WriteCloser, work *workspace, next renderer) renderer {
	return &dotRenderer{next: next, dest: dest, work: work}
}

func (e *dotRenderer) Close() error {
	e.dest.Close()
	return e.next.Close()
}

func (e *dotRenderer) Print(label *label.Label, line string) {
	e.next.Print(label, line)
}

func (e *dotRenderer) RequirementLoading(label *label.Label, version string) {
	e.next.RequirementLoading(label, version)
}

func (e *dotRenderer) RequirementLoaded(label *label.Label, version string) {
	e.next.RequirementLoaded(label, version)
}

func (e *dotRenderer) RequirementLoadFailed(label *label.Label, version string, err error) {
	e.next.RequirementLoadFailed(label, version, err)
}

func (e *dotRenderer) ModuleLoading(label *label.Label) {
	e.next.ModuleLoading(label)
}

func (e *dotRenderer) ModuleLoaded(label *label.Label) {
	e.next.ModuleLoaded(label)
}

func (e *dotRenderer) ModuleLoadFailed(label *label.Label, err error) {
	e.next.ModuleLoadFailed(label, err)
}

func (e *dotRenderer) LoadDone(err error) {
	e.next.LoadDone(err)
}

func (e *dotRenderer) TargetUpToDate(label *label.Label) {
	e.decorateNode(label, func(n *node) { n.status = "up-to-date" })
	e.next.TargetUpToDate(label)
}

func (e *dotRenderer) TargetEvaluating(label *label.Label, reason string, diff diff.ValueDiff) {
	e.decorateNode(label, func(n *node) {
		n.status = "evaluated"
		n.reason = reason
		n.diff = diff
	})
	e.next.TargetEvaluating(label, reason, diff)
}

func (e *dotRenderer) TargetFailed(label *label.Label, err error) {
	e.decorateNode(label, func(n *node) { n.status = "failed" })
	e.next.TargetFailed(label, err)
}

func (e *dotRenderer) TargetSucceeded(label *label.Label, changed bool) {
	e.decorateNode(label, func(n *node) { n.status = "succeeded" })
	e.next.TargetSucceeded(label, changed)
}

func (e *dotRenderer) RunDone(err error) {
	e.work.graph.dot(e.dest, func(n *node) bool { return n.status != "" && n.status != "up-to-date" })
	e.next.RunDone(err)
}

func (e *dotRenderer) FileChanged(label *label.Label) {
	e.next.FileChanged(label)
}

func (e *dotRenderer) decorateNode(label *label.Label, decorator func(n *node)) {
	e.m.Lock()
	defer e.m.Unlock()

	if node, ok := e.work.graph[label.String()]; ok {
		decorator(node)
	}
}

// JSON renderer
type jsonRenderer struct {
	m      sync.Mutex
	next   renderer
	enc    *json.Encoder
	closer io.Closer
}

func newJSONRenderer(dest io.WriteCloser, next renderer) renderer {
	enc := json.NewEncoder(dest)
	enc.SetIndent("", "    ")
	return &jsonRenderer{next: next, enc: enc, closer: dest}
}

func (e *jsonRenderer) Close() error {
	e.closer.Close()
	return e.next.Close()
}

func (e *jsonRenderer) Print(label *label.Label, line string) {
	e.event("Print", label, "line", line)
	e.next.Print(label, line)
}

func (e *jsonRenderer) RequirementLoading(label *label.Label, version string) {
	e.event("RequirementLoading", label, "version", version)
	e.next.RequirementLoading(label, version)
}

func (e *jsonRenderer) RequirementLoaded(label *label.Label, version string) {
	e.event("RequirementLoaded", label, "version", version)
	e.next.RequirementLoaded(label, version)
}

func (e *jsonRenderer) RequirementLoadFailed(label *label.Label, version string, err error) {
	e.event("RequirementLoadFailed", label, "version", version, "err", errMessage(err))
	e.next.RequirementLoadFailed(label, version, err)
}

func (e *jsonRenderer) ModuleLoading(label *label.Label) {
	e.event("ModuleLoading", label)
	e.next.ModuleLoading(label)
}

func (e *jsonRenderer) ModuleLoaded(label *label.Label) {
	e.event("ModuleLoaded", label)
	e.next.ModuleLoaded(label)
}

func (e *jsonRenderer) ModuleLoadFailed(label *label.Label, err error) {
	e.event("ModuleLoadFailed", label, "err", errMessage(err))
	e.next.ModuleLoadFailed(label, err)
}

func (e *jsonRenderer) LoadDone(err error) {
	e.event("LoadDone", nil, "err", errMessage(err))
	e.next.LoadDone(err)
}

func (e *jsonRenderer) TargetUpToDate(label *label.Label) {
	e.event("TargetUpToDate", label)
	e.next.TargetUpToDate(label)
}

var starlarkJSONEncode = starlarkjson.Module.Members["encode"]

func (e *jsonRenderer) TargetEvaluating(label *label.Label, reason string, diff diff.ValueDiff) {
	var diffJSON starlark.Value = starlark.String("null")
	if diff != nil {
		thread := starlark.Thread{Name: "json.encode"}
		diffJSON, _ = starlark.Call(&thread, starlarkJSONEncode, starlark.Tuple{diff}, nil)
	}

	e.event("TargetEvaluating", label, "reason", reason, "diff", json.RawMessage(diffJSON.(starlark.String)))
	e.next.TargetEvaluating(label, reason, diff)
}

func (e *jsonRenderer) TargetFailed(label *label.Label, err error) {
	e.event("TargetFailed", label, "err", errMessage(err))
	e.next.TargetFailed(label, err)
}

func (e *jsonRenderer) TargetSucceeded(label *label.Label, changed bool) {
	e.event("TargetSucceeded", label, "changed", changed)
	e.next.TargetSucceeded(label, changed)
}

func (e *jsonRenderer) RunDone(err error) {
	e.event("RunDone", nil, "err", errMessage(err))
	e.next.RunDone(err)
}

func (e *jsonRenderer) FileChanged(label *label.Label) {
	e.event("FileChanged", label)
	e.next.FileChanged(label)
}

func (e *jsonRenderer) event(kind string, label *label.Label, pairs ...interface{}) {
	e.m.Lock()
	defer e.m.Unlock()

	event := map[string]interface{}{"kind": kind}
	if label != nil {
		event["label"] = label.String()
	}
	if len(pairs)%2 != 0 {
		panic("oddly-sized pairs")
	}
	for i := 0; i < len(pairs); i += 2 {
		event[pairs[i].(string)] = pairs[i+1]
	}
	e.enc.Encode(event)
}

// default renderer:
// +--------------------------------------------+
// | completed targets/diffs/verbose output...  |
// | evaluating targets...                      |
// | status line                                |
// | system stats                               |
// +--------------------------------------------+

type target struct {
	label  *label.Label
	reason string

	diff      diff.ValueDiff
	diffShown bool

	status string
	failed bool
	start  time.Time

	lines []string

	prev *target
	next *target
}

func (t *target) setStatus(message string) {
	t.status = fmt.Sprintf("[%v] %v", t.label, message)
}

func (t *target) stamp(now time.Time) string {
	d := now.Sub(t.start).Truncate(time.Second)
	if d.Seconds() < 1 {
		return t.status
	}
	return fmt.Sprintf("%s (%v)", t.status, duration(d))
}

type targetList struct {
	head *target
	tail *target
}

func (l *targetList) append(t *target) {
	if l.head == nil {
		l.head = t
	} else {
		l.tail.next = t
		t.prev = l.tail
	}
	l.tail = t
}

func (l *targetList) remove(t *target) {
	if t.prev != nil {
		t.prev.next = t.next
	}
	if t.next != nil {
		t.next.prev = t.prev
	}

	if l.head == t {
		l.head = t.next
	}
	if l.tail == t {
		l.tail = t.prev
	}

	t.next, t.prev = nil, nil
}

type statusRenderer struct {
	m sync.Mutex

	ticker  *time.Ticker
	stats   systemStats
	targets map[string]*target

	maxWidth int

	verbose bool
	diff    bool
	lines   []string

	lastUpdate time.Time
	dirty      bool
	rewind     int
	evaluating targetList
	done       targetList
	statusLine string
	loaded     bool

	onLoaded func()

	stdout io.Writer
}

func (e *statusRenderer) line(text string) {
	e.rewind++

	if e.maxWidth > 0 && len(text) > e.maxWidth {
		text = text[:e.maxWidth-1]
	}
	fmt.Fprintf(e.stdout, "%s\n", text)
}

func (e *statusRenderer) render(now time.Time, closed bool) {
	e.m.Lock()
	defer e.m.Unlock()

	if !e.dirty {
		return
	}
	e.lastUpdate = now

	// Re-home the cursor.
	for ; e.rewind > 0; e.rewind-- {
		term.CursorUp(e.stdout)
		term.ClearLine(e.stdout, e.maxWidth)
	}

	// Write any verbose output that has come in since the last frame.
	for _, l := range e.lines {
		fmt.Fprintln(e.stdout, l)
	}
	e.lines = e.lines[:0]

	// Render diffs.
	if e.diff {
		for t := e.evaluating.head; t != nil; t = t.next {
			if !t.diffShown && t.label.Kind != "module" {
				e.renderDiff(t)
				t.diffShown = true
			}
		}
		for t := e.done.head; t != nil; t = t.next {
			if !t.diffShown && t.label.Kind != "module" {
				e.renderDiff(t)
				t.diffShown = true
			}
		}
	}

	// Write any targets that have finished since the last frame.
	for t := e.done.head; t != nil; t = t.next {
		fmt.Fprintf(e.stdout, "%s\n", t.status)

		if t.failed {
			for _, line := range t.lines {
				fmt.Fprintln(e.stdout, line)
			}
		}
	}
	e.done = targetList{}

	// Render in-progress targets.
	for t := e.evaluating.head; t != nil; t = t.next {
		e.line(t.stamp(now))
	}

	if e.statusLine != "" && !closed {
		e.line(e.statusLine)
		e.statusLine = ""
	}

	if !closed && e.evaluating.head != nil {
		e.line(e.stats.line())
	}

	// If the project finished loading during the last quantum, inform any waiters.
	if e.loaded {
		if e.onLoaded != nil {
			e.onLoaded()
		}
		e.loaded = false
	}

	e.dirty = false
}

var (
	colorRed    = color.New(color.FgRed)
	colorGreen  = color.New(color.FgGreen)
	colorYellow = color.New(color.FgYellow)
)

func (e *statusRenderer) renderDiff(t *target) {
	reason := t.reason
	if reason == "" {
		reason = "out-of-date"
	}

	// print the diff header
	fmt.Fprintf(e.stdout, "[%v] %s\n", t.label, colorYellow.Sprint(reason))

	// print the diff
	if t.diff != nil {
		printDiff(e.stdout, "", t.diff)
		fmt.Fprintln(e.stdout, "")
	}
}

func printDiff(w io.Writer, indent string, d diff.ValueDiff) {
	switch d := d.(type) {
	case *diff.LiteralDiff:
		fmt.Fprintf(w, "%v => %v", colorRed.Sprint(d.Old()), colorGreen.Sprint(d.New()))
	case *diff.MappingDiff:
		printMappingDiff(w, indent, d)
	case *diff.SetDiff:
		printSetDiff(w, indent, d)
	case *diff.SliceableDiff:
		printSliceDiff(w, indent, d)
	default:
		return
	}
}

var starlarkSorted = starlark.Universe["sorted"]

func tryStarlarkSorted(v starlark.Iterable) starlark.Iterable {
	thread := starlark.Thread{Name: "sorted"}
	sortedKeys, err := starlark.Call(&thread, starlarkSorted, starlark.Tuple{v}, nil)
	if err != nil {
		return v
	}
	return sortedKeys.(starlark.Iterable)
}

func printMappingDiff(w io.Writer, indent string, d *diff.MappingDiff) {
	fmt.Fprintf(w, "{\n")
	indent += "  "

	it := tryStarlarkSorted(d.Edits()).Iterate()
	defer it.Done()

	var key starlark.Value
	for it.Next(&key) {
		if val, ok, _ := d.Edits().Get(key); ok {
			edit := val.(*diff.Edit)
			switch edit.Kind() {
			case diff.EditKindDelete:
				fmt.Fprintf(w, "%s%s,\n", indent, colorRed.Sprintf("%v: %v", key, edit.Index(0)))
			case diff.EditKindAdd:
				fmt.Fprintf(w, "%s%s,\n", indent, colorGreen.Sprintf("%v: %v", key, edit.Index(0)))
			case diff.EditKindReplace:
				fmt.Fprintf(w, "%s%s", indent, colorYellow.Sprintf("%v: ", key))
				printDiff(w, indent, edit.Sliceable.Index(0).(diff.ValueDiff))
				fmt.Fprint(w, ",\n")
			}
		}
	}

	indent = indent[:len(indent)-2]
	fmt.Fprintf(w, "%s}", indent)
}

func printSetDiff(w io.Writer, indent string, d *diff.SetDiff) {
	fmt.Fprintf(w, "set(\n")
	indent += "  "

	it := d.Edits().Iterate()
	defer it.Done()

	var val starlark.Value
	for it.Next(&val) {
		edit := val.(*diff.Edit)
		switch edit.Kind() {
		case diff.EditKindDelete:
			fmt.Fprintf(w, "%s%s,\n", indent, colorRed.Sprintf("%v", val))
		case diff.EditKindAdd:
			fmt.Fprintf(w, "%s%s,\n", indent, colorGreen.Sprintf("%v", val))
		}
	}

	indent = indent[:len(indent)-2]
	fmt.Fprintf(w, "%s)", indent)
}

func quote(s string) string {
	q := strconv.Quote(s)
	return q[1 : len(q)-1]
}

func contextSize(edits starlark.Tuple, index, contextLen int) (headContext, tailContext bool, totalContext int) {
	headContext, tailContext = index > 0, index < len(edits)-1

	totalContext = 0
	if headContext {
		totalContext += contextLen
	}
	if tailContext {
		totalContext += contextLen
	}

	return
}

func printStringDiff(w io.Writer, d *diff.SliceableDiff) {
	const contextLen = 10

	var old, new strings.Builder
	for i, val := range d.Edits() {
		edit := val.(*diff.Edit)
		switch edit.Kind() {
		case diff.EditKindDelete:
			colorRed.Fprint(&old, quote(string(edit.Sliceable.(starlark.String))))
		case diff.EditKindCommon:
			s := string(edit.Sliceable.(starlark.String))

			headContext, tailContext, totalContext := contextSize(d.Edits(), i, contextLen)
			if len(s) <= totalContext+3 {
				old.WriteString(quote(s))
				new.WriteString(quote(s))
			} else {
				if headContext {
					old.WriteString(quote(s[:contextLen]))
					new.WriteString(quote(s[:contextLen]))
				}
				old.WriteString("...")
				new.WriteString("...")
				if tailContext {
					old.WriteString(quote(s[len(s)-contextLen:]))
					new.WriteString(quote(s[len(s)-contextLen:]))
				}
			}
		case diff.EditKindAdd:
			colorGreen.Fprint(&new, quote(string(edit.Sliceable.(starlark.String))))
		case diff.EditKindReplace:
			lit := edit.Index(0).(*diff.LiteralDiff)
			colorRed.Fprint(&old, quote(string(lit.Old().(starlark.String))))
			colorGreen.Fprint(&new, quote(string(lit.New().(starlark.String))))
		}
	}

	fmt.Fprintf(w, "\"%s\" => \"%s\"", old.String(), new.String())
}

func printBytesDiff(w io.Writer) {
	colorYellow.Fprint(w, "<binary data differs>")
}

func printSliceDiff(w io.Writer, indent string, d *diff.SliceableDiff) {
	const contextLen = 2

	switch d.Old().(type) {
	case starlark.String:
		if _, ok := d.New().(starlark.String); ok {
			printStringDiff(w, d)
			return
		}
	case starlark.Bytes:
		if _, ok := d.New().(starlark.Bytes); ok {
			printBytesDiff(w)
			return
		}
	}

	fmt.Fprintf(w, "[\n")
	indent += "  "

	for i, val := range d.Edits() {
		edit := val.(*diff.Edit)
		values := edit.Sliceable

		switch edit.Kind() {
		case diff.EditKindDelete:
			printSliceDiffElements(w, indent, values, color.Set(color.FgRed))
		case diff.EditKindCommon:
			headContext, tailContext, totalContext := contextSize(d.Edits(), i, contextLen)
			if values.Len() <= totalContext+contextLen {
				printSliceDiffElements(w, indent, values, color.Set())
			} else {
				if headContext {
					printSliceDiffElements(w, indent, values.Slice(0, contextLen, 1).(starlark.Sliceable), color.Set())
				}
				fmt.Fprintf(w, "%s...\n", indent)
				if tailContext {
					printSliceDiffElements(w, indent, values.Slice(values.Len()-contextLen, values.Len(), 1).(starlark.Sliceable), color.Set())
				}
			}
		case diff.EditKindAdd:
			printSliceDiffElements(w, indent, values, color.Set(color.FgGreen))
		case diff.EditKindReplace:
			printSliceDiffElements(w, indent, values, color.Set())
		}
	}

	indent = indent[:len(indent)-2]
	fmt.Fprintf(w, "%s]", indent)
}

func printSliceDiffElements(w io.Writer, indent string, values starlark.Sliceable, c *color.Color) {
	for i, len := 0, values.Len(); i < len; i++ {
		v := values.Index(i)

		fmt.Fprint(w, indent)
		if d, ok := v.(diff.ValueDiff); ok {
			printDiff(w, indent, d)
		} else {
			fmt.Fprint(w, c.Sprint(v))
		}
		fmt.Fprint(w, ",\n")
	}
}

func (e *statusRenderer) Print(label *label.Label, line string) {
	e.m.Lock()
	defer e.m.Unlock()

	t := e.targets[label.String()]
	t.lines = append(t.lines, line)

	if e.verbose {
		e.lines = append(e.lines, fmt.Sprintf("[%v] %v", label, line))
	}
}

func (e *statusRenderer) RequirementLoading(label *label.Label, version string) {
	e.targetStarted(label, "", nil, fmt.Sprintf("downloading %v...", version))
}

func (e *statusRenderer) RequirementLoaded(label *label.Label, version string) {
	e.targetDone(label, color.GreenString(fmt.Sprintf("downloaded %v", version)), true, false)
}

func (e *statusRenderer) RequirementLoadFailed(label *label.Label, version string, err error) {
	e.targetDone(label, color.RedString("failed: %v", errMessage(err)), true, true)
}

func (e *statusRenderer) ModuleLoading(label *label.Label) {
	e.targetStarted(label, "", nil, "loading...")
}

func (e *statusRenderer) ModuleLoaded(label *label.Label) {
	e.targetDone(label, "loaded", false, false)
}

func (e *statusRenderer) ModuleLoadFailed(label *label.Label, err error) {
	e.targetDone(label, color.RedString("failed: %v", errMessage(err)), true, true)
}

func (e *statusRenderer) LoadDone(err error) {
	e.m.Lock()
	defer e.m.Unlock()

	e.loaded, e.dirty = true, true
}

func (e *statusRenderer) TargetUpToDate(label *label.Label) {
	e.m.Lock()
	defer e.m.Unlock()

	e.statusLine = color.WhiteString("[%s] up-to-date", label)
	e.dirty = true
}

func (e *statusRenderer) TargetEvaluating(label *label.Label, reason string, diff diff.ValueDiff) {
	e.targetStarted(label, reason, diff, "running...")
}

func (e *statusRenderer) targetStarted(label *label.Label, reason string, diff diff.ValueDiff, message string) {
	e.m.Lock()
	defer e.m.Unlock()

	t := &target{label: label, reason: reason, diff: diff, start: time.Now()}
	e.targets[label.String()] = t

	t.setStatus(message)

	e.evaluating.append(t)

	e.dirty = true
}

func (e *statusRenderer) targetDone(label *label.Label, message string, changed, failed bool) {
	e.m.Lock()
	defer e.m.Unlock()

	t := e.targets[label.String()]
	if t == nil {
		t = &target{label: label, start: time.Now()}
	}

	t.setStatus(message)
	t.status = t.stamp(time.Now())
	t.failed = failed

	e.evaluating.remove(t)

	if changed || failed {
		e.done.append(t)
	}

	delete(e.targets, label.String())

	e.dirty = true
}

func (e *statusRenderer) TargetFailed(label *label.Label, err error) {
	e.targetDone(label, color.RedString("failed: %v", errMessage(err)), true, true)
}

func (e *statusRenderer) TargetSucceeded(label *label.Label, changed bool) {
	e.targetDone(label, color.GreenString("done"), changed, false)
}

func (e *statusRenderer) RunDone(err error) {
}

func (e *statusRenderer) FileChanged(label *label.Label) {
	e.m.Lock()
	defer e.m.Unlock()

	e.statusLine = color.YellowString("[%s] changed", label)
	e.dirty = true
}

func (e *statusRenderer) Close() error {
	e.ticker.Stop()
	e.render(time.Now(), true)
	return nil
}

func newRenderer(verbose, diff bool, onLoaded func()) (renderer, error) {
	new := func(_ renderer) renderer {
		if !term.IsTerminal(os.Stdout) {
			return &lineRenderer{stdout: os.Stdout, stderr: os.Stderr, onLoaded: onLoaded}
		}

		width, _, err := term.GetSize(os.Stdout)
		if err != nil {
			return &lineRenderer{stdout: os.Stdout, stderr: os.Stderr, onLoaded: onLoaded}
		}

		events := &statusRenderer{
			ticker:     time.NewTicker(16 * time.Millisecond),
			targets:    map[string]*target{},
			maxWidth:   width,
			verbose:    verbose,
			diff:       diff,
			stdout:     os.Stdout,
			lastUpdate: time.Now(),
			onLoaded:   onLoaded,
		}

		events.stats.update(time.Now())

		go func() {
			for now := range events.ticker.C {
				if now.Sub(events.lastUpdate).Seconds() >= 1 {
					events.dirty = true
				}
				if events.stats.update(now) {
					events.dirty = true
				}
				events.render(now, false)
			}
		}()

		return events
	}

	type rendererFunc func(next renderer) renderer
	pipeline := []rendererFunc{new}

	if buildJSON != "" {
		if buildJSON == "-" {
			return newJSONRenderer(os.Stdout, discardRenderer), nil
		}

		f, err := os.Create(buildJSON)
		if err != nil {
			return nil, err
		}
		pipeline = append(pipeline, func(next renderer) renderer {
			return newJSONRenderer(f, next)
		})
	}

	if buildDOT != "" {
		if buildDOT == "-" {
			return newDOTRenderer(os.Stdout, work, discardRenderer), nil
		}

		f, err := os.Create(buildDOT)
		if err != nil {
			return nil, err
		}
		pipeline = append(pipeline, func(next renderer) renderer {
			return newDOTRenderer(f, work, next)
		})
	}

	var r renderer
	for _, p := range pipeline {
		r = p(r)
	}
	return r, nil
}

func errMessage(err error) string {
	if err == nil {
		return ""
	}

	var evalErr *starlark.EvalError
	var cdErr *runner.CyclicDependencyError
	switch {
	case errors.As(err, &cdErr):
		return cdErr.Trace()
	case errors.As(err, &evalErr):
		return evalErr.Backtrace()
	}
	return err.Error()
}

type resolveEvents struct {
	events dawn.Events
}

func (r resolveEvents) ProjectLoading(req project.RequirementConfig) {
	r.events.RequirementLoading(&label.Label{Kind: "project", Project: req.Path}, req.Version)
}

func (r resolveEvents) ProjectLoaded(req project.RequirementConfig) {
	r.events.RequirementLoaded(&label.Label{Kind: "project", Project: req.Path}, req.Version)
}

func (r resolveEvents) ProjectLoadFailed(req project.RequirementConfig, err error) {
	r.events.RequirementLoadFailed(&label.Label{Kind: "project", Project: req.Path}, req.Version, err)
}
