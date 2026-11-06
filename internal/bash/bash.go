// Package bash is the Bash tokenizer. It is used by lang/bash (Bash) and by the languages that embed Bash
// (Bash Session, Docker, Makefile).
//
// Token types and boundaries reproduce the rules of Chroma 2.27.0's bash.xml without regular expressions.
package bash

import (
	"strings"

	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/lexkit"
)

// The states of bash.xml in Chroma.
type state int

const (
	root      state = iota
	str             // inside "…"
	paren           // $( … )
	math            // $(( … ))
	backticks       // ` … `
	curly           // ${ … }
)

// keywords holds the candidates of the first rule of basic, in the same order as the regular
// expression.
var keywords = [...]string{
	"if", "fi", "else", "while", "do", "done", "for", "then", "return", "function", "case",
	"select", "continue", "until", "esac", "elif",
}

// builtins holds the candidates of the second rule of basic, in the same order as the regular
// expression.
var builtins = [...]string{
	"alias", "bg", "bind", "break", "builtin", "caller", "cd", "command", "compgen", "complete",
	"declare", "dirs", "disown", "echo", "enable", "eval", "exec", "exit", "export", "false", "fc",
	"fg", "getopts", "hash", "help", "history", "jobs", "kill", "let", "local", "logout", "popd",
	"printf", "pushd", "pwd", "read", "readonly", "set", "shift", "shopt", "source", "suspend",
	"test", "time", "times", "trap", "true", "type", "typeset", "ulimit", "umask", "unalias",
	"unset", "wait",
}

// lexer is the state of one lexing run.
type lexer struct {
	x     *lexkit.Tokens
	src   string
	stack []state
}

// Lex splits src into tokens (fasthighlight.Language.Lex).
func Lex(src string, end int) []fh.Token {
	x := lexkit.NewTokens(src)
	LexInto(x, src, end)
	return x.Result()
}

// LexInto splits src into tokens and adds them to x. x's src is either the same as src or was made with
// x.Sub for src (Bash embedded in another language, as with Chroma's using: src is the whole embedded text,
// so \A and the lookaheads see only that text).
func LexInto(x *lexkit.Tokens, src string, end int) {
	l := &lexer{x: x, src: src, stack: []state{root}}
	for i := 0; i < end && len(l.stack) > 0; {
		st := l.stack[len(l.stack)-1]
		n := l.step(st, i)
		if n == 0 {
			// No rule matches. In a state other than root, Chroma returns to root on reaching a newline.
			if src[i] == '\n' && st != root {
				l.stack = l.stack[:1]
				l.stack[0] = root
				continue
			}
			n = l.x.Error(i)
		}
		i += n
	}
}

func (l *lexer) emit(t fh.TokenType, s, e int) int { l.x.Emit(t, s, e); return e - s }
func (l *lexer) push(s state)                      { l.stack = append(l.stack, s) }
func (l *lexer) pop()                              { l.stack = l.stack[:len(l.stack)-1] }

// step tries the rules of state st in order, appends the token of the matching rule, and returns the
// number of bytes advanced. It returns 0 if none matches.
func (l *lexer) step(st state, i int) int {
	src := l.src
	c := src[i]
	switch st {
	case str:
		if c == '"' {
			l.pop()
			return l.emit(fh.LiteralStringDouble, i, i+1)
		}
		// (?s)(\\\\|\\[0-7]+|\\.|[^"\\$])+
		if e := escapedRun(src, i, "\"$"); e > i {
			return l.emit(fh.LiteralStringDouble, i, e)
		}
		return l.interp(i)
	case paren:
		if c == ')' {
			l.pop()
			return l.emit(fh.Keyword, i, i+1)
		}
	case math:
		if strings.HasPrefix(src[i:], "))") {
			l.pop()
			return l.emit(fh.Keyword, i, i+2)
		}
		if strings.IndexByte("-+*/%^|&", c) >= 0 { // [-+*/%^|&]|\*\*|\|\| (the one-character alternative matches first)
			return l.emit(fh.Operator, i, i+1)
		}
		if d := digitsEnd(src, i); d > i { // \d+#\d+, \d+#(?! ), \d+
			if d < len(src) && src[d] == '#' {
				if e := digitsEnd(src, d+1); e > d+1 {
					return l.emit(fh.LiteralNumber, i, e)
				}
				if d+1 >= len(src) || src[d+1] != ' ' {
					return l.emit(fh.LiteralNumber, i, d+1)
				}
			}
			return l.emit(fh.LiteralNumber, i, d)
		}
	case backticks:
		if c == '`' {
			l.pop()
			return l.emit(fh.LiteralStringBacktick, i, i+1)
		}
	case curly:
		switch {
		case c == '}':
			l.pop()
			return l.emit(fh.LiteralStringInterpol, i, i+1)
		case strings.HasPrefix(src[i:], ":-"):
			return l.emit(fh.Keyword, i, i+2)
		}
		if n := lexkit.Span(src, i, lexkit.IsWord); n > 0 { // \w+
			return l.emit(fh.NameVariable, i, i+n)
		}
		if n := lexkit.Span(src, i, notCurlySpecial); n > 0 { // [^}:"\'`$\\]+
			return l.emit(fh.Punctuation, i, i+n)
		}
		if c == ':' {
			return l.emit(fh.Punctuation, i, i+1)
		}
	}
	// root (paren, math, backticks, and curly also include root at the end)
	return l.root(i)
}

