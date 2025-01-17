package project

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"golang.org/x/mod/semver"
)

type RequirementConfig struct {
	Path    string `toml:"path,inline"`
	Version string `toml:"version,inline"`
}

type Config struct {
	Name    string `toml:"name,omitempty"`
	Version string `toml:"version,omitempty"`

	Ignore []string `toml:"ignore,omitempty"`

	Requirements map[string]RequirementConfig `toml:"requirements,omitempty"`
}

func LoadConfigFile(path string) (*Config, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return LoadConfigBytes(contents)
}

func LoadConfigBytes(contents []byte) (*Config, error) {
	var c Config
	if err := toml.Unmarshal(contents, &c); err != nil {
		return nil, err
	}

	var errs []error
	for _, k := range slices.Sorted(maps.Keys(c.Requirements)) {
		req := c.Requirements[k]
		if !semver.IsValid(req.Version) || semver.Canonical(req.Version) != req.Version {
			errs = append(errs, fmt.Errorf("invalid version %q for dependency %q", req.Version, k))
		}
		req.Path = CleanPath(req.Path)
		c.Requirements[k] = req
	}
	if len(errs) != 0 {
		return nil, errors.Join(errs...)
	}

	return &c, nil
}

func WriteConfigFile(path string, c *Config) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	has := false
	print := func(format string, args ...any) {
		fmt.Fprintf(f, format, args...)
		has = true
	}
	printSection := func(format string, args ...any) {
		if has {
			fmt.Fprintln(f)
		}
		print(format, args...)
	}

	if c.Name != "" {
		print("name = %v\n", encodeValue(c.Name))
	}
	if c.Version != "" {
		print("version = %v\n", encodeValue(c.Version))
	}

	if len(c.Ignore) != 0 {
		printSection("ignore = %v\n", encodeValue(c.Ignore))
	}

	if len(c.Requirements) != 0 {
		printSection("[requirements]\n")
		for _, name := range slices.Sorted(maps.Keys(c.Requirements)) {
			req := c.Requirements[name]

			mustQuote := strings.ContainsFunc(name, func(r rune) bool { return !isPlainRune(r) })
			if mustQuote {
				name = encodeValue(name)
			}
			print("%v = {path = %v, version = %v}\n", name, encodeValue(req.Path), encodeValue(req.Version))
		}
	}

	return nil
}

func encodeValue(v any) string {
	var b strings.Builder
	err := toml.NewEncoder(&b).SetTablesInline(true).Encode(v)
	if err != nil {
		return "<invalid>"
	}
	return b.String()
}

func isPlainRune(r rune) bool {
	return r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_' || r == '-'
}
