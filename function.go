package dawn

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pgavlin/dawn/label"
	"github.com/pgavlin/dawn/pickle"
	"github.com/pgavlin/dawn/util"
	"go.starlark.net/starlark"
)

// A function is the primary representation of a dawn build target.
//
// A function is considered out-of-date if its environment--the globals, default parameter
// values, and free variables references by the function--has changed with respect to its
// last execution.
type function struct {
	proj   *Project
	module *module
	label  *label.Label

	always     bool
	targetInfo targetInfo
	deps       []string
	sources    []string
	gens       []string
	docs       string
	function   starlark.HasEnv
	oldEnv     starlark.Value
	newEnv     starlark.Value

	out *lineWriter
}

func (f *function) Name() string {
	return f.label.Package + ":" + f.label.Name
}

func (f *function) Doc() string {
	return f.docs
}

func (f *function) String() string        { return f.label.String() }
func (f *function) Type() string          { return "target" }
func (f *function) Freeze()               {} // immutable
func (f *function) Truth() starlark.Bool  { return starlark.True }
func (f *function) Hash() (uint32, error) { return starlark.String(f.label.String()).Hash() }

func (f *function) Attr(name string) (starlark.Value, error) {
	switch name {
	case "label":
		return f.label, nil
	case "always":
		return starlark.Bool(f.always), nil
	case "env":
		globals, defaults, freevars := f.function.Env()
		return starlark.Tuple{globals, defaults, freevars}, nil
	case "dependencies":
		return util.StringList(f.deps).List(), nil
	case "sources":
		return util.StringList(f.sources).List(), nil
	case "generates":
		return util.StringList(f.gens).List(), nil
	default:
		return nil, nil
	}
}

func (f *function) AttrNames() []string {
	return []string{"label", "always", "env", "dependencies", "generates", "sources"}
}

func (f *function) Project() *Project {
	return f.proj
}

func (f *function) Label() *label.Label {
	return f.label
}

func (f *function) Dependencies() []*label.Label {
	return targetDependencies(f)
}

func (f *function) dependencies() []string {
	return f.deps
}

func (f *function) generates() []string {
	return f.gens
}

func (f *function) info() targetInfo {
	return f.targetInfo
}

func (f *function) upToDate() (bool, error) {
	// check env
	newEnv, err := functionEnv(f.function)
	if err != nil {
		return false, fmt.Errorf("computing function environment: %w", err)
	}
	f.newEnv = newEnv

	// if this target always runs, skip the equality check
	if f.always {
		f.targetInfo.Rerun = true
		return true, nil
	}

	eq, err := starlark.EqualDepth(f.oldEnv, f.newEnv, 1000)
	if err != nil {
		return false, fmt.Errorf("comparing function environments: %w", err)
	}
	if !eq {
		return false, nil
	}

	// if this target generates files, check to see that they exist
	for _, out := range f.gens {
		if _, err = os.Stat(out); err != nil {
			if os.IsNotExist(err) {
				return false, nil
			}
			return false, fmt.Errorf("checking generated files: %w", err)
		}
	}
	return true, nil
}

func (f *function) newThread() *starlark.Thread {
	thread := &starlark.Thread{
		Name: f.label.String(),
		Print: func(_ *starlark.Thread, msg string) {
			f.proj.events.Print(f.label, msg)
		},
		Load: func(_ *starlark.Thread, module string) (starlark.StringDict, error) {
			return nil, errors.New("targets cannot load modules")
		},
	}

	components := label.Split(f.label.Package)[1:]
	wd := filepath.Join(f.proj.root, filepath.Join(components...))
	util.Chdir(thread, wd)

	util.SetStdio(thread, f.out, f.out)

	thread.SetLocal("root", f.proj.root)
	thread.SetLocal("module", f.module)

	return thread
}

func (f *function) evaluate() (data string, changed bool, err error) {
	defer f.out.Flush()

	var args starlark.Tuple
	if fn, ok := f.function.(*starlark.Function); ok && fn.NumParams() > 0 {
		args = starlark.Tuple{f}
	}

	_, err = starlark.Call(f.newThread(), f.function, args, nil)
	if err != nil {
		return "", false, err
	}

	var buf bytes.Buffer
	b64 := base64.NewEncoder(base64.StdEncoding, &buf)
	if err := pickle.NewEncoder(b64, pickle.PicklerFunc(envPickler)).Encode(f.function); err != nil {
		return "", false, err
	}
	b64.Close()

	f.oldEnv = f.newEnv
	return buf.String(), true, nil
}

func (f *function) load() error {
	// load info
	info, err := f.proj.loadTargetInfo(f.label)
	if err != nil {
		return fmt.Errorf("loading prior function environment: %w", err)
	}
	f.targetInfo = info
	if f.always {
		f.targetInfo.Rerun = true
	}

	// refresh the target info
	//
	// TODO: move this into saveIndex
	if err = f.proj.saveTargetInfo(f.label, info); err != nil {
		return fmt.Errorf("refreshing target info: %w", err)
	}

	if len(info.Data) == 0 {
		f.oldEnv = starlark.None
	} else {
		b64 := base64.NewDecoder(base64.StdEncoding, strings.NewReader(info.Data))
		f.oldEnv, err = pickle.NewDecoder(b64, pickle.UnpicklerFunc(envUnpickler)).Decode()
		if err != nil {
			return fmt.Errorf("loading prior function environment: %w", err)
		}
	}

	return nil
}

// functionEnv returns the given function's environment by round-tripping it through the
// pickler.
func functionEnv(f starlark.HasEnv) (starlark.Value, error) {
	var buf bytes.Buffer
	if err := pickle.NewEncoder(&buf, pickle.PicklerFunc(envPickler)).Encode(f); err != nil {
		return nil, err
	}
	return pickle.NewDecoder(&buf, pickle.UnpicklerFunc(envUnpickler)).Decode()
}

// envPickler provides support for pickling functions.
//
// Functions are pickled as (NEWOBJ "dawn" "Function" (globals, defaults, freevars, code)).
func envPickler(x starlark.Value) (module, name string, args starlark.Tuple, err error) {
	switch x := x.(type) {
	case *function:
		return "dawn", "Target", starlark.Tuple{starlark.String(x.label.String())}, nil
	case starlark.HasEnv:
		globals, defaults, freevars := x.Env()

		var code []byte
		if fn, ok := x.(*starlark.Function); ok {
			code = fn.Code()
		}

		return "dawn", "Function", starlark.Tuple{globals, defaults, freevars, starlark.Bytes(code)}, nil
	default:
		return "", "", nil, fmt.Errorf("cannot pickle value of type %s", x.Type())
	}
}

// envUnpickler provides support for unpickling functions.
//
// Functions are unpickled from (NEWOBJ "dawn" "Function" (globals, defaults, freevars, code))
// into (globals, defaults, freevars, code).
func envUnpickler(module, name string, args starlark.Tuple) (starlark.Value, error) {
	if module != "dawn" {
		return nil, fmt.Errorf("cannot unpickle value of type %s.%s", module, name)
	}

	switch name {
	case "Target":
		if len(args) != 1 {
			return nil, fmt.Errorf("expcted 1 arg, got %v", len(args))
		}
		return args[0], nil
	case "Function":
		if len(args) != 4 {
			return nil, fmt.Errorf("expcted 4 args, got %v", len(args))
		}
		return args, nil
	default:
		return nil, fmt.Errorf("cannot unpickle value of type %s.%s", module, name)
	}
}