func notCurlySpecial(r rune) bool { return !strings.ContainsRune("}:\"'`$\\", r) }

// root tries the rules of the root state (basic, `, data, interp) in this order.
func (l *lexer) root(i int) int {
	if n := l.basic(i); n > 0 {
		return n
	}
	if l.src[i] == '`' {
		l.push(backticks)
		return l.emit(fh.LiteralStringBacktick, i, i+1)
	}
	if n := l.data(i); n > 0 {
		return n
	}
	return l.interp(i)
}

// basic tries the rules of the basic state.
func (l *lexer) basic(i int) int {
	src := l.src
	c := src[i]
	if isASCIILetter(c) && lexkit.WordBoundary(src, i) {
		// \b(if|fi|…|elif)(\s*)\b
		for _, w := range keywords {
			if !strings.HasPrefix(src[i:], w) {
				continue
			}
			e := -1
			lexkit.Run(src, i+len(w), 0, lexkit.IsSpace, func(k int) bool {
				if lexkit.WordBoundary(src, k) {
					e = k
					return true
				}
				return false
			})
			if e >= 0 {
				l.emit(fh.Keyword, i, i+len(w))
				l.emit(fh.Text, i+len(w), e)
				return e - i
			}
		}
		// \b(alias|…|wait)(?=[\s)`])
		for _, w := range builtins {
			if strings.HasPrefix(src[i:], w) && i+len(w) < len(src) {
				if r, _ := lexkit.Rune(src, i+len(w)); lexkit.IsSpace(r) || r == ')' || r == '`' {
					return l.emit(fh.NameBuiltin, i, i+len(w))
				}
			}
		}
	}
	if c == '#' {
		// \A#!.+\n
		if i == 0 && strings.HasPrefix(src, "#!") {
			if le := lexkit.LineEnd(src, 2); le > 2 && le < len(src) {
				return l.emit(fh.CommentPreproc, 0, le+1)
			}
		}
		// #.*(\S|$)
		return l.emit(fh.CommentSingle, i, lexkit.LineEnd(src, i))
	}
	// \\[\w\W]
	if c == '\\' && i+1 < len(src) {
		_, n := lexkit.Rune(src, i+1)
		return l.emit(fh.LiteralStringEscape, i, i+1+n)
	}
	// (\b\w+)(\s*)(\+?=)
	if r, _ := lexkit.Rune(src, i); lexkit.IsWord(r) && lexkit.WordBoundary(src, i) {
		w := i + lexkit.Span(src, i, lexkit.IsWord)
		s := w + lexkit.Span(src, w, lexkit.IsSpace)
		op := 0
		switch {
		case strings.HasPrefix(src[s:], "+="):
			op = 2
		case strings.HasPrefix(src[s:], "="):
			op = 1
		}
		if op > 0 {
			l.emit(fh.NameVariable, i, w)
			l.emit(fh.Text, w, s)
			l.emit(fh.Operator, s, s+op)
			return s + op - i
		}
	}
	// [\[\]{}()=]
	if strings.IndexByte("[]{}()=", c) >= 0 {
		return l.emit(fh.Operator, i, i+1)
	}
	if strings.HasPrefix(src[i:], "<<<") {
		return l.emit(fh.Operator, i, i+3)
	}
	if e := heredocEnd(src, i); e > 0 {
		return l.emit(fh.LiteralString, i, e)
	}
	if strings.HasPrefix(src[i:], "&&") || strings.HasPrefix(src[i:], "||") {
		return l.emit(fh.Operator, i, i+2)
	}
	return 0
}

// heredocEnd returns the end of the part matching <<-?\s*(\'?)\\?(\w+)[\w\W]+?\2. It returns 0 if
// there is no match. The terminator (\2) matches the first identical string that appears later, even
// if it is not at the start of a line. If none is found, backtracking in the regular expression
// shortens the terminator and searches again.
func heredocEnd(src string, i int) int {
	if !strings.HasPrefix(src[i:], "<<") {
		return 0
	}
	j := i + 2
	if j < len(src) && src[j] == '-' {
		j++
	}
	j += lexkit.Span(src, j, lexkit.IsSpace)
	if j < len(src) && src[j] == '\'' {
		j++
	}
	if j < len(src) && src[j] == '\\' {
		j++
	}
	e := -1
	lexkit.Run(src, j, 1, lexkit.IsWord, func(q int) bool {
		if q >= len(src) {
			return false
		}
		_, n := lexkit.Rune(src, q) // first character of [\w\W]+?
		if k := strings.Index(src[q+n:], src[j:q]); k >= 0 {
			e = q + n + k + (q - j)
			return true
		}
		return false
	})
	if e < 0 {
		return 0
	}
	return e
}

