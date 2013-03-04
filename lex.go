// TODO: Backquoted strings.
// TODO: Comments

package main

import (
	"strings"
	"unicode/utf8"
)

type tokenType int

const eof rune = '\000'

const (
	tokenError tokenType = iota
	tokenNewline
	tokenWord
	tokenPipeInclude
	tokenRedirInclude
	tokenColon
	tokenAssign
	tokenRecipe
)

func (typ tokenType) String() string {
	switch typ {
	case tokenError:
		return "[Error]"
	case tokenNewline:
		return "[Newline]"
	case tokenWord:
		return "[Word]"
	case tokenPipeInclude:
		return "[PipeInclude]"
	case tokenRedirInclude:
		return "[RedirInclude]"
	case tokenColon:
		return "[Colon]"
	case tokenAssign:
		return "[Assign]"
	case tokenRecipe:
		return "[Recipe]"
	}
	return "[MysteryToken]"
}

type token struct {
	typ  tokenType // token type
	val  string    // token string
	line int       // line where it was found
	col  int       // column on which the token began
}

func (t *token) String() string {
	if t.typ == tokenError {
		return t.val
	} else if t.typ == tokenNewline {
		return "\\n"
	}

	return t.val
}

type lexer struct {
	input    string     // input string to be lexed
	output   chan token // channel on which tokens are sent
	start    int        // token beginning
	startcol int        // column on which the token begins
	pos      int        // position within input
	line     int        // line within input
	col      int        // column within input
	errmsg   string     // set to an appropriate error message when necessary
	indented bool       // true if the only whitespace so far on this line
}

// A lexerStateFun is simultaneously the the state of the lexer and the next
// action the lexer will perform.
type lexerStateFun func(*lexer) lexerStateFun

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
	l.startcol = l.col
}

func (l *lexer) emit(typ tokenType) {
	l.output <- token{typ, l.input[l.start:l.pos], l.line, l.startcol}
	l.start = l.pos
	l.startcol = 0
}

// Consume the next run if it is in the given string.
func (l *lexer) accept(valid string) bool {
	if strings.IndexRune(valid, l.peek()) >= 0 {
		l.next()
		return true
	}
	return false
}

// Skip the next rune if it is in the valid string. Return true if it was
// skipped.
func (l *lexer) ignore(valid string) bool {
	if strings.IndexRune(valid, l.peek()) >= 0 {
		l.skip()
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
func (l *lexer) skipRun(valid string) int {
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
	l := &lexer{input: input, output: make(chan token), line: 1, col: 0, indented: true}
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

func lexTopLevel(l *lexer) lexerStateFun {
	for {
		l.skipRun(" \t\r")
		// emit a newline token if we are ending a non-empty line.
		if l.peek() == '\n' && !l.indented {
			l.next()
			l.emit(tokenNewline)
		}
		l.skipRun(" \t\r\n")

		if l.peek() == '\\' && l.peekN(1) == '\n' {
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
	case eof:
		return nil
	case '#':
		return lexComment
	case '<':
		return lexInclude
	case ':':
		return lexColon
	case '=':
		return lexAssign
	case '"':
		return lexDoubleQuotedWord
	case '\'':
		return lexSingleQuotedWord
	case '`':
		return lexBackQuotedWord
	}

	// TODO: No! The lexer can get stuck in a loop this way.
	// Check if the next charar is a valid bare string chacter. If not, error.
	return lexBareWord
}

func lexColon(l *lexer) lexerStateFun {
	l.next()
	l.emit(tokenColon)
	return lexTopLevel
}

func lexAssign(l *lexer) lexerStateFun {
	l.next()
	l.emit(tokenAssign)
	return lexTopLevel
}

func lexComment(l *lexer) lexerStateFun {
	l.skip() // '#'
	l.skipUntil("\n")
	return lexTopLevel
}

func lexInclude(l *lexer) lexerStateFun {
	l.next() // '<'
	if l.accept("|") {
		l.emit(tokenPipeInclude)
	} else {
		l.emit(tokenRedirInclude)
	}
	return lexTopLevel
}

func lexDoubleQuotedWord(l *lexer) lexerStateFun {
	l.next() // '"'
	for l.peek() != '"' {
		l.acceptUntil("\\\"")
		if l.accept("\\") {
			l.accept("\"")
		}
	}
	l.next() // '"'
	return lexBareWord
}

func lexBackQuotedWord(l *lexer) lexerStateFun {
	l.next() // '`'
	l.acceptUntil("`")
	l.next() // '`'
	return lexBareWord
}

func lexSingleQuotedWord(l *lexer) lexerStateFun {
	l.next() // '\''
	l.acceptUntil("'")
	l.next() // '\''
	return lexBareWord
}

func lexRecipe(l *lexer) lexerStateFun {
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

func lexBareWord(l *lexer) lexerStateFun {
	l.acceptUntil(" \t\n\r\\=:#'\"")
	if l.peek() == '"' {
		return lexDoubleQuotedWord
	} else if l.peek() == '\'' {
		return lexSingleQuotedWord
	} else if l.peek() == '`' {
		return lexBackQuotedWord
	}

	if l.start < l.pos {
		l.emit(tokenWord)
	}

	return lexTopLevel
}
