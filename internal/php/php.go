// Package php is the PHP tokenizer. lang/php registers it. Options (the switches for the intended
// differences from Chroma) exist only for the comparison in compat/ and are not exposed.
//
// The token types and boundaries reproduce the rules of Chroma 2.27.0's php.xml without regular expressions.
// In Chroma, both the language name php and the file name *.php use this PHP definition (without HTML).
// PHTML (lexers/php.go), which has PHP inside HTML, applies only to the language name phtml and is out of scope.
//
// php.xml is case-insensitive (case_insensitive), and . also matches a newline (dot_all).
//
// Intended difference from Chroma: in Chroma, the <? of <?php becomes an operator, and once ?> leaves no state,
// the rest becomes all Error. In fasthighlight, <?php and ?> are CommentPreproc, and the text from ?> to the
// next <? is read as HTML (as in PHTML). Setting Options.ChromaTags to true makes it the same as Chroma.
//
// Intended difference from Chroma: user-defined class names in type declarations (parameter types, return
// types and typed properties) are NameClass instead of NameOther (types.go). Options.ChromaTypes turns it off.
package php

import (
	"strings"
	"unicode"

	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/html"
	"github.com/toshi-kuji/fasthighlight/internal/js"
	"github.com/toshi-kuji/fasthighlight/internal/lexkit"
)

// Indexes of the word lists (lists.go).
const (
	lKeywords = iota
	lConstants
	lMagicConstants
	lMagicFuncs
)

var lists *lexkit.FoldWordLists

// Prepare builds the word tables and the like (the Prepare of fasthighlight.Language).
func Prepare() {
	lists = lexkit.NewFoldWordLists(keywords, constants, magicConstants, magicFuncs)
	html.Prepare()
}

// Options holds the switches for the intended differences from Chroma. The zero value is the production
// setting (the one lang/php registers).
type Options struct {
	// ChromaTags, if true, reads <?php and ?> the same as Chroma (<? is an operator, and everything after
	// ?> is Error).
	ChromaTags bool
	// ChromaTypes, if true, colors names in type declarations the same as Chroma (NameOther). In
	// production, a user-defined name in a parameter type (User $u, ?User $u, A|B $u, A&B $u), a return
	// type (): User, ): ?A|B) or a typed property (private User $u;) is NameClass. Built-in types (int,
	// string, ...) and keywords keep their own types.
	ChromaTypes bool
}

type state uint8

const (
	root state = iota
	str
	classname
	functionname
	variablevariable
	attribute
	attributeparams
)

type lexer struct {
	o     Options
	x     *lexkit.Tokens
	src   string
	stack []state
	words lexkit.FoldWordCache
	end   int // end of Lex
	// dollarFrom and dollarEnd are the last run of "$": any "$" inside it is answered without scanning
	// the run again.
	dollarFrom, dollarEnd int
	// typeEnd is the end of the type being read (production only): before it, a name is NameClass.
	// noTypeEnd is the end of the last scan that found no type, so names before it are not scanned again.
	typeEnd, noTypeEnd int
	// quoteFail is, for ' and ` (quoteFail[0] and [1]), where the last failed quoted scan stopped (0 if
	// none): a quote before it was read by that scan as an escaped character, so a scan from it would follow
	// the same path and fail too.
	quoteFail [2]int
}

// dollars returns the end of the run of "$" at i, remembering it so that a long run is scanned once.
func (l *lexer) dollars(i int) int {
	if l.dollarFrom <= i && i < l.dollarEnd {
		return l.dollarEnd
	}
	j := i
	for l.at(j) == '$' {
		j++
	}
	l.dollarFrom, l.dollarEnd = i, j
	return j
}

// Lex splits src into tokens (with the same contract as the Lex of fasthighlight.Language). compat/ uses it
// to compare with Chroma, switching o.
func Lex(src string, end int, o Options) []fh.Token {
	x := lexkit.NewTokens(src)
	lexInto(x, src, end, o)
	return x.Result()
}

