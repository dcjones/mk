
// String substitution and expansion.

package main

import (
    "strings"
    "unicode/utf8"
)


// Expand a word. This includes substituting variables and handling quotes.
func expand(input string, vars map[string][]string, expandBackticks bool) []string {
    parts := make([]string, 0)
    expanded := ""
	var i, j int
	for i = 0; i < len(input); {
		j = i + strings.IndexAny(input[i:], "\"'`$\\")

		if j < 0 {
            expanded += input[i:]
			break
		}

        println("-------------------")
        println(len(input))
        println(i)
        println(j)

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
                out, off = expandBackQuoted(input[i:], vars)
            } else {
                out = input
                off = len(input)
            }
            expanded += out

		case '$':
            var outparts []string
            outparts, off = expandSigil(input[i:], vars)
            if len(outparts) > 0 {
                outparts[0] = expanded + outparts[0]
                expanded = outparts[len(outparts)-1]
                parts = append(parts, outparts[:len(outparts)-1]...)
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
	return string(c), w
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

		_, w := utf8.DecodeRuneInString(input[j:])
		j += w

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
    if c == '{' {
        j := strings.IndexRune(input[w:], '}')
        if j < 0 {
            return []string{"$" + input}, len(input)
        }

        varname = input[w:j]
        offset = j + 1
    } else {
        // try to match a variable name
        i := 0
        j := i
        for j < len(input) {
            c, w = utf8.DecodeRuneInString(input)
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
        if c == '%' {
            expanded = append(expanded, stem...) 
            i += w
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


// TODO: expand RegexpRefs


// Expand a backtick quoted string, by executing the contents.
func expandBackQuoted(input string, vars map[string][]string) (string, int) {
    // TODO: expand sigils?
	j := strings.Index(input, "`")
	if j < 0 {
		return input, len(input)
	}

	output := executeRecipe("sh", nil, input[:j], false, false, true)
	return output, (j + 1)
}


// Split a string on whitespace taking into account escaping and quoting.
//func splitQuoted(input string) []string {
    //parts := make([]string, 0)
    //var i, j int
    //i = 0
    //for {
        //// skip all unescaped whitespace
        //for i < len(input) {
            //c, w := utf8.DecodeRuneInString(input[i:])
            //if strings.IndexRune(" \t", c) < 0 {
                //break
            //}
            //i += w
        //}

        //if i >= len(input) {
            //break
        //}

        //// Ugh. Will this take into account quoting in variables?

        //switch c {
        //case '"':
        //case '\'':
        //default:

        //}
    //}

    //return parts
//}





