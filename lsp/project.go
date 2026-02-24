package lsp

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/pgavlin/dawn/internal/project"
	"github.com/pgavlin/dawn/label"
)

// ProjectContext holds information about the Dawn project.
type ProjectContext struct {
	Root         string
	Config       *project.Config
	Requirements map[string]string // alias -> project path
	BuildFiles   map[string]string // package path -> absolute file path
}

// newProjectContext discovers and loads a Dawn project from the given root URI.
func newProjectContext(rootURI string) *ProjectContext {
	root := uriToPath(rootURI)
	if root == "" {
		return nil
	}

	// Search upward for dawn.toml
	projectRoot := findProjectRoot(root)
	if projectRoot == "" {
		return &ProjectContext{Root: root, BuildFiles: map[string]string{}}
	}

	ctx := &ProjectContext{
		Root:         projectRoot,
		Requirements: map[string]string{},
		BuildFiles:   map[string]string{},
	}

	// Load config
	configPath := filepath.Join(projectRoot, "dawn.toml")
	cfg, err := project.LoadConfigFile(configPath)
	if err != nil {
		// Try legacy config
		configPath = filepath.Join(projectRoot, ".dawnconfig")
		cfg, err = project.LoadConfigFile(configPath)
		if err != nil {
			cfg = &project.Config{}
		}
	}
	ctx.Config = cfg

	for alias, req := range cfg.Requirements {
		ctx.Requirements[alias] = req.Path
	}

	// Scan for BUILD.dawn files
	ctx.scanBuildFiles()

	return ctx
}

func findProjectRoot(dir string) string {
	for {
		if _, err := os.Stat(filepath.Join(dir, "dawn.toml")); err == nil {
			return dir
		}
		if _, err := os.Stat(filepath.Join(dir, ".dawnconfig")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func (ctx *ProjectContext) scanBuildFiles() {
	ignoreSet := map[string]bool{
		".dawn": true,
		".git":  true,
	}
	if ctx.Config != nil {
		for _, pattern := range ctx.Config.Ignore {
			ignoreSet[pattern] = true
		}
	}

	_ = filepath.Walk(ctx.Root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			name := info.Name()
			if ignoreSet[name] {
				return filepath.SkipDir
			}
			return nil
		}
		if info.Name() == "BUILD.dawn" {
			rel, err := filepath.Rel(ctx.Root, filepath.Dir(path))
			if err != nil {
				return nil
			}
			pkg := "//"
			if rel != "." {
				pkg = "//" + filepath.ToSlash(rel)
			}
			ctx.BuildFiles[pkg] = path
		}
		return nil
	})
}

// ResolveLoadPath resolves a load() module string to an absolute file path.
func (ctx *ProjectContext) ResolveLoadPath(currentFile, moduleStr string) string {
	if ctx == nil {
		return ""
	}

	l, err := label.Parse(moduleStr)
	if err != nil {
		return ""
	}

	// Determine current package from file path
	currentPkg := ctx.packageForFile(currentFile)

	if l.Project == "" {
		if l.Name == "" {
			l.Name = "BUILD.dawn"
		}
		l, err = l.RelativeTo(currentPkg)
		if err != nil {
			return ""
		}
	} else if l.IsAlias() {
		// Alias resolution - we can't fully resolve external projects statically
		return ""
	}

	components := label.Split(l.Package)
	if len(components) > 0 && components[0] == "//" {
		components = components[1:]
	}

	path := filepath.Join(ctx.Root, filepath.Join(components...), l.Name)
	if _, err := os.Stat(path); err == nil {
		return path
	}

	return ""
}

// packageForFile returns the package path for a file.
func (ctx *ProjectContext) packageForFile(filePath string) string {
	if ctx == nil || ctx.Root == "" {
		return "//"
	}

	dir := filepath.Dir(filePath)
	rel, err := filepath.Rel(ctx.Root, dir)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "//"
	}
	if rel == "." {
		return "//"
	}
	return "//" + filepath.ToSlash(rel)
}

// uriToPath converts a file:// URI to a file path.
func uriToPath(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		return ""
	}
	if u.Scheme == "file" {
		return u.Path
	}
	return uri
}

// pathToURI converts a file path to a file:// URI.
func pathToURI(path string) string {
	return "file://" + path
}
