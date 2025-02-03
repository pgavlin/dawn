package mvs

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"golang.org/x/mod/module"

	"github.com/pgavlin/dawn/internal/project"
	"github.com/pgavlin/dawn/internal/vcs"
)

func versionRequirement(m module.Version) project.RequirementConfig {
	return project.RequirementConfig{
		Path:    m.Path,
		Version: m.Version,
	}
}

func requirementVersion(p project.RequirementConfig) module.Version {
	return module.Version{
		Path:    p.Path,
		Version: p.Version,
	}
}

type ResolveEvents interface {
	// ProjectLoading is called when a referenced project is being loaded.
	ProjectLoading(req project.RequirementConfig)
	// ProjectLoaded is called when a referenced project has finished loading.
	ProjectLoaded(req project.RequirementConfig)
	// ProjectLoadFailed is called when a referenced project fails to load.
	ProjectLoadFailed(req project.RequirementConfig, err error)
}

type discardResolveEventsT int

// DiscardResolveEvents is an implementation of ResolveEvents that discards all events.
var DiscardResolveEvents = discardResolveEventsT(0)

func (discardResolveEventsT) ProjectLoading(req project.RequirementConfig)               {}
func (discardResolveEventsT) ProjectLoaded(req project.RequirementConfig)                {}
func (discardResolveEventsT) ProjectLoadFailed(req project.RequirementConfig, err error) {}

type Dialer interface {
	dialRepository(ctx context.Context, vcs, address string) (vcs.Repository, error)
}

var DefaultDialer = defaultDialer(0)

type defaultDialer int

func (defaultDialer) dialRepository(ctx context.Context, vcsKind, address string) (vcs.Repository, error) {
	switch vcsKind {
	case "git":
		return vcs.DialGitRepository(ctx, address, nil)
	}
	return nil, fmt.Errorf("unsupported VCS kind %v", vcsKind)
}

type mvsProject struct {
	Name         string
	Version      module.Version
	Requirements []module.Version
}

type projectRepository struct {
	repository  vcs.Repository
	projectPath string
}

type Resolver struct {
	cacheDir string
	dialer   Dialer
	events   ResolveEvents

	projectRepositories sync.Map
	projectSummaries    sync.Map
	projectVersions     sync.Map
}

func NewResolver(cacheDir string, dialer Dialer, events ResolveEvents) *Resolver {
	if events == nil {
		events = DiscardResolveEvents
	}
	return &Resolver{cacheDir: cacheDir, dialer: dialer, events: events}
}

func (r *Resolver) findProjectRepository(ctx context.Context, projectPath string) (vcs.Repository, string, error) {
	key := project.TrimPathVersion(projectPath)

	repoV, ok := r.projectRepositories.Load(key)
	if ok {
		r := repoV.(*projectRepository)
		return r.repository, r.projectPath, nil
	}

	repo, relPath, err := func() (vcs.Repository, string, error) {
		kind, address, relPath, ok := vcs.IsWellKnown(key)
		if ok {
			repo, err := r.dialer.dialRepository(ctx, kind, address)
			if err != nil {
				return nil, "", fmt.Errorf("dialing repository: %w", err)
			}
			return repo, relPath, nil
		}

		// TODO(pdg): cache, parallelize, additional VCS systems
		address, relPath = key, ""
		for address != "" {
			repo, err := r.dialer.dialRepository(ctx, "git", address)
			if err == nil {
				return repo, relPath, nil
			}

			lastSep := strings.LastIndexByte(address, '/')
			if lastSep == -1 {
				break
			}
			address, relPath = address[:lastSep], path.Join(address[lastSep+1:], relPath)
		}
		return nil, "", fmt.Errorf("could not locate repository for %v", projectPath)
	}()
	if err != nil {
		return nil, "", err
	}
	repoV, _ = r.projectRepositories.LoadOrStore(key, &projectRepository{repository: repo, projectPath: relPath})
	pr := repoV.(*projectRepository)
	return pr.repository, pr.projectPath, nil
}

