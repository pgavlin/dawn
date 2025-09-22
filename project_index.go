package dawn

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"

	"github.com/pgavlin/dawn/diff"
	"github.com/pgavlin/dawn/label"
	"github.com/pgavlin/starlark-go/starlark"
	"github.com/sugawarayuuta/sonnet"
)

// A TargetSummary contains summarial information about a build target.
type TargetSummary struct {
	// Label is the target's label.
	Label *label.Label `json:"label"`
	// Summary is a summary of the target's docstring (as returned by DocSummary(t)).
	Summary string `json:"summary"`
}

type index struct {
	Flags   []*Flag         `json:"flags"`
	Targets []TargetSummary `json:"targets"`
}

// Targets returns the targets listed in the index file of the project rooted at the given
// directory.
func Targets(root string) ([]TargetSummary, error) {
	work := filepath.Join(root, ".dawn", "build")

	//nolint:gosec
	f, err := os.Open(filepath.Join(work, "index.json"))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var index index
	if err := sonnet.NewDecoder(f).Decode(&index); err != nil {
		return nil, err
	}

	return index.Targets, nil
}

// An indexTarget is a synthetic target created by an index-only load.
type indexTarget struct {
	proj    *Project
	label   *label.Label
	deps    []string
	depData map[string]string
	doc     string
	data    string
}

func (t *indexTarget) Name() string {
	return t.label.String()
}

func (t *indexTarget) Doc() string {
	return t.doc
}

func (t *indexTarget) String() string        { return t.label.String() }
func (t *indexTarget) Type() string          { return t.label.Kind }
func (t *indexTarget) Freeze()               {} // immutable
func (t *indexTarget) Truth() starlark.Bool  { return starlark.True }
func (t *indexTarget) Hash() (uint32, error) { return starlark.String(t.label.String()).Hash() }

func (t *indexTarget) Project() *Project {
	return t.proj
}

func (t *indexTarget) Label() *label.Label {
	return t.label
}

func (t *indexTarget) Dependencies() []*label.Label {
	return targetDependencies(t)
}

func (t *indexTarget) dependencies() []string {
	return t.deps
}

func (t *indexTarget) generates() []string {
	return nil
}

func (t *indexTarget) info() targetInfo {
	return targetInfo{
		Doc:          t.doc,
		Dependencies: t.depData,
		Data:         t.data,
	}
}

func (t *indexTarget) upToDate(_ context.Context) (bool, string, diff.ValueDiff, error) {
	return true, "", nil, nil
}

func (*indexTarget) evaluate(_ context.Context) (data string, changed bool, err error) {
	return "", false, errors.New("index targets are not executable; please reload the project")
}

func (proj *Project) loadIndex() error {
	f, err := os.Open(filepath.Join(proj.work, "index.json"))
	if err != nil {
		return err
	}
	defer f.Close()

	var index index
	if err := sonnet.NewDecoder(f).Decode(&index); err != nil {
		return err
	}

	for _, flag := range index.Flags {
		proj.flags[flag.Name] = flag
	}

	for _, summary := range index.Targets {
		l := summary.Label

		info, err := proj.loadTargetInfo(l)
		if err != nil {
			return err
		}

		var target Target
		if IsSource(l) {
			components := label.Split(l.Package)[1:]
			path := filepath.Join(proj.root, filepath.Join(components...), l.Name)

			target = &sourceFile{
				proj:  proj,
				label: l,
				path:  path,
			}
		} else {
			deps := make([]string, 0, len(info.Dependencies))
			for k := range info.Dependencies {
				deps = append(deps, k)
			}
			sort.Strings(deps)
			target = &indexTarget{
				proj:    proj,
				label:   l,
				doc:     info.Doc,
				deps:    deps,
				depData: info.Dependencies,
				data:    info.Data,
			}
		}

		proj.targets[l.String()] = &runTarget{target: target}
	}

	return nil
}

func (proj *Project) saveIndex() error {
	f, err := os.Create(filepath.Join(proj.work, "index.json"))
	if err != nil {
		return err
	}
	defer f.Close()

	index := index{
		Flags:   make([]*Flag, 0, len(proj.args)),
		Targets: make([]TargetSummary, 0, len(proj.targets)),
	}
	for _, arg := range proj.flags {
		index.Flags = append(index.Flags, arg)
	}
	sort.Slice(index.Flags, func(i, j int) bool { return index.Flags[i].Name < index.Flags[j].Name })
	for _, t := range proj.targets {
		index.Targets = append(index.Targets, TargetSummary{
			Label:   t.target.Label(),
			Summary: DocSummary(t.target),
		})
	}
	sort.Slice(index.Targets, func(i, j int) bool { return index.Targets[i].Label.String() < index.Targets[j].Label.String() })

	enc := sonnet.NewEncoder(f)
	enc.SetIndent("", "    ")
	return enc.Encode(index)
}
