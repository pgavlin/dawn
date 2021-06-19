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

	var pattern strings.Builder
	pattern.WriteRune('^')
	for i, g := range globs {
		if i > 0 {
			pattern.WriteRune('|')
		}
		pattern.WriteRune('(')
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
		pattern.WriteRune(')')
	}
	pattern.WriteRune('$')

	return regexp.Compile(pattern.String())
}
