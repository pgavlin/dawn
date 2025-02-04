package util

import (
	"errors"
	"regexp"
	"strings"
)

func CompileGlobs(globs []string) (*regexp.Regexp, error) {
	// *  -> [^[:sep:]]*
	// ** -> .*
	// ?  -> .
	// \* -> \*
	// \\ -> \\
	// \? -> \?
	// \[ -> \?
	// \] -> \?

	if len(globs) == 0 {
		return regexp.Compile("^$")
	}

	var pattern strings.Builder
	for i, g := range globs {
		if i > 0 {
			pattern.WriteRune('|')
		}
		pattern.WriteString("(^")
		for i := 0; i < len(g); {
			switch b := g[i]; b {
			case '\\':
				if i == len(g)-1 {
					return nil, errors.New("invalid escape sequence")
				}

				switch c := g[i+1]; c {
				case '\\', '*', '?', '[', ']':
					pattern.WriteByte(b)
					pattern.WriteByte(c)
					i++
				default:
					return nil, errors.New("invalid escape sequence")
				}

			case '*':
				if i < len(g)-1 && g[i+1] == '*' {
					pattern.WriteString(".*")
					i++
				} else {
					pattern.WriteString("[^/]*")
				}
			case '?':
				pattern.WriteByte('.')
			case '.', '+', '(', ')', '|', '{', '}', '^', '$':
				pattern.WriteByte('\\')
				pattern.WriteByte(b)
			default:
				pattern.WriteByte(b)
			}
			i++
		}
		pattern.WriteString("$)")
	}

	return regexp.Compile(pattern.String())
}
