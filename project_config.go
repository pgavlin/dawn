package dawn

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/pgavlin/dawn/internal/mvs"
	"github.com/pgavlin/dawn/internal/project"
	"github.com/pgavlin/glob"
)

func (proj *Project) loadConfigFile(p string) error {
	c, err := project.LoadConfigFile(p)
	if err != nil {
		return err
	}

	if len(c.Ignore) == 0 {
		proj.ignore = nil
	} else {
		ignore, err := glob.New(c.Ignore, nil)
		if err != nil {
			return fmt.Errorf("invalid ignores: %w", err)
		}
		proj.ignore = ignore
	}

	reqs := make(map[string]string)
	for name, req := range c.Requirements {
		reqs[name] = req.Path
	}
	proj.requirements = reqs

	buildList, err := mvs.BuildList(context.TODO(), c, proj.resolver)
	if err != nil {
		return fmt.Errorf("computing requirements: %w", err)
	}
	proj.buildList = buildList

	return nil
}

func (proj *Project) loadConfig() (err error) {
	for _, name := range []string{"dawn.toml", ".dawnconfig"} {
		path := filepath.Join(proj.root, name)
		if err = proj.loadConfigFile(path); err == nil {
			proj.configPath = path
			return nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	return err
}
