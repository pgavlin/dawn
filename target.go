package dawn

import (
	"fmt"
	"sort"
	"strings"

	"github.com/pgavlin/dawn/label"
	"github.com/pgavlin/dawn/runner"
	"go.starlark.net/starlark"
)

// A Target represents a build target within a Project.
type Target interface {
	starlark.Value

	// Project returns the project that owns the target.
	Project() *Project
	// Label returns the target's label.
	Label() *label.Label
	// Dependencies returns the labels of the targets the target depends upon.
	Dependencies() []*label.Label
	// Doc returns the target's documentation string.
	Doc() string

	dependencies() []string
	generates() []string
	info() targetInfo
	upToDate() (bool, error)
	evaluate() (data string, changed bool, err error)
}

// runTarget implements runner.Target.
type runTarget struct {
	target  Target
	changed bool
	data    string
}

func (t *runTarget) Evaluate(engine runner.Engine) error {
	proj, label, info := t.target.Project(), t.target.Label(), t.target.info()

	// Evaluate the target's dependencies.
	depsUpToDate := true
	deps := t.target.dependencies()
	depData := map[string]string{}
	for i, dep := range engine.EvaluateTargets(deps...) {
		if dep.Error != nil {
			switch err := dep.Error.(type) {
			case UnknownTargetError:
				proj.events.TargetFailed(label, fmt.Errorf("missing dependency: %w", dep.Error))
			case runner.CyclicDependencyError:
				proj.events.TargetFailed(label, err)
			}
			return fmt.Errorf("dependency %v failed", deps[i])
		}

		label := deps[i]

		newData := dep.Target.(*runTarget).data
		depData[label] = newData

		prevData, ok := info.Dependencies[label]
		if !ok || dep.Target.(*runTarget).changed || newData != prevData {
			depsUpToDate = false
		}
	}

	// Check whether the target is up-to-date.
	upToDate, err := t.target.upToDate()
	if err != nil {
		proj.events.TargetFailed(label, err)
		return err
	}
	// If all dependencies are up-to-date, the target is up-to-date, and the target is not
	// being forced to re-run, we can terminate here.
	if !proj.always && depsUpToDate && upToDate && !info.Rerun {
		proj.events.TargetUpToDate(label)
		return nil
	}

	proj.events.TargetEvaluating(label, "")

	if proj.dryrun {
		proj.events.TargetSucceeded(label, true)
		return nil
	}

	// Otherwise, evaluate the target.
	data, changed, err := t.target.evaluate()
	if err != nil {
		proj.events.TargetFailed(label, err)

		// If the target fails, record that it must be re-run on the next build.
		proj.saveTargetInfo(label, targetInfo{
			Doc:          t.target.Doc(),
			Dependencies: depData,
			Rerun:        true,
		})
		return err
	}

	// Save the target's metadata.
	t.changed, t.data = changed, data
	err = proj.saveTargetInfo(label, targetInfo{
		Doc:          t.target.Doc(),
		Dependencies: depData,
		Data:         data,
	})
	if err != nil {
		proj.events.TargetFailed(label, err)
		return err
	}
	proj.events.TargetSucceeded(label, changed)
	return nil
}

func targetDependencies(t Target) []*label.Label {
	deps := t.dependencies()
	labels := make([]*label.Label, len(deps))
	for i, dep := range deps {
		l, err := label.Parse(dep)
		if err != nil {
			panic(err)
		}
		labels[i] = l
	}
	sort.Slice(labels, func(i, j int) bool {
		return labels[i].String() < labels[j].String()
	})
	return labels
}

// DocSummary returns a summary of the target's docstring.
func DocSummary(t Target) string {
	summary := strings.TrimSpace(t.Doc())
	newline := strings.IndexByte(summary, '\n')
	if newline != -1 {
		summary = summary[:newline]
	}
	return summary
}
