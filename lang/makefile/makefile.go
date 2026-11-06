// Package makefile registers Makefile (make, BSD make, and justfiles).
//
//	import _ "github.com/toshi-kuji/fasthighlight/lang/makefile"
//
// Token types and boundaries reproduce the rules of Chroma 2.27.0's makefile.xml without regular
// expressions. Indented lines (recipes) and the values of variable assignments are read as Bash
// (internal/bash).
package makefile

import (
	"strings"
	"unicode/utf8"

	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/bash"
	"github.com/toshi-kuji/fasthighlight/internal/lexkit"
)

// Language is Makefile. Its name, aliases, and filename patterns are listed in the catalog of the main
// package (fasthighlight.Builtin).
var Language = fh.Builtin("Makefile")

func init() {
	Language.Lex = lex
	fh.Register(Language)
}

// The states of makefile.xml in Chroma.
type state int

const (
	root        state = iota
	expansion         // $( … )
	export            // the names after export
	blockHeader       // the prerequisites after target:
)

type lexer struct {
	x     *lexkit.Tokens
	src   string
	stack []state
	// The end of the run of [\w${}().-] (for the assignment rule) and of [^\n:] (for the target rule) found
	// last, with the position they were looked up from. A position inside a run has the same end, so
	// the characters of a long run that turn out to be errors are not scanned again for each position.
	nameFrom, nameEnd     int
	targetFrom, targetEnd int
	exportFrom, exportEnd int // the same for the run of [\w${}\t -] (for the lookahead after export)
	// unclosed records, for " and ', that a quote with no closing quote was found.
	unclosed [2]bool
	// lineEnd is the end of the line that holds the last position lineEndAt was asked about. Positions only
	// move forward, so every later position up to it is on the same line, and a long line is scanned once,
	// not once per target or comment on it.
	lineEnd int
}

func lex(src string, end int) []fh.Token {
	l := &lexer{x: lexkit.NewTokens(src), src: src, stack: []state{root}, nameFrom: -1, targetFrom: -1, exportFrom: -1,
		lineEnd: -1}
	for i := 0; i < end; {
		st := l.stack[len(l.stack)-1]
		var n int
		switch st {
		case root:
			n = l.root(i)
		case expansion:
			n = l.expansion(i)
		case export:
			n = l.export(i)
		case blockHeader:
			n = l.blockHeader(i)
		}
		if n == 0 {
			// No rule matches. In a state other than root, Chroma returns to root on reaching a newline.
			if src[i] == '\n' && st != root {
				l.stack = l.stack[:1]
				continue
			}
			n = l.x.Error(i)
		}
		i += n
	}
	return l.x.Result()
}

// lineEndAt returns lexkit.LineEnd(l.src, i), reusing the end found for an earlier position on the same line.
func (l *lexer) lineEndAt(i int) int {
	if i > l.lineEnd {
		l.lineEnd = lexkit.LineEnd(l.src, i)
	}
	return l.lineEnd
}

func (l *lexer) emit(t fh.TokenType, s, e int) int { l.x.Emit(t, s, e); return e - s }
func (l *lexer) push(s state)                      { l.stack = append(l.stack, s) }
func (l *lexer) pop() {
	if len(l.stack) > 1 {
		l.stack = l.stack[:len(l.stack)-1]
	}
}

// bash reads src[s:e] as Bash (using Bash).
func (l *lexer) bash(s, e int) {
	bash.LexInto(l.x.Sub(l.src[s:e], s), l.src[s:e], e-s)
}

