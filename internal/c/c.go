// Package c is the tokenizer for C and C++. It is used by lang/c and lang/cpp.
//
// The token types and boundaries reproduce the rules of Chroma 2.27.0's c.xml and c++.xml without regular
// expressions. The two definitions share the state names and many rules, so Options.CPP selects between them.
//
// Chroma's usingself (re-reading the matched part from the same language's root) is reproduced by reading
// just that part again as new input (line start, \b and lookbehind are decided only within that part).
package c

import (
	"strings"
	"unicode/utf8"

	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/lexkit"
)

// Options switches between C and C++, and the intended differences from Chroma.
type Options struct {
	// If CPP is true it is C++ (c++.xml); otherwise it is C (c.xml).
	CPP bool
	// Setting ChromaRawString to true makes the C++ raw string rule the same as Chroma's. In regexp2,
	// the {,16} in Chroma's rule (R)(")([^\\()\s]{,16})(\()((?:.|\n)*?)(\)\3)(") is not a repetition but
	// means one character followed by the literal text "{,16}", so R"(…)" is not a raw string. If false,
	// {,16} is read as a repetition of 0 to 16 times.
	ChromaRawString bool
}

// The numbers of the word lists (lists.go).
const (
	lCKeywords = iota
	lCTypes
	lCReserved
	lCReservedM
	lCReservedUnderscore
	lCBuiltins
	lCPPKeywords
	lCPPCKeywords
	lCPPTypes
	lCPPReserved
	lCPPReservedM
	lCPPReservedUnderscore
	lCPPBuiltins
	numLists
)

var words map[string]uint16

// Prepare builds the word tables.
func Prepare() {
	m := map[string]uint16{}
	for k, alts := range [numLists]string{cKeywords, cTypes, cReserved, cReservedM, cReservedUnderscore, cBuiltins,
		cppKeywords, cppCKeywords, cppTypes, cppReserved, cppReservedM, cppReservedUnderscore, cppBuiltins} {
		for _, w := range strings.Split(alts, "|") {
			m[w] |= 1 << k
		}
	}
	words = m
}

// The states of Chroma.
type state uint8

const (
	root state = iota
	statement
	function
	str
	macro
	if0
	classname // C++ only
)

type lexer struct {
	o     Options
	x     *lexkit.Tokens
	src   string
	stack []state
	// wpos, wlen and wmask are the last position examined, the length of the run of word characters from
	// there, and the lists that contain that word.
	wpos, wlen int
	wmask      uint16
	// semiFrom and semiAt cache the position of the first ; at or after position semiFrom (len(src) if there is none).
	semiFrom, semiAt int
	// spFrom and spEnd are the last run of whitespace found by spaces: from any position inside it, the run
	// ends at spEnd (each line start in a run of blank lines asks again).
	spFrom, spEnd int
}

// Lex splits src into tokens (same contract as Lex in fasthighlight.Language).
func Lex(src string, end int, o Options) []fh.Token {
	x := lexkit.NewTokens(src)
	lexInto(x, src, end, o)
	return x.Result()
}

func lexInto(x *lexkit.Tokens, src string, end int, o Options) {
	l := &lexer{o: o, x: x, src: src, stack: []state{root}, wpos: -1, semiFrom: -1}
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
}

// nested reads src[s:e] from root as new input, as Chroma's usingself does.
func (l *lexer) nested(s, e int) {
	if s < e {
		lexInto(l.x.Sub(l.src[s:e], s), l.src[s:e], e-s, l.o)
	}
}

func (l *lexer) emit(t fh.TokenType, s, e int) (int, bool) { l.x.Emit(t, s, e); return e - s, true }
func (l *lexer) push(s state)                              { l.stack = append(l.stack, s) }
func (l *lexer) pop()                                      { l.stack = l.stack[:len(l.stack)-1] }

// at returns src[i] (0 if outside the range).
func (l *lexer) at(i int) byte {
	if i < len(l.src) {
		return l.src[i]
	}
	return 0
}

func (l *lexer) bol(i int) bool { return i == 0 || l.src[i-1] == '\n' }

func (l *lexer) spaces(i int) int {
	if l.spFrom <= i && i < l.spEnd {
		return l.spEnd - i
	}
	n := lexkit.Span(l.src, i, lexkit.IsSpace)
	if n > 0 {
		l.spFrom, l.spEnd = i, i+n
	}
	return n
}

