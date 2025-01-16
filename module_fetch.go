package dawn

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/blang/semver"
	"github.com/pgavlin/dawn/label"
)

func extractFile(f *zip.File, destDir string) error {
	if path.IsAbs(f.Name) {
		return fmt.Errorf("zip file contains absolute paths (%v)", f.Name)
	}
	name := path.Clean(f.Name)
	if name == ".." || strings.HasPrefix(name, "../") {
		return fmt.Errorf("zip file refers to parent directory (%v)", f.Name)
	}
	components := strings.Split(name, "/")

	reader, err := f.Open()
	if err != nil {
		return fmt.Errorf("extracting zip file: %w", err)
	}
	defer reader.Close()

	file, err := os.OpenFile(filepath.Join(destDir, filepath.Join(components...)), os.O_CREATE|os.O_WRONLY, os.FileMode(f.Mode()))
	if err != nil {
		return fmt.Errorf("extracting zip file: %w", err)
	}
	defer file.Close()

	_, err = io.Copy(file, reader)
	return err
}

func getFile(url string) (io.ReadCloser, error) {
	if strings.HasPrefix(url, "file://") {
		return os.Open(filepath.FromSlash(url[len("file://"):]))
	}

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("%v", resp.Status)
	}
	return resp.Body, nil
}

func (proj *Project) downloadModule(l *label.Label, version, destPath string) error {
	tmp, err := os.CreateTemp("", "dawn-module-*")
	if err != nil {
		return err
	}
	defer tmp.Close()
	defer os.Remove(tmp.Name())

	// TODO: make two requests: one to determine the latest version, one to fetch the archive
	// - necessary in order to extract to the proper location

	url := fmt.Sprintf("%v/%v/%v/@v/%v.zip", proj.moduleProxy, l.Module, l.Package[2:], version)

	contents, err := getFile(url)
	if err != nil {
		return err
	}
	defer contents.Close()

	if _, err = io.Copy(tmp, contents); err != nil {
		return err
	}
	if _, err = tmp.Seek(0, 0); err != nil {
		return err
	}
	info, err := tmp.Stat()
	if err != nil {
		return err
	}

	reader, err := zip.NewReader(tmp, info.Size())
	if err != nil {
		return err
	}

	tmpDir, err := os.MkdirTemp("", "dawn-module-*")
	if err != nil {
		return err
	}

	for _, f := range reader.File {
		if err := extractFile(f, tmpDir); err != nil {
			return err
		}
	}

	return os.Rename(tmpDir, destPath)
}

func (proj *Project) fetchModule(l *label.Label) (path string, err error) {
	filename := l.Name
	if filename == "" {
		filename = "BUILD.dawn"
	}
	components := label.Split(l.Package)[1:]

	if l.Module == "" {
		return filepath.Join(proj.root, filepath.Join(components...), filename), nil
	}

	version := l.Version
	if version == nil {
		contents, err := getFile(fmt.Sprintf("%v/%v/%v/@latest", proj.moduleProxy, l.Module, l.Package[2:]))
		if err != nil {
			return "", fmt.Errorf("determining latest version: %w", err)
		}
		vs, err := io.ReadAll(contents)
		if err != nil {
			return "", fmt.Errorf("determining latest version: %w", err)
		}
		v, err := semver.ParseTolerant(strings.TrimSpace(string(vs)))
		if err != nil {
			return "", fmt.Errorf("determining latest version: %w", err)
		}
		version = &v
	}

	moduleComponents := strings.Split(l.Module, "/")
	cacheDir := filepath.Join(proj.moduleCache, filepath.Join(moduleComponents...), version.String(), filepath.Join(components...))
	cachePath := filepath.Join(cacheDir, filename)

retry:
	for {
		// Is the module already in the cache?
		if _, err := os.Stat(cacheDir); err == nil {
			return cachePath, nil
		}

		// Make sure the cache directory exists.
		if err = os.MkdirAll(filepath.Dir(cacheDir), 0700); err != nil {
			return "", fmt.Errorf("creating cache directory: %w", err)
		}

		// Attempt to obtain the module lock.
		lockPath := cacheDir + ".lock"
		lockFile, err := os.Create(lockPath)
		if err != nil {
			if !os.IsExist(err) {
				return "", fmt.Errorf("obtaining module lock: %w", err)
			}
			for {
				time.Sleep(5 * time.Second)
				if _, err := os.Stat(lockPath); err != nil {
					if !os.IsNotExist(err) {
						return "", fmt.Errorf("obtaining module lock: %w", err)
					}
					continue retry
				}
			}
		}
		defer lockFile.Close()
		defer os.Remove(lockPath)

		// Download and extract the module.
		if err := proj.downloadModule(l, version.String(), cacheDir); err != nil {
			return "", fmt.Errorf("downloading module: %w", err)
		}
		return cachePath, nil
	}
}
