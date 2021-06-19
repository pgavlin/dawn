package dawn

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
	"github.com/pgavlin/dawn/util"
)

type moduleConfig struct {
	ModuleProxy string
	ModuleCache string
}

type config struct {
	Modules moduleConfig

	Ignore []string
}

func (proj *Project) loadConfigFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	var c config
	if err = toml.NewDecoder(f).Decode(&c); err != nil {
		return err
	}
	if c.Modules.ModuleProxy != "" {
		proj.moduleProxy = c.Modules.ModuleProxy
	}
	if c.Modules.ModuleCache != "" {
		proj.moduleCache = c.Modules.ModuleCache
	}

	if len(c.Ignore) == 0 {
		proj.ignore = nil
	} else {
		ignore, err := util.CompileGlobs(c.Ignore)
		if err != nil {
			return fmt.Errorf("invalid ignores: %w", err)
		}
		proj.ignore = ignore
	}

	return nil
}

func (proj *Project) loadConfig() error {
	// load global settings first
	globalPath := filepath.Join(proj.root, ".dawnconfig")
	if err := proj.loadConfigFile(globalPath); err != nil {
		return err
	}

	// load user settings second
	userPath := filepath.Join(proj.root, ".dawnconfig.local")
	if _, err := os.Stat(userPath); err == nil {
		return proj.loadConfigFile(userPath)
	}
	return nil
}