// root tries the rules of the root state in order.
func (l *lexer) root(i int) int {
	src := l.src
	c := src[i]
	// ^(?:[\t ]+.*\n|\n)+
	if i == 0 || src[i-1] == '\n' {
		e := i
		for e < len(src) {
			if src[e] == '\n' {
				e++
				continue
			}
			if src[e] != '\t' && src[e] != ' ' {
				break
			}
			le := lexkit.LineEnd(src, e)
			if le >= len(src) {
				break
			}
			e = le + 1
		}
		if e > i {
			l.bash(i, e)
			return e - i
		}
	}
	// \$[<@$+%?|*]
	if c == '$' && i+1 < len(src) && strings.IndexByte("<@$+%?|*", src[i+1]) >= 0 {
		return l.emit(fh.Keyword, i, i+2)
	}
	if n := lexkit.Span(src, i, lexkit.IsSpace); n > 0 { // \s+
		return l.emit(fh.Text, i, i+n)
	}
	if c == '#' { // #.*?\n
		if le := l.lineEndAt(i); le < len(src) {
			return l.emit(fh.Comment, i, le+1)
		}
	}
	if strings.HasPrefix(src[i:], "export") {
		k := i + len("export")
		if n := lexkit.Span(src, k, lexkit.IsSpace); n > 0 {
			// (export)(\s+)(?=[\w${}\t -]+\n)
			if s := l.exportSpace(k, k+n); s > 0 {
				l.emit(fh.Keyword, i, k)
				l.emit(fh.Text, k, s)
				l.push(export)
				return s - i
			}
			// export\s+
			return l.emit(fh.Keyword, i, k+n)
		}
	}
	if n := l.assignment(i); n > 0 {
		return n
	}
	// (?s)"(\\\\|\\.|[^"\\])*" and (?s)'(\\\\|\\.|[^'\\])*'
	if c == '"' || c == '\'' {
		k := 0
		if c == '\'' {
			k = 1
		}
		// Once a quote has no closing quote, no later quote of the same kind has one either (a later quote
		// is escaped in the failed scan, so the scan from it joins the failed one right after it).
		if !l.unclosed[k] {
			if e := quotedEnd(src, i); e > 0 {
				if c == '"' {
					return l.emit(fh.LiteralStringDouble, i, e)
				}
				return l.emit(fh.LiteralStringSingle, i, e)
			}
			l.unclosed[k] = true
		}
	}
	// ([^\n:]+)(:+)([ \t]*)
	if l.targetFrom < 0 || i < l.targetFrom || i >= l.targetEnd {
		l.targetFrom, l.targetEnd = i, l.lineEndAt(i)
		if k := strings.IndexByte(src[i:l.targetEnd], ':'); k >= 0 {
			l.targetEnd = i + k
		}
	}
	if e := l.targetEnd; e > i && e < len(src) && src[e] == ':' {
		o := e
		for o < len(src) && src[o] == ':' {
			o++
		}
		s := o + spaceTab(src, o)
		l.emit(fh.NameFunction, i, e)
		l.emit(fh.Operator, e, o)
		l.emit(fh.Text, o, s)
		l.push(blockHeader)
		return s - i
	}
	if strings.HasPrefix(src[i:], "$(") { // \$\(
		l.push(expansion)
		return l.emit(fh.Keyword, i, i+2)
	}
	return 0
}

// exportSpace returns the end of \s+ (between k and the greedy end g) after which the lookahead
// (?=[\w${}\t -]+\n) holds, trying the longest first, or 0 if there is none.
func (l *lexer) exportSpace(k, g int) int {
	src := l.src
	// stop is the first position at or after p (p being the candidate end) that is not in [\w${}\t -]:
	// the lookahead holds when it is past p and is a newline.
	if l.exportFrom < 0 || g < l.exportFrom || g >= l.exportEnd {
		l.exportFrom, l.exportEnd = g, g+lexkit.Span(src, g, isExportChar)
	}
	stop := l.exportEnd
	for p := g; p > k; {
		if p < g {
			if r, _ := lexkit.Rune(src, p); !isExportChar(r) {
				stop = p
			}
		}
		if stop > p && stop < len(src) && src[stop] == '\n' {
			return p
		}
		_, n := utf8.DecodeLastRuneInString(src[k:p])
		p -= n
	}
	return 0
}

func isExportChar(r rune) bool {
	return lexkit.IsWord(r) || r == '$' || r == '{' || r == '}' || r == '\t' || r == ' ' || r == '-'
}

func isNameChar(r rune) bool {
	return lexkit.IsWord(r) || strings.ContainsRune("${}().-", r)
}

func spaceTab(src string, i int) int {
	j := i
	for j < len(src) && (src[j] == ' ' || src[j] == '\t') {
		j++
	}
	return j - i
}

