package project

import (
	"fmt"
	"path"
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