func (r *Resolver) resolveProjectRevision(ctx context.Context, p module.Version) (vcs.Repository, string, string, error) {
	repo, projectPath, err := r.findProjectRepository(ctx, p.Path)
	if err != nil {
		return nil, "", "", err
	}

	rev, err := module.PseudoVersionRev(p.Version)
	if err == nil {
		return repo, projectPath, rev, nil
	}

	versions, err := repo.Versions(ctx)
	if err != nil {
		return nil, "", "", fmt.Errorf("listing versions: %w", err)
	}
	i := slices.IndexFunc(versions, func(v *vcs.Version) bool { return v.Version == p })
	if i == -1 {
		return nil, "", "", errors.New("no such version")
	}
	taggedVersion := versions[i]

	return repo, taggedVersion.ProjectPath, taggedVersion.RevisionID, nil
}

func (r *Resolver) resolveProject(ctx context.Context, p module.Version) (*mvsProject, error) {
	summaryV, ok := r.projectSummaries.Load(p.String())
	if ok {
		return summaryV.(*mvsProject), nil
	}

	fetchedPath, err := r.FetchProject(ctx, versionRequirement(p))
	if err != nil {
		return nil, fmt.Errorf("fetching project: %w", err)
	}

	projectFilePath := filepath.Join(fetchedPath, "dawn.toml")
	config, err := project.LoadConfigFile(projectFilePath)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("loading config file: %w", err)
		}
		projectFilePath = filepath.Join(fetchedPath, ".dawnconfig")
		config, err = project.LoadConfigFile(projectFilePath)
		if err != nil {
			return nil, fmt.Errorf("loading config file: %w", err)
		}
	}

	reqs := make([]module.Version, 0, len(config.Requirements))
	for _, name := range slices.Sorted(maps.Keys(config.Requirements)) {
		req := config.Requirements[name]
		reqs = append(reqs, module.Version{
			Path:    req.Path,
			Version: req.Version,
		})
	}
	summary := &mvsProject{
		Name:         config.Name,
		Version:      p,
		Requirements: reqs,
	}

	summaryV, _ = r.projectSummaries.LoadOrStore(p.String(), summary)
	return summaryV.(*mvsProject), nil
}

func (r *Resolver) listVersions(ctx context.Context, p module.Version) ([]module.Version, error) {
	versionsV, ok := r.projectVersions.Load(p.String())
	if ok {
		return versionsV.([]module.Version), nil
	}

	repo, _, err := r.findProjectRepository(ctx, p.Path)
	if err != nil {
		return nil, err
	}
	taggedVersions, err := repo.Versions(ctx)
	if err != nil {
		return nil, err
	}

	var versions []module.Version
	for _, v := range taggedVersions {
		if v.Version.Path == p.Path {
			versions = append(versions, v.Version)
		}
	}

	versionsV, _ = r.projectVersions.LoadOrStore(p.String(), versions)
	return versionsV.([]module.Version), nil
}

func (r *Resolver) FetchProject(ctx context.Context, p project.RequirementConfig) (string, error) {
	cacheDir := filepath.Join(r.cacheDir, fmt.Sprintf("%v@%v", project.TrimPathVersion(p.Path), p.Version))

	doFetch := func() (err error) {
		// TODO: eliminate internal races (will ensure events are only issued once per project)

		r.events.ProjectLoading(p)

		repo, projectPath, revisionID, err := r.resolveProjectRevision(ctx, requirementVersion(p))
		if err != nil {
			return err
		}

		revision, err := repo.GetRevision(ctx, revisionID)
		if err != nil {
			return err
		}

		tmpDir, err := os.MkdirTemp("", "dawn-fetch-*")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmpDir)

		err = repo.FetchRevision(ctx, projectPath, revision, tmpDir)
		if err != nil {
			return err
		}

		srcPath := tmpDir
		if projectPath != "" {
			srcPath = filepath.Join(tmpDir, filepath.FromSlash(projectPath))
		}
		return os.Rename(srcPath, cacheDir)
	}

	// Is the project already in the cache?
	if _, err := os.Stat(cacheDir); err == nil {
		return cacheDir, nil
	}

	// Make sure the cache directory exists.
	if err := os.MkdirAll(filepath.Dir(cacheDir), 0700); err != nil {
		return "", fmt.Errorf("creating cache directory: %w", err)
	}

	// Fetch the module's project.
	if err := doFetch(); err != nil {
		if errors.Is(err, fs.ErrExist) {
			if _, err := os.Stat(cacheDir); err == nil {
				r.events.ProjectLoaded(p)
				return cacheDir, nil
			}
		}
		r.events.ProjectLoadFailed(p, err)
		return "", fmt.Errorf("downloading project: %w", err)
	}
	r.events.ProjectLoaded(p)
	return cacheDir, nil
}