func (l *lexer) step(st state, i int) (int, bool) {
	switch st {
	case root:
		if n, ok := l.whitespace(i); ok {
			return n, true
		}
		if n, ok := l.funcDef(i); ok {
			return n, true
		}
		l.push(statement)
		return 0, true
	case statement:
		if n, ok := l.whitespace(i); ok {
			return n, true
		}
		if n, ok := l.statements(i); ok {
			return n, true
		}
		switch c := l.src[i]; {
		case l.o.CPP && c == '{':
			l.push(root)
			return l.emit(fh.Punctuation, i, i+1)
		case l.o.CPP && (c == ';' || c == '}'), !l.o.CPP && c == ';':
			l.pop()
			return l.emit(fh.Punctuation, i, i+1)
		case !l.o.CPP && (c == '{' || c == '}'):
			return l.emit(fh.Punctuation, i, i+1)
		}
		return 0, false
	case function:
		if n, ok := l.whitespace(i); ok {
			return n, true
		}
		if n, ok := l.statements(i); ok {
			return n, true
		}
		switch l.src[i] {
		case ';':
			return l.emit(fh.Punctuation, i, i+1)
		case '{':
			l.push(function)
			return l.emit(fh.Punctuation, i, i+1)
		case '}':
			l.pop()
			return l.emit(fh.Punctuation, i, i+1)
		}
		return 0, false
	case str:
		return l.str(i)
	case macro:
		return l.macro(i)
	case if0:
		return l.if0(i)
	case classname:
		return l.classname(i)
	}
	return 0, false
}

// ifZero returns the end if src[p:] matches #if\s+0, or 0 if it does not.
func (l *lexer) ifZero(p int) int {
	if !strings.HasPrefix(l.src[p:], "#if") {
		return 0
	}
	n := l.spaces(p + 3)
	if n > 0 && l.at(p+3+n) == '0' {
		return p + 3 + n + 1
	}
	return 0
}

// leading passes to f the candidate ends of ^(\s*(?:/[*].*?[*]/\s*)?) starting at line start i, in the order
// the regular expression tries them. If f returns true, that candidate is the one chosen and leading returns true.
func (l *lexer) leading(i int, f func(q int) bool) bool {
	src := l.src
	p := i + l.spaces(i)
	if strings.HasPrefix(src[p:], "/*") {
		for m := p + 2; ; {
			k := strings.Index(src[m:], "*/")
			if k < 0 || strings.IndexByte(src[p+2:m+k], '\n') >= 0 {
				break
			}
			ce := m + k + 2
			if f(ce + l.spaces(ce)) {
				return true
			}
			m += k + 1
		}
	}
	return f(p)
}

// whitespace tries the rules of whitespace.
func (l *lexer) whitespace(i int) (int, bool) {
	src := l.src
	c := src[i]
	if l.bol(i) {
		if c == '#' {
			if e := l.ifZero(i); e > 0 { // ^#if\s+0
				l.push(if0)
				return l.emit(fh.CommentPreproc, i, e)
			}
			l.push(macro) // ^#
			return l.emit(fh.CommentPreproc, i, i+1)
		}
		// ^(\s*(?:/[*].*?[*]/\s*)?)(#if\s+0) and ^(…)(#). The first part is read with usingself.
		for _, zero := range [...]bool{true, false} {
			var q, e int
			if l.leading(i, func(p int) bool {
				q = p
				if zero {
					e = l.ifZero(p)
				} else if l.at(p) == '#' {
					e = p + 1
				}
				return e > 0
			}) {
				l.nested(i, q)
				l.x.Emit(fh.CommentPreproc, q, e)
				if zero {
					l.push(if0)
				} else {
					l.push(macro)
				}
				return e - i, true
			}
		}
	}
	if c == '\n' {
		return l.emit(fh.Text, i, i+1)
	}
	if n := l.spaces(i); n > 0 {
		return l.emit(fh.Text, i, i+n)
	}
	if c == '\\' && l.at(i+1) == '\n' {
		return l.emit(fh.Text, i, i+2)
	}
	if c != '/' {
		return 0, false
	}
	// //(\n|[\w\W]*?[^\\]\n)
	if l.at(i+1) == '/' {
		if l.at(i+2) == '\n' {
			return l.emit(fh.CommentSingle, i, i+3)
		}
		for k := i + 3; k <= len(src); {
			j := strings.IndexByte(src[k:], '\n')
			if j < 0 {
				break
			}
			k += j
			if src[k-1] != '\\' {
				return l.emit(fh.CommentSingle, i, k+1)
			}
			k++
		}
		return 0, false
	}
	// /(\\\n)?[*][\w\W]*?[*](\\\n)?/ and /(\\\n)?[*][\w\W]*
	j := i + 1
	if strings.HasPrefix(src[j:], "\\\n") {
		j += 2
	}
	if l.at(j) != '*' {
		return 0, false
	}
	for m := j + 1; ; {
		k := strings.IndexByte(src[m:], '*')
		if k < 0 {
			break
		}
		m += k
		if strings.HasPrefix(src[m+1:], "\\\n/") {
			return l.emit(fh.CommentMultiline, i, m+4)
		}
		if l.at(m+1) == '/' {
			return l.emit(fh.CommentMultiline, i, m+2)
		}
		m++
	}
	return l.emit(fh.CommentMultiline, i, len(src))
}

