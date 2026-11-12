// Package csharp is the C# tokenizer. lang/csharp registers it. Options (the switches for the intended
// differences from Chroma) exist only for the comparison in compat/ and are not exposed.
//
// Token types and boundaries reproduce the rules of Chroma 2.27.0's c#.xml without regular expressions.
// c#.xml is dot_all, so . also matches a newline.
package csharp

import (
	"strings"

	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/lexkit"
)

// Options holds the switches for the intended differences from Chroma. The zero value is the production
// behavior.
type Options struct {
	// ChromaTypes, if true, makes only the built-in types (int, string, ...) types (KeywordType), the
	// same as Chroma. In production, a user-defined name in a type position is a type too: type
	// arguments (List<User>, Get<User>()), the type of a declaration (User u = ..., a field, a
	// property, a parameter, a return type: User Get(...)), and the type after new or as.
	ChromaTypes bool
}

// Indexes of the word lists (lists.go).
const (
	lPreprocessor = iota
	lKeywords
	lDeclarations
	lTypes
	lConstants
)

var lists *lexkit.WordLists

// Prepare builds the word lists. It is Language.Prepare of lang/csharp.
func Prepare() {
	lists = lexkit.NewWordLists(preprocessor, keywords, declarations, types, constants)
}

type state uint8

const (
	root state = iota
	class
	namespace
)

type lexer struct {
	x     *lexkit.Tokens
	src   string
	stack []state
	words lexkit.WordCache
	sp    lexkit.SpanCache // the last run of whitespace
	o     Options
	// typeEnd is the end of the type being read (production only): before it, a name is a type
	// (KeywordType) instead of Name.
	typeEnd int
	// argsFrom and argsStop are the start and stop of the last failed type-argument scan (types.go).
	argsFrom, argsStop int
	// attrEnd remembers a failed search for the ] of an attribute at a line start, so that many lines
	// starting with [ and no ] are not each searched to the end.
	attrEnd lexkit.Finder
}

