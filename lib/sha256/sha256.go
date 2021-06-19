package sha256

import (
	"os"
	"strings"

	"github.com/pgavlin/dawn/util"
	"go.starlark.net/starlark"
)

func Hash(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var data string
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "data", &data); err != nil {
		return nil, err
	}

	sum, err := util.SHA256(strings.NewReader(data))
	if err != nil {
		return nil, err
	}
	return starlark.String(sum), nil
}

func File(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "path", &path); err != nil {
		return nil, err
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sum, err := util.SHA256(f)
	if err != nil {
		return nil, err
	}
	return starlark.String(sum), nil
}
