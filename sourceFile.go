package dawn

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/pgavlin/dawn/diff"
	"github.com/pgavlin/dawn/label"
	"github.com/pgavlin/starlark-go/starlark"
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

func (f *sourceFile) upToDate(ctx context.Context) (bool, string, diff.ValueDiff, error) {
	sum, err := fileSum(ctx, f.path)
	if err != nil && !os.IsNotExist(err) {
		return false, "", nil, err
	}
	f.sum = sum

	if f.oldSum == f.sum {
		return true, "", nil, nil
	}

	return false, "file contents changed", nil, nil
}

func (f *sourceFile) evaluate(_ context.Context) (data string, changed bool, err error) {
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

func fileSum(ctx context.Context, path string) (string, error) {
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
		return dirSum(ctx, path, f)
	}

	h := sha256.New()
	buf := make([]byte, min(64<<20, stat.Size()))
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		n, err := f.Read(buf)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return "", err
		}
		h.Write(buf[:n])
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func dirSum(ctx context.Context, path string, dir *os.File) (string, error) {
	entries, err := dir.ReadDir(0)
	if err != nil {
		return "", err
	}

	h := sha256.New()
	for _, entry := range entries {
		sum, err := fileSum(ctx, filepath.Join(path, entry.Name()))
		if err != nil {
			return "", err
		}
		h.Write([]byte(sum))
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
