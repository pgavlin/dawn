package mvs

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/pgavlin/dawn/internal/project"
	"github.com/pgavlin/dawn/internal/vcs"
	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

type versionQuery struct {
	path  string
	query string
}

type querier struct {
	resolver *Resolver
}

func newQuerier(resolver *Resolver) *querier {
	return &querier{resolver: resolver}
}

func parseVersionQuery(q string) versionQuery {
	// - foo/bar/baz@v1 parses as a semver range query
	// - foo/bar/baz@latest parses as a latest query for version v1
	// - foo/bar/baz@v2@latest parses as a latest query for version v2

	path, query := project.SplitPathVersion(q)
	if query != "" && semver.Major(query) == query {
		return versionQuery{path: q, query: "latest"}
	}
	return versionQuery{path: path, query: query}
}

func (q *querier) resolveVersionQuery(ctx context.Context, buildList []module.Version, query versionQuery) (module.Version, error) {
	repo, _, err := q.resolver.findProjectRepository(ctx, query.path)
	if err != nil {
		return module.Version{}, err
	}

	query.path = project.CleanPath(query.path)
	_, majorVersion := project.SplitPathVersion(query.path)

	switch query.query {
	case "", "latest":
		return q.resolveLatestQuery(ctx, repo, majorVersion, query)
	case "upgrade":
		return q.resolveUpgradeQuery(ctx, buildList, repo, majorVersion, query)
	case "patch":
		return q.resolvePatchQuery(ctx, buildList, repo, majorVersion, query)
	default:
		switch query.query[0] {
		case '<', '>':
			return q.resolveSemverRangeQuery(ctx, repo, majorVersion, query)
		case 'v':
			if semver.IsValid(query.query) {
				return q.resolveSemverRangeQuery(ctx, repo, majorVersion, query)
			}
			return q.resolveRefQuery(ctx, repo, majorVersion, query)
		default:
			return q.resolveRefQuery(ctx, repo, majorVersion, query)
		}
	}
}

func (q *querier) resolveLatestQuery(ctx context.Context, repo vcs.Repository, majorVersion string, query versionQuery) (module.Version, error) {
	// Find the latest release version.
	versions, err := repo.Versions(ctx)
	if err != nil {
		return module.Version{}, err
	}

	var prerelease *vcs.Version
	for _, v := range slices.Backward(versions) {
		if !majorVersionMatch(majorVersion, v.Version.Version) || v.Version.Path != query.path {
			continue
		}
		if semver.Prerelease(v.Version.Version) == "" {
			return v.Version, nil
		} else if prerelease == nil {
			prerelease = v
		}
	}
	if prerelease != nil {
		return prerelease.Version, nil
	}

	defaultBranch, err := repo.DefaultRef(ctx)
	if err != nil {
		return module.Version{}, fmt.Errorf("resolving default branch: %w", err)
	}
	return q.resolveRefQuery(ctx, repo, majorVersion, versionQuery{path: query.path, query: defaultBranch})
}

func (q *querier) resolveUpgradeQuery(ctx context.Context, buildList []module.Version, repo vcs.Repository, majorVersion string, query versionQuery) (module.Version, error) {
	// Return the latest version or the current version, whichever is greater.
	newVersion, err := q.resolveLatestQuery(ctx, repo, majorVersion, query)
	if err != nil {
		return module.Version{}, err
	}

	for _, v := range buildList {
		if v.Path == newVersion.Path {
			if semver.Compare(newVersion.Version, v.Version) < 0 {
				return v, nil
			}
			break
		}
	}
	return newVersion, nil
}

func (q *querier) resolvePatchQuery(ctx context.Context, buildList []module.Version, repo vcs.Repository, majorVersion string, query versionQuery) (module.Version, error) {
	i := slices.IndexFunc(buildList, func(v module.Version) bool { return v.Path == query.path })
	if i == -1 {
		return q.resolveLatestQuery(ctx, repo, majorVersion, query)
	}
	currentVersion := buildList[i]
	currentMajorMinor := semver.MajorMinor(currentVersion.Version)

	versions, err := repo.Versions(ctx)
	if err != nil {
		return module.Version{}, err
	}

	for _, v := range slices.Backward(versions) {
		if v.Version.Path == currentVersion.Path && semver.MajorMinor(v.Version.Version) == currentMajorMinor && semver.Compare(v.Version.Version, currentVersion.Version) > 0 {
			return v.Version, nil
		}
	}
	return currentVersion, nil
}

