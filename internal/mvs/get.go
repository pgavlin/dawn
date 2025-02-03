package mvs

import (
	"context"
	"fmt"
	"path"
	"slices"

	"github.com/pgavlin/dawn/internal/project"
	"github.com/pgavlin/mvs"
	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

func transformReqs(
	ctx context.Context,
	root *project.Config,
	resolver *Resolver,
	tx func(ctx context.Context, root *mvsProject) ([]module.Version, error),
) (map[string]project.RequirementConfig, error) {
	oldProjects := make(map[string][]string)
	versions := make([]module.Version, 0, len(root.Requirements))
	for name, r := range root.Requirements {
		oldProjects[r.Path] = append(oldProjects[r.Path], name)
		versions = append(versions, requirementVersion(r))
	}

	rootProject := &mvsProject{
		Version:      module.Version{},
		Requirements: versions,
	}
	newVersions, err := tx(ctx, rootProject)
	if err != nil {
		return nil, err
	}

	newReqs := make(map[string]project.RequirementConfig)

	// First add requirements that existed in the old project.
	for _, v := range newVersions {
		if v.Path == "" {
			continue
		}
		names, ok := oldProjects[v.Path]
		if !ok {
			continue
		}
		for _, n := range names {
			newReqs[n] = versionRequirement(v)
		}
	}

	// Next add requirements that did not exist in the old project.
	for _, v := range newVersions {
		if v.Path == "" {
			continue
		}
		names := oldProjects[v.Path]
		if len(names) != 0 {
			continue
		}

		proj, err := resolver.resolveProject(ctx, v)
		if err != nil {
			return nil, err
		}

		name := proj.Name
		if name == "" {
			name = path.Base(v.Path)
		} else {
			_, major := project.SplitPathVersion(v.Path)
			name = project.JoinPathVersion(name, major)
		}
		for n, suffix := name, 1; ; n, suffix = fmt.Sprintf("%v-%v", name, suffix), suffix+1 {
			if _, ok := newReqs[n]; !ok {
				name = n
				break
			}
		}

		newReqs[name] = versionRequirement(v)
	}
	return newReqs, nil
}

func Get(ctx context.Context, root *project.Config, resolver *Resolver, query string) (map[string]project.RequirementConfig, error) {
	return transformReqs(ctx, root, resolver, func(ctx context.Context, root *mvsProject) ([]module.Version, error) {
		return get(ctx, root, resolver, parseVersionQuery(query))
	})
}

func get(ctx context.Context, root *mvsProject, resolver *Resolver, query versionQuery) ([]module.Version, error) {
	q := newQuerier(resolver)

	reqs := newReqs(root, resolver)
	buildList, err := mvs.BuildList(ctx, []module.Version{root.Version}, reqs)
	if err != nil {
		return nil, err
	}

	version, err := q.resolveVersionQuery(ctx, buildList, query)
	if err != nil {
		return nil, err
	}

	buildList, err = mvs.BuildList(ctx, []module.Version{root.Version}, reqs)
	if err != nil {
		return nil, err
	}

	currentVersion := ""
	if i := slices.IndexFunc(buildList, func(v module.Version) bool { return v.Path == version.Path }); i != -1 {
		currentVersion = buildList[i].Version
	} else {
		return append([]module.Version{version}, root.Requirements...), nil
	}

	switch semver.Compare(currentVersion, version.Version) {
	default:
		return root.Requirements, nil
	case -1:
		buildList, err = mvs.Upgrade(ctx, root.Version, reqs, version)
	case 1:
		buildList, err = mvs.Downgrade(ctx, root.Version, reqs, version)
	}
	if err != nil {
		return nil, err
	}

	i := slices.IndexFunc(root.Requirements, func(v module.Version) bool { return v.Path == version.Path })
	if i != -1 {
		root.Requirements[i].Version = version.Version
	} else {
		root.Requirements = append(root.Requirements, version)
	}
	return mvs.ReqList(ctx, root.Version, buildList, nil, reqs)
}

func UpgradeAll(ctx context.Context, root *project.Config, resolver *Resolver) (map[string]project.RequirementConfig, error) {
	return transformReqs(ctx, root, resolver, func(ctx context.Context, root *mvsProject) ([]module.Version, error) {
		reqs := newReqs(root, resolver)
		buildList, err := mvs.UpgradeAll(ctx, root.Version, reqs)
		if err != nil {
			return nil, err
		}

		return mvs.ReqList(ctx, root.Version, buildList, nil, reqs)
	})
}

func BuildList(ctx context.Context, root *project.Config, resolver *Resolver) (map[string]string, error) {
	versions := make([]module.Version, 0, len(root.Requirements))
	for _, r := range root.Requirements {
		versions = append(versions, requirementVersion(r))
	}

	rootProject := &mvsProject{
		Version:      module.Version{},
		Requirements: versions,
	}
	buildList, err := mvs.BuildList(ctx, []module.Version{rootProject.Version}, newReqs(rootProject, resolver))
	if err != nil {
		return nil, err
	}

	versionMap := make(map[string]string)
	for _, v := range buildList {
		versionMap[v.Path] = v.Version
	}
	return versionMap, nil
}

func Tidy(ctx context.Context, root *project.Config, resolver *Resolver) (map[string]project.RequirementConfig, error) {
	return transformReqs(ctx, root, resolver, func(ctx context.Context, root *mvsProject) ([]module.Version, error) {
		return mvs.Req(ctx, root.Version, nil, newReqs(root, resolver))
	})
}
