package vcs

import (
	"context"
	"iter"
	"path"
	"strings"
	"time"

	"golang.org/x/mod/module"
)

type Revision interface {
	ID() string
	PseudoID() string
	When() time.Time
	History() iter.Seq[Revision]
}

type Version struct {
	Version     module.Version
	ProjectPath string
	RevisionID  string
}

type Repository interface {
	Path() string

	DefaultRef(ctx context.Context) (string, error)
	Versions(ctx context.Context) ([]*Version, error)
	ResolveRef(ctx context.Context, ref string) (string, error)
	GetRevision(ctx context.Context, id string) (Revision, error)

	FetchRevision(ctx context.Context, projectPath string, revision Revision, destDir string) error
}

func IsWellKnown(address string) (kind, repoAddress, projectPath string, ok bool) {
	return isGitHub(strings.Split(address, "/"))
}

func isGitHub(components []string) (string, string, string, bool) {
	// GitHub repositorys are of the form "github.com/org/repo"
	if len(components) < 3 || components[0] != "github.com" {
		return "", "", "", false
	}

	return "git", path.Join(components[:3]...), path.Join(components[3:]...), true
}
