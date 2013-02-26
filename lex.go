
package main

import (
    "fmt"
    "strings"
    "unicode/utf8"
)

type tokenType int

const eof rune = '\000'

const (
    tokenError tokenType = iota
    tokenBareString
    tokenQuotedString
    tokenInclude
    tokenColon
    tokenAssign
    tokenRecipe
)


func (typ tokenType) String() string {
    switch typ {
    case tokenError:        return "[Error]"
    case tokenBareString:   return "[BareString]"
    case tokenQuotedString: return "[QuotedString]"
    case tokenInclude:      return "[Include]"
    case tokenColon:        return "[Colon]"
    case tokenAssign:       return "[Assign]"
    case tokenRecipe:       return "[Recipe]"
    }
    return "[MysteryToken]"
}


type token struct {
    typ tokenType // token type
    val string    // token string
}


func (t *token) String() string {
    if t.typ == tokenError {
        return t.val
    }

    return fmt.Sprintf("%s %q", t.typ, t.val)
}


type lexer struct {
    input    string     // input string to be lexed
    output   chan token // channel on which tokens are sent
    start    int        // token beginning
    pos      int        // position within input
    line     int        // line within input
    col      int        // column within input
    errmsg   string     // set to an appropriate error message when necessary
    indented bool       // true if the only whitespace so far on this line
}


// A lexerStateFun is simultaneously the the state of the lexer and the next
// action the lexer will perform.
type lexerStateFun func (*lexer) lexerStateFun


func (l *lexer) lexerror(what string) {
    l.errmsg = what
    l.emit(tokenError)
}


// Return the nth character without advancing.
func (l *lexer) peekN(n int) (c rune) {
    pos := l.pos
    var width int
    i := 0
    for ; i <= n && pos < len(l.input); i++ {
        c, width = utf8.DecodeRuneInString(l.input[pos:])
        pos += width
    }

    if i <= n {
        return eof
    }

    return
}


// Return the next character without advancing.
func (l *lexer) peek() rune {
    return l.peekN(0)
}


// Consume and return the next character in the lexer input.
func (l *lexer) next() rune {
    if l.pos >= len(l.input) {
        return eof
    }
    c, width := utf8.DecodeRuneInString(l.input[l.pos:])
    l.pos += width

    if c == '\n' {
        l.col = 0
        l.line += 1
        l.indented = true
    } else {
        l.col += 1
        if strings.IndexRune(" \t", c) < 0 {
            l.indented = false
        }
    }

    return c
}


// Skip and return the next character in the lexer input.
func (l *lexer) skip() {
    l.next()
    l.start = l.pos
}


func (l *lexer) emit(typ tokenType) {
    l.output <- token{typ, l.input[l.start:l.pos]}
    l.start = l.pos
}


// Consume the next run if it is in the given string.
func (l *lexer) accept(valid string) bool {
    if strings.IndexRune(valid, l.peek()) >= 0 {
        l.next()
        return true
    }
    return false
}


// Consume characters from the valid string until the next is not.
func (l *lexer) acceptRun(valid string) int {
    prevpos := l.pos
    for strings.IndexRune(valid, l.peek()) >= 0 {
        l.next()
    }
    return l.pos - prevpos
}


// Accept until something from the given string is encountered.
func (l *lexer) acceptUntil(invalid string) {
    for l.pos < len(l.input) && strings.IndexRune(invalid, l.peek()) < 0 {
        l.next()
    }
}


// Skip characters from the valid string until the next is not.
func (l* lexer) skipRun(valid string) int {
    prevpos := l.pos
    for strings.IndexRune(valid, l.peek()) >= 0 {
        l.skip()
    }
    return l.pos - prevpos
}


// Skip until something from the given string is encountered.
func (l *lexer) skipUntil(invalid string) {
    for l.pos < len(l.input) && strings.IndexRune(invalid, l.peek()) < 0 {
        l.skip()
    }
}


// Start a new lexer to lex the given input.
func lex(input string) (*lexer, chan token) {
    l := &lexer{input: input, output: make(chan token)}
    go l.run()
    return l, l.output
}


func (l *lexer) run() {
    for state := lexTopLevel; state != nil; {
        state = state(l)
    }
    close(l.output)
}


// What do we need?
// A function that consumes non-newline whitespace.
// A way of determining if the current line might be a recipe.


func lexTopLevel (l *lexer) lexerStateFun {

    for {
        l.skipRun(" \t\n\r")
        if l.peek() == '\'' && l.peekN(1) == '\n' {
            l.next()
            l.next()
            l.indented = false
        } else {
            break
        }
    }

    if l.indented && l.col > 0 {
        return lexRecipe
    }

    c := l.peek()
    switch c {
    case eof: return nil
    case '#': return lexComment
    case '<': return lexInclude
    case '"': return lexDoubleQuote
    case '\'': return lexSingleQuote
    case ':': return lexColon
    case '=': return lexAssign
    }

    return lexBareString
}


func lexColon (l* lexer) lexerStateFun {
    l.next()
    l.emit(tokenColon)
    return lexTopLevel
}


func lexAssign (l* lexer) lexerStateFun {
    l.next()
    l.emit(tokenAssign)
    return lexTopLevel
}


func lexComment (l* lexer) lexerStateFun {
    l.skip() // '#'
    l.skipUntil("\n")
    return lexTopLevel
}


func lexInclude (l* lexer) lexerStateFun {
    l.skip() // '<'
    l.skipRun(" \t\n\r")
    l.acceptUntil("\n\r")
    l.emit(tokenInclude)
    return lexTopLevel
}


func lexDoubleQuote (l *lexer) lexerStateFun {
    l.skip() // '"'
    for l.peek() != '"' {
        l.acceptUntil("\\\"")
        if l.accept("\\") {
            l.accept("\"")
        }
    }
    l.emit(tokenQuotedString)
    l.skip() // skip '"'
    return lexTopLevel
}


func lexSingleQuote (l *lexer) lexerStateFun {
    l.skip() // '\''
    l.acceptUntil("'")
    l.emit(tokenQuotedString)
    l.skip() // '\''
    return lexTopLevel
}


func lexRecipe (l *lexer) lexerStateFun {

    for {
        l.acceptUntil("\n")
        l.acceptRun(" \t\n\r")
        if !l.indented || l.col == 0 {
            break
        }
    }

    // TODO: don't emit if there is only whitespace in the recipe
    l.emit(tokenRecipe)
    return lexTopLevel
}


func lexBareString (l *lexer) lexerStateFun {
    // TODO: allow escaping spaces and tabs?
    // TODO: allow adjacent quoted string, e.g.: foo"bar"baz?
    l.acceptUntil(" \t\n\r\\=:#'\"")
    l.emit(tokenBareString)
    return lexTopLevel
}


