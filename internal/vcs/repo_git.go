package vcs

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"iter"
	"os"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"

	"github.com/pgavlin/dawn/internal/project"
)

type gitCommit struct {
	r *gitRepository
	c *object.Commit
}

func (c *gitCommit) ID() string {
	return hex.EncodeToString(c.c.Hash[:6])
}

func (c *gitCommit) When() time.Time {
	return c.c.Committer.When
}

func (c *gitCommit) History() iter.Seq[Revision] {
	return func(yield func(Revision) bool) {
		at, iter := c.c, object.NewCommitPreorderIter(c.c, nil, nil)
		for {
			// Try to get the next commit. If there's an error, fetch the repo history and resume iteration.
			next, err := iter.Next()
			if err != nil {
				if errors.Is(err, io.EOF) {
					return
				}
				err = c.r.r.FetchContext(context.TODO(), &git.FetchOptions{
					RemoteName: "origin",
					RefSpecs:   []config.RefSpec{config.RefSpec("+refs/heads/*:refs/heads/*"), config.RefSpec("+refs/tags/*:refs/tags/*")},
					Tags:       git.NoTags,
					Force:      true,
				})
				if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
					return
				}

				iter = object.NewCommitPreorderIter(at, nil, nil)
				next, err = iter.Next()
				if err != nil {
					return
				}
				next, err = iter.Next()
				if err != nil {
					return
				}
			}
			if !yield(&gitCommit{r: c.r, c: next}) {
				return
			}
			at = next
		}
	}
}

type gitRepository struct {
	r *git.Repository

	dir           string
	path          string
	defaultBranch string
	refs          map[string]string
	versions      []*Version
}

type DialGitOptions struct {
	Insecure  bool
	AllowFile bool
}

func (o *DialGitOptions) protocols() []string {
	if o == nil {
		return []string{"https", "ssh"}
	}
	switch {
	case !o.Insecure && o.AllowFile:
		return []string{"file", "https", "ssh"}
	case o.Insecure && !o.AllowFile:
		return []string{"http", "https", "ssh", "git"}
	case o.Insecure && o.AllowFile:
		return []string{"file", "http", "https", "ssh", "git"}
	default:
		return []string{"https", "ssh"}
	}
}

