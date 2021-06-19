package dawn

import "go.starlark.net/resolve"

func init() {
	resolve.AllowRecursion = true
	resolve.AllowGlobalReassign = true
	resolve.AllowSet = true
}
