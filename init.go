package dawn

import "go.starlark.net/resolve"

func init() {
	// Relax some Starlark restrictions.
	resolve.AllowRecursion = true
	resolve.AllowGlobalReassign = true
	resolve.AllowSet = true
}
