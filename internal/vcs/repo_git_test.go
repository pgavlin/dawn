package vcs

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/codeclysm/extract/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/mod/module"
)

func dialRepo(t *testing.T) Repository {
	t.Helper()

	f, err := os.Open(filepath.Join(".", "testdata", "repo.tar.gz"))
	require.NoError(t, err)
	defer f.Close()

	repoPath := t.TempDir()

	err = extract.Gz(context.Background(), f, repoPath, nil)
	require.NoError(t, err)

	repoPath = filepath.Join(repoPath, "repo")

	repo, err := DialGitRepository(context.Background(), filepath.ToSlash(repoPath), &DialGitOptions{AllowFile: true})
	require.NoError(t, err)

	return repo
}

func TestGitRepository(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skipf("skipped on windows due to issues in CI/CD")
	}

	t.Run("DefaultRef", func(t *testing.T) {
		t.Parallel()

		defaultRef, err := dialRepo(t).DefaultRef(context.Background())
		require.NoError(t, err)

		assert.Equal(t, "main", defaultRef)
	})

	t.Run("Versions", func(t *testing.T) {
		t.Parallel()

		repo := dialRepo(t)

		versions, err := repo.Versions(context.Background())
		require.NoError(t, err)

		expected := []*Version{
			{
				Version: module.Version{
					Path:    path.Join(repo.Path(), "subdir"),
					Version: "v0.0.1",
				},
				ProjectPath: "subdir",
				RevisionID:  "7c34061b3388a83d7a8bc323f4bf479b2862211f",
			},
			{
				Version: module.Version{
					Path:    repo.Path(),
					Version: "v0.1.0",
				},
				ProjectPath: ".",
				RevisionID:  "b7f0897e2ca01b82e8bf1ddfaaff47f5680570f6",
			},
		}

		assert.Equal(t, expected, versions)
	})

	t.Run("Refs", func(t *testing.T) {
		t.Parallel()
		refs := map[string]string{
			"subdir/v0.0.1": "7c34061b3388a83d7a8bc323f4bf479b2862211f",
			"v0.1.0":        "b7f0897e2ca01b82e8bf1ddfaaff47f5680570f6",
			"e3d9fa3042a5e8cfbe8a3662e4d5d258b08972ba": "e3d9fa3042a5e8cfbe8a3662e4d5d258b08972ba",
			"e3d9fa3042a5": "e3d9fa3042a5e8cfbe8a3662e4d5d258b08972ba",
		}
		for r, expected := range refs {
			t.Run(r, func(t *testing.T) {
				t.Parallel()

				repo := dialRepo(t)

				refID, err := repo.ResolveRef(context.Background(), r)
				require.NoError(t, err)
				assert.Equal(t, expected, refID)

				rev, err := repo.GetRevision(context.Background(), refID)
				require.NoError(t, err)
				assert.Equal(t, expected, rev.ID())
				assert.Equal(t, expected[:12], rev.PseudoID())
			})
		}
	})

	t.Run("FetchRevision", func(t *testing.T) {
		t.Parallel()

		t.Run("v0.1.0", func(t *testing.T) {
			t.Parallel()

			repo := dialRepo(t)

			rev, err := repo.GetRevision(context.Background(), "b7f0897e2ca01b82e8bf1ddfaaff47f5680570f6")
			require.NoError(t, err)

			temp := t.TempDir()
			err = repo.FetchRevision(context.Background(), "", rev, temp)
			require.NoError(t, err)

			const expected = "# Test\n\nA test repo for the Git dialer.\n"
			actual, err := os.ReadFile(filepath.Join(temp, "README.md")) //nolint:gosec
			require.NoError(t, err)
			assert.Equal(t, expected, string(actual))
		})

		t.Run("subdir/v0.0.1", func(t *testing.T) {
			t.Parallel()

			repo := dialRepo(t)

			rev, err := repo.GetRevision(context.Background(), "7c34061b3388a83d7a8bc323f4bf479b2862211f")
			require.NoError(t, err)

			temp := t.TempDir()
			err = repo.FetchRevision(context.Background(), "", rev, temp)
			require.NoError(t, err)

			const expected = "# Subdirectory\n"
			actual, err := os.ReadFile(filepath.Join(temp, "subdir", "README.md")) //nolint:gosec
			require.NoError(t, err)
			assert.Equal(t, expected, string(actual))
		})
	})

	t.Run("History", func(t *testing.T) {
		t.Parallel()

		repo := dialRepo(t)

		refID, err := repo.ResolveRef(context.Background(), "main")
		require.NoError(t, err)

		rev, err := repo.GetRevision(context.Background(), refID)
		require.NoError(t, err)

		history := slices.Collect(rev.History())
		assert.Len(t, history, 3)
	})
}