// TODO: should this accept config for a cache directory or something?
func DialGitRepository(ctx context.Context, repoPath string, options *DialGitOptions) (Repository, error) {
	tmpDir, err := os.MkdirTemp("", "mvs-repo-*")
	if err != nil {
		return nil, err
	}

	r, err := git.PlainInit(tmpDir, false)
	if err != nil {
		return nil, err
	}

	dial := func(address string) ([]*plumbing.Reference, error) {
		r.DeleteRemote("origin")

		origin, err := r.CreateRemote(&config.RemoteConfig{Name: "origin", URLs: []string{address}})
		if err != nil {
			return nil, fmt.Errorf("creating remote: %w", err)
		}

		return origin.ListContext(ctx, &git.ListOptions{
			PeelingOption: git.AppendPeeled,
		})
	}

	var refs []*plumbing.Reference
	found := false
	for _, protocol := range options.protocols() {
		if refs, err = dial(fmt.Sprintf("%v://%v", protocol, repoPath)); err == nil {
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("no reachable repository for %q", repoPath)
	}

	refTable := map[string]string{}
	var defaultBranch plumbing.ReferenceName
	var versions []*Version
	for _, r := range refs {
		switch {
		case r.Name() == "HEAD":
			defaultBranch = r.Target()
			refTable["HEAD"] = r.Hash().String()
		case r.Name().IsTag():
			tagName, _ := strings.CutSuffix(r.Name().String(), "^{}")
			name := plumbing.ReferenceName(tagName)
			refTable[name.String()] = r.Hash().String()

			projectPath, version := path.Split(name.Short())
			projectPath = path.Clean(projectPath)

			if !semver.IsValid(version) {
				// Skip this tag.
				continue
			}

			versions = append(versions, &Version{
				Version: module.Version{
					Path:    project.JoinPathVersion(path.Join(repoPath, projectPath), semver.Major(version)),
					Version: version,
				},
				ProjectPath: projectPath,
				RevisionID:  r.Hash().String(),
			})
		case r.Name().IsBranch():
			refTable[r.Name().String()] = r.Hash().String()
		}
	}
	if defaultBranch == "" {
		return nil, fmt.Errorf("no default branch")
	}

	slices.SortStableFunc(versions, func(a, b *Version) int {
		return -semver.Compare(a.Version.Version, b.Version.Version)
	})

	return &gitRepository{
		r:             r,
		dir:           tmpDir,
		path:          repoPath,
		defaultBranch: defaultBranch.Short(),
		refs:          refTable,
		versions:      versions,
	}, nil
}

func (r *gitRepository) Path() string {
	return r.path
}

func (r *gitRepository) DefaultRef(ctx context.Context) (string, error) {
	return r.defaultBranch, nil
}

func (r *gitRepository) Versions(ctx context.Context) ([]*Version, error) {
	return r.versions, nil
}

func (r *gitRepository) ResolveRef(ctx context.Context, ref string) (string, error) {
	var hash string
	if hash = r.refs[ref]; hash != "" {
		// OK
	} else if hash = r.refs["refs/heads/"+ref]; hash != "" {
		// OK
	} else if hash = r.refs["refs/tags/"+ref]; hash != "" {
		// OK
	} else {
		rev, err := r.GetRevision(ctx, ref)
		if err != nil {
			return "", fmt.Errorf("unknown revision %q", ref)
		}
		hash = rev.(*gitCommit).c.Hash.String()
	}
	return hash, nil
}

func (r *gitRepository) GetRevision(ctx context.Context, id string) (Revision, error) {
	h, err := hex.DecodeString(id)
	if err != nil || len(h) < 6 || len(h) > 20 {
		return nil, fmt.Errorf("invalid revision %q", id)
	}

	// Check cached refs for this revision.
	var ref string
	for r, hash := range r.refs {
		if strings.HasPrefix(hash, id) {
			ref = r
			break
		}
	}

	// If we have a ref, fetch that ref. Otherwise, fetch the entire remote.
	refSpecs, depth := []config.RefSpec{config.RefSpec("+refs/heads/*:refs/heads/*"), config.RefSpec("+refs/tags/*:refs/tags/*")}, 0
	if ref != "" {
		// TODO: use a shallow fetch. Blocked on some issues with go-git when attempting to unshallow.
		refSpecs, depth = []config.RefSpec{config.RefSpec(fmt.Sprintf("+%v:refs/dummy", ref))}, 0
	}
	err = r.r.FetchContext(ctx, &git.FetchOptions{
		RemoteName: "origin",
		RefSpecs:   refSpecs,
		Depth:      depth,
		Tags:       git.NoTags,
		Force:      true,
	})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return nil, err
	}

	hash, err := r.r.ResolveRevision(plumbing.Revision(id))
	if err != nil {
		return nil, err
	}

	commit, err := r.r.CommitObject(*hash)
	if err != nil {
		return nil, err
	}
	return &gitCommit{r: r, c: commit}, nil
}

func (r *gitRepository) FetchRevision(ctx context.Context, projectPath string, revision Revision, destDir string) error {
	commit := revision.(*gitCommit)

	tree, err := r.r.Worktree()
	if err != nil {
		return err
	}

	var sparseCheckoutDirectories []string
	if projectPath != "" && projectPath != "." {
		sparseCheckoutDirectories = []string{projectPath}
	}
	err = tree.Checkout(&git.CheckoutOptions{
		Hash:                      commit.c.Hash,
		SparseCheckoutDirectories: sparseCheckoutDirectories,
		Force:                     true,
	})
	if err != nil {
		return err
	}

	return os.CopyFS(destDir, os.DirFS(r.dir))
}
