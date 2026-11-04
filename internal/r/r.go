// Package r is the R tokenizer. lang/r registers it. Options (the switches for the intended
// differences from Chroma) exist only for the comparison in compat/ and are not exposed.
//
// The token types and boundaries reproduce the rules of Chroma 2.27.0's r.xml without regular expressions.
// Intended difference from Chroma: the name-start rule [a-zA-z] also includes the symbols between Z and a
// ([ \ ] ^ _ `), which is a typo, so it becomes [a-zA-Z]. Setting Options.ChromaNames to true makes it
// the same as Chroma.
package r

import (
	"strings"

	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/lexkit"
)

var lists *lexkit.WordLists

// constants are the constants of builtin_symbols made of word characters only (KeywordConstant).
const constants = "NULL|NA|NA_integer_|NA_real_|NA_complex_|NA_character_|letters|LETTERS|Inf|TRUE|FALSE|NaN|pi"

// Prepare builds the word tables and the like (the Prepare of fasthighlight.Language).
func Prepare() { lists = lexkit.NewWordLists(keywords, constants) }

type state uint8

const (
	root state = iota
	squote
	dquote
)

// Options holds the switches for the intended differences from Chroma. The zero value is the production
// setting (the one lang/r registers).
type Options struct {
	// ChromaNames, if true, makes the start of a name the same as Chroma's [a-zA-z].
	ChromaNames bool
}

type lexer struct {
	o     Options
	x     *lexkit.Tokens
	src   string
	stack []state
	words lexkit.WordCache
}

// Lex splits src into tokens (with the same contract as the Lex of fasthighlight.Language). compat/ uses it
// to compare with Chroma, switching o.
func Lex(src string, end int, o Options) []fh.Token {
	l := &lexer{o: o, x: lexkit.NewTokens(src), src: src, stack: []state{root}, words: lexkit.WordCache{Lists: lists}}
	for i := 0; i < end; {
		st := l.stack[len(l.stack)-1]
		n, ok := l.step(st, i)
		if !ok {
			if src[i] == '\n' && st != root {
				l.stack = append(l.stack[:0], root)
				continue
			}
			n = l.x.Error(i)
		}
		i += n
	}
	return l.x.Result()
}

func (l *lexer) emit(t fh.TokenType, s, e int) (int, bool) { l.x.Emit(t, s, e); return e - s, true }

func (l *lexer) at(i int) byte {
	if i < len(l.src) {
		return l.src[i]
	}
	return 0
}

func isDec(c byte) bool { return '0' <= c && c <= '9' }

// isNameChar is [\w_.].
func isNameChar(r rune) bool { return r == '.' || lexkit.IsWord(r) }

// backtick returns the length of `[^`\\]*(?:\\.[^`\\]*)*` (0 if it does not match).
func (l *lexer) backtick(i int) int {
	src := l.src
	if l.at(i) != '`' {
		return 0
	}
	for j := i + 1; j < len(src); {
		switch src[j] {
		case '`':
			return j + 1 - i
		case '\\':
			if j+1 >= len(src) || src[j+1] == '\n' {
				return 0
			}
			_, n := lexkit.Rune(src, j+1)
			j += 1 + n
			continue
		}
		j++
	}
	return 0
}

// names passes the end of each alternative of the valid_name rule
// (?:`…`)|(?:(?:[a-zA-z]|[_.][^0-9])[\w_.]*) to f, in the order the regular expression tries them.
// If f returns true, it returns that end (0 if none matches).
func (l *lexer) names(i int, f func(e int) bool) int {
	src := l.src
	if n := l.backtick(i); n > 0 && f(i+n) {
		return i + n
	}
	c := src[i]
	if 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || l.o.ChromaNames && 'A' <= c && c <= 'z' { // [a-zA-Z] (Chroma has [a-zA-z])
		if e := i + 1 + lexkit.Span(src, i+1, isNameChar); f(e) {
			return e
		}
	}
	if (c == '_' || c == '.') && i+1 < len(src) && !isDec(src[i+1]) { // [_.][^0-9]
		_, n := lexkit.Rune(src, i+1)
		if e := i + 1 + n + lexkit.Span(src, i+1+n, isNameChar); f(e) {
			return e
		}
	}
	return 0
}

// quoted returns the length of ([^q\\]|\\.)*q (0 if it does not match).
func (l *lexer) quoted(i int, q byte) int {
	src := l.src
	if strings.IndexByte(src[i:], q) < 0 {
		return 0
	}
	for j := i; j < len(src); {
		switch src[j] {
		case q:
			return j + 1 - i
		case '\\':
			if j+1 >= len(src) || src[j+1] == '\n' {
				return 0
			}
			_, n := lexkit.Rune(src, j+1)
			j += 1 + n
			continue
		}
		j++
	}
	return 0
}

