package dawn

import "github.com/pgavlin/starlark-go/resolve"

func init() {
	// Relax some Starlark restrictions.
	resolve.AllowRecursion = true
	resolve.AllowGlobalReassign = true
	resolve.AllowSet = true
}