// assignment tries ([\w${}().-]+)(\s*)([!?:+]?=)([ \t]*)((?:.*\\\n)+|.*\n). None of its parts gains from
// backtracking: the characters after a shorter part never fit what follows.
func (l *lexer) assignment(i int) int {
	src := l.src
	if l.nameFrom < 0 || i < l.nameFrom || i >= l.nameEnd {
		l.nameFrom, l.nameEnd = i, i+lexkit.Span(src, i, isNameChar)
	}
	w := l.nameEnd
	if w == i {
		return 0
	}
	s := w + lexkit.Span(src, w, lexkit.IsSpace)
	o := s
	if o < len(src) && strings.IndexByte("!?:+", src[o]) >= 0 && o+1 < len(src) && src[o+1] == '=' {
		o += 2
	} else if o < len(src) && src[o] == '=' {
		o++
	} else {
		return 0
	}
	v := o + spaceTab(src, o)
	// (?:.*\\\n)+ takes the lines that end with a backslash (not the line after them); otherwise .*\n.
	e := v
	for {
		le := lexkit.LineEnd(src, e)
		if le >= len(src) || le == e || src[le-1] != '\\' {
			break
		}
		e = le + 1
	}
	if e == v {
		le := lexkit.LineEnd(src, v)
		if le >= len(src) {
			return 0
		}
		e = le + 1
	}
	l.emit(fh.NameVariable, i, w)
	l.emit(fh.Text, w, s)
	l.emit(fh.Operator, s, o)
	l.emit(fh.Text, o, v)
	l.bash(v, e)
	return e - i
}

// quotedEnd returns the end of q(\\\\|\\.|[^q\\])*q (dot_all; q is src[i]), or 0 if it does not match. A
// backslash always takes the next character, so the match ends at the first quote without one.
func quotedEnd(src string, i int) int {
	q := src[i]
	for j := i + 1; j < len(src); {
		switch src[j] {
		case q:
			return j + 1
		case '\\':
			if j+1 >= len(src) {
				return 0
			}
			_, n := lexkit.Rune(src, j+1)
			j += 1 + n
		default:
			j++
		}
	}
	return 0
}

// expansion tries the rules of the expansion state. Every character matches one of them.
func (l *lexer) expansion(i int) int {
	src := l.src
	switch c := src[i]; {
	case c == '$':
		return l.emit(fh.Keyword, i, i+1)
	case c == '(':
		l.push(expansion)
		return l.emit(fh.Keyword, i, i+1)
	case c == ')':
		l.pop()
		return l.emit(fh.Keyword, i, i+1)
	case isLetter(c):
		e := i
		for e < len(src) && isLetter(src[e]) {
			e++
		}
		return l.emit(fh.NameVariable, i, e)
	}
	e := i
	for e < len(src) && !isLetter(src[e]) && strings.IndexByte("$()", src[e]) < 0 {
		e++
	}
	return l.emit(fh.Text, i, e)
}

func isLetter(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_' }

// export tries the rules of the export state.
func (l *lexer) export(i int) int {
	src := l.src
	if n := lexkit.Span(src, i, func(r rune) bool {
		return lexkit.IsWord(r) || r == '$' || r == '{' || r == '}' || r == '-'
	}); n > 0 {
		return l.emit(fh.NameVariable, i, i+n)
	}
	if src[i] == '\n' {
		l.pop()
		return l.emit(fh.Text, i, i+1)
	}
	if n := lexkit.Span(src, i, lexkit.IsSpace); n > 0 {
		return l.emit(fh.Text, i, i+n)
	}
	return 0
}

// blockHeader tries the rules of the block-header state. Every character matches one of them.
func (l *lexer) blockHeader(i int) int {
	src := l.src
	switch c := src[i]; {
	case c == ',' || c == '|':
		return l.emit(fh.Punctuation, i, i+1)
	case c == '#': // #.*?\n
		if le := lexkit.LineEnd(src, i); le < len(src) {
			l.pop()
			return l.emit(fh.Comment, i, le+1)
		}
	case c == '\\' && i+1 < len(src) && src[i+1] == '\n':
		return l.emit(fh.Text, i, i+2)
	case c == '$' && i+1 < len(src) && src[i+1] == '(':
		l.push(expansion)
		return l.emit(fh.Keyword, i, i+2)
	case isLetter(c):
		e := i
		for e < len(src) && isLetter(src[e]) {
			e++
		}
		return l.emit(fh.Name, i, e)
	case c == '\n':
		l.pop()
		return l.emit(fh.Text, i, i+1)
	}
	_, n := lexkit.Rune(src, i) // .
	return l.emit(fh.Text, i, i+n)
}