func (l *lexer) step(st state, i int) (int, bool) {
	src := l.src
	switch st {
	case squote, dquote:
		q := byte('\'')
		if st == dquote {
			q = '"'
		}
		if n := l.quoted(i, q); n > 0 {
			l.stack = l.stack[:len(l.stack)-1]
			return l.emit(fh.LiteralString, i, i+n)
		}
		return 0, false
	}
	c := src[i]
	// keywords: (if|else|…)(?![\w.])
	if n := l.words.Match(src, i, 0); n > 0 && l.at(i+n) != '.' {
		return l.emit(fh.KeywordReserved, i, i+n)
	}
	// name(\s*)(?=\()
	if e := l.names(i, func(e int) bool { return l.at(e+lexkit.Span(src, e, lexkit.IsSpace)) == '(' }); e > 0 {
		e += lexkit.Span(src, e, lexkit.IsSpace)
		return l.emit(fh.NameFunction, i, e)
	}
	// statements
	if c == '#' {
		return l.emit(fh.CommentSingle, i, lexkit.LineEnd(src, i))
	}
	if n := lexkit.Span(src, i, lexkit.IsSpace); n > 0 {
		return l.emit(fh.Text, i, i+n)
	}
	if c == '\'' {
		l.stack = append(l.stack, squote)
		return l.emit(fh.LiteralString, i, i+1)
	}
	if c == '"' {
		l.stack = append(l.stack, dquote)
		return l.emit(fh.LiteralString, i, i+1)
	}
	// builtin_symbols
	if n := l.words.Match(src, i, 1); n > 0 && l.at(i+n) != '.' {
		return l.emit(fh.KeywordConstant, i, i+n)
	}
	if c == '.' && l.at(i+1) == '.' { // \.\.(\.|[0-9]+)(?![\w.])
		e := 0
		if l.at(i+2) == '.' {
			e = i + 3
		} else if isDec(l.at(i + 2)) {
			e = i + 3
			for isDec(l.at(e)) {
				e++
			}
		}
		if e > 0 && l.at(e) != '.' && !l.wordAt(e) {
			return l.emit(fh.KeywordConstant, i, e)
		}
	}
	if n, _ := l.words.Word(src, i); n == 1 && (c == 'T' || c == 'F') { // (T|F)\b
		return l.emit(fh.NameBuiltinPseudo, i, i+1)
	}
	if e := l.names(i, func(int) bool { return true }); e > 0 {
		return l.emit(fh.Name, i, e)
	}
	if e, t := l.number(i); e > 0 {
		return l.emit(t, i, e)
	}
	// punctuation: \[{1,2}|\]{1,2}|\(|\)|;|,
	switch c {
	case '[', ']':
		e := i + 1
		if l.at(e) == c {
			e++
		}
		return l.emit(fh.Punctuation, i, e)
	case '(', ')', ';', ',':
		return l.emit(fh.Punctuation, i, i+1)
	}
	if n := l.operator(i); n > 0 {
		return l.emit(fh.Operator, i, i+n)
	}
	if c == '{' || c == '}' {
		return l.emit(fh.Punctuation, i, i+1)
	}
	// .
	if c != '\n' {
		_, n := lexkit.Rune(src, i)
		return l.emit(fh.Text, i, i+n)
	}
	return 0, false
}

func (l *lexer) wordAt(i int) bool {
	if i >= len(l.src) {
		return false
	}
	r, _ := lexkit.Rune(l.src, i)
	return lexkit.IsWord(r)
}

func isHex(c byte) bool {
	return '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F'
}

// number returns the end and the kind of the numbers rule (0 if it does not match).
func (l *lexer) number(i int) (int, fh.TokenType) {
	src := l.src
	li := func(j int) int {
		if c := l.at(j); c == 'L' || c == 'i' {
			return j + 1
		}
		return j
	}
	// 0[xX][a-fA-F0-9]+([pP][0-9]+)?[Li]?
	if src[i] == '0' && (l.at(i+1) == 'x' || l.at(i+1) == 'X') && isHex(l.at(i+2)) {
		j := i + 3
		for isHex(l.at(j)) {
			j++
		}
		if c := l.at(j); (c == 'p' || c == 'P') && isDec(l.at(j+1)) {
			j += 2
			for isDec(l.at(j)) {
				j++
			}
		}
		return li(j), fh.LiteralNumberHex
	}
	// [+-]?([0-9]+(\.[0-9]+)?|\.[0-9]+|\.)([eE][+-]?[0-9]+)?[Li]?
	k := i
	if c := src[i]; (c == '+' || c == '-') && (isDec(l.at(i+1)) || l.at(i+1) == '.') {
		k++
	}
	var j int
	switch {
	case isDec(l.at(k)):
		j = k + 1
		for isDec(l.at(j)) {
			j++
		}
		if l.at(j) == '.' && isDec(l.at(j+1)) {
			j += 2
			for isDec(l.at(j)) {
				j++
			}
		}
	case l.at(k) == '.':
		j = k + 1
		for isDec(l.at(j)) {
			j++
		}
	default:
		return 0, 0
	}
	if c := l.at(j); c == 'e' || c == 'E' {
		m := j + 1
		if s := l.at(m); (s == '+' || s == '-') && isDec(l.at(m+1)) {
			m++
		}
		if isDec(l.at(m)) {
			for isDec(l.at(m)) {
				m++
			}
			j = m
		}
	}
	return li(j), fh.LiteralNumber
}

// operator returns the length of the operators rule (0 if it does not match).
func (l *lexer) operator(i int) int {
	src := l.src
	// <<?-|->>?|-|==|<=|>=|<|>|&&?|!=|\|\|?|\?
	for _, op := range [...]string{"<<-", "<-", "->>", "->", "-", "==", "<=", ">=", "<", ">", "&&", "&", "!=", "||", "|", "?"} {
		if strings.HasPrefix(src[i:], op) {
			return len(op)
		}
	}
	// \*|\+|\^|/|!|%[^%]*%|=|~|\$|@|:{1,3}
	switch c := src[i]; c {
	case '*', '+', '^', '/', '!', '=', '~', '$', '@':
		return 1
	case '%':
		if k := strings.IndexByte(src[i+1:], '%'); k >= 0 {
			return k + 2
		}
	case ':':
		n := 1
		for n < 3 && l.at(i+n) == ':' {
			n++
		}
		return n
	}
	return 0
}