// isFuncChar is [\w*\s].
func isFuncChar(r rune) bool { return r == '*' || lexkit.IsWord(r) || lexkit.IsSpace(r) }

func isIdentStart(c byte) bool { return c == '_' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' }

// semicolon returns the position of the first ; at or after position p (len(src) if there is none).
func (l *lexer) semicolon(p int) int {
	if l.semiFrom < 0 || p < l.semiFrom || p > l.semiAt {
		l.semiFrom = p
		if k := strings.IndexByte(l.src[p:], ';'); k >= 0 {
			l.semiAt = p + k
		} else {
			l.semiAt = len(l.src)
		}
	}
	return l.semiAt
}

// funcDef tries root's rules for function definitions and declarations.
//
//	((?:[\w*\s])+?(?:\s|[*]))([a-zA-Z_]\w*)(\s*\([^;]*?\))([^;{]*)(\{)
//	((?:[\w*\s])+?(?:\s|[*]))([a-zA-Z_]\w*)(\s*\([^;]*?\))([^;]*)(;)
//
// The first part consists only of [\w*\s], and the name is followed by \s*\(, so the ( is right after the run of
// [\w*\s] that continues from i. The name is fixed as the last word of that run (followed only by whitespace).
func (l *lexer) funcDef(i int) (int, bool) {
	src := l.src
	r := i + lexkit.Span(src, i, isFuncChar)
	if l.at(r) != '(' {
		return 0, false
	}
	e := r
	for e > i {
		c, n := utf8.DecodeLastRuneInString(src[i:e])
		if !lexkit.IsSpace(c) {
			break
		}
		e -= n
	}
	g := e
	for g > i {
		c, n := utf8.DecodeLastRuneInString(src[i:g])
		if !lexkit.IsWord(c) {
			break
		}
		g -= n
	}
	if g == e || g < i+2 || !isIdentStart(src[g]) {
		return 0, false
	}
	s := l.semicolon(r + 1)
	// Function definition: after the ) there is a { before any ;. The ) is the first such one.
	if b := strings.LastIndexByte(src[r+1:s], '{'); b >= 0 {
		b += r + 1
		if p := strings.IndexByte(src[r+1:b], ')'); p >= 0 {
			p += r + 1
			q := p + 1 + strings.IndexByte(src[p+1:], '{')
			l.nested(i, g)
			l.x.Emit(fh.NameFunction, g, e)
			l.nested(e, p+1)
			l.nested(p+1, q)
			l.x.Emit(fh.Punctuation, q, q+1)
			l.push(function)
			return q + 1 - i, true
		}
	}
	// Function declaration: there is a ; after the ).
	if s < len(src) {
		if p := strings.IndexByte(src[r+1:s], ')'); p >= 0 {
			p += r + 1
			l.nested(i, g)
			l.x.Emit(fh.NameFunction, g, e)
			l.nested(e, p+1)
			l.nested(p+1, s)
			l.x.Emit(fh.Punctuation, s, s+1)
			return s + 1 - i, true
		}
	}
	return 0, false
}

// word returns the length of the run of word characters starting at position i, and the lists that contain that word.
func (l *lexer) word(i int) (int, uint16) {
	if l.wpos != i {
		l.wpos, l.wlen, l.wmask = i, lexkit.Span(l.src, i, lexkit.IsWord), 0
		if l.wlen > 0 {
			l.wmask = words[l.src[i:i+l.wlen]]
		}
	}
	return l.wlen, l.wmask
}

// listWord returns the length and true if a word of list k ((a|b|…)\b) matches at position i. All the
// words in the lists consist only of word characters, so the \b after a word is equivalent to the run of
// word characters ending there.
func (l *lexer) listWord(i, k int) (int, bool) {
	n, mask := l.word(i)
	return n, mask&(1<<k) != 0
}

func isHex(c byte) bool {
	return '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F'
}
func isOct(c byte) bool   { return '0' <= c && c <= '7' }
func isBin(c byte) bool   { return c == '0' || c == '1' }
func isDec(c byte) bool   { return '0' <= c && c <= '9' }
func isLU(c byte) bool    { return c == 'L' || c == 'l' || c == 'U' || c == 'u' }
func isFloat(c byte) bool { return c == 'f' || c == 'F' }

// run returns the end of the run of bytes satisfying f starting at src[j:].
func (l *lexer) run(j int, f func(byte) bool) int {
	for j < len(l.src) && f(l.src[j]) {
		j++
	}
	return j
}

// sepRun returns the end of X('?X+)* (src[j] satisfies X).
func (l *lexer) sepRun(j int, f func(byte) bool) int {
	j = l.run(j, f)
	for l.at(j) == '\'' && j+1 < len(l.src) && f(l.src[j+1]) {
		j = l.run(j+1, f)
	}
	return j
}

func (l *lexer) digits(i int) int { return lexkit.Span(l.src, i, lexkit.IsDigit) }

// exponent returns the length matched by [eE][+-]?\d+[LlUu]*.
func (l *lexer) exponent(j int) int {
	if c := l.at(j); c != 'e' && c != 'E' {
		return 0
	}
	k := j + 1
	if c := l.at(k); c == '+' || c == '-' {
		if l.digits(k+1) > 0 {
			k++
		}
	}
	d := l.digits(k)
	if d == 0 {
		return 0
	}
	return l.run(k+d, isLU) - j
}

// number tries the number rules. If one matches, it returns the end and the type.
func (l *lexer) number(i int) (int, fh.TokenType) {
	src := l.src
	d := l.digits(i)
	// (\d+\.\d*|\.\d+|\d+)[eE][+-]?\d+[LlUu]*
	if d > 0 && l.at(i+d) == '.' {
		j := i + d + 1
		j += l.digits(j)
		if x := l.exponent(j); x > 0 {
			return j + x, fh.LiteralNumberFloat
		}
	}
	if src[i] == '.' {
		if d2 := l.digits(i + 1); d2 > 0 {
			j := i + 1 + d2
			if x := l.exponent(j); x > 0 {
				return j + x, fh.LiteralNumberFloat
			}
		}
	}
	if d > 0 {
		if x := l.exponent(i + d); x > 0 {
			return i + d + x, fh.LiteralNumberFloat
		}
	}
	// (\d+\.\d*|\.\d+|\d+[fF])[fF]?
	j := -1
	switch {
	case d > 0 && l.at(i+d) == '.':
		j = i + d + 1
		j += l.digits(j)
	case src[i] == '.' && l.digits(i+1) > 0:
		j = i + 1 + l.digits(i+1)
	case d > 0 && isFloat(l.at(i+d)):
		j = i + d + 1
	}
	if j >= 0 {
		if isFloat(l.at(j)) {
			j++
		}
		return j, fh.LiteralNumberFloat
	}
	if src[i] == '0' {
		c := l.at(i + 1)
		if !l.o.CPP {
			if c == 'x' && isHex(l.at(i+2)) { // 0x[0-9a-fA-F]+[LlUu]*
				return l.run(l.run(i+2, isHex), isLU), fh.LiteralNumberHex
			}
			if isOct(c) { // 0[0-7]+[LlUu]*
				return l.run(l.run(i+1, isOct), isLU), fh.LiteralNumberOct
			}
		} else {
			if (c == 'x' || c == 'X') && isHex(l.at(i+2)) { // 0[xX]([0-9A-Fa-f]('?[0-9A-Fa-f]+)*)[LlUu]*
				return l.run(l.sepRun(i+2, isHex), isLU), fh.LiteralNumberHex
			}
			// 0('?[0-7]+)+[LlUu]*
			k := i + 1
			if c == '\'' && isOct(l.at(i+2)) {
				k++
			}
			if isOct(l.at(k)) {
				return l.run(l.sepRun(k, isOct), isLU), fh.LiteralNumberOct
			}
			if (c == 'b' || c == 'B') && isBin(l.at(i+2)) { // 0[Bb][01]('?[01]+)*[LlUu]*
				return l.run(l.sepRun(i+2, isBin), isLU), fh.LiteralNumberBin
			}
		}
	}
	if l.o.CPP {
		if isDec(src[i]) { // [0-9]('?[0-9]+)*[LlUu]*
			return l.run(l.sepRun(i, isDec), isLU), fh.LiteralNumberInteger
		}
	} else if d > 0 { // \d+[LlUu]*
		return l.run(i+d, isLU), fh.LiteralNumberInteger
	}
	return 0, 0
}

// attribute returns the end if \[\[.+\]\] matches, or 0 if it does not. .+ is greedy, so the match closes at the
// last ]] in the line.
func (l *lexer) attribute(i int) int {
	if !strings.HasPrefix(l.src[i:], "[[") || i+3 > len(l.src) {
		return 0
	}
	le := lexkit.LineEnd(l.src, i+2)
	if i+3 > le {
		return 0
	}
	if k := strings.LastIndex(l.src[i+3:le], "]]"); k >= 0 {
		return i + 3 + k + 2
	}
	return 0
}

// rawString tries the C++ raw string rule. src[i:] is R".
func (l *lexer) rawString(i int) (int, bool) {
	src := l.src
	j := i + 2
	isDelim := func(r rune) bool { return r != '\\' && r != '(' && r != ')' && !lexkit.IsSpace(r) }
	var d int // end of the delimiter
	if l.o.ChromaRawString {
		// one character of [^\\()\s], and the literal text "{,16}"
		if j >= len(src) {
			return 0, false
		}
		r, n := lexkit.Rune(src, j)
		if !isDelim(r) || !strings.HasPrefix(src[j+n:], "{,16}") {
			return 0, false
		}
		d = j + n + 5
	} else {
		// [^\\()\s]{0,16}. It is taken greedily; if no ( follows, a shorter one does not match either (the next
		// character is a delimiter character).
		d = j
		for count := 0; count < 16 && d < len(src); count++ {
			r, n := lexkit.Rune(src, d)
			if !isDelim(r) {
				break
			}
			d += n
		}
	}
	if l.at(d) != '(' {
		return 0, false
	}
	closing := ")" + src[j:d] + `"`
	k := strings.Index(src[d+1:], closing)
	if k < 0 {
		return 0, false
	}
	c := d + 1 + k // position of the )
	l.x.Emit(fh.LiteralStringAffix, i, i+1)
	l.x.Emit(fh.LiteralString, i+1, i+2)
	l.x.Emit(fh.LiteralStringDelimiter, j, d)
	l.x.Emit(fh.LiteralStringDelimiter, d, d+1)
	l.x.Emit(fh.LiteralString, d+1, c)
	l.x.Emit(fh.LiteralStringDelimiter, c, c+len(closing)-1)
	l.x.Emit(fh.LiteralString, c+len(closing)-1, c+len(closing))
	return c + len(closing) - i, true
}

// charLen returns the length of the inner part of (')(\\.|\\[0-7]{1,3}|\\x[a-fA-F0-9]{1,2}|[^\\\'\n])(')
// (0 if it does not match). src[j] is '.
func (l *lexer) charLen(j int) int {
	src := l.src
	k := j + 1
	if k >= len(src) {
		return 0
	}
	if src[k] == '\\' {
		// \\.
		if k+1 < len(src) && src[k+1] != '\n' {
			_, n := lexkit.Rune(src, k+1)
			if l.at(k+1+n) == '\'' {
				return 1 + n
			}
		}
		// \\[0-7]{1,3}
		if o := l.run(k+1, isOct) - (k + 1); o > 0 {
			if o > 3 {
				o = 3
			}
			if l.at(k+1+o) == '\'' {
				return 1 + o
			}
		}
		// \\x[a-fA-F0-9]{1,2}
		if l.at(k+1) == 'x' {
			if h := l.run(k+2, isHex) - (k + 2); h > 0 {
				if h > 2 {
					h = 2
				}
				if l.at(k+2+h) == '\'' {
					return 2 + h
				}
			}
		}
		return 0
	}
	if src[k] == '\'' || src[k] == '\n' {
		return 0
	}
	_, n := lexkit.Rune(src, k)
	if l.at(k+n) == '\'' {
		return n
	}
	return 0
}

// statements tries the rules of statements.
func (l *lexer) statements(i int) (int, bool) {
	src := l.src
	c := src[i]
	cpp := l.o.CPP
	if cpp {
		if n, ok := l.listWord(i, lCPPKeywords); ok {
			return l.emit(fh.Keyword, i, i+n)
		}
		// (enum)\b(\s+)(class)\b(\s*)
		if n, _ := l.word(i); n == 4 && src[i:i+4] == "enum" {
			if a := l.spaces(i + 4); a > 0 && strings.HasPrefix(src[i+4+a:], "class") && lexkit.WordBoundary(src, i+4+a+5) {
				k := i + 4 + a + 5
				b := l.spaces(k)
				l.x.Emit(fh.Keyword, i, i+4)
				l.x.Emit(fh.Text, i+4, i+4+a)
				l.x.Emit(fh.Keyword, i+4+a, k)
				l.x.Emit(fh.Text, k, k+b)
				l.push(classname)
				return k + b - i, true
			}
		}
		// (class|struct|enum|union)\b(\s*)
		if n, _ := l.word(i); n > 0 {
			switch src[i : i+n] {
			case "class", "struct", "enum", "union":
				b := l.spaces(i + n)
				l.x.Emit(fh.Keyword, i, i+n)
				l.x.Emit(fh.Text, i+n, i+n+b)
				l.push(classname)
				return n + b, true
			}
		}
		if c == '[' {
			if e := l.attribute(i); e > 0 {
				return l.emit(fh.NameAttribute, i, e)
			}
		}
		if c == 'R' && l.at(i+1) == '"' {
			if n, ok := l.rawString(i); ok {
				return n, true
			}
		}
		// (u8|u|U)(")
		a := 0
		switch {
		case c == 'u' && l.at(i+1) == '8' && l.at(i+2) == '"':
			a = 2
		case (c == 'u' || c == 'U') && l.at(i+1) == '"':
			a = 1
		}
		if a > 0 {
			l.x.Emit(fh.LiteralStringAffix, i, i+a)
			l.x.Emit(fh.LiteralString, i+a, i+a+1)
			l.push(str)
			return a + 1, true
		}
	}
	// (L?)(") and (L?)(')(…)(')
	a := 0
	if c == 'L' && (l.at(i+1) == '"' || l.at(i+1) == '\'') {
		a = 1
	}
	switch l.at(i + a) {
	case '"':
		l.x.Emit(fh.LiteralStringAffix, i, i+a)
		l.x.Emit(fh.LiteralString, i+a, i+a+1)
		l.push(str)
		return a + 1, true
	case '\'':
		if n := l.charLen(i + a); n > 0 {
			j := i + a
			l.x.Emit(fh.LiteralStringAffix, i, j)
			l.x.Emit(fh.LiteralStringChar, j, j+1)
			l.x.Emit(fh.LiteralStringChar, j+1, j+1+n)
			l.x.Emit(fh.LiteralStringChar, j+1+n, j+2+n)
			return a + n + 2, true
		}
	}
	if lexkit.IsDigit(l.runeAt(i)) || c == '.' {
		if e, t := l.number(i); e > 0 {
			return l.emit(t, i, e)
		}
	}
	if c == '*' && l.at(i+1) == '/' {
		return l.emit(fh.Error, i, i+2)
	}
	if strings.IndexByte("~!%^&*+=|?:<>/-", c) >= 0 {
		return l.emit(fh.Operator, i, i+1)
	}
	if strings.IndexByte("()[],.", c) >= 0 {
		return l.emit(fh.Punctuation, i, i+1)
	}
	kw, ty, res, resM, resU, bi := lCKeywords, lCTypes, lCReserved, lCReservedM, lCReservedUnderscore, lCBuiltins
	if cpp {
		kw, ty, res, resM, resU, bi = lCPPCKeywords, lCPPTypes, lCPPReserved, lCPPReservedM, lCPPReservedUnderscore, lCPPBuiltins
	}
	if n, ok := l.listWord(i, kw); ok {
		return l.emit(fh.Keyword, i, i+n)
	}
	if n, ok := l.listWord(i, ty); ok {
		return l.emit(fh.KeywordType, i, i+n)
	}
	// C only: \b[a-z]\w*_t\b
	if n, _ := l.word(i); !cpp && n >= 3 && 'a' <= c && c <= 'z' && strings.HasSuffix(src[i:i+n], "_t") && lexkit.WordBoundary(src, i) {
		return l.emit(fh.KeywordType, i, i+n)
	}
	for _, k := range [...]int{res, resM, resU} {
		if n, ok := l.listWord(i, k); ok {
			return l.emit(fh.KeywordReserved, i, i+n)
		}
	}
	if n, ok := l.listWord(i, bi); ok {
		return l.emit(fh.NameBuiltin, i, i+n)
	}
	if !isIdentStart(c) {
		return 0, false
	}
	n, _ := l.word(i)
	// ([a-zA-Z_]\w*)(\s*)(:)(?!:)
	s := i + n + l.spaces(i+n)
	if l.at(s) == ':' && l.at(s+1) != ':' {
		l.x.Emit(fh.NameLabel, i, i+n)
		l.x.Emit(fh.Text, i+n, s)
		l.x.Emit(fh.Punctuation, s, s+1)
		return s + 1 - i, true
	}
	// C only: \b[A-Za-z_]\w*(?=\s*\()
	if !cpp && l.at(s) == '(' && lexkit.WordBoundary(src, i) {
		return l.emit(fh.NameFunction, i, i+n)
	}
	return l.emit(fh.Name, i, i+n)
}

func (l *lexer) runeAt(i int) rune {
	r, _ := lexkit.Rune(l.src, i)
	return r
}

// str tries the rules of the string state.
func (l *lexer) str(i int) (int, bool) {
	src := l.src
	switch src[i] {
	case '"':
		l.pop()
		return l.emit(fh.LiteralString, i, i+1)
	case '\\':
		// \\([\\abfnrtv"\']|x[a-fA-F0-9]{2,4}|u[a-fA-F0-9]{4}|U[a-fA-F0-9]{8}|[0-7]{1,3})
		switch c := l.at(i + 1); {
		case c != 0 && strings.IndexByte("\\abfnrtv\"'", c) >= 0:
			return l.emit(fh.LiteralStringEscape, i, i+2)
		case c == 'x':
			if h := l.run(i+2, isHex) - (i + 2); h >= 2 {
				return l.emit(fh.LiteralStringEscape, i, i+2+min(h, 4))
			}
		case c == 'u':
			if l.run(i+2, isHex)-(i+2) >= 4 {
				return l.emit(fh.LiteralStringEscape, i, i+6)
			}
		case c == 'U':
			if l.run(i+2, isHex)-(i+2) >= 8 {
				return l.emit(fh.LiteralStringEscape, i, i+10)
			}
		case isOct(c):
			return l.emit(fh.LiteralStringEscape, i, i+1+min(l.run(i+1, isOct)-(i+1), 3))
		case c == '\n':
			return l.emit(fh.LiteralString, i, i+2) // \\\n
		}
		return l.emit(fh.LiteralString, i, i+1) // \\
	case '\n':
		return 0, false
	}
	j := i + 1
	for j < len(src) && src[j] != '\\' && src[j] != '"' && src[j] != '\n' {
		j++
	}
	return l.emit(fh.LiteralString, i, j)
}

// macro tries the rules of the macro state.
func (l *lexer) macro(i int) (int, bool) {
	src := l.src
	if strings.HasPrefix(src[i:], "include") {
		if l.o.CPP {
			// (include)(\s+)("[^"]+?"|<[^>]+?>)
			if a := l.spaces(i + 7); a > 0 {
				p := i + 7 + a
				var closeQ byte
				switch l.at(p) {
				case '"':
					closeQ = '"'
				case '<':
					closeQ = '>'
				}
				if closeQ != 0 {
					if k := strings.IndexByte(src[p+1:], closeQ); k > 0 {
						l.x.Emit(fh.CommentPreproc, i, i+7)
						l.x.Emit(fh.Text, i+7, p)
						l.x.Emit(fh.CommentPreprocFile, p, p+1+k+1)
						return p + 1 + k + 1 - i, true
					}
				}
			}
		} else if q, ok := l.includeFile(i + 7); ok {
			// (include)(\s*(?:/[*].*?[*]/\s*)?)([^\n]+)
			e := lexkit.LineEnd(src, q)
			l.x.Emit(fh.CommentPreproc, i, i+7)
			l.x.Emit(fh.Text, i+7, q)
			l.x.Emit(fh.CommentPreprocFile, q, e)
			return e - i, true
		}
	}
	switch c := src[i]; c {
	case '/':
		if l.at(i+1) == '*' { // /[*](.|\n)*?[*]/
			if k := strings.Index(src[i+2:], "*/"); k >= 0 {
				return l.emit(fh.CommentMultiline, i, i+2+k+2)
			}
		}
		if l.at(i+1) == '/' { // //.*?\n
			if k := strings.IndexByte(src[i+2:], '\n'); k >= 0 {
				l.pop()
				return l.emit(fh.CommentSingle, i, i+2+k+1)
			}
		}
		return l.emit(fh.CommentPreproc, i, i+1)
	case '\n':
		if i > 0 && src[i-1] == '\\' { // (?<=\\)\n
			return l.emit(fh.CommentPreproc, i, i+1)
		}
		l.pop()
		return l.emit(fh.CommentPreproc, i, i+1)
	}
	// [^/\n]+
	j := i + 1
	for j < len(src) && src[j] != '/' && src[j] != '\n' {
		j++
	}
	return l.emit(fh.CommentPreproc, i, j)
}

// includeFile tries (\s*(?:/[*].*?[*]/\s*)?)([^\n]+) after a C include, from p, in the same order as the
// regular expression's backtracking, and returns the start of the second part.
func (l *lexer) includeFile(p int) (int, bool) {
	src := l.src
	file := func(q int) bool { return q < len(src) && src[q] != '\n' }
	var q int
	ok := lexkit.Run(src, p, 0, lexkit.IsSpace, func(a int) bool {
		if strings.HasPrefix(src[a:], "/*") {
			for m := a + 2; ; {
				k := strings.Index(src[m:], "*/")
				if k < 0 || strings.IndexByte(src[a+2:m+k], '\n') >= 0 {
					break
				}
				if lexkit.Run(src, m+k+2, 0, lexkit.IsSpace, func(b int) bool { q = b; return file(b) }) {
					return true
				}
				m += k + 1
			}
		}
		q = a
		return file(a)
	})
	return q, ok
}

// if0 tries the rules of the if0 state.
func (l *lexer) if0(i int) (int, bool) {
	src := l.src
	if l.bol(i) {
		j := i + l.spaces(i)
		rest := src[j:]
		// ^\s*#if.*?(?<!\\)\n and ^\s*#endif.*?(?<!\\)\n
		for _, w := range [...]string{"#if", "#endif"} {
			if strings.HasPrefix(rest, w) {
				if k := strings.IndexByte(src[j+len(w):], '\n'); k >= 0 {
					k += j + len(w)
					if src[k-1] != '\\' {
						if w == "#if" {
							l.push(if0)
						} else {
							l.pop()
						}
						return l.emit(fh.CommentPreproc, i, k+1)
					}
				}
			}
			if w == "#if" && (strings.HasPrefix(rest, "#else") || strings.HasPrefix(rest, "#elif")) {
				// ^\s*#el(?:se|if).*\n
				if k := strings.IndexByte(src[j+5:], '\n'); k >= 0 {
					l.pop()
					return l.emit(fh.CommentPreproc, i, j+5+k+1)
				}
			}
		}
	}
	// .*?\n
	if k := strings.IndexByte(src[i:], '\n'); k >= 0 {
		return l.emit(fh.Comment, i, i+k+1)
	}
	return 0, false
}

// classname tries the rules of the C++ classname state.
func (l *lexer) classname(i int) (int, bool) {
	src := l.src
	// (\[\[.+\]\])(\s*)
	if e := l.attribute(i); e > 0 {
		b := l.spaces(e)
		l.x.Emit(fh.NameAttribute, i, e)
		l.x.Emit(fh.Text, e, e+b)
		return e + b - i, true
	}
	if isIdentStart(src[i]) {
		n, _ := l.word(i)
		l.pop()
		return l.emit(fh.NameClass, i, i+n)
	}
	// \s*(?=[>{])
	b := l.spaces(i)
	if c := l.at(i + b); c == '>' || c == '{' {
		l.pop()
		return l.emit(fh.Text, i, i+b)
	}
	return 0, false
}
