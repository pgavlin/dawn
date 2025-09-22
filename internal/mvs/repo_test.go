package mvs

import (
	"context"
	"errors"
	"iter"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"time"

	"github.com/pgavlin/dawn/internal/project"
	"github.com/pgavlin/dawn/internal/vcs"
	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

type testRevision struct {
	id       string
	when     time.Time
	projects map[string]*mvsProject

	parent *testRevision
}

func (r *testRevision) ID() string {
	return r.id
}

func (r *testRevision) PseudoID() string {
	return r.id
}

func (r *testRevision) When() time.Time {
	return r.when
}

func (r *testRevision) Equals(other vcs.Revision) bool {
	or, ok := other.(*testRevision)
	return ok && r.id == or.id
}

func (r *testRevision) History() iter.Seq[vcs.Revision] {
	return func(yield func(vcs.Revision) bool) {
		for r != nil {
			if !yield(r) {
				break
			}
			r = r.parent
		}
	}
}

func testRevisions(revs []map[string]*mvsProject) *testRevision {
	var head *testRevision
	for i, r := range revs {
		head = &testRevision{
			id:       strconv.FormatInt(int64(i+1), 10),
			when:     time.Unix(100*int64(i+1), 0),
			projects: r,
			parent:   head,
		}
	}
	return head
}

type testRepository struct {
	path string

	defaultRef string
	head       *testRevision
	refs       map[string]string

	versions []*vcs.Version
}

func (r *testRepository) Path() string {
	return r.path
}

func (r *testRepository) DefaultRef(ctx context.Context) (string, error) {
	return r.defaultRef, nil
}

func (r *testRepository) Versions(ctx context.Context) ([]*vcs.Version, error) {
	if r.versions == nil {
		// NOTE: it is important that this is a non-nil value. Do not replace this with `var []...`.
		tags := make([]*vcs.Version, 0)
		for ref, revisionID := range r.refs {
			projectPath, version := path.Split(ref)
			projectPath = path.Clean(projectPath)

			if !semver.IsValid(version) {
				// Skip this tag.
				continue
			}

			tags = append(tags, &vcs.Version{
				Version: module.Version{
					Path:    project.JoinPathVersion(path.Join(r.Path(), projectPath), semver.Major(version)),
					Version: version,
				},
				ProjectPath: projectPath,
				RevisionID:  revisionID,
			})
		}

		slices.SortStableFunc(tags, func(a, b *vcs.Version) int {
			return semver.Compare(a.Version.Version, b.Version.Version)
		})
		r.versions = tags
	}
	return r.versions, nil
}

func (r *testRepository) ResolveRef(ctx context.Context, ref string) (string, error) {
	revisionID, ok := r.refs[ref]
	if !ok {
		return "", errors.New("no such reference")
	}
	return revisionID, nil
}

func (r *testRepository) GetRevision(ctx context.Context, id string) (vcs.Revision, error) {
	for rev := r.head; rev != nil; rev = rev.parent {
		if rev.id == id {
			return rev, nil
		}
	}
	return nil, errors.New("no such revision")
}

func (r *testRepository) FetchRevision(ctx context.Context, projectPath string, revision vcs.Revision, destDir string) error {
	rev := revision.(*testRevision)
	summary, ok := rev.projects[projectPath]
	if !ok {
		return errors.New("no such project")
	}

	reqs := map[string]project.RequirementConfig{}
	for i, r := range summary.Requirements {
		reqs[strconv.FormatInt(int64(i), 10)] = project.RequirementConfig{
			Path:    r.Path,
			Version: r.Version,
		}
	}

	projectDir := filepath.Join(destDir, filepath.FromSlash(projectPath))
	err := os.MkdirAll(projectDir, 0o700)
	if err != nil {
		return err
	}

	return project.WriteConfigFile(filepath.Join(projectDir, "dawn.toml"), &project.Config{
		Name:         summary.Name,
		Version:      summary.Version.Version,
		Requirements: reqs,
	})
}
