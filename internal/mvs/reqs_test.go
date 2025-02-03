package mvs

import (
	"context"
	"errors"
	"fmt"
	"path"
	"testing"

	"github.com/pgavlin/dawn/internal/project"
	"github.com/pgavlin/dawn/internal/vcs"
	"github.com/pgavlin/mvs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/mod/module"
)

type testDialer struct {
	repos map[string]*testRepository
}

func (d testDialer) dialRepository(ctx context.Context, kind, address string) (vcs.Repository, error) {
	if r, ok := d.repos[address]; ok {
		return r, nil
	}
	return nil, errors.New("unreachable")
}

func TestMVS(t *testing.T) {
	const sandbox = "github.com/pgavlin/sandbox"
	m := func(project string, version int) module.Version {
		return module.Version{Path: path.Join(sandbox, project), Version: fmt.Sprintf("v1.%v.0", version)}
	}

	r := func(project string, version int) project.RequirementConfig {
		return versionRequirement(m(project, version))
	}

	dialer := testDialer{
		repos: map[string]*testRepository{
			sandbox: &testRepository{
				path:       sandbox,
				defaultRef: "main",
				refs: map[string]string{
					"main":     "1",
					"b/v1.1.0": "1",
					"c/v1.1.0": "1",
					"c/v1.2.0": "2",
					"c/v1.3.0": "3",
					"c/v1.4.0": "4",
					"d/v1.2.0": "2",
					"d/v1.3.0": "3",
					"d/v1.4.0": "4",
					"d/v1.5.0": "5",
					"e/v1.1.0": "1",
					"e/v1.2.0": "2",
					"f/v1.1.0": "1",
					"g/v1.1.0": "1",
					"h/v1.1.0": "1",
				},
				head: testRevisions([]map[string]*mvsProject{
					{
						"b": {
							Version:      m("b", 1),
							Requirements: []module.Version{m("d", 3)},
						},
						"c": {
							Version:      m("c", 1),
							Requirements: []module.Version{m("d", 2)},
						},
						"e": {
							Version: m("e", 1),
						},
						"f": {
							Version: m("f", 1),
						},
						"g": {
							Version:      m("g", 1),
							Requirements: []module.Version{m("c", 4)},
						},
						"h": {
							Version: m("h", 1),
						},
					},
					{
						"c": {
							Version:      m("c", 2),
							Requirements: []module.Version{m("d", 4)},
						},
						"d": {
							Version:      m("d", 2),
							Requirements: []module.Version{m("e", 1)},
						},
						"e": {
							Version: m("e", 2),
						},
					},
					{
						"c": {
							Version:      m("c", 3),
							Requirements: []module.Version{m("d", 5)},
						},
						"d": {
							Version:      m("d", 3),
							Requirements: []module.Version{m("e", 2)},
						},
					},
					{
						"c": {
							Version:      m("c", 4),
							Requirements: []module.Version{m("g", 1)},
						},
						"d": {
							Version:      m("d", 4),
							Requirements: []module.Version{m("e", 2), m("f", 1)},
						},
					},
					{
						"d": {
							Version:      m("d", 5),
							Requirements: []module.Version{m("e", 2)},
						},
					},
				}),
			},
		},
	}

	t.Run("Reqs", func(t *testing.T) {
		root := &mvsProject{
			Version:      module.Version{},
			Requirements: []module.Version{m("b", 1), m("c", 2)},
		}

		cacheDir := t.TempDir()
		reqs := newReqs(root, NewResolver(cacheDir, dialer, nil))

		t.Run("BuildList", func(t *testing.T) {
			versions, err := mvs.BuildList(context.Background(), []module.Version{root.Version}, reqs)
			require.NoError(t, err)

			expected := []module.Version{{}, m("b", 1), m("c", 2), m("d", 4), m("e", 2), m("f", 1)}
			assert.Equal(t, expected, versions)
		})
		t.Run("UpgradeAll", func(t *testing.T) {
			versions, err := mvs.UpgradeAll(context.Background(), root.Version, reqs)
			require.NoError(t, err)

			expected := []module.Version{{}, m("b", 1), m("c", 4), m("d", 5), m("e", 2), m("f", 1), m("g", 1)}
			assert.Equal(t, expected, versions)
		})
		t.Run("Upgrade", func(t *testing.T) {
			versions, err := mvs.Upgrade(context.Background(), root.Version, reqs, m("c", 4))
			require.NoError(t, err)

			expected := []module.Version{{}, m("b", 1), m("c", 4), m("d", 4), m("e", 2), m("f", 1), m("g", 1)}
			assert.Equal(t, expected, versions)
		})
		t.Run("Downgrade", func(t *testing.T) {
			versions, err := mvs.Downgrade(context.Background(), root.Version, reqs, m("c", 1))
			require.NoError(t, err)

			expected := []module.Version{{}, m("b", 1), m("c", 1), m("d", 4), m("e", 2), m("f", 1)}
			assert.Equal(t, expected, versions)
		})
	})

	t.Run("Get", func(t *testing.T) {
		root := &project.Config{
			Requirements: map[string]project.RequirementConfig{
				"b": r("b", 1),
				"c": r("c", 2),
			},
		}

		cacheDir := t.TempDir()
		resolver := NewResolver(cacheDir, dialer, nil)

		t.Run("BuildList", func(t *testing.T) {
			versions, err := BuildList(context.Background(), root, resolver)
			require.NoError(t, err)

			expected := map[string]string{
				"":                             "",
				"github.com/pgavlin/sandbox/b": "v1.1.0",
				"github.com/pgavlin/sandbox/c": "v1.2.0",
				"github.com/pgavlin/sandbox/d": "v1.4.0",
				"github.com/pgavlin/sandbox/e": "v1.2.0",
				"github.com/pgavlin/sandbox/f": "v1.1.0",
			}
			assert.Equal(t, expected, versions)
		})

		t.Run("UpgradeAll", func(t *testing.T) {
			reqs, err := UpgradeAll(context.Background(), root, resolver)
			require.NoError(t, err)

			// TODO: it is frustrating that f1 is appearing in the reqs list, since it is not in fact required by any
			// of the other projects at their selected versions--it is only required by older versions of other
			// projects.
			//
			// This is intentional per https://go-review.googlesource.com/c/go/+/186537 and
			// https://go-review.googlesource.com/c/go/+/193397. Need to better understand why and if this is a problem
			// for dawn.

			expected := map[string]project.RequirementConfig{
				"b": r("b", 1),
				"c": r("c", 4),
				"d": r("d", 5),
				"f": r("f", 1),
			}
			assert.Equal(t, expected, reqs)
		})

		t.Run("Add", func(t *testing.T) {
			reqs, err := Get(context.Background(), root, resolver, "github.com/pgavlin/sandbox/g")
			require.NoError(t, err)

			expected := map[string]project.RequirementConfig{
				"b": r("b", 1),
				"c": r("c", 2),
				"g": r("g", 1),
			}
			assert.Equal(t, expected, reqs)
		})

		t.Run("Add ref", func(t *testing.T) {
			reqs, err := Get(context.Background(), root, resolver, "github.com/pgavlin/sandbox/g@main")
			require.NoError(t, err)

			expected := map[string]project.RequirementConfig{
				"b": r("b", 1),
				"c": r("c", 2),
				"g": r("g", 1),
			}
			assert.Equal(t, expected, reqs)
		})

		t.Run("Upgrade upgrade", func(t *testing.T) {
			reqs, err := Get(context.Background(), root, resolver, "github.com/pgavlin/sandbox/d@upgrade")
			require.NoError(t, err)

			expected := map[string]project.RequirementConfig{
				"b": r("b", 1),
				"c": r("c", 2),
				"d": r("d", 5),
			}
			assert.Equal(t, expected, reqs)
		})

		t.Run("Upgrade latest", func(t *testing.T) {
			reqs, err := Get(context.Background(), root, resolver, "github.com/pgavlin/sandbox/d@latest")
			require.NoError(t, err)

			expected := map[string]project.RequirementConfig{
				"b": r("b", 1),
				"c": r("c", 2),
				"d": r("d", 5),
			}
			assert.Equal(t, expected, reqs)
		})

		t.Run("Upgrade patch", func(t *testing.T) {
			reqs, err := Get(context.Background(), root, resolver, "github.com/pgavlin/sandbox/d@patch")
			require.NoError(t, err)

			expected := map[string]project.RequirementConfig{
				"b": r("b", 1),
				"c": r("c", 2),
			}
			assert.Equal(t, expected, reqs)
		})

		t.Run("Upgrade semver prefix", func(t *testing.T) {
			reqs, err := Get(context.Background(), root, resolver, "github.com/pgavlin/sandbox/c@v1.4")
			require.NoError(t, err)

			expected := map[string]project.RequirementConfig{
				"b": r("b", 1),
				"c": r("c", 4),
				"d": r("d", 4),
			}
			assert.Equal(t, expected, reqs)
		})

		t.Run("Upgrade semver GT", func(t *testing.T) {
			reqs, err := Get(context.Background(), root, resolver, "github.com/pgavlin/sandbox/c@>v1.3")
			require.NoError(t, err)

			expected := map[string]project.RequirementConfig{
				"b": r("b", 1),
				"c": r("c", 4),
				"d": r("d", 4),
			}
			assert.Equal(t, expected, reqs)
		})

		t.Run("Upgrade semver GTE", func(t *testing.T) {
			reqs, err := Get(context.Background(), root, resolver, "github.com/pgavlin/sandbox/c@>=v1.3")
			require.NoError(t, err)

			expected := map[string]project.RequirementConfig{
				"b": r("b", 1),
				"c": r("c", 4),
				"d": r("d", 4),
			}
			assert.Equal(t, expected, reqs)
		})

		t.Run("Upgrade semver LT", func(t *testing.T) {
			reqs, err := Get(context.Background(), root, resolver, "github.com/pgavlin/sandbox/c@<v1.4")
			require.NoError(t, err)

			expected := map[string]project.RequirementConfig{
				"b": r("b", 1),
				"c": r("c", 3),
				"f": r("f", 1),
			}
			assert.Equal(t, expected, reqs)
		})

		t.Run("Upgrade semver LTE", func(t *testing.T) {
			reqs, err := Get(context.Background(), root, resolver, "github.com/pgavlin/sandbox/c@<=v1.3")
			require.NoError(t, err)

			expected := map[string]project.RequirementConfig{
				"b": r("b", 1),
				"c": r("c", 3),
				"f": r("f", 1),
			}
			assert.Equal(t, expected, reqs)
		})

	})
}
