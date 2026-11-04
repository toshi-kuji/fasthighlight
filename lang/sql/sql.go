// Package sql registers SQL.
//
//	import _ "github.com/toshi-kuji/fasthighlight/lang/sql"
//
// Token types and boundaries reproduce the rules of Chroma 2.27.0's sql.xml without regular expressions.
// sql.xml is case-insensitive (case_insensitive). As with regexp2, comparison uses lexkit.FoldEq.
package sql

import (
	"strings"

	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/lexkit"
)

// Language is SQL. Its name, aliases, and filename patterns are listed in the catalog of the main
// package (fasthighlight.Builtin).
var Language = fh.Builtin("SQL")

func init() {
	Language.Lex = lex
	Language.Prepare = prepare
	fh.Register(Language)
}

var lists *lexkit.FoldWordLists

func prepare() { lists = lexkit.NewFoldWordLists(keywords, builtins) }

type state uint8

const (
	root state = iota
	comment
	single
	double
)

type lexer struct {
	x     *lexkit.Tokens
	src   string
	stack []state
	words lexkit.FoldWordCache
}

func lex(src string, end int) []fh.Token {
	l := &lexer{x: lexkit.NewTokens(src), src: src, stack: []state{root}, words: lexkit.FoldWordCache{Lists: lists}}
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

func (l *lexer) step(st state, i int) (int, bool) {
	src := l.src
	c := src[i]
	switch st {
	case comment:
		switch {
		case c == '/' && l.at(i+1) == '*':
			l.stack = append(l.stack, comment)
			return l.emit(fh.CommentMultiline, i, i+2)
		case c == '*' && l.at(i+1) == '/':
			l.stack = l.stack[:len(l.stack)-1]
			return l.emit(fh.CommentMultiline, i, i+2)
		case c == '/' || c == '*':
			return l.emit(fh.CommentMultiline, i, i+1)
		}
		j := i + 1
		for j < len(src) && src[j] != '/' && src[j] != '*' {
			j++
		}
		return l.emit(fh.CommentMultiline, i, j)
	case single, double:
		q, t := byte('\''), fh.LiteralStringSingle
		if st == double {
			q, t = '"', fh.LiteralStringDouble
		}
		if c == q {
			if l.at(i+1) == q {
				return l.emit(t, i, i+2)
			}
			l.stack = l.stack[:len(l.stack)-1]
			return l.emit(t, i, i+1)
		}
		j := i + 1
		for j < len(src) && src[j] != q {
			j++
		}
		return l.emit(t, i, j)
	}
	if n := lexkit.Span(src, i, lexkit.IsSpace); n > 0 {
		return l.emit(fh.TextWhitespace, i, i+n)
	}
	switch {
	case c == '-' && l.at(i+1) == '-': // --.*\n?
		e := lexkit.LineEnd(src, i)
		if e < len(src) {
			e++
		}
		return l.emit(fh.CommentSingle, i, e)
	case c == '/' && l.at(i+1) == '*':
		l.stack = append(l.stack, comment)
		return l.emit(fh.CommentMultiline, i, i+2)
	case c == '\'':
		l.stack = append(l.stack, single)
		return l.emit(fh.LiteralStringSingle, i, i+1)
	case c == '"':
		l.stack = append(l.stack, double)
		return l.emit(fh.LiteralStringDouble, i, i+1)
	}
	if n := l.words.Match(src, i, 0); n > 0 {
		return l.emit(fh.Keyword, i, i+n)
	}
	if n := l.words.Match(src, i, 1); n > 0 {
		return l.emit(fh.NameBuiltin, i, i+n)
	}
	if strings.IndexByte("+*/<>=~!@#%^&|`?-", c) >= 0 {
		return l.emit(fh.Operator, i, i+1)
	}
	if '0' <= c && c <= '9' {
		j := i + 1
		for '0' <= l.at(j) && l.at(j) <= '9' {
			j++
		}
		return l.emit(fh.LiteralNumberInteger, i, j)
	}
	// [a-z_][\w$]*
	if r, n := lexkit.Rune(src, i); r == '_' || 'a' <= lexkit.FoldASCII(r) && lexkit.FoldASCII(r) <= 'z' {
		e := i + n + lexkit.Span(src, i+n, func(r rune) bool { return r == '$' || lexkit.IsWord(r) })
		return l.emit(fh.Name, i, e)
	}
	if strings.IndexByte(";:()[],.", c) >= 0 {
		return l.emit(fh.Punctuation, i, i+1)
	}
	return 0, false
}
