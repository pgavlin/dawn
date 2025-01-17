package dawn

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pgavlin/dawn/internal/project"
	"github.com/pgavlin/dawn/label"
)

func (proj *Project) fetchModule(ctx context.Context, l *label.Label) (path string, _ map[string]string, err error) {
	filename := l.Name
	if filename == "" {
		filename = "BUILD.dawn"
	}
	components := label.Split(l.Package)[1:]

	// Local modules are always on disk.
	if l.Project == "" {
		return filepath.Join(proj.root, filepath.Join(components...), filename), proj.requirements, nil
	}
	if l.IsAlias() {
		return "", nil, fmt.Errorf("internal error: unresolved alias in label %v", l)
	}

	// Find the version for the project path.
	requirementVersion, ok := proj.buildList[l.Project]
	if !ok {
		return "", nil, fmt.Errorf("unknown project %v", l.Project)
	}

	projectDir, err := proj.resolver.FetchProject(ctx, project.RequirementConfig{Path: l.Project, Version: requirementVersion})
	if err != nil {
		return "", nil, fmt.Errorf("fetching project %v: %w", l.Project, err)
	}

	projectFilePath := filepath.Join(projectDir, "dawn.toml")
	config, err := project.LoadConfigFile(projectFilePath)
	if err != nil {
		if !os.IsNotExist(err) {
			return "", nil, fmt.Errorf("loading config file: %w", err)
		}
		projectFilePath = filepath.Join(projectDir, ".dawnconfig")
		config, err = project.LoadConfigFile(projectFilePath)
		if err != nil {
			return "", nil, fmt.Errorf("loading config file: %w", err)
		}
	}

	moduleReqs := make(map[string]string)
	for name, req := range config.Requirements {
		moduleReqs[name] = req.Path
	}

	return filepath.Join(projectDir, filepath.Join(components...), filename), moduleReqs, nil
}