// Lex splits src into tokens (the same contract as Lex of fasthighlight.Language). compat/ uses it
// to compare with Chroma, switching o.
func Lex(src string, end int, o Options) []fh.Token {
	l := &lexer{x: lexkit.NewTokens(src), src: src, stack: []state{root}, words: lexkit.WordCache{Lists: lists}, o: o}
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

func (l *lexer) spaces(i int) int { return l.sp.Span(l.src, i, lexkit.IsSpace) }

func isNameStart(c byte) bool { return c == '_' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' }

// name returns the length of @?[_a-zA-Z]\w*.
func (l *lexer) name(i int) int {
	j := i
	if l.at(j) == '@' {
		j++
	}
	if !isNameStart(l.at(j)) {
		return 0
	}
	return j + 1 + lexkit.Span(l.src, j+1, lexkit.IsWord) - i
}

func (l *lexer) step(st state, i int) (int, bool) {
	switch st {
	case class:
		l.stack = l.stack[:len(l.stack)-1]
		if n := l.name(i); n > 0 {
			return l.emit(fh.NameClass, i, i+n)
		}
		return 0, true
	case namespace:
		if l.src[i] == '(' { // (?=\()
			l.stack = l.stack[:len(l.stack)-1]
			return 0, true
		}
		// (@?[_a-zA-Z]\w*|\.)+
		j := i
		for {
			if l.at(j) == '.' {
				j++
			} else if n := l.name(j); n > 0 {
				j += n
			} else {
				break
			}
		}
		if j > i {
			l.stack = l.stack[:len(l.stack)-1]
			return l.emit(fh.NameNamespace, i, j)
		}
		return 0, false
	}
	return l.root(i)
}

// verbatim returns the end of src[j:] if it matches "(""|[^"])*" (0 if there is no match). src[j]
// is ". If there is no closing ", backtracking in the regular expression closes it at the first " of
// the last "".
func (l *lexer) verbatim(j int) int {
	src := l.src
	lastPair := -1
	for k := j + 1; k < len(src); {
		if src[k] == '"' {
			if l.at(k+1) == '"' {
				lastPair = k
				k += 2
				continue
			}
			return k + 1
		}
		k++
	}
	if lastPair >= 0 {
		return lastPair + 1
	}
	return 0
}

func (l *lexer) root(i int) (int, bool) {
	src := l.src
	c := src[i]
	// ^\s*\[.*?\]
	if i == 0 || src[i-1] == '\n' {
		if j := i + l.spaces(i); l.at(j) == '[' {
			if k := l.attrEnd.Index(src, j+1, "]"); k >= 0 {
				return l.emit(fh.NameAttribute, i, j+1+k+1)
			}
		}
	}
	// [^\S\n]+
	if n := lexkit.Span(src, i, func(r rune) bool { return r != '\n' && lexkit.IsSpace(r) }); n > 0 {
		return l.emit(fh.Text, i, i+n)
	}
	switch c {
	case '\\':
		if l.at(i+1) == '\n' {
			return l.emit(fh.Text, i, i+2)
		}
	case '/':
		if l.at(i+1) == '/' { // ///[^\n\r]* and //[^\n\r]*
			e := i + 2
			for e < len(src) && src[e] != '\n' && src[e] != '\r' {
				e++
			}
			if l.at(i+2) == '/' {
				return l.emit(fh.CommentSpecial, i, e)
			}
			return l.emit(fh.CommentSingle, i, e)
		}
		if l.at(i+1) == '*' { // /[*].*?[*]/
			if k := strings.Index(src[i+2:], "*/"); k >= 0 {
				return l.emit(fh.CommentMultiline, i, i+2+k+2)
			}
		}
	case '\n':
		return l.emit(fh.Text, i, i+1)
	}
	if strings.IndexByte("~!%^&*()+=|[]:;,.<>/?-{}", c) >= 0 {
		if c == '<' && !l.o.ChromaTypes && i >= l.typeEnd && i > 0 && isNameByte(src[i-1]) {
			if e := l.typeArgsEnd(i); e > 0 && l.genericFollows(e) {
				l.typeEnd = e
			}
		}
		return l.emit(fh.Punctuation, i, i+1)
	}
	// @"(""|[^"])*" and \$@?"(""|[^"])*"
	q := -1
	switch {
	case c == '@' && l.at(i+1) == '"':
		q = i + 1
	case c == '$' && l.at(i+1) == '"':
		q = i + 1
	case c == '$' && l.at(i+1) == '@' && l.at(i+2) == '"':
		q = i + 2
	}
	if q >= 0 {
		if e := l.verbatim(q); e > 0 {
			return l.emit(fh.LiteralString, i, e)
		}
	}
	// "(\\\\|\\"|[^"\n])*["\n]
	if c == '"' {
		for k := i + 1; k < len(src); {
			switch src[k] {
			case '\\':
				if d := l.at(k + 1); d == '\\' || d == '"' {
					k += 2
					continue
				}
			case '"', '\n':
				return l.emit(fh.LiteralString, i, k+1)
			}
			k++
		}
	}
	// '\\.'|'[^\\]'
	if c == '\'' {
		if l.at(i+1) == '\\' {
			if i+2 < len(src) {
				_, n := lexkit.Rune(src, i+2)
				if l.at(i+2+n) == '\'' {
					return l.emit(fh.LiteralStringChar, i, i+3+n)
				}
			}
		} else if i+1 < len(src) {
			_, n := lexkit.Rune(src, i+1)
			if l.at(i+1+n) == '\'' {
				return l.emit(fh.LiteralStringChar, i, i+2+n)
			}
		}
	}
	if e := l.number(i); e > 0 {
		return l.emit(fh.LiteralNumber, i, e)
	}
	// #[ \t]*(if|endif|…)\b
	if c == '#' {
		j := i + 1
		for l.at(j) == ' ' || l.at(j) == '\t' {
			j++
		}
		if j < len(src) {
			if n := l.words.Match(src, j, lPreprocessor); n > 0 {
				return l.emit(fh.CommentPreproc, i, j+n)
			}
		}
	}
	// \b(extern)(\s+)(alias)\b
	if strings.HasPrefix(src[i:], "extern") && lexkit.WordBoundary(src, i) {
		if n := l.spaces(i + 6); n > 0 && strings.HasPrefix(src[i+6+n:], "alias") && lexkit.WordBoundary(src, i+6+n+5) {
			l.x.Emit(fh.Keyword, i, i+6)
			l.x.Emit(fh.Text, i+6, i+6+n)
			l.x.Emit(fh.Keyword, i+6+n, i+6+n+5)
			return 6 + n + 5, true
		}
	}
	if n := l.words.Match(src, i, lKeywords); n > 0 {
		if !l.o.ChromaTypes && i >= l.typeEnd && (src[i:i+n] == "new" || src[i:i+n] == "as") {
			l.typeAfterKeyword(i + n)
		}
		return l.emit(fh.Keyword, i, i+n)
	}
	// (global)(::)
	if strings.HasPrefix(src[i:], "global::") {
		l.x.Emit(fh.Keyword, i, i+6)
		l.x.Emit(fh.Punctuation, i+6, i+8)
		return 8, true
	}
	if n := l.words.Match(src, i, lDeclarations); n > 0 {
		return l.emit(fh.KeywordDeclaration, i, i+n)
	}
	if n := l.words.Match(src, i, lTypes); n > 0 { // (…)\b\??
		if l.at(i+n) == '?' {
			n++
		}
		return l.emit(fh.KeywordType, i, i+n)
	}
	if n := l.words.Match(src, i, lConstants); n > 0 {
		return l.emit(fh.KeywordConstant, i, i+n)
	}
	// (class|struct|record|interface)(\s+) and (namespace|using)(\s+)
	for _, r := range [...]struct {
		w    string
		next state
	}{{"class", class}, {"struct", class}, {"record", class}, {"interface", class}, {"namespace", namespace}, {"using", namespace}} {
		if strings.HasPrefix(src[i:], r.w) {
			if n := l.spaces(i + len(r.w)); n > 0 {
				l.x.Emit(fh.Keyword, i, i+len(r.w))
				l.x.Emit(fh.Text, i+len(r.w), i+len(r.w)+n)
				l.stack = append(l.stack, r.next)
				return len(r.w) + n, true
			}
		}
	}
	if n := l.name(i); n > 0 {
		if !l.o.ChromaTypes && i >= l.typeEnd {
			l.declaredType(i)
		}
		if i < l.typeEnd {
			return l.emit(fh.KeywordType, i, i+n)
		}
		return l.emit(fh.Name, i, i+n)
	}
	return 0, false
}

func isHex(c byte) bool {
	return '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F'
}

// number returns the end of 0[xX][0-9a-fA-F]+[Ll]?|\d[_\d]*(\.\d*)?([eE][+-]?\d+)?[flFLdD]? (0 if there
// is no match).
func (l *lexer) number(i int) int {
	src := l.src
	if src[i] == '0' && (l.at(i+1) == 'x' || l.at(i+1) == 'X') && isHex(l.at(i+2)) {
		j := i + 3
		for isHex(l.at(j)) {
			j++
		}
		if c := l.at(j); c == 'l' || c == 'L' {
			j++
		}
		return j
	}
	r, n := lexkit.Rune(src, i)
	if !lexkit.IsDigit(r) {
		return 0
	}
	j := i + n + lexkit.Span(src, i+n, func(r rune) bool { return r == '_' || lexkit.IsDigit(r) })
	if l.at(j) == '.' {
		j += 1 + lexkit.Span(src, j+1, lexkit.IsDigit)
	}
	if c := l.at(j); c == 'e' || c == 'E' {
		k := j + 1
		if s := l.at(k); s == '+' || s == '-' {
			k++
		}
		// If no digit follows the sign, it does not match even without the sign (the next character is the sign).
		if d := lexkit.Span(src, k, lexkit.IsDigit); d > 0 {
			j = k + d
		}
	}
	if c := l.at(j); c != 0 && strings.IndexByte("flFLdD", c) >= 0 {
		j++
	}
	return j
}
