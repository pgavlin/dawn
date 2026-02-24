package lsp

// BuiltinInfo describes a predeclared Dawn builtin.
type BuiltinInfo struct {
	Name      string
	Signature string // e.g. "target(name=None, deps=None, ...)"
	Doc       string
	Members   []BuiltinMember // for module-like builtins (sh, os, json)
}

// BuiltinMember describes a member of a module builtin.
type BuiltinMember struct {
	Name      string
	Signature string
	Doc       string
	Members   []BuiltinMember // for nested modules (os.path)
}

// builtinRegistry contains documentation for all Dawn predeclared names.
var builtinRegistry = map[string]*BuiltinInfo{
	"target": {
		Name:      "target",
		Signature: "target(name=None, deps=None, sources=None, generates=None, function=None, default=None, always=None, docs=None)",
		Doc: `Defines a new build target in the current package. Typically used as a
decorator, in which case the decorated function is treated as the value
of the function parameter.`,
	},
	"glob": {
		Name:      "glob",
		Signature: "glob(include, exclude=None, dirs=None)",
		Doc: `Return a list of paths relative to the calling module's directory that match
the given include and exclude patterns. Typically passed to the sources parameter
of target.`,
	},
	"path": {
		Name:      "path",
		Signature: "path(label)",
		Doc:       `Returns the absolute OS path that corresponds to the given label.`,
	},
	"label": {
		Name:      "label",
		Signature: "label(path)",
		Doc:       `Returns the label that corresponds to the given project-relative path, if any.`,
	},
	"contains": {
		Name:      "contains",
		Signature: "contains(path)",
		Doc: `Returns the label that corresponds to the given OS path if the path is
contained in the current project. If the path is not contained in the
current project, contains returns (None, False).`,
	},
	"parse_flag": {
		Name:      "parse_flag",
		Signature: "parse_flag(name, default=None, type=None, choices=None, required=None, help=None)",
		Doc: `Defines and parses a new project flag in the current package.
Returns the flag's value.`,
	},
	"fail": {
		Name:      "fail",
		Signature: "fail(message)",
		Doc:       `Fails the calling target with the given message.`,
	},
	"host": {
		Name: "host",
		Doc:  `A struct with host platform information.`,
		Members: []BuiltinMember{
			{Name: "arch", Doc: "The host architecture (e.g. amd64, arm64)."},
			{Name: "os", Doc: "The host operating system (e.g. linux, darwin)."},
		},
	},
	"Cache": {
		Name: "Cache",
		Doc:  `The build cache directory path.`,
	},
	"package": {
		Name: "package",
		Doc:  `The current module's package path (e.g. "//cmd/dawn").`,
	},
	"json": {
		Name: "json",
		Doc:  `The json module provides JSON encoding and decoding.`,
		Members: []BuiltinMember{
			{Name: "encode", Signature: "json.encode(x)", Doc: "Encodes a Starlark value to a JSON string."},
			{Name: "decode", Signature: "json.decode(x)", Doc: "Decodes a JSON string to a Starlark value."},
			{Name: "indent", Signature: "json.indent(s, prefix=None, indent=None)", Doc: "Returns an indented form of a JSON-encoded string."},
		},
	},
	"os": {
		Name: "os",
		Doc:  `Provides a platform-independent interface to host operating system functionality.`,
		Members: []BuiltinMember{
			{Name: "path", Doc: "The path module provides functions to manipulate host paths.", Members: []BuiltinMember{
				{Name: "join", Signature: "os.path.join(*args)", Doc: "Join path elements."},
				{Name: "dir", Signature: "os.path.dir(path)", Doc: "Returns the directory component of a path."},
				{Name: "base", Signature: "os.path.base(path)", Doc: "Returns the base component of a path."},
				{Name: "ext", Signature: "os.path.ext(path)", Doc: "Returns the file extension."},
				{Name: "abs", Signature: "os.path.abs(path)", Doc: "Returns the absolute path."},
				{Name: "rel", Signature: "os.path.rel(basepath, targpath)", Doc: "Returns a relative path."},
			}},
			{Name: "environ", Signature: "os.environ(key, default=None)", Doc: "Returns the value of an environment variable."},
			{Name: "look_path", Signature: "os.look_path(file)", Doc: "Searches for an executable in PATH."},
			{Name: "exec", Signature: "os.exec(command, *args, env=None, dir=None)", Doc: "Executes a command."},
			{Name: "output", Signature: "os.output(command, *args, env=None, dir=None)", Doc: "Executes a command and returns its output."},
			{Name: "exists", Signature: "os.exists(path)", Doc: "Returns True if the path exists."},
			{Name: "getcwd", Signature: "os.getcwd()", Doc: "Returns the current working directory."},
			{Name: "mkdir", Signature: "os.mkdir(path)", Doc: "Creates a directory."},
			{Name: "makedirs", Signature: "os.makedirs(path)", Doc: "Creates a directory and all parents."},
		},
	},
	"sh": {
		Name: "sh",
		Doc: `The sh module provides functions for executing POSIX Shell, Bash, and
mksh commands.`,
		Members: []BuiltinMember{
			{Name: "exec", Signature: "sh.exec(command, env=None, dir=None)", Doc: "Executes a shell command."},
			{Name: "output", Signature: "sh.output(command, env=None, dir=None)", Doc: "Executes a shell command and returns its stdout."},
		},
	},
}

// membersForName returns the members available on a builtin via dot-access.
func membersForName(name string) []BuiltinMember {
	if info, ok := builtinRegistry[name]; ok {
		return info.Members
	}
	return nil
}
