package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/pgavlin/dawn"
	"github.com/pgavlin/dawn/cmd/dawn/internal/term"
	"github.com/pgavlin/dawn/label"
)

type renderer interface {
	io.Closer
	dawn.Events
}

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

func (e *lineRenderer) ModuleLoading(label *label.Label) {
	e.print(label, "loading")
}

func (e *lineRenderer) ModuleLoaded(label *label.Label) {
	e.print(label, "loaded")
}

func (e *lineRenderer) ModuleLoadFailed(label *label.Label, err error) {
	e.printe(label, fmt.Sprintf("failed: %v", err))
}

func (e *lineRenderer) LoadDone(err error) {
	e.m.Lock()
	defer e.m.Unlock()

	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load project: %v", err)
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

func (e *lineRenderer) TargetEvaluating(label *label.Label, reason string) {
	e.print(label, "evaluating...")
}

func (e *lineRenderer) TargetFailed(label *label.Label, err error) {
	e.printe(label, fmt.Sprintf("failed: %v", err))
}

func (e *lineRenderer) TargetSucceeded(label *label.Label, changed bool) {
	e.print(label, "done")
}

func (e *lineRenderer) RunDone(err error) {
	e.m.Lock()
	defer e.m.Unlock()

	if err != nil {
		fmt.Fprintf(os.Stderr, "build failed: %v", err)
	} else {
		fmt.Fprintf(os.Stdout, "build succeeded")
	}
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

// JSON renderer
type jsonRenderer struct {
	m      sync.Mutex
	next   renderer
	enc    *json.Encoder
	closer io.Closer
}

func (e *jsonRenderer) Close() error {
	e.closer.Close()
	return e.next.Close()
}

func (e *jsonRenderer) Print(label *label.Label, line string) {
	e.event("Print", label, "line", line)
	e.next.Print(label, line)
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
	e.event("ModuleLoadFailed", label, "err", err)
	e.next.ModuleLoadFailed(label, err)
}

func (e *jsonRenderer) LoadDone(err error) {
	e.event("LoadDone", nil, "err", err)
	e.next.LoadDone(err)
}

func (e *jsonRenderer) TargetUpToDate(label *label.Label) {
	e.event("TargetUpToDate", label)
	e.next.TargetUpToDate(label)
}

func (e *jsonRenderer) TargetEvaluating(label *label.Label, reason string) {
	e.event("TargetEvaluating", label, "reason", reason)
	e.next.TargetEvaluating(label, reason)
}

func (e *jsonRenderer) TargetFailed(label *label.Label, err error) {
	e.event("TargetFailed", label, "err", err)
	e.next.TargetFailed(label, err)
}

func (e *jsonRenderer) TargetSucceeded(label *label.Label, changed bool) {
	e.event("TargetSucceeded", label, "changed", changed)
	e.next.TargetSucceeded(label, changed)
}

func (e *jsonRenderer) RunDone(err error) {
	e.event("RunDone", nil, "err", err)
	e.next.RunDone(err)
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
// +-----------------------------+
// | completed targets...        |
// | evaluating targets...       |
// | status line                 |
// | system stats                |
// +-----------------------------+

type target struct {
	label  *label.Label
	reason string
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

func (e *statusRenderer) Print(label *label.Label, line string) {
	e.m.Lock()
	defer e.m.Unlock()

	t := e.targets[label.String()]
	t.lines = append(t.lines, line)
}

func (e *statusRenderer) ModuleLoading(label *label.Label) {
	e.targetStarted(label, "", "loading...")
}

func (e *statusRenderer) ModuleLoaded(label *label.Label) {
	e.targetDone(label, "loaded", false, false)
}

func (e *statusRenderer) ModuleLoadFailed(label *label.Label, err error) {
	e.targetDone(label, color.RedString("failed: %v", err), true, true)
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

func (e *statusRenderer) TargetEvaluating(label *label.Label, reason string) {
	e.targetStarted(label, reason, "running...")
}

func (e *statusRenderer) targetStarted(label *label.Label, reason, message string) {
	e.m.Lock()
	defer e.m.Unlock()

	t := &target{label: label, reason: reason, start: time.Now()}
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
	e.targetDone(label, color.RedString("failed: %v", err), true, true)
}

func (e *statusRenderer) TargetSucceeded(label *label.Label, changed bool) {
	e.targetDone(label, color.GreenString("done"), changed, false)
}

func (e *statusRenderer) RunDone(err error) {
}

func (e *statusRenderer) Close() error {
	e.ticker.Stop()
	e.render(time.Now(), true)
	return nil
}

func newRenderer(onLoaded func()) (renderer, error) {
	new := func() renderer {
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

	if buildJSON != "" {
		var next renderer
		dest := os.Stdout
		if buildJSON != "-" {
			f, err := os.Create(buildJSON)
			if err != nil {
				return nil, err
			}
			dest, next = f, new()
		}

		r := &jsonRenderer{next: next, enc: json.NewEncoder(dest), closer: dest}
		r.enc.SetIndent("", "    ")
		return r, nil
	}

	return new(), nil
}
