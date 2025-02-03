package mvs

import (
	"context"

	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

type Reqs struct {
	root     *mvsProject
	resolver *Resolver
}

func newReqs(root *mvsProject, resolver *Resolver) *Reqs {
	return &Reqs{
		root:     root,
		resolver: resolver,
	}
}

func (r *Reqs) Required(ctx context.Context, p module.Version) ([]module.Version, error) {
	// Is this the root module?
	if p.Path == "" {
		return r.root.Requirements, nil
	}

	// Resolve the version to a project summary.
	summary, err := r.resolver.resolveProject(ctx, p)
	if err != nil {
		return nil, err
	}
	return summary.Requirements, nil
}

func (r *Reqs) Max(_, v1, v2 string) string {
	if cmpVersion(v1, v2) == -1 {
		return v2
	}
	return v1
}

func (r *Reqs) Upgrade(ctx context.Context, p module.Version) (module.Version, error) {
	if p.Path == "" {
		return p, nil
	}

	versions, err := r.resolver.listVersions(ctx, p)
	if err != nil {
		return module.Version{}, err
	}

	major := semver.Major(p.Version)
	selected := p.Version
	for _, v := range versions {
		if semver.Major(v.Version) == major && semver.Compare(v.Version, selected) > 0 {
			selected = v.Version
		}
	}
	return module.Version{Path: p.Path, Version: selected}, nil
}

func (r *Reqs) Previous(ctx context.Context, p module.Version) (module.Version, error) {
	if p.Path == "" {
		return p, nil
	}

	versions, err := r.resolver.listVersions(ctx, p)
	if err != nil {
		return module.Version{}, err
	}

	major := semver.Major(p.Version)
	selected := ""
	for _, v := range versions {
		if semver.Major(v.Version) == major && semver.Compare(v.Version, p.Version) < 0 && semver.Compare(v.Version, selected) > 0 {
			selected = v.Version
		}
	}
	return module.Version{Path: p.Path, Version: selected}, nil
}

func cmpVersion(v1, v2 string) int {
	if v2 == "" {
		if v1 == "" {
			return 0
		}
		return -1
	}
	if v1 == "" {
		return 1
	}
	return semver.Compare(v1, v2)
}
