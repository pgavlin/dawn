package dawn

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/pgavlin/dawn/label"
	"github.com/pgavlin/dawn/util"
	"go.starlark.net/starlark"
)

// A sourceFile represents a single source file inside of a project.
type sourceFile struct {
	proj       *Project
	label      *label.Label
	targetInfo targetInfo
	generator  *label.Label
	path       string
	oldSum     string
	sum        string
}

func repoSourcePath(pkg, sourcePath string) (string, error) {
	if sourcePath == "" {
		return "", errors.New("path must not be empty")
	}

	if !path.IsAbs(sourcePath) {
		sourcePath = path.Join(pkg[2:], sourcePath)
	}

	sourcePath = path.Clean(sourcePath)
	if sourcePath == ".." || strings.HasPrefix(sourcePath, "../") {
		return "", fmt.Errorf("source file %v is outside of the project root", sourcePath)
	}

	return sourcePath, nil
}

func sourceLabel(pkg, sourcePath string) (*label.Label, error) {
	sourcePath, err := repoSourcePath(pkg, sourcePath)
	if err != nil {
		return nil, err
	}

	sourcePath = filepath.ToSlash(sourcePath)

	pkg, target := "//", sourcePath
	if lastSlash := strings.LastIndexByte(sourcePath, '/'); lastSlash != -1 {
		pkg, target = "//"+sourcePath[:lastSlash], sourcePath[lastSlash+1:]
	}
	return label.New("source", "", pkg, target)
}

func (f *sourceFile) Name() string {
	return f.label.String()
}

func (f *sourceFile) Doc() string {
	return ""
}

func (f *sourceFile) String() string        { return f.label.String() }
func (f *sourceFile) Type() string          { return "source" }
func (f *sourceFile) Freeze()               {} // immutable
func (f *sourceFile) Truth() starlark.Bool  { return starlark.True }
func (f *sourceFile) Hash() (uint32, error) { return starlark.String(f.label.String()).Hash() }

func (f *sourceFile) Project() *Project {
	return f.proj
}

func (f *sourceFile) Label() *label.Label {
	return f.label
}

func (f *sourceFile) Dependencies() []*label.Label {
	return targetDependencies(f)
}

func (f *sourceFile) dependencies() []string {
	if f.generator != nil {
		return []string{f.generator.String()}
	}
	return nil
}

func (f *sourceFile) generates() []string {
	return nil
}

func (f *sourceFile) info() targetInfo {
	return f.targetInfo
}

func (f *sourceFile) upToDate() (bool, error) {
	sum, err := fileSum(f.path)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	f.sum = sum

	return f.oldSum == f.sum, nil
}

func (f *sourceFile) evaluate() (data string, changed bool, err error) {
	f.oldSum = f.sum
	return f.sum, true, nil
}

func (f *sourceFile) load() error {
	info, err := f.proj.loadTargetInfo(f.label)
	if err != nil {
		return err
	}

	f.targetInfo = info
	f.oldSum = info.Data
	return nil
}

func fileSum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return "", err
	}
	if stat.IsDir() {
		return dirSum(path, f)
	}

	return util.SHA256(f)
}

func dirSum(path string, dir *os.File) (string, error) {
	entries, err := dir.ReadDir(0)
	if err != nil {
		return "", err
	}

	h := sha256.New()
	for _, entry := range entries {
		sum, err := fileSum(filepath.Join(path, entry.Name()))
		if err != nil {
			return "", err
		}
		if _, err := h.Write([]byte(sum)); err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
