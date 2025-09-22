package label

import (
	"encoding"
	"errors"
	"strings"
)

// A Label represents a parsed label.
//
// The general form represented is:
//
//	[kind:][[[project]//]package][:name]
//
// If the package is omitted and the name is present, the label is relative to the
// current package.
type Label struct {
	Kind    string
	Project string
	Package string
	Name    string
}

var (
	_ encoding.TextMarshaler   = (*Label)(nil)
	_ encoding.TextUnmarshaler = (*Label)(nil)
)

// Parse parses rawlabel into a Label structure.
func Parse(rawlabel string) (*Label, error) {
	nameColon := strings.LastIndexByte(rawlabel, ':')
	if nameColon == -1 {
		nameColon = len(rawlabel)
	}

	kindProjectAndPkg := rawlabel[:nameColon]

	var kind, projectAndPkg string
	if kindColon := strings.IndexByte(kindProjectAndPkg, ':'); kindColon != -1 {
		kind, projectAndPkg = kindProjectAndPkg[:kindColon], kindProjectAndPkg[kindColon+1:]
	} else {
		projectAndPkg = kindProjectAndPkg
	}

	project, pkg := "", projectAndPkg
	if i := strings.Index(projectAndPkg, "//"); i != -1 {
		project, pkg = projectAndPkg[:i], projectAndPkg[i:]
	}
	if strings.ContainsAny(project, ":") {
		return nil, errors.New("project may not contain ':'")
	}

	pkg, err := Clean(pkg)
	if err != nil {
		return nil, err
	}

	name := ""
	if nameColon < len(rawlabel) {
		name = rawlabel[nameColon+1:]
		if strings.ContainsRune(name, '/') {
			return nil, errors.New("names may not contain ':' or '/'")
		}
	}

	l := &Label{
		Kind:    kind,
		Project: project,
		Package: pkg,
		Name:    name,
	}
	if project != "" && !l.IsAbs() {
		return nil, errors.New("labels with projects must be absolute")
	}
	return l, nil
}

func New(kind, project, pkg, name string) (*Label, error) {
	if strings.ContainsAny(kind, ":/") {
		return nil, errors.New("kind may not contain ':' or '/'")
	}
	if strings.ContainsAny(project, ":") {
		return nil, errors.New("project may not contain ':'")
	}
	pkg, err := Clean(pkg)
	if err != nil {
		return nil, err
	}
	if project != "" && !strings.HasPrefix(pkg, "//") {
		return nil, errors.New("labels with projects must use absolute package paths")
	}
	if strings.ContainsAny(name, ":/") {
		return nil, errors.New("name may not contain ':' or '/'")
	}
	return &Label{Kind: kind, Project: project, Package: pkg, Name: name}, nil
}

func (l *Label) MarshalText() ([]byte, error) {
	return []byte(l.String()), nil
}

func (l *Label) UnmarshalText(text []byte) error {
	ll, err := Parse(string(text))
	if err != nil {
		return err
	}
	*l = *ll
	return nil
}

func (l *Label) IsAbs() bool {
	return strings.HasPrefix(l.Package, "//")
}

func (l *Label) IsAlias() bool {
	return strings.IndexByte(l.Project, '/') == -1
}

func (l *Label) RelativeTo(pkg string) (*Label, error) {
	if l.IsAbs() {
		return l, nil
	}
	pkg, err := Join(pkg, l.Package)
	if err != nil {
		return nil, err
	}
	return &Label{
		Kind:    l.Kind,
		Project: l.Project,
		Package: pkg,
		Name:    l.Name,
	}, nil
}

func (l *Label) String() string {
	var b strings.Builder
	if l.Kind != "" {
		b.WriteString(l.Kind)
		b.WriteRune(':')
	}
	if l.Project != "" {
		b.WriteString(l.Project)
	}
	b.WriteString(l.Package)
	if l.Name != "" {
		b.WriteRune(':')
		b.WriteString(l.Name)
	}
	return b.String()
}

// A lazybuf is a lazily constructed path buffer.
// It supports append, reading previously appended bytes,
// and retrieving the final string. It does not allocate a buffer
// to hold the output until that output diverges from s.
type lazybuf struct {
	s   string
	buf []byte
	w   int
}

