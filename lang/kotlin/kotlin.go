// Package kotlin registers Kotlin.
//
//	import _ "github.com/toshi-kuji/fasthighlight/lang/kotlin"
//
// Token types and boundaries reproduce the rules of Chroma 2.27.0's kotlin.xml without regular expressions.
// kotlin.xml is dot_all, so . also matches a newline. The continuation characters of a name are
// [\p{L}\p{N}], which does not include _.
package kotlin

import (
	"strings"
	"unicode"

	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/lexkit"
)

// Language is Kotlin. Its name, aliases, and filename patterns are listed in the catalog of the main
// package (fasthighlight.Builtin).
var Language = fh.Builtin("Kotlin")

func init() {
	Language.Lex = lex
	Language.Prepare = prepare
	fh.Register(Language)
}

var lists *lexkit.WordLists

func prepare() { lists = lexkit.NewWordLists(keywords) }

type state uint8

const (
	root state = iota
	str
	rawstring
	pkg
	class
	property
	function
	generics
)

type lexer struct {
	x     *lexkit.Tokens
	src   string
	stack []state
	words lexkit.WordCache
}

func lex(src string, end int) []fh.Token {
	l := &lexer{x: lexkit.NewTokens(src), src: src, stack: []state{root}, words: lexkit.WordCache{Lists: lists}}
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
func (l *lexer) push(s state)                              { l.stack = append(l.stack, s) }
func (l *lexer) pop()                                      { l.stack = l.stack[:len(l.stack)-1] }

func (l *lexer) at(i int) byte {
	if i < len(l.src) {
		return l.src[i]
	}
	return 0
}

func (l *lexer) runeAt(i int) (rune, int) {
	if i >= len(l.src) {
		return -1, 0
	}
	return lexkit.Rune(l.src, i)
}

func (l *lexer) spaces(i int) int { return lexkit.Span(l.src, i, lexkit.IsSpace) }

func isStart(r rune) bool { return r == '_' || unicode.IsLetter(r) }
func isCont(r rune) bool  { return unicode.IsLetter(r) || unicode.IsNumber(r) }

// ident returns the length of (?:[_\p{L}][\p{L}\p{N}]*|`@?[_\p{L}][\p{L}\p{N}]+`).
func (l *lexer) ident(i int) int {
	r, n := l.runeAt(i)
	if n == 0 {
		return 0
	}
	if isStart(r) {
		return n + lexkit.Span(l.src, i+n, isCont)
	}
	if r != '`' {
		return 0
	}
	j := i + 1
	if l.at(j) == '@' {
		j++
	}
	r, n = l.runeAt(j)
	if n == 0 || !isStart(r) {
		return 0
	}
	m := lexkit.Span(l.src, j+n, isCont)
	if m == 0 || l.at(j+n+m) != '`' {
		return 0
	}
	return j + n + m + 1 - i
}

// backtick returns the length of \x60[^\x60]+?\x60.
func (l *lexer) backtick(i int) int {
	if l.at(i) != '`' {
		return 0
	}
	if k := strings.IndexByte(l.src[i+1:], '`'); k > 0 {
		return k + 2
	}
	return 0
}

func (l *lexer) step(st state, i int) (int, bool) {
	src := l.src
	switch st {
	case root:
		return l.root(i)
	case str:
		return l.str(i)
	case rawstring:
		if strings.HasPrefix(src[i:], `"""`) {
			l.pop()
			return l.emit(fh.LiteralString, i, i+3)
		}
		// (?:[^$"]+|\"{1,2}[^"])+
		j := i
		for j < len(src) {
			if c := src[j]; c != '$' && c != '"' {
				for j < len(src) && src[j] != '$' && src[j] != '"' {
					j++
				}
				continue
			}
			if src[j] != '"' {
				break
			}
			q := 1
			if l.at(j+1) == '"' && j+2 < len(src) && src[j+2] != '"' {
				q = 2
			} else if j+1 >= len(src) || src[j+1] == '"' {
				break
			}
			_, n := lexkit.Rune(src, j+q)
			j += q + n
		}
		if j > i {
			return l.emit(fh.LiteralString, i, j)
		}
		if n, ok := l.interpol(i); ok {
			return n, true
		}
		if src[i] == '$' {
			return l.emit(fh.LiteralString, i, i+1)
		}
	case pkg:
		if n := lexkit.Span(src, i, func(r rune) bool { return !lexkit.IsSpace(r) }); n > 0 {
			l.pop()
			return l.emit(fh.NameNamespace, i, i+n)
		}
	case class, property:
		t := fh.NameClass
		if st == property {
			t = fh.NameProperty
		}
		n := l.backtick(i)
		if n == 0 {
			n = l.ident(i)
		}
		if n > 0 {
			l.pop()
			return l.emit(t, i, i+n)
		}
	case function:
		if src[i] == '<' {
			l.push(generics)
			return l.emit(fh.Punctuation, i, i+1)
		}
		n := l.backtick(i)
		if n == 0 {
			n = l.ident(i)
		}
		if n > 0 {
			l.pop()
			return l.emit(fh.NameFunction, i, i+n)
		}
		if n := l.spaces(i); n > 0 {
			return l.emit(fh.Text, i, i+n)
		}
	case generics:
		switch src[i] {
		case '<':
			l.push(generics)
			return l.emit(fh.Punctuation, i, i+1)
		case '>':
			l.pop()
			return l.emit(fh.Punctuation, i, i+1)
		case ',', ':', '*', '?':
			return l.emit(fh.Punctuation, i, i+1)
		}
		for _, w := range [...]string{"in", "out", "reified"} { // (in|out|reified)
			if strings.HasPrefix(src[i:], w) {
				return l.emit(fh.Keyword, i, i+len(w))
			}
		}
		n := l.backtick(i)
		if n == 0 {
			n = l.ident(i)
		}
		if n > 0 {
			return l.emit(fh.NameClass, i, i+n)
		}
		if n := l.spaces(i); n > 0 {
			return l.emit(fh.Text, i, i+n)
		}
	}
	return 0, false
}

// interpol tries the rules of string-interpol.
func (l *lexer) interpol(i int) (int, bool) {
	if l.src[i] != '$' {
		return 0, false
	}
	if n := l.ident(i + 1); n > 0 { // \$ident
		return l.emit(fh.LiteralStringInterpol, i, i+1+n)
	}
	if l.at(i+1) == '{' { // \${[^}\n]*}
		if k := strings.IndexAny(l.src[i+2:], "}\n"); k >= 0 && l.src[i+2+k] == '}' {
			return l.emit(fh.LiteralStringInterpol, i, i+2+k+1)
		}
	}
	return 0, false
}

// str tries the rules of the string state.
func (l *lexer) str(i int) (int, bool) {
	src := l.src
	switch src[i] {
	case '\\':
		if c := l.at(i + 1); c != 0 && strings.IndexByte("tbnr'\"\\$", c) >= 0 {
			return l.emit(fh.LiteralStringEscape, i, i+2)
		}
		if l.at(i+1) == 'u' && hexCount(src, i+2) >= 4 {
			return l.emit(fh.LiteralStringEscape, i, i+6)
		}
		return 0, false
	case '"':
		l.pop()
		return l.emit(fh.LiteralStringDouble, i, i+1)
	case '$':
		if n, ok := l.interpol(i); ok {
			return n, true
		}
		return l.emit(fh.LiteralStringDouble, i, i+1)
	case '\n':
		return 0, false
	}
	j := i + 1
	for j < len(src) && strings.IndexByte("\n\\\"$", src[j]) < 0 {
		j++
	}
	return l.emit(fh.LiteralStringDouble, i, j)
}

func isDec(c byte) bool { return '0' <= c && c <= '9' }
func isHex(c byte) bool {
	return '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F'
}
func isBin(c byte) bool { return c == '0' || c == '1' }

func hexCount(src string, i int) int {
	n := 0
	for i+n < len(src) && isHex(src[i+n]) {
		n++
	}
	return n
}

func (l *lexer) run(j int, f func(byte) bool) int {
	for j < len(l.src) && f(l.src[j]) {
		j++
	}
	return j
}

// digits returns the end of X+(_+X+)* (-1 if src[j] is not X).
func (l *lexer) digits(j int, f func(byte) bool) int {
	if !f(l.at(j)) {
		return -1
	}
	j = l.run(j, f)
	for {
		k := l.run(j, func(c byte) bool { return c == '_' })
		if k == j || !f(l.at(k)) {
			return j
		}
		j = l.run(k, f)
	}
}

// exponent returns the end of ([eE][+-]?D)?.
func (l *lexer) exponent(j int) int {
	if c := l.at(j); c != 'e' && c != 'E' {
		return j
	}
	k := j + 1
	if c := l.at(k); (c == '+' || c == '-') && isDec(l.at(k+1)) {
		k++
	}
	if e := l.digits(k, isDec); e >= 0 {
		return e
	}
	return j
}

func (l *lexer) suffix(j int, s string) int {
	if c := l.at(j); c != 0 && strings.IndexByte(s, c) >= 0 {
		return j + 1
	}
	return j
}

// number tries the number rules. On a match it returns the end and the type.
func (l *lexer) number(i int) (int, fh.TokenType) {
	c := l.at(i)
	uL := func(j int) int { return l.suffix(l.suffix(j, "uU"), "L") }
	if c == '0' {
		switch l.at(i + 1) {
		case 'x', 'X':
			if e := l.digits(i+2, isHex); e >= 0 {
				return uL(e), fh.LiteralNumberHex
			}
		case 'b', 'B':
			if e := l.digits(i+2, isBin); e >= 0 {
				return uL(e), fh.LiteralNumberBin
			}
		}
	}
	d := l.digits(i, isDec)
	if d >= 0 && l.at(d) == '.' {
		if e := l.digits(d+1, isDec); e >= 0 {
			return l.suffix(l.exponent(e), "fF"), fh.LiteralNumberFloat
		}
	}
	if c == '.' {
		if e := l.digits(i+1, isDec); e >= 0 {
			return l.suffix(l.exponent(e), "fF"), fh.LiteralNumberFloat
		}
	}
	if d < 0 {
		return 0, 0
	}
	if e := l.exponent(d); e > d {
		return l.suffix(e, "fF"), fh.LiteralNumberFloat
	}
	if c := l.at(d); c == 'f' || c == 'F' {
		return d + 1, fh.LiteralNumberFloat
	}
	return uL(d), fh.LiteralNumberInteger
}

func (l *lexer) root(i int) (int, bool) {
	src := l.src
	c := src[i]
	// ^\s*\[.*?\]
	if i == 0 || src[i-1] == '\n' {
		if j := i + l.spaces(i); l.at(j) == '[' {
			if k := strings.IndexByte(src[j+1:], ']'); k >= 0 {
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
		if l.at(i+1) == '/' { // //[^\n]*\n?
			e := lexkit.LineEnd(src, i)
			if e < len(src) {
				e++
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
	// !==|!in|!is|===
	for _, op := range [...]string{"!==", "!in", "!is", "==="} {
		if strings.HasPrefix(src[i:], op) {
			return l.emit(fh.Operator, i, i+3)
		}
	}
	if i+1 < len(src) {
		switch src[i : i+2] {
		case "%=", "&&", "*=", "++", "+=", "--", "-=", "->", "..", "/=", "::", "<=", "==", ">=", "!!", "!=", "||", "?:", "?.":
			return l.emit(fh.Operator, i, i+2)
		}
	}
	if c == '{' || c == '}' {
		return l.emit(fh.Punctuation, i, i+1)
	}
	if c == '"' {
		if strings.HasPrefix(src[i:], `"""`) {
			l.push(rawstring)
			return l.emit(fh.LiteralString, i, i+3)
		}
		l.push(str)
		return l.emit(fh.LiteralStringDouble, i, i+1)
	}
	if c == '\'' {
		// (')(\\u[0-9a-fA-F]{4})(')
		if l.at(i+1) == '\\' && l.at(i+2) == 'u' && hexCount(src, i+3) >= 4 && l.at(i+7) == '\'' {
			l.x.Emit(fh.LiteralStringChar, i, i+1)
			l.x.Emit(fh.LiteralStringEscape, i+1, i+7)
			l.x.Emit(fh.LiteralStringChar, i+7, i+8)
			return 8, true
		}
		// '\\.'|'[^\\]'
		if l.at(i+1) == '\\' {
			if _, n := l.runeAt(i + 2); n > 0 && l.at(i+2+n) == '\'' {
				return l.emit(fh.LiteralStringChar, i, i+3+n)
			}
		} else if _, n := l.runeAt(i + 1); n > 0 && l.at(i+1+n) == '\'' {
			return l.emit(fh.LiteralStringChar, i, i+2+n)
		}
	}
	if isDec(c) || c == '.' {
		if e, t := l.number(i); e > 0 {
			return l.emit(t, i, e)
		}
	}
	if strings.IndexByte("~!%^&*()+=|[]:;,.<>/?-", c) >= 0 {
		return l.emit(fh.Punctuation, i, i+1)
	}
	// (companion)(\s+)(object)
	if strings.HasPrefix(src[i:], "companion") {
		if n := l.spaces(i + 9); n > 0 && strings.HasPrefix(src[i+9+n:], "object") {
			l.x.Emit(fh.Keyword, i, i+9)
			l.x.Emit(fh.Text, i+9, i+9+n)
			l.x.Emit(fh.Keyword, i+9+n, i+9+n+6)
			return 9 + n + 6, true
		}
	}
	for _, r := range [...]struct {
		w    string
		next state
	}{{"class", class}, {"interface", class}, {"object", class}, {"package", pkg}, {"import", pkg},
		{"val", property}, {"var", property}, {"fun", function}} {
		if strings.HasPrefix(src[i:], r.w) {
			if n := l.spaces(i + len(r.w)); n > 0 {
				l.x.Emit(fh.Keyword, i, i+len(r.w))
				l.x.Emit(fh.Text, i+len(r.w), i+len(r.w)+n)
				l.push(r.next)
				return len(r.w) + n, true
			}
		}
	}
	if n := l.words.Match(src, i, 0); n > 0 {
		return l.emit(fh.Keyword, i, i+n)
	}
	if c == '@' {
		if n := l.ident(i + 1); n > 0 {
			return l.emit(fh.NameDecorator, i, i+1+n)
		}
	}
	// (?:\p{Lu}[_\p{L}]*)(?=\.)
	if r, n := l.runeAt(i); n > 0 && unicode.IsUpper(r) {
		e := i + n + lexkit.Span(src, i+n, func(r rune) bool { return r == '_' || unicode.IsLetter(r) })
		if l.at(e) == '.' {
			return l.emit(fh.NameClass, i, e)
		}
	}
	if n := l.ident(i); n > 0 {
		return l.emit(fh.Name, i, i+n)
	}
	return 0, false
}
