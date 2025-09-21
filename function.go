package dawn

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/pgavlin/dawn/diff"
	"github.com/pgavlin/dawn/label"
	"github.com/pgavlin/dawn/pickle"
	"github.com/pgavlin/dawn/util"
	fxs "github.com/pgavlin/fx/v2/slices"
	"github.com/pgavlin/starlark-go/starlark"
	"github.com/pgavlin/starlark-go/starlarkstruct"
	"github.com/pgavlin/starlark-go/syntax"
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
	function   starlark.Callable
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
	case "function":
		return f.function, nil
	case "dependencies":
		return util.StringList(f.deps).List(), nil
	case "sources":
		return util.StringList(f.sources).List(), nil
	case "generates":
		return util.StringList(f.gens).List(), nil
	case "position":
		if hasPosition, ok := f.function.(interface{ Position() syntax.Position }); ok {
			pos := hasPosition.Position()
			return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
				"filename": starlark.String(pos.Filename()),
				"line":     starlark.MakeInt(int(pos.Line)),
				"column":   starlark.MakeInt(int(pos.Col)),
			}), nil
		}
		return starlark.None, nil
	default:
		return nil, nil
	}
}

func (f *function) AttrNames() []string {
	return []string{"label", "always", "function", "dependencies", "generates", "position", "sources"}
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

var functionEnvKeys = []starlark.String{
	"names",
	"constant values",
	"predeclared values",
	"universal values",
	"function values",
	"global values",
	"default parameter values",
	"free variables",
	"code",
}

func (f *function) diffEnv() (bool, string, diff.ValueDiff, error) {
	if f.oldEnv == starlark.None {
		return false, "target has never been run", nil, nil
	}

	eq, err := starlark.EqualDepth(f.oldEnv, f.newEnv, 1000)
	if err != nil {
		return false, "", nil, fmt.Errorf("comparing function environments: %w", err)
	}
	if eq {
		return true, "", nil, nil
	}

	oldEnv, ok := f.oldEnv.(*starlark.Dict)
	if !ok {
		return false, "", nil, fmt.Errorf("old environment is not a dict (%v)", oldEnv.Type())
	}
	newEnv, ok := f.newEnv.(*starlark.Dict)
	if !ok {
		return false, "", nil, fmt.Errorf("new environment is not a dict (%v)", newEnv.Type())
	}

	d, err := diff.DiffDepth(f.oldEnv, f.newEnv, 1000)
	if err != nil {
		return false, "", nil, fmt.Errorf("diffing environments: %w", err)
	}
	md, ok := d.(*diff.MappingDiff)
	if !ok {
		panic(fmt.Errorf("expected a diff in unequal environments"))
	}

	reasons := slices.Collect(fxs.FMap(functionEnvKeys, func(k starlark.String) (string, bool) {
		return string(k), bool(md.Has(k))
	}))

	var reason string
	switch len(reasons) {
	case 1:
		reason = reasons[0]
	case 2:
		reason = reasons[0] + " and " + reasons[1]
	default:
		reason = strings.Join(reasons[:len(reasons)-1], ", ") + ", and " + reasons[len(reasons)-1]
	}
	return false, reason + " changed", d, nil
}

func (f *function) upToDate(_ context.Context) (bool, string, diff.ValueDiff, error) {
	// check env
	newEnv, err := functionEnv(f.function)
	if err != nil {
		return false, "", nil, fmt.Errorf("computing function environment: %w", err)
	}
	f.newEnv = newEnv

	// if this target always runs, skip the equality check
	if f.always {
		f.targetInfo.Rerun = true
		return true, "", nil, nil
	}

	eq, reason, diff, err := f.diffEnv()
	if err != nil || !eq {
		return false, reason, diff, err
	}

	// if this target generates files, check to see that they exist
	for _, out := range f.gens {
		if _, err = os.Stat(out); err != nil {
			if os.IsNotExist(err) {
				// TODO: make this path relative to the project root
				reason := fmt.Sprintf("generated file %v does not exist", out)
				return false, reason, nil, nil
			}
			return false, "", nil, fmt.Errorf("checking generated files: %w", err)
		}
	}
	return true, "", nil, nil
}

func (f *function) newThread(ctx context.Context) (*starlark.Thread, func()) {
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

	return thread, util.SetContext(ctx, thread)
}

func (f *function) evaluate(ctx context.Context) (data string, changed bool, err error) {
	defer f.out.Flush()

	var args starlark.Tuple
	if fn, ok := f.function.(*starlark.Function); ok && fn.NumParams() > 0 {
		args = starlark.Tuple{f}
	}

	thread, done := f.newThread(ctx)
	defer done()
	_, err = starlark.Call(thread, f.function, args, nil)
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
func functionEnv(f starlark.Callable) (starlark.Value, error) {
	var buf bytes.Buffer
	if err := pickle.NewEncoder(&buf, pickle.PicklerFunc(envPickler)).Encode(f); err != nil {
		return nil, err
	}
	return pickle.NewDecoder(&buf, pickle.UnpicklerFunc(envUnpickler)).Decode()
}

// envPickler provides support for pickling functions and modules.
//
// - Builtins are pickled as (NEWOBJ "dawn" "Builtin" ())
// - Function code is pickled as (NEWOBJ "dawn" "FunctionCode" (module, globals, bytecode))
// - Functions are pickled as (NEWOBJ "dawn" "Function" (defaults, freevars, code)).
func envPickler(x starlark.Value) (module, name string, args starlark.Tuple, err error) {
	switch x := x.(type) {
	case *function:
		return "dawn", "Target", starlark.Tuple{starlark.String(x.label.String())}, nil
	case *starlark.Builtin:
		return "dawn", "Builtin", starlark.Tuple{}, nil
	case *starlark.FunctionCode:
		module, globals := x.ModuleEnv()
		return "dawn", "FunctionCode", starlark.Tuple{module, globals, starlark.Bytes(x.Bytecode())}, nil
	case *starlark.Function:
		defaults, freevars := x.Env()
		return "dawn", "Function", starlark.Tuple{defaults, freevars, x.Code()}, nil
	default:
		return "", "", nil, pickle.ErrCannotPickle
	}
}

// envUnpickler provides support for unpickling functions and modules.
//
//   - Builtins are unpickled from (NEWOBJ "dawn" "Builtin" ()) into ()
//   - Function code is unpickled from (NEWOBJ "dawn" "FunctionCode" (module, globals, bytecode))
//     into a dictionary.
//   - Functions are unpickled from (NEWOBJ "dawn" "Function" (defaults, freevars, code))
//     into a dictionary.
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
	case "Builtin":
		if len(args) != 0 {
			return nil, fmt.Errorf("expected 0 args, got %v", len(args))
		}
		return args, nil
	case "FunctionCode":
		if len(args) != 3 {
			return nil, fmt.Errorf("expcted 3 args, got %v", len(args))
		}
		module, globals, bytecode := args[0].(starlark.Tuple), args[1], args[2]
		names, constants, predeclared, universals, functions := module[0], module[1], module[2], module[3], module[4]

		dict := starlark.NewDict(7)
		dict.SetKey(starlark.String("names"), names)
		dict.SetKey(starlark.String("constant values"), constants)
		dict.SetKey(starlark.String("predeclared values"), makeDictFromAssociationList(predeclared))
		dict.SetKey(starlark.String("universal values"), makeDictFromAssociationList(universals))
		dict.SetKey(starlark.String("function values"), functions)
		dict.SetKey(starlark.String("global values"), makeDictFromAssociationList(globals))
		dict.SetKey(starlark.String("code"), bytecode)
		return dict, nil
	case "Function":
		if len(args) != 3 {
			return nil, fmt.Errorf("expcted 3 args, got %v", len(args))
		}
		defaults, freeVars, funcode := args[0], args[1], args[2].(*starlark.Dict)

		funcode.SetKey(starlark.String("default parameter values"), makeDictFromAssociationList(defaults))
		funcode.SetKey(starlark.String("free variables"), makeDictFromAssociationList(freeVars))
		return funcode, nil
	default:
		return nil, fmt.Errorf("cannot unpickle value of type %s.%s", module, name)
	}
}

func makeDictFromAssociationList(al starlark.Value) starlark.Value {
	pairs, ok := al.(starlark.Tuple)
	if !ok {
		return starlark.None
	}

	dict := starlark.NewDict(len(pairs))
	for _, pv := range pairs {
		pair := pv.(starlark.Tuple)
		dict.SetKey(pair[0].(starlark.String), pair[1])
	}
	return dict
}