func lexInto(x *lexkit.Tokens, src string, end int, o Options) {
	l := &lexer{o: o, x: x, src: src, end: end, stack: []state{root}, words: lexkit.FoldWordCache{Lists: lists}}
	i := 0
	for i < end && len(l.stack) > 0 {
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
	// Once no state is left, the rest (including the newline appended at the end) becomes a single Error.
	if len(l.stack) == 0 && i < len(src) {
		l.x.Emit(fh.Error, i, len(src))
	}
}

// nested reads src[s:e] from root as new input, the same as Chroma's usingself.
func (l *lexer) nested(s, e int) {
	if s < e {
		lexInto(l.x.Sub(l.src[s:e], s), l.src[s:e], e-s, l.o)
	}
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

func (l *lexer) spaces(i int) int { return lexkit.Span(l.src, i, lexkit.IsSpace) }

// fold returns the number of bytes if src[i:] matches word (lower case) case-insensitively.
func (l *lexer) fold(i int, word string) int { return lexkit.FoldPrefix(l.src, i, word) }

// ident returns the length of (?:[_a-z]|[^\x00-\x7f])(?:\w|[^\x00-\x7f])*. With backslash, it also includes \
// ((?:[\\_a-z]|[^\x00-\x7f])(?:[\\\w]|[^\x00-\x7f])*). Even with (?i), the characters that match do not change
// (K and ſ match [a-z], and non-ASCII characters other than those match [^\x00-\x7f]).
func (l *lexer) ident(i int, backslash bool) int {
	src := l.src
	if i >= len(src) {
		return 0
	}
	c := src[i]
	if !(c >= 0x80 || c == '_' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || backslash && c == '\\') {
		return 0
	}
	_, n := lexkit.Rune(src, i)
	j := i + n
	for j < len(src) {
		r, w := lexkit.Rune(src, j)
		if !(r >= 0x80 || lexkit.IsWord(r) || backslash && r == '\\') {
			break
		}
		j += w
	}
	return j - i
}

func (l *lexer) step(st state, i int) (int, bool) {
	src := l.src
	switch st {
	case root:
		return l.root(i)
	case str:
		return l.str(i)
	case classname:
		if n := l.ident(i, true); n > 0 {
			l.pop()
			return l.emit(fh.NameClass, i, i+n)
		}
		return 0, false
	case functionname:
		if n := l.words.Match(src, i, lMagicFuncs); n > 0 {
			return l.emit(fh.NameFunctionMagic, i, i+n)
		}
		l.pop()
		if n := l.ident(i, true); n > 0 {
			return l.emit(fh.NameFunction, i, i+n)
		}
		return 0, true
	case variablevariable:
		if src[i] == '}' {
			l.pop()
			return l.emit(fh.NameVariable, i, i+1)
		}
		return l.root(i)
	case attribute:
		switch src[i] {
		case ']':
			l.pop()
			return l.emit(fh.Punctuation, i, i+1)
		case '(':
			l.push(attributeparams)
			return l.emit(fh.Punctuation, i, i+1)
		}
		if n := l.ident(i, true); n > 0 {
			return l.emit(fh.NameDecorator, i, i+n)
		}
		return l.root(i)
	case attributeparams:
		if src[i] == ')' {
			l.pop()
			return l.emit(fh.Punctuation, i, i+1)
		}
		return l.root(i)
	}
	return 0, false
}

// quoted returns the length of q([^q\\]*(?:\\.[^q\\]*)*)q (0 if it does not match). src[i] is q.
func (l *lexer) quoted(i int, q byte) int {
	src := l.src
	fail := &l.quoteFail[0]
	if q == '`' {
		fail = &l.quoteFail[1]
	}
	if i < *fail {
		return 0
	}
	for j := i + 1; j < len(src); {
		switch src[j] {
		case q:
			return j + 1 - i
		case '\\':
			if j+1 >= len(src) {
				*fail = len(src)
				return 0
			}
			_, n := lexkit.Rune(src, j+1)
			j += 1 + n
			continue
		}
		j++
	}
	*fail = len(src)
	return 0
}

// heredoc tries (<<<)(['"]?)(ident)(\2\n.*?\n\s*)(\3)(;?)(\n). src[i:] is <<<.
func (l *lexer) heredoc(i int) (int, bool) {
	src := l.src
	j := i + 3
	var q byte
	if c := l.at(j); c == '\'' || c == '"' {
		q = c
		j++
	}
	n := l.ident(j, false)
	if n == 0 {
		return 0, false
	}
	name := src[j : j+n]
	k := j + n
	if q != 0 {
		if l.at(k) != q {
			return 0, false
		}
		k++
	}
	if l.at(k) != '\n' {
		return 0, false
	}
	// Try the \n after .*? starting from the nearest. \s* is tried starting from the longest (lexkit.Run).
	for m := k + 1; m <= len(src); {
		nl := strings.IndexByte(src[m:], '\n')
		if nl < 0 {
			break
		}
		p := m + nl // position of \n
		var close, after int
		if lexkit.Run(src, p+1, 0, lexkit.IsSpace, func(s int) bool {
			e := l.backref(s, name)
			if e < 0 || !(l.at(e) == '\n' || l.at(e) == ';' && l.at(e+1) == '\n') {
				return false
			}
			close, after = s, e
			return true
		}) {
			semi := after
			if l.at(after) == ';' {
				semi++
			}
			l.x.Emit(fh.LiteralString, i, i+3)
			if q != 0 {
				l.x.Emit(fh.LiteralString, j-1, j)
			}
			l.x.Emit(fh.LiteralStringDelimiter, j, j+n)
			l.x.Emit(fh.LiteralString, j+n, close)
			l.x.Emit(fh.LiteralStringDelimiter, close, after)
			l.x.Emit(fh.Punctuation, after, semi)
			l.x.Emit(fh.Text, semi, semi+1)
			return semi + 1 - i, true
		}
		m = p + 1
	}
	return 0, false
}

// backref returns the end if the start of src[s:] matches name (a backreference; compared with
// unicode.ToLower, as in regexp2), or -1 if it does not match.
func (l *lexer) backref(s int, name string) int {
	j := s
	for _, w := range name {
		if j >= len(l.src) {
			return -1
		}
		r, n := lexkit.Rune(l.src, j)
		if unicode.ToLower(r) != unicode.ToLower(w) {
			return -1
		}
		j += n
	}
	return j
}

// closeTag makes ?> a CommentPreproc and reads the text after it up to just before the next <? as HTML.
// The next <? is also made a CommentPreproc, and it returns to PHP (an intended difference from Chroma).
func (l *lexer) closeTag(i int) (int, bool) {
	src := l.src
	l.x.Emit(fh.CommentPreproc, i, i+2)
	s := i + 2
	e := len(src)
	if k := strings.Index(src[s:], "<?"); k >= 0 {
		e = s + k
	}
	if s < e {
		htmlEnd := min(e, l.end) - s
		html.LexInto(l.x.Sub(src[s:e], s), src[s:e], max(htmlEnd, 0), js.Options{})
	}
	if e >= len(src) || e >= l.end {
		// No <? (or it is after the newline appended at the end): finish here. HTML leaves nothing unread.
		return len(src) - i, true
	}
	n := 2 + l.fold(e+2, "php")
	l.x.Emit(fh.CommentPreproc, e, e+n)
	return e + n - i, true
}

func (l *lexer) root(i int) (int, bool) {
	src := l.src
	c := src[i]
	if c == '?' && l.at(i+1) == '>' { // \?>
		if !l.o.ChromaTags && len(l.stack) == 1 {
			return l.closeTag(i)
		}
		l.pop()
		return l.emit(fh.CommentPreproc, i, i+2)
	}
	if !l.o.ChromaTags && c == '<' && l.at(i+1) == '?' { // <\?(php)? (the same as PHTML)
		return l.emit(fh.CommentPreproc, i, i+2+l.fold(i+2, "php"))
	}
	if strings.HasPrefix(src[i:], "<<<") {
		if n, ok := l.heredoc(i); ok {
			return n, true
		}
	}
	if n := l.spaces(i); n > 0 {
		return l.emit(fh.Text, i, i+n)
	}
	if c == '#' {
		if l.at(i+1) == '[' {
			l.push(attribute)
			return l.emit(fh.Punctuation, i, i+2)
		}
		if k := strings.IndexByte(src[i:], '\n'); k >= 0 { // #.*?\n
			return l.emit(fh.CommentSingle, i, i+k+1)
		}
	}
	if c == '/' {
		if l.at(i+1) == '/' { // //.*?\n
			if k := strings.IndexByte(src[i:], '\n'); k >= 0 {
				return l.emit(fh.CommentSingle, i, i+k+1)
			}
		}
		if strings.HasPrefix(src[i:], "/**/") {
			return l.emit(fh.CommentMultiline, i, i+4)
		}
		if strings.HasPrefix(src[i:], "/**") { // /\*\*.*?\*/
			if k := strings.Index(src[i+3:], "*/"); k >= 0 {
				return l.emit(fh.LiteralStringDoc, i, i+3+k+2)
			}
		}
		if strings.HasPrefix(src[i:], "/*") { // /\*.*?\*/
			if k := strings.Index(src[i+2:], "*/"); k >= 0 {
				return l.emit(fh.CommentMultiline, i, i+2+k+2)
			}
		}
	}
	// (->|::)(\s*)(ident). A non-ASCII space (such as U+00A0) can also be a name character, so \s* backtracks.
	if strings.HasPrefix(src[i:], "->") || strings.HasPrefix(src[i:], "::") {
		var s, n int
		if lexkit.Run(src, i+2, 0, lexkit.IsSpace, func(e int) bool { s, n = e, l.ident(e, false); return n > 0 }) {
			l.x.Emit(fh.Operator, i, i+2)
			l.x.Emit(fh.Text, i+2, s)
			l.x.Emit(fh.NameAttribute, s, s+n)
			return s + n - i, true
		}
	}
	isOp := func(c byte) bool { return strings.IndexByte("~!%^&*+=|:.<>/@-", c) >= 0 }
	if isOp(c) {
		j := i + 1
		for j < len(src) && isOp(src[j]) {
			j++
		}
		if !l.o.ChromaTypes && j == i+1 && c == ':' {
			l.returnType(i)
		}
		return l.emit(fh.Operator, i, j)
	}
	if c == '?' {
		return l.emit(fh.Operator, i, i+1)
	}
	isPunct := func(c byte) bool { return strings.IndexByte("[]{}();,", c) >= 0 }
	if isPunct(c) {
		j := i + 1
		for j < len(src) && isPunct(src[j]) {
			j++
		}
		return l.emit(fh.Punctuation, i, j)
	}
	// (new)(\s+)(class)\b
	if n := l.fold(i, "new"); n > 0 {
		if s := l.spaces(i + n); s > 0 {
			if m := l.fold(i+n+s, "class"); m > 0 && lexkit.WordBoundary(src, i+n+s+m) {
				l.x.Emit(fh.Keyword, i, i+n)
				l.x.Emit(fh.Text, i+n, i+n+s)
				l.x.Emit(fh.Keyword, i+n+s, i+n+s+m)
				return n + s + m, true
			}
		}
	}
	// (class)(\s+)
	if n := l.fold(i, "class"); n > 0 {
		if s := l.spaces(i + n); s > 0 {
			l.x.Emit(fh.Keyword, i, i+n)
			l.x.Emit(fh.Text, i+n, i+n+s)
			l.push(classname)
			return n + s, true
		}
	}
	if n := l.fold(i, "function"); n > 0 {
		s := l.spaces(i + n)
		if l.at(i+n+s) == '(' { // (function)(\s*)(?=\()
			l.x.Emit(fh.Keyword, i, i+n)
			l.x.Emit(fh.Text, i+n, i+n+s)
			return n + s, true
		}
		if s > 0 { // (function)(\s+)(&?)(\s*)
			j := i + n + s
			a := 0
			if l.at(j) == '&' {
				a = 1
			}
			t := l.spaces(j + a)
			l.x.Emit(fh.Keyword, i, i+n)
			l.x.Emit(fh.Text, i+n, j)
			l.x.Emit(fh.Operator, j, j+a)
			l.x.Emit(fh.Text, j+a, j+a+t)
			l.push(functionname)
			return j + a + t - i, true
		}
	}
	// (const)(\s+)(ident)
	if n := l.fold(i, "const"); n > 0 {
		var s, m int
		if lexkit.Run(src, i+n, 1, lexkit.IsSpace, func(e int) bool { s, m = e, l.ident(e, true); return m > 0 }) {
			l.x.Emit(fh.Keyword, i, i+n)
			l.x.Emit(fh.Text, i+n, s)
			l.x.Emit(fh.NameConstant, s, s+m)
			return s + m - i, true
		}
	}
	if n := l.words.Match(src, i, lKeywords); n > 0 {
		return l.emit(fh.Keyword, i, i+n)
	}
	if n := l.words.Match(src, i, lConstants); n > 0 {
		return l.emit(fh.KeywordConstant, i, i+n)
	}
	if n := l.words.Match(src, i, lMagicConstants); n > 0 {
		return l.emit(fh.NameConstant, i, i+n)
	}
	if c == '$' {
		if l.at(i+1) == '{' { // \$\{
			l.push(variablevariable)
			return l.emit(fh.NameVariable, i, i+2)
		}
		j := l.dollars(i)
		if n := l.ident(j, true); n > 0 { // \$+ident
			return l.emit(fh.NameVariable, i, j+n)
		}
	}
	if n := l.ident(i, true); n > 0 {
		if !l.o.ChromaTypes && l.isType(i, i+n) {
			return l.emit(fh.NameClass, i, i+n)
		}
		return l.emit(fh.NameOther, i, i+n)
	}
	if e, t := l.number(i); e > 0 {
		return l.emit(t, i, e)
	}
	switch c {
	case '\'':
		if n := l.quoted(i, '\''); n > 0 {
			return l.emit(fh.LiteralStringSingle, i, i+n)
		}
	case '`':
		if n := l.quoted(i, '`'); n > 0 {
			return l.emit(fh.LiteralStringBacktick, i, i+n)
		}
	case '"':
		l.push(str)
		return l.emit(fh.LiteralStringDouble, i, i+1)
	}
	return 0, false
}

func isDec(c byte) bool { return '0' <= c && c <= '9' }

// number tries the number rules. If one matches, it returns the end and the kind.
func (l *lexer) number(i int) (int, fh.TokenType) {
	src := l.src
	digits := func(j int) int { return lexkit.Span(src, j, lexkit.IsDigit) }
	exp := func(j int) int { // length of (e[+-]?[0-9]+) (case-insensitive)
		if c := l.at(j); c != 'e' && c != 'E' {
			return 0
		}
		k := j + 1
		if c := l.at(k); (c == '+' || c == '-') && isDec(l.at(k+1)) {
			k++
		}
		if !isDec(l.at(k)) {
			return 0
		}
		for isDec(l.at(k)) {
			k++
		}
		return k - j
	}
	d := digits(i)
	// (\d+\.\d*|\d*\.\d+)(e[+-]?[0-9]+)?
	if d > 0 && l.at(i+d) == '.' {
		j := i + d + 1
		j += digits(j)
		return j + exp(j), fh.LiteralNumberFloat
	}
	if l.at(i+d) == '.' {
		if d2 := digits(i + d + 1); d2 > 0 {
			j := i + d + 1 + d2
			return j + exp(j), fh.LiteralNumberFloat
		}
	}
	if d == 0 {
		return 0, 0
	}
	if x := exp(i + d); x > 0 { // \d+e[+-]?[0-9]+
		return i + d + x, fh.LiteralNumberFloat
	}
	if src[i] == '0' {
		isOctU := func(c byte) bool { return '0' <= c && c <= '7' || c == '_' }
		k := i + 1 // 0o?[0-7_]+
		if (l.at(k) == 'o' || l.at(k) == 'O') && isOctU(l.at(k+1)) {
			k++
		}
		if isOctU(l.at(k)) {
			for isOctU(l.at(k)) {
				k++
			}
			return k, fh.LiteralNumberOct
		}
		isHexU := func(c byte) bool {
			return isDec(c) || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F' || c == '_'
		}
		if (l.at(i+1) == 'x' || l.at(i+1) == 'X') && isHexU(l.at(i+2)) { // 0x[a-f0-9_]+
			k := i + 2
			for isHexU(l.at(k)) {
				k++
			}
			return k, fh.LiteralNumberHex
		}
	}
	// \d[\d_]*
	return i + d + lexkit.Span(src, i+d, func(r rune) bool { return r == '_' || lexkit.IsDigit(r) }), fh.LiteralNumberInteger
}

// str tries the rules of the string state.
func (l *lexer) str(i int) (int, bool) {
	src := l.src
	c := src[i]
	if c == '"' {
		l.pop()
		return l.emit(fh.LiteralStringDouble, i, i+1)
	}
	if c != '{' && c != '$' && c != '\\' { // [^{$"\\]+
		j := i + 1
		for j < len(src) && strings.IndexByte("{$\"\\", src[j]) < 0 {
			j++
		}
		return l.emit(fh.LiteralStringDouble, i, j)
	}
	if c == '\\' { // \\([nrt"$\\]|[0-7]{1,3}|x[0-9a-f]{1,2})
		switch d := l.at(i + 1); {
		case d != 0 && strings.IndexByte("nrtNRT\"$\\", d) >= 0:
			return l.emit(fh.LiteralStringEscape, i, i+2)
		case '0' <= d && d <= '7':
			k := i + 1
			for k < i+4 && '0' <= l.at(k) && l.at(k) <= '7' {
				k++
			}
			return l.emit(fh.LiteralStringEscape, i, k)
		case d == 'x' || d == 'X':
			isHex := func(c byte) bool { return isDec(c) || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F' }
			if isHex(l.at(i + 2)) {
				k := i + 3
				if isHex(l.at(k)) {
					k++
				}
				return l.emit(fh.LiteralStringEscape, i, k)
			}
		}
		return l.emit(fh.LiteralStringDouble, i, i+1) // [${\\]
	}
	if c == '$' {
		// \$ident(\[\S+?\]|->ident)?
		if n := l.ident(i+1, false); n > 0 {
			e := i + 1 + n
			if l.at(e) == '[' {
				if k := l.bracket(e); k > 0 {
					return l.emit(fh.LiteralStringInterpol, i, e+k)
				}
			}
			if strings.HasPrefix(src[e:], "->") {
				if m := l.ident(e+2, false); m > 0 {
					return l.emit(fh.LiteralStringInterpol, i, e+2+m)
				}
			}
			return l.emit(fh.LiteralStringInterpol, i, e)
		}
	}
	if strings.HasPrefix(src[i:], "{${") { // (\{\$\{)(.*?)(\}\})
		if k := strings.Index(src[i+3:], "}}"); k >= 0 {
			e := i + 3 + k
			l.x.Emit(fh.LiteralStringInterpol, i, i+3)
			l.nested(i+3, e)
			l.x.Emit(fh.LiteralStringInterpol, e, e+2)
			return e + 2 - i, true
		}
	}
	if strings.HasPrefix(src[i:], "{$") { // (\{)(\$.*?)(\})
		if k := strings.IndexByte(src[i+2:], '}'); k >= 0 {
			e := i + 2 + k
			l.x.Emit(fh.LiteralStringInterpol, i, i+1)
			l.nested(i+1, e)
			l.x.Emit(fh.LiteralStringInterpol, e, e+1)
			return e + 1 - i, true
		}
	}
	if strings.HasPrefix(src[i:], "${") { // (\$\{)(\S+)(\})
		run := lexkit.Span(src, i+2, func(r rune) bool { return !lexkit.IsSpace(r) })
		if k := strings.LastIndexByte(src[i+2:i+2+run], '}'); k > 0 {
			e := i + 2 + k
			l.x.Emit(fh.LiteralStringInterpol, i, i+2)
			l.x.Emit(fh.NameVariable, i+2, e)
			l.x.Emit(fh.LiteralStringInterpol, e, e+1)
			return e + 1 - i, true
		}
	}
	return l.emit(fh.LiteralStringDouble, i, i+1) // [${\\]
}

// bracket returns the length if src[e:] matches \[\S+?\] (0 if it does not match). src[e] is [.
func (l *lexer) bracket(e int) int {
	src := l.src
	j := e + 1
	if j >= len(src) {
		return 0
	}
	if r, _ := lexkit.Rune(src, j); lexkit.IsSpace(r) {
		return 0
	}
	_, n := lexkit.Rune(src, j)
	for k := j + n; k < len(src); {
		r, w := lexkit.Rune(src, k)
		if r == ']' {
			return k + 1 - e
		}
		if lexkit.IsSpace(r) {
			return 0
		}
		k += w
	}
	return 0
}