func (b *lazybuf) index(i int) byte {
	if b.buf != nil {
		return b.buf[i]
	}
	return b.s[i]
}

func (b *lazybuf) append(c byte) {
	if b.buf == nil {
		if b.w < len(b.s) && b.s[b.w] == c {
			b.w++
			return
		}
		b.buf = make([]byte, len(b.s))
		copy(b.buf, b.s[:b.w])
	}
	b.buf[b.w] = c
	b.w++
}

func (b *lazybuf) string() string {
	if b.buf == nil {
		return b.s[:b.w]
	}
	return string(b.buf[:b.w])
}

// Clean returns the shortest pkg name equivalent to pkg and checks
// the pkg path for errors. It replaces multiple slashes with a single
// slash and rejects pkg paths that contain colons, '..' or '.' elements.
func Clean(pkg string) (string, error) {
	if pkg == "" {
		return "", nil
	}

	rooted := len(pkg) >= 2 && pkg[0] == '/' && pkg[1] == '/'
	n := len(pkg)

	// Invariants:
	//	reading from pkg; r is index of next byte to process.
	//	writing to buf; w is index of next byte to write.
	//	dotdot is index in buf where .. must stop, either because
	//		it is the leading slash or it is a leading ../../.. prefix.
	out := lazybuf{s: pkg}
	r := 0
	if rooted {
		out.append('/')
		out.append('/')
		r = 2
	}

	if !rooted && pkg[r] == '/' {
		return "", errors.New("absolute pkg paths must begin with '//'")
	}

	for r < n {
		switch {
		case pkg[r] == ':':
			return "", errors.New("pkg paths may not contain ':'")
		case pkg[r] == '/':
			// empty pkg element
			r++
		case pkg[r] == '.' && (r+1 == n || pkg[r+1] == '/'):
			return "", errors.New("pkg paths may not contain '.' or '..' elements")
		case pkg[r] == '.' && pkg[r+1] == '.' && (r+2 == n || pkg[r+2] == '/'):
			return "", errors.New("pkg paths may not contain '.' or '..' elements")
		default:
			// real pkg element.
			// add slash if needed
			if rooted && out.w != 2 || !rooted && out.w != 0 {
				out.append('/')
			}
			// copy element
			for ; r < n && pkg[r] != '/' && pkg[r] != ':'; r++ {
				out.append(pkg[r])
			}
		}
	}

	return out.string(), nil
}

// Parent returns the parent of pkg.
func Parent(pkg string) string {
	raw, adjust := pkg, 0
	if strings.HasPrefix(pkg, "//") {
		pkg, adjust = pkg[2:], 2
	}

	lastSlash := strings.LastIndexByte(pkg, '/')
	if lastSlash == -1 {
		return raw[:adjust]
	}
	return raw[:lastSlash+adjust]
}

// Dir returns the last component of pkg.
func Dir(pkg string) string {
	pkg = strings.TrimPrefix(pkg, "//")

	lastSlash := strings.LastIndexByte(pkg, '/')
	if lastSlash == -1 {
		return pkg
	}
	return pkg[lastSlash+1:]
}

// Split splits a package path into its components. If the path is absolute, the first
// component will be "//". Multiple slashes separating components are treated as a single
// slash.
func Split(pkg string) []string {
	var components []string

	i := 0
	if strings.HasPrefix(pkg, "//") {
		components, pkg = append(components, "//"), pkg[2:]
	}

	for i < len(pkg) {
		switch pkg[i] {
		case '/':
			i++
		default:
			start := i
			for ; i < len(pkg) && pkg[i] != '/'; i++ {
			}
			components = append(components, pkg[start:i])
		}
	}

	return components
}

// Join joins any number of path elements into a single path,
// separating them with slashes. Empty elements are ignored.
// The result is Cleaned. However, if the argument list is
// empty or all its elements are empty, Join returns
// an empty string.
func Join(elem ...string) (string, error) {
	size := 0
	for _, e := range elem {
		size += len(e)
	}
	if size == 0 {
		return "", nil
	}
	buf := make([]byte, 0, size+len(elem)-1)
	for _, e := range elem {
		if len(buf) > 0 || e != "" {
			if len(buf) > 0 {
				buf = append(buf, '/')
			}
			buf = append(buf, e...)
		}
	}
	return Clean(string(buf))
}
