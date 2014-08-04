// String substitution and expansion.

package main

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// Expand a word. This includes substituting variables and handling quotes.
func expand(input string, vars map[string][]string, expandBackticks bool) []string {
	parts := make([]string, 0)
	expanded := ""
	var i, j int
	for i = 0; i < len(input); {
		j = strings.IndexAny(input[i:], "\"'`$\\")

		if j < 0 {
			expanded += input[i:]
			break
		}
		j += i

		expanded += input[i:j]
		c, w := utf8.DecodeRuneInString(input[j:])
		i = j + w

		var off int
		var out string
		switch c {
		case '\\':
			out, off = expandEscape(input[i:])
			expanded += out

		case '"':
			out, off = expandDoubleQuoted(input[i:], vars, expandBackticks)
			expanded += out

		case '\'':
			out, off = expandSingleQuoted(input[i:])
			expanded += out

		case '`':
			if expandBackticks {
				var outparts []string
				outparts, off = expandBackQuoted(input[i:], vars)
				if len(outparts) > 0 {
					outparts[0] = expanded + outparts[0]
					expanded = outparts[len(outparts)-1]
					parts = append(parts, outparts[:len(outparts)-1]...)
				}
			} else {
				out = input
				off = len(input)
				expanded += out
			}

		case '$':
			var outparts []string
			outparts, off = expandSigil(input[i:], vars)
			if len(outparts) > 0 {
				firstpart := expanded + outparts[0]
				if len(outparts) > 1 {
					parts = append(parts, firstpart)
					if len(outparts) > 2 {
						parts = append(parts, outparts[1:len(outparts)-1]...)
					}
					expanded = outparts[len(outparts)-1]
				} else {
					expanded = firstpart
				}
			}
		}

		i += off
	}

	if len(expanded) > 0 {
		parts = append(parts, expanded)
	}

	return parts
}

// Expand following a '\\'
func expandEscape(input string) (string, int) {
	c, w := utf8.DecodeRuneInString(input)
	if c == '\t' || c == ' ' {
		return string(c), w
	}
	return "\\" + string(c), w
}

// Expand a double quoted string starting after a '\"'
func expandDoubleQuoted(input string, vars map[string][]string, expandBackticks bool) (string, int) {
	// find the first non-escaped "
	j := 0
	for {
		j = strings.IndexAny(input[j:], "\"\\")
		if j < 0 {
			break
		}

		c, w := utf8.DecodeRuneInString(input[j:])
		j += w

		if c == '"' {
			return strings.Join(expand(input[:j], vars, expandBackticks), " "), (j + w)
		}

		if c == '\\' {
			if j+w < len(input) {
				j += w
				_, w := utf8.DecodeRuneInString(input[j:])
				j += w
			} else {
				break
			}
		}
	}

	return input, len(input)
}

// Expand a single quoted string starting after a '\''
func expandSingleQuoted(input string) (string, int) {
	j := strings.Index(input, "'")
	if j < 0 {
		return input, len(input)
	}
	return input[:j], (j + 1)
}

