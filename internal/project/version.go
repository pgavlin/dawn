package project

import (
	"fmt"
	"path"
	"strings"

	"golang.org/x/mod/semver"
)

func CleanPath(p string) string {
	p, v := SplitPathVersion(p)
	return JoinPathVersion(path.Clean(p), v)
}

func SplitPathVersion(p string) (string, string) {
	for i := len(p) - 1; i >= 0 && p[i] != '/'; i-- {
		if p[i] == '@' {
			return p[:i], p[i+1:]
		}
	}
	return p, ""
}

func TrimPathVersion(p string) string {
	p, _ = SplitPathVersion(p)
	return p
}

func JoinPathVersion(p, major string) string {
	if major == "" || major == "v0" || major == "v1" {
		return p
	}
	return fmt.Sprintf("%v@%v", p, major)
}

// CheckDawnVersion verifies that current satisfies the project's minimum Dawn
// version. Development builds are accepted so locally-built Dawn binaries can
// be used to develop modules that require a released version.
func CheckDawnVersion(current, required string) error {
	if required == "" || current == "" || current == "development" {
		return nil
	}

	rawCurrent := current
	if !strings.HasPrefix(current, "v") {
		current = "v" + current
	}
	if !semver.IsValid(current) {
		return fmt.Errorf("invalid current Dawn version %q", rawCurrent)
	}
	if semver.Compare(current, required) < 0 {
		return fmt.Errorf("requires Dawn %s or newer (running %s)", required, current)
	}
	return nil
}
