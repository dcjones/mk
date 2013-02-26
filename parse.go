
package main

import (
    "fmt"
    "os"
    "os/exec"
)

/* Grammar, to the best of my knowledge:

Should we deviate at all from mk?

Yes! I want to simplify things by saying recipes have nonzero indentation and
everything else has zero.

rule ::= targets ':' attributes ':' prereqs NEWLINE RECIPE |
         targets ':' prereqs NEWLINE RECIPE

targets ::= string | string "," targets

attributes ::= SCALAR | SCALAR attributes

prereqs ::= string | string "," prereqs

include ::= '<' string NEWLINE

string ::= SCALAR | QSTRING

assignment ::= SCALAR '=' string

How do we handle escaping new lines?
Is newline a token that's emitted?

*/


// The parser for mk files is terribly simple. There are only three sorts of
// statements in mkfiles: variable assignments, rules (possibly with
// accompanying recipes), and includes.



//
// Maybe this is the wrong way to organize things.
// We should perhaps have a type for a parsed mkfile that includes every
// assignment as well as every rule.
//
// Rule order should not matter.
//
// Includes are tricky. If they were straight up includes, the could be
// evaluated in place, but they could contain shell script, etc.
//
// No...we still have to evaluate them in place. That means figuring out how to
// spawn shells from go.
//


type parser struct {
    l *lexer         // underlying lexer
    tokenbuf []token // tokens consumed on the current statement
    rules *ruleSet   // current ruleSet
}


// A parser state function takes a parser and the next token and returns a new
// state function, or nil if there was a parse error.
type parserStateFun func (*parser, token) parserStateFun


// Parse a mkfile, returning a new ruleSet.
func parse(input string) *ruleSet {
    rules := &ruleSet{}
    parseInto(input, rules)
    return rules
}


// Parse a mkfile inserting rules and variables into a given ruleSet.
func parseInto(input string, rules *ruleSet) {
    l, tokens := lex(input)
    p := &parser{l, []token{}, rules}
    state := parseTopLevel
    for t := range tokens {
        if t.typ == tokenError {
            // TODO: fancier error messages
            fmt.Fprintf(os.Stderr, "Error: %s", l.errmsg)
            break
        }

        state = state(p, t)
    }

    // TODO: Handle the case when state is not top level.
}


func parseTopLevel(p *parser, t token) parserStateFun {
    switch t.typ {
        case tokenPipeInclude: return parsePipeInclude(p, t)
        // TODO: all others
    }

    return parseTopLevel
}


func parsePipeInclude(p *parser, t token) parserStateFun {
    // TODO: We need to split this up into arguments so we can feed it into
    // executeRecipe.
    return parseTopLevel
}


func parseRedirInclude(p *parser, t token) parserStateFun {
    // TODO: Open the file, read its context, call parseInto recursively.
    return parseTopLevel
}