// Expand something starting with at '$'.
func expandSigil(input string, vars map[string][]string) ([]string, int) {
	c, w := utf8.DecodeRuneInString(input)
	var offset int
	var varname string
	var namelist_pattern = regexp.MustCompile(`^\s*([^:]+)\s*:\s*([^%]*)%([^=]*)\s*=\s*([^%]*)%([^%]*)\s*`)

	// escaping of "$" with "$$"
	if c == '$' {
		return []string{"$"}, 2
		// match bracketed expansions: ${foo}, or ${foo:a%b=c%d}
	} else if c == '{' {
		j := strings.IndexRune(input[w:], '}')
		if j < 0 {
			return []string{"$" + input}, len(input)
		}
		varname = input[w : w+j]
		offset = w + j + 1

		// is this a namelist?
		mat := namelist_pattern.FindStringSubmatch(varname)
		if mat != nil && isValidVarName(mat[1]) {
			// ${varname:a%b=c%d}
			varname = mat[1]
			a, b, c, d := mat[2], mat[3], mat[4], mat[5]
			values, ok := vars[varname]
			if !ok {
				return []string{}, offset
			}

			pat := regexp.MustCompile(strings.Join([]string{`^\Q`, a, `\E(.*)\Q`, b, `\E$`}, ""))
			expanded_values := make([]string, len(values))
			for i, value := range values {
				value_match := pat.FindStringSubmatch(value)
				if value_match != nil {
					expanded_values[i] = strings.Join([]string{c, value_match[1], d}, "")
				} else {
					expanded_values[i] = value
				}
			}

			return expanded_values, offset
		}
		// bare variables: $foo
	} else {
		// try to match a variable name
		i := 0
		j := i
		for j < len(input) {
			c, w = utf8.DecodeRuneInString(input[j:])
			if !(isalpha(c) || c == '_' || (j > i && isdigit(c))) {
				break
			}
			j += w
		}

		if j > i {
			varname = input[i:j]
			offset = j
		} else {
			return []string{"$" + input}, len(input)
		}
	}

	if isValidVarName(varname) {
		varvals, ok := vars[varname]
		if ok {
			return varvals, offset
		} else {
			return []string{"$" + input[:offset]}, offset
		}
	}

	return []string{"$" + input}, len(input)
}

// Find and expand all sigils.
func expandSigils(input string, vars map[string][]string) []string {
	parts := make([]string, 0)
	expanded := ""
	for i := 0; i < len(input); {
		j := strings.IndexRune(input[i:], '$')
		if j < 0 {
			expanded += input[i:]
			break
		}

		ex, k := expandSigil(input[j+1:], vars)
		if len(ex) > 0 {
			ex[0] = expanded + ex[0]
			expanded = ex[len(ex)-1]
			parts = append(parts, ex[:len(ex)-1]...)
		}
		i = k
	}

	if len(expanded) > 0 {
		parts = append(parts, expanded)
	}

	return parts
}

// Find and expand all sigils in a recipe, producing a flat string.
func expandRecipeSigils(input string, vars map[string][]string) string {
	expanded := ""
	for i := 0; i < len(input); {
		off := strings.IndexAny(input[i:], "$\\")
		if off < 0 {
			expanded += input[i:]
			break
		}
		expanded += input[i : i+off]
		i += off

		c, w := utf8.DecodeRuneInString(input[i:])
		if c == '$' {
			i += w
			ex, k := expandSigil(input[i:], vars)
			expanded += strings.Join(ex, " ")
			i += k
		} else if c == '\\' {
			i += w
			c, w := utf8.DecodeRuneInString(input[i:])
			if c == '$' {
				expanded += "$"
			} else {
				expanded += "\\" + string(c)
			}
			i += w
		}
	}

	return expanded
}

// Expand all unescaped '%' characters.
func expandSuffixes(input string, stem string) string {
	expanded := make([]byte, 0)
	for i := 0; i < len(input); {
		j := strings.IndexAny(input[i:], "\\%")
		if j < 0 {
			expanded = append(expanded, input[i:]...)
			break
		}

		c, w := utf8.DecodeRuneInString(input[j:])
		expanded = append(expanded, input[i:j]...)
		if c == '%' {
			expanded = append(expanded, stem...)
			i = j + w
		} else {
			j += w
			c, w := utf8.DecodeRuneInString(input[j:])
			if c == '%' {
				expanded = append(expanded, '%')
				i = j + w
			}
		}
	}

	return string(expanded)
}

// Expand a backtick quoted string, by executing the contents.
func expandBackQuoted(input string, vars map[string][]string) ([]string, int) {
	// TODO: expand sigils?
	j := strings.Index(input, "`")
	if j < 0 {
		return []string{input}, len(input)
	}

	// TODO: handle errors
	output, _ := subprocess("sh", nil, input[:j], true)

	parts := make([]string, 0)
	_, tokens := lexWords(output)
	for t := range tokens {
		parts = append(parts, t.val)
	}

	return parts, (j + 1)
}
