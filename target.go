package dawn

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/pgavlin/dawn/diff"
	"github.com/pgavlin/dawn/label"
	"github.com/pgavlin/dawn/runner"
	fxs "github.com/pgavlin/fx/v2/slices"
	"github.com/pgavlin/fx/v2/try"
	"github.com/pgavlin/starlark-go/starlark"
)

var ErrDependenciesFailed = errors.New("dependencies failed")

// A Target represents a build target within a Project.
type Target interface {
	starlark.Value

	// Project returns the project that owns the target.
	Project() *Project
	// Label returns the target's label.
	Label() *label.Label
	// Dependencies returns the labels of the targets the target depends upon.
	Dependencies() []*label.Label
	// Name returns the target's name, which is its stringified label.
	Name() string
	// Doc returns the target's documentation string.
	Doc() string
	// Pos returns the position in the target's module where the target is defined.
	Pos() string

	dependencies() []string
	generates() []string
	info() targetInfo
	upToDate(ctx context.Context) (bool, string, diff.ValueDiff, error)
	evaluate(ctx context.Context) (data string, changed bool, err error)
}

// runTarget implements runner.Target.
type runTarget struct {
	target  Target
	changed bool
	data    string
}

func (t *runTarget) Evaluate(ctx context.Context, engine runner.Engine) error {
	proj, label, info := t.target.Project(), t.target.Label(), t.target.info()

	// Copy the current version of the data.
	t.data = info.Data

	// Evaluate the target's dependencies.
	depsUpToDate := true
	deps := t.target.dependencies()

	depData := map[string]string{}
	var cyclicDepErr *runner.CyclicDependencyError
	var missingDeps []string
	var hasFailedDeps bool
	var outOfDateDeps []string
	for i, dep := range engine.EvaluateTargets(ctx, deps...) {
		if dep.Error != nil {
			switch err := dep.Error.(type) {
			case UnknownTargetError:
				missingDeps = append(missingDeps, err.Error())
			case *runner.CyclicDependencyError:
				if cyclicDepErr == nil || cyclicDepErr.On >= err.On {
					cyclicDepErr = err
				}
			default:
				hasFailedDeps = true
			}
			continue
		}

		label := deps[i]

		newData := dep.Target.(*runTarget).data
		depData[label] = newData

		prevData, ok := info.Dependencies[label]
		if !ok || dep.Target.(*runTarget).changed || newData != prevData {
			outOfDateDeps = append(outOfDateDeps, label)
			depsUpToDate = false
		}
	}

	// Check for missing dependencies.
	switch len(missingDeps) {
	case 0:
		// OK
	case 1:
		err := fmt.Errorf("missing dependency %v", missingDeps[0])
		proj.events.TargetFailed(label, err)
		return err
	case 2:
		err := fmt.Errorf("missing dependencies %v and %v", missingDeps[0], missingDeps[1])
		proj.events.TargetFailed(label, err)
		return err
	default:
		err := fmt.Errorf("missing dependencies: %v", strings.Join(missingDeps, ","))
		proj.events.TargetFailed(label, err)
		return err
	}

	// Check for cyclic dependencies.
	if cyclicDepErr != nil {
		proj.events.TargetFailed(label, cyclicDepErr)
		return errors.New(cyclicDepErr.Error())
	}

	// Check for failed deps.
	if hasFailedDeps {
		return ErrDependenciesFailed
	}

	// Check whether the target is up-to-date.
	upToDate, reason, diff, err := t.target.upToDate(ctx)
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

	switch {
	case !upToDate:
		// OK
	case proj.always:
		reason = "always"
	case !depsUpToDate:
		reason = fmt.Sprintf("out-of-date dependencies: %v", strings.Join(outOfDateDeps, ", "))
	case info.Rerun:
		reason = "failed during last run"
	}

	proj.events.TargetEvaluating(label, reason, diff)

	if proj.dryrun {
		// For dry runs, conservatively assume that the target changed.
		t.changed = true

		proj.events.TargetSucceeded(label, true)
		return nil
	}

	// Otherwise, evaluate the target.
	data, changed, err := t.target.evaluate(ctx)
	if err != nil {
		proj.events.TargetFailed(label, err)

		// If the target fails, record that it must be re-run on the next build.
		saveErr := proj.saveTargetInfo(label, targetInfo{
			Doc:          t.target.Doc(),
			Pos:          t.target.Pos(),
			Dependencies: info.Dependencies,
			Data:         t.data,
			Rerun:        true,
		})
		return errors.Join(saveErr, err)
	}

	// Save the target's metadata.
	t.changed = changed
	if changed {
		t.data = data
	}
	err = proj.saveTargetInfo(label, targetInfo{
		Doc:          t.target.Doc(),
		Pos:          t.target.Pos(),
		Dependencies: depData,
		Data:         t.data,
	})
	if err != nil {
		proj.events.TargetFailed(label, err)
		return err
	}
	proj.events.TargetSucceeded(label, changed)
	return nil
}

func targetDependencies(t Target) []*label.Label {
	return slices.SortedFunc(
		try.Must(fxs.MapUnpack(t.dependencies(), label.Parse)),
		func(a, b *label.Label) int { return cmp.Compare(a.String(), b.String()) },
	)
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