func (q *querier) resolveSemverRangeQuery(ctx context.Context, repo vcs.Repository, majorVersion string, query versionQuery) (module.Version, error) {
	accept, err := parseSemverRangeQuery(query.query)
	if err != nil {
		return module.Version{}, fmt.Errorf("invalid query: %w", err)
	}

	versions, err := repo.Versions(ctx)
	if err != nil {
		return module.Version{}, err
	}
	for _, v := range slices.Backward(versions) {
		if !majorVersionMatch(majorVersion, v.Version.Version) || v.Version.Path != query.path {
			continue
		}
		if accept(v.Version.Version) {
			return v.Version, nil
		}
	}

	return module.Version{}, fmt.Errorf("no acceptable version for range %q", query)
}

func (q *querier) resolveRefQuery(ctx context.Context, repo vcs.Repository, majorVersion string, query versionQuery) (module.Version, error) {
	// Resolve the reference.
	revisionID, err := repo.ResolveRef(ctx, query.query)
	if err != nil {
		return module.Version{}, fmt.Errorf("resolving %q: %w", query, err)
	}
	revision, err := repo.GetRevision(ctx, revisionID)
	if err != nil {
		return module.Version{}, fmt.Errorf("resolving revision %q: %w", revisionID, err)
	}

	// Find the closest tagged version.
	versions, err := repo.Versions(ctx)
	if err != nil {
		return module.Version{}, err
	}

	var version *vcs.Version
	for ancestor := range revision.History() {
		for _, v := range slices.Backward(versions) {
			if v.Version.Path == query.path && majorVersionMatch(majorVersion, v.Version.Version) && v.RevisionID == ancestor.ID() {
				version = v
				break
			}
		}
	}

	// If the closes tagged version is an exact match, return it.
	if version != nil && version.RevisionID == revision.ID() {
		return version.Version, nil
	}

	baseVersion := majorVersion
	if version != nil {
		baseVersion = version.Version.Version
	}

	pseudoVersion := module.PseudoVersion(majorVersion, baseVersion, revision.When(), revision.PseudoID())
	return module.Version{Path: query.path, Version: pseudoVersion}, nil
}

type semverRange func(s string) bool

func parseSemverRangeQuery(s string) (semverRange, error) {
	switch s[0] {
	case 'v':
		return parseSemverPrefix(s)
	case '>':
		return parseSemverGTE(s[1:])
	case '<':
		return parseSemverLTE(s[1:])
	default:
		panic(fmt.Errorf("unexpected range query %v", s))
	}
}

func parseSemverPrefix(s string) (semverRange, error) {
	if !semver.IsValid(s) {
		return nil, fmt.Errorf("invalid version %q", s)
	}

	canon := semver.Canonical(s)
	if canon == s {
		return func(v string) bool { return semver.Compare(canon, v) == 0 }, nil
	}

	return func(v string) bool { return semver.Compare(canon, v) <= 0 }, nil
}

func parseSemverGTE(s string) (semverRange, error) {
	if len(s) == 0 {
		return nil, errors.New("missing version")
	}

	mk := func(canon string) semverRange { return func(v string) bool { return semver.Compare(canon, v) < 0 } }
	if s[0] == '=' {
		mk = func(canon string) semverRange { return func(v string) bool { return semver.Compare(canon, v) <= 0 } }
		s = s[1:]
	}

	if !semver.IsValid(s) {
		return nil, fmt.Errorf("invalid version %q", s)
	}

	return mk(semver.Canonical(s)), nil
}

func parseSemverLTE(s string) (semverRange, error) {
	if len(s) == 0 {
		return nil, errors.New("missing version")
	}

	mk := func(canon string) semverRange { return func(v string) bool { return semver.Compare(canon, v) > 0 } }
	if s[0] == '=' {
		mk = func(canon string) semverRange { return func(v string) bool { return semver.Compare(canon, v) >= 0 } }
		s = s[1:]
	}

	if !semver.IsValid(s) {
		return nil, fmt.Errorf("invalid version %q", s)
	}

	return mk(semver.Canonical(s)), nil
}

func majorVersionMatch(major, ver string) bool {
	ver = semver.Major(ver)
	return major == ver || major == "" && (ver == "v0" || ver == "v1")
}