// data tries the rules of the data state.
func (l *lexer) data(i int) int {
	src := l.src
	c := src[i]
	// (?s)\$?"(\\\\|\\[0-7]+|\\.|[^"\\$])*"
	if q := i; c == '"' || (c == '$' && i+1 < len(src) && src[i+1] == '"') {
		if c == '$' {
			q++
		}
		if e := escapedRun(src, q+1, "\"$"); e < len(src) && src[e] == '"' {
			return l.emit(fh.LiteralStringDouble, i, e+1)
		}
	}
	if c == '"' {
		l.push(str)
		return l.emit(fh.LiteralStringDouble, i, i+1)
	}
	// (?s)\$'(\\\\|\\[0-7]+|\\.|[^'\\])*'
	if c == '$' && i+1 < len(src) && src[i+1] == '\'' {
		if e := escapedRun(src, i+2, "'"); e < len(src) && src[e] == '\'' {
			return l.emit(fh.LiteralStringSingle, i, e+1)
		}
	}
	// (?s)'.*?'
	if c == '\'' {
		if k := strings.IndexByte(src[i+1:], '\''); k >= 0 {
			return l.emit(fh.LiteralStringSingle, i, i+1+k+1)
		}
	}
	switch c {
	case ';', '&', '|':
		return l.emit(fh.Punctuation, i, i+1)
	}
	if n := lexkit.Span(src, i, lexkit.IsSpace); n > 0 {
		return l.emit(fh.Text, i, i+n)
	}
	// \d+(?= |$)
	if d := digitsEnd(src, i); d > i && (d == len(src) || src[d] == ' ' || src[d] == '\n') {
		return l.emit(fh.LiteralNumber, i, d)
	}
	// [^=\s\[\]{}()$"\'`\\<&|;]+
	if n := lexkit.Span(src, i, isPlain); n > 0 {
		return l.emit(fh.Text, i, i+n)
	}
	if c == '<' {
		return l.emit(fh.Text, i, i+1)
	}
	return 0
}

func isPlain(r rune) bool {
	return !lexkit.IsSpace(r) && !strings.ContainsRune("=[]{}()$\"'`\\<&|;", r)
}

// interp tries the rules of the interp state.
func (l *lexer) interp(i int) int {
	src := l.src
	if src[i] != '$' {
		return 0
	}
	rest := src[i+1:]
	switch {
	case strings.HasPrefix(rest, "(("):
		l.push(math)
		return l.emit(fh.Keyword, i, i+3)
	case strings.HasPrefix(rest, "("):
		l.push(paren)
		return l.emit(fh.Keyword, i, i+2)
	case strings.HasPrefix(rest, "{#"):
		l.push(curly)
		return l.emit(fh.LiteralStringInterpol, i, i+3)
	case strings.HasPrefix(rest, "{"):
		l.push(curly)
		return l.emit(fh.LiteralStringInterpol, i, i+2)
	}
	// \$[a-zA-Z_]\w*
	if rest != "" && (isASCIILetter(rest[0]) || rest[0] == '_') {
		return l.emit(fh.NameVariable, i, i+2+lexkit.Span(src, i+2, lexkit.IsWord))
	}
	// \$(?:\d+|[#$?!_*@-])
	if d := digitsEnd(src, i+1); d > i+1 {
		return l.emit(fh.NameVariable, i, d)
	}
	if rest != "" && strings.IndexByte("#$?!_*@-", rest[0]) >= 0 {
		return l.emit(fh.NameVariable, i, i+2)
	}
	return l.emit(fh.Text, i, i+1)
}

// escapedRun returns the end of the part matching (\\\\|\\[0-7]+|\\.|[^…\\])* from i (. includes a
// newline). stop holds the characters excluded by […]. A backslash is read together with the next
// character (if there is no next character, it stops there).
func escapedRun(src string, i int, stop string) int {
	for i < len(src) {
		c := src[i]
		if c == '\\' {
			if i+1 >= len(src) {
				return i
			}
			_, n := lexkit.Rune(src, i+1)
			i += 1 + n
			continue
		}
		if strings.IndexByte(stop, c) >= 0 {
			return i
		}
		_, n := lexkit.Rune(src, i)
		i += n
	}
	return i
}

// digitsEnd returns the end of the run of \d (Unicode digits) from i.
func digitsEnd(src string, i int) int { return i + lexkit.Span(src, i, lexkit.IsDigit) }

func isASCIILetter(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }
