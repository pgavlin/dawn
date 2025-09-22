package os

import (
	"os"
	"strings"

	"github.com/pgavlin/dawn/util"
	starlark "github.com/pgavlin/starlark-go/starlark"
)

var environValue starlark.Value

func init() {
	env := os.Environ()
	envV := starlark.NewDict(len(env))
	for _, kvp := range env {
		eq := strings.IndexByte(kvp, '=')
		key, value := kvp[:eq], kvp[eq+1:]
		util.Must(envV.SetKey(starlark.String(key), starlark.String(value)))
	}
	envV.Freeze()
	environValue = envV
}

// starlark
//
//	def environ():
//	    """
//	    Returns a mapping object where keys and values are strings that represent
//	    the process environment. This mapping is captured at startup time.
//	    """
//
//starlark:builtin factory=NewEnviron,function=Environ
func environ(thread *starlark.Thread, fn *starlark.Builtin) (starlark.Value, error) {
	return environValue, nil
}
