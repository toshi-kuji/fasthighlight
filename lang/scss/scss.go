// Package scss registers SCSS.
//
//	import _ "github.com/toshi-kuji/fasthighlight/lang/scss"
//
// Token types and boundaries reproduce the rules of Chroma 2.27.0's scss.xml without regular expressions.
// scss.xml is case-insensitive (case_insensitive), and . also matches a newline (dot_all). As with regexp2,
// letters are compared with lexkit.FoldEq. The word lists (lists.go) are copied from scss.xml. SCSS's rules
// share no rules or word lists with CSS's (css.xml), so internal/css is not used.
package scss

import (
	"strings"
	"unicode/utf8"

	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/lexkit"
)

// Language is SCSS. Its name, aliases, and filename patterns are listed in the catalog of the main
// package (fasthighlight.Builtin).
var Language = fh.Builtin("SCSS")

func init() {
	Language.Lex = lex
	Language.Prepare = prepare
	fh.Register(Language)
}

// entry is an alternative of the entities list and its index in the list.
type entry struct {
	word string
	idx  int
}

var (
	rootProps, valueProps, colorSet map[string]bool
	// entityIdx holds the entities made only of word characters (lowercased) and their index in the list.
	entityIdx map[string]int
	// entityIrr holds, by the first (lowercased) byte, the entities that contain non-word characters (such
	// as - or ;), in list order.
	entityIrr [utf8.RuneSelf][]entry
)

func set(alts string) map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Split(strings.ToLower(alts), "|") {
		m[w] = true
	}
	return m
}

func prepare() {
	rootProps = set(rootProperties)
	valueProps = set(valueProperties)
	colorSet = set(colors)
	entityIdx = map[string]int{}
	for idx, w := range strings.Split(strings.ToLower(entities), "|") {
		if lexkit.Span(w, 0, lexkit.IsWord) == len(w) {
			if _, ok := entityIdx[w]; !ok {
				entityIdx[w] = idx
			}
			continue
		}
		entityIrr[w[0]] = append(entityIrr[w[0]], entry{w, idx})
	}
}

// The states of Chroma's scss.xml.
type state uint8

const (
	root state = iota
	selector
	value
	attr
	interpolation
	each
	forLoop
	stringDouble
	stringSingle
	stringURL
	inlineComment
	pseudoClass
	class
	id
)

// finder remembers the result of a search for the first position q >= x that satisfies a condition
// depending only on q: there is no such position in [from, at), and at is one (or len(src)). Searches from
// increasing positions then take linear time in total.
type finder struct{ from, at int }

func newFinder() finder { return finder{1, 0} }

func (f *finder) find(src string, x int, match func(src string, q int) bool) int {
	if f.from <= x && x <= f.at {
		return f.at
	}
	q := x
	for q < len(src) && !match(src, q) {
		q++
	}
	f.from, f.at = x, q
	return q
}

func isNewline(src string, q int) bool { return src[q] == '\n' }
func isSep(src string, q int) bool     { return src[q] == ';' || src[q] == ',' }

// isBlockingBrace reports whether src[q] is a { not preceded by # (which (?:#{|[^{]) cannot pass).
func isBlockingBrace(src string, q int) bool { return src[q] == '{' && (q == 0 || src[q-1] != '#') }
func isCommentClose(src string, q int) bool {
	return src[q] == '*' && q+1 < len(src) && src[q+1] == '/'
}

type lexer struct {
	x     *lexkit.Tokens
	src   string
	stack []state
	base  state // the state the stack goes back to at a newline that matches no rule

	// The word of word characters at a position (wordPos is the position + 1).
	wordPos, wordN int
	wordASCII      bool
	wordKey        []byte
	propKey        []byte

	// Caches for the property rule without a word list and for comments.
	runFrom, runEnd       int
	lbPos, lbStart        int
	sep, block, nl, close finder
}

func newLexer(x *lexkit.Tokens, src string, base state) *lexer {
	return &lexer{x: x, src: src, stack: []state{base}, base: base, runFrom: 1, lbPos: -1,
		sep: newFinder(), block: newFinder(), nl: newFinder(), close: newFinder()}
}

func lex(src string, end int) []fh.Token {
	l := newLexer(lexkit.NewTokens(src), src, root)
	l.run(end)
	return l.x.Result()
}

// run reads src up to end. If the stack becomes empty (only when reading a part with usingself), the rest
// becomes one Error, as in Chroma.
func (l *lexer) run(end int) {
	src := l.src
	for i := 0; i < end; {
		if len(l.stack) == 0 {
			l.x.Emit(fh.Error, i, len(src))
			return
		}
		st := l.stack[len(l.stack)-1]
		n, ok := l.step(st, i)
		if !ok {
			if src[i] == '\n' && st != l.base {
				l.stack = append(l.stack[:0], l.base)
				continue
			}
			n = l.x.Error(i)
		}
		i += n
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

// isAZ is [a-z] under (?i) (A-Z, and the Kelvin sign and ſ too).
func isAZ(r rune) bool {
	f := lexkit.FoldASCII(r)
	return 'a' <= f && f <= 'z'
}

func isWordOrDash(r rune) bool { return lexkit.IsWord(r) || r == '-' }
func isASCIIDigit(r rune) bool { return '0' <= r && r <= '9' }
func isBlank(r rune) bool      { return r == ' ' || r == '\t' }
func isNotSpaceOrSemi(r rune) bool {
	return r != ';' && !lexkit.IsSpace(r)
}

// isAttrChar is [^ \t:=\"\[;(),.].
func isAttrChar(r rune) bool { return !strings.ContainsRune(" \t:=\"[;(),.", r) }

func spanBytes(src string, i int, chars string) int {
	j := i
	for j < len(src) && strings.IndexByte(chars, src[j]) >= 0 {
		j++
	}
	return j - i
}

func (l *lexer) step(st state, i int) (int, bool) {
	switch st {
	case root:
		return l.root(i)
	case selector:
		return l.selector(i)
	case value:
		return l.value(i)
	case attr:
		if l.interpolation(i) {
			return 2, true
		}
		if k := i + lexkit.Span(l.src, i, isBlank); l.at(k) == ':' {
			l.push(value)
			return l.emit(fh.Operator, i, k+1)
		}
		l.pop()
		return 0, true
	case interpolation:
		if l.src[i] == '}' {
			l.pop()
			return l.emit(fh.LiteralStringInterpol, i, i+1)
		}
		return l.value(i)
	case each:
		if n := lexkit.FoldPrefix(l.src, i, "in"); n > 0 {
			return l.emit(fh.OperatorWord, i, i+n)
		}
		return l.value(i)
	case forLoop:
		for _, w := range [...]string{"from", "to", "through"} {
			if n := lexkit.FoldPrefix(l.src, i, w); n > 0 {
				return l.emit(fh.OperatorWord, i, i+n)
			}
		}
		return l.value(i)
	case stringDouble:
		return l.quoted(i, '"', fh.LiteralStringDouble)
	case stringSingle:
		return l.quoted(i, '\'', fh.LiteralStringSingle)
	case stringURL:
		return l.url(i)
	case inlineComment:
		return l.comment(i)
	case pseudoClass:
		return l.name(i, fh.NameDecorator)
	case class:
		return l.name(i, fh.NameClass)
	case id:
		return l.name(i, fh.NameNamespace)
	}
	return 0, false
}

// interpolation tries #\{ (LiteralStringInterpol, push interpolation).
func (l *lexer) interpolation(i int) bool {
	if l.src[i] == '#' && l.at(i+1) == '{' {
		l.x.Emit(fh.LiteralStringInterpol, i, i+2)
		l.push(interpolation)
		return true
	}
	return false
}

// name tries the rules of the pseudo-class, class and id states: [\w-]+ as t, #\{, or pop.
func (l *lexer) name(i int, t fh.TokenType) (int, bool) {
	if n := lexkit.Span(l.src, i, isWordOrDash); n > 0 {
		return l.emit(t, i, i+n)
	}
	if l.interpolation(i) {
		return 2, true
	}
	l.pop()
	return 0, true
}

func (l *lexer) root(i int) (int, bool) {
	src := l.src
	c := src[i]
	switch c {
	case '{', '}', '(', ')':
		return l.emit(fh.Punctuation, i, i+1)
	}
	if n := lexkit.Span(src, i, lexkit.IsSpace); n > 0 {
		return l.emit(fh.Text, i, i+n)
	}
	if strings.HasPrefix(src[i:], "//") { // //.*?\n
		if q := l.nl.find(src, i+2, isNewline); q < len(src) {
			return l.emit(fh.CommentSingle, i, q+1)
		}
	}
	if strings.HasPrefix(src[i:], "/*") { // /\*.*?\*/
		if q := l.close.find(src, i+2, isCommentClose); q < len(src) {
			return l.emit(fh.CommentMultiline, i, q+2)
		}
	}
	if c == '@' {
		if n, ok := l.atRule(i); ok {
			return n, ok
		}
	}
	if c == '$' { // (\$[\w-]*\w)([ \t]*:)
		if r := lexkit.Span(src, i+1, isWordOrDash); r > 0 && src[i+r] != '-' {
			e := i + 1 + r
			if k := e + lexkit.Span(src, e, isBlank); l.at(k) == ':' {
				l.x.Emit(fh.NameVariable, i, e)
				l.x.Emit(fh.Operator, e, k+1)
				l.push(value)
				return k + 1 - i, true
			}
		}
	}
	if l.interpolation(i) {
		return 2, true
	}
	if e := l.property(i, rootProps); e > 0 {
		l.push(attr)
		return l.emit(fh.NameAttribute, i, e)
	}
	if e := l.anyProperty(i); e > 0 {
		l.push(attr)
		return l.emit(fh.NameAttribute, i, e)
	}
	l.push(selector)
	return 0, true
}

// atRule tries the root rules that start with @.
func (l *lexer) atRule(i int) (int, bool) {
	src := l.src
	if n := lexkit.FoldPrefix(src, i, "@import"); n > 0 {
		l.push(value)
		return l.emit(fh.Keyword, i, i+n)
	}
	if n, ok := l.use(i); ok {
		return n, ok
	}
	if n := lexkit.FoldPrefix(src, i, "@for"); n > 0 {
		l.push(forLoop)
		return l.emit(fh.Keyword, i, i+n)
	}
	if n := lexkit.FoldPrefix(src, i, "@each"); n > 0 {
		l.push(each)
		return l.emit(fh.Keyword, i, i+n)
	}
	for _, w := range [...]string{"@debug", "@warn", "@else if", "@if", "@while", "@return"} {
		if n := lexkit.FoldPrefix(src, i, w); n > 0 {
			l.push(value)
			return l.emit(fh.Keyword, i, i+n)
		}
	}
	// (@(?:mixin|function))( [\w-]+) and (@include)( [\w-]+)
	for _, w := range [...]string{"@mixin", "@function", "@include"} {
		if n := lexkit.FoldPrefix(src, i, w); n > 0 && l.at(i+n) == ' ' {
			if m := lexkit.Span(src, i+n+1, isWordOrDash); m > 0 {
				t := fh.NameFunction
				if w == "@include" {
					t = fh.NameDecorator
				}
				l.x.Emit(fh.Keyword, i, i+n)
				l.x.Emit(t, i+n, i+n+1+m)
				l.push(value)
				return n + 1 + m, true
			}
		}
	}
	if n := lexkit.FoldPrefix(src, i, "@extend"); n > 0 {
		l.push(selector)
		return l.emit(fh.Keyword, i, i+n)
	}
	if n := lexkit.FoldPrefix(src, i, "@media"); n > 0 {
		if m := lexkit.Span(src, i+n, lexkit.IsSpace); m > 0 {
			l.x.Emit(fh.Keyword, i, i+n)
			l.x.Emit(fh.Text, i+n, i+n+m)
			l.push(value)
			return n + m, true
		}
	}
	if n := lexkit.Span(src, i+1, isWordOrDash); n > 0 {
		l.push(selector)
		return l.emit(fh.Keyword, i, i+1+n)
	}
	return 0, false
}

// use tries (@(?:use|forward))(\s+)([^\s;]+)(?:(\s+)(as)(\s+)(\w+|\*))?(?:(\s+)(with))?. The third group
// is read in the selector state as a separate text (usingself).
func (l *lexer) use(i int) (int, bool) {
	src := l.src
	n := lexkit.FoldPrefix(src, i, "@use")
	if n == 0 {
		n = lexkit.FoldPrefix(src, i, "@forward")
	}
	if n == 0 {
		return 0, false
	}
	a := i + n
	b := a + lexkit.Span(src, a, lexkit.IsSpace)
	if b == a {
		return 0, false
	}
	e := b + lexkit.Span(src, b, isNotSpaceOrSemi)
	if e == b {
		return 0, false
	}
	l.x.Emit(fh.Keyword, i, a)
	l.x.Emit(fh.Text, a, b)
	sub := newLexer(l.x.Sub(src[b:e], b), src[b:e], selector)
	sub.run(e - b)
	k := e
	if s := k + lexkit.Span(src, k, lexkit.IsSpace); s > k {
		if m := lexkit.FoldPrefix(src, s, "as"); m > 0 {
			if t := s + m + lexkit.Span(src, s+m, lexkit.IsSpace); t > s+m {
				w := lexkit.Span(src, t, lexkit.IsWord)
				if w == 0 && l.at(t) == '*' {
					w = 1
				}
				if w > 0 {
					l.x.Emit(fh.Text, k, s)
					l.x.Emit(fh.Keyword, s, s+m)
					l.x.Emit(fh.Text, s+m, t)
					l.x.Emit(fh.NameNamespace, t, t+w)
					k = t + w
				}
			}
		}
	}
	if s := k + lexkit.Span(src, k, lexkit.IsSpace); s > k {
		if m := lexkit.FoldPrefix(src, s, "with"); m > 0 {
			l.x.Emit(fh.Text, k, s)
			l.x.Emit(fh.Keyword, s, s+m)
			k = s + m
		}
	}
	l.push(value)
	return k - i, true
}

// property tries \b(properties)(?=\s*:) and returns the end of the name (0 if it does not match). The
// names consist only of [a-z-], so the only possible match is the whole run of [a-z-] (under (?i)).
func (l *lexer) property(i int, props map[string]bool) int {
	src := l.src
	if !lexkit.WordBoundary(src, i) {
		return 0
	}
	l.propKey = l.propKey[:0]
	j := i
	for j < len(src) {
		r, n := lexkit.Rune(src, j)
		f := lexkit.FoldASCII(r)
		if !('a' <= f && f <= 'z' || f == '-') {
			break
		}
		l.propKey = append(l.propKey, byte(f))
		j += n
	}
	if j == i || !props[string(l.propKey)] {
		return 0
	}
	if k := j + lexkit.Span(src, j, lexkit.IsSpace); l.at(k) == ':' {
		return j
	}
	return 0
}

// anyProperty tries (?<!:\s*)[^ \t:=\"\[;(),.]+(?=[ \t]*:(?:#{|[^{])+?[;,]) and returns the end of the name
// (0 if it does not match). The name cannot contain space, tab or :, so the only possible match is the
// whole run. After the colon there must be a ; or , that comes after at least one character and before any
// { not preceded by #.
func (l *lexer) anyProperty(i int) int {
	src := l.src
	if !(l.runFrom <= i && i < l.runEnd) {
		l.runFrom, l.runEnd = i, i+lexkit.Span(src, i, isAttrChar)
	}
	e := l.runEnd
	if e == i {
		return 0
	}
	k := e + lexkit.Span(src, e, isBlank)
	if l.at(k) != ':' {
		return 0
	}
	s := k + 1
	if s >= len(src) {
		return 0
	}
	p := l.sep.find(src, s+1, isSep)
	if p >= len(src) || l.block.find(src, s, isBlockingBrace) < p {
		return 0
	}
	if l.afterColon(i) {
		return 0
	}
	return e
}

// afterColon reports whether position i is preceded by :\s* (the lookbehind (?<!:\s*) fails).
func (l *lexer) afterColon(i int) bool {
	src := l.src
	j := i
	for j > 0 {
		if j == l.lbPos {
			j = l.lbStart
			break
		}
		r, n := utf8.DecodeLastRuneInString(src[:j])
		if !lexkit.IsSpace(r) {
			break
		}
		j -= n
	}
	l.lbPos, l.lbStart = i, j
	return j > 0 && src[j-1] == ':'
}

func (l *lexer) selector(i int) (int, bool) {
	src := l.src
	c := src[i]
	switch c {
	case ' ', '\t':
		return l.emit(fh.Text, i, i+lexkit.Span(src, i, isBlank))
	case ':':
		l.push(pseudoClass)
		return l.emit(fh.NameDecorator, i, i+1)
	case '.':
		l.push(class)
		return l.emit(fh.NameClass, i, i+1)
	case '#':
		if l.interpolation(i) {
			return 2, true
		}
		l.push(id)
		return l.emit(fh.NameNamespace, i, i+1)
	case '&':
		return l.emit(fh.Keyword, i, i+1)
	}
	if c < utf8.RuneSelf && strings.IndexByte("~^*![]()<>|+=@,./?-", c) >= 0 {
		return l.emit(fh.Operator, i, i+1)
	}
	switch c {
	case '%':
		if n := lexkit.Span(src, i+1, isWordOrDash); n > 0 {
			l.x.Emit(fh.Operator, i, i+1)
			l.x.Emit(fh.NameClass, i+1, i+1+n)
			return 1 + n, true
		}
	case '"':
		l.push(stringDouble)
		return l.emit(fh.LiteralStringDouble, i, i+1)
	case '\'':
		l.push(stringSingle)
		return l.emit(fh.LiteralStringSingle, i, i+1)
	case '\n':
		return l.emit(fh.Text, i, i+1)
	case ';', '{', '}':
		l.pop()
		return l.emit(fh.Punctuation, i, i+1)
	}
	if n := lexkit.Span(src, i, isWordOrDash); n > 0 {
		return l.emit(fh.NameTag, i, i+n)
	}
	return 0, false
}

// word returns the length of the run of word characters at i, and whether it consists only of characters
// that fold to ASCII (wordKey then holds it lowercased).
func (l *lexer) word(i int) (int, bool) {
	if l.wordPos == i+1 {
		return l.wordN, l.wordASCII
	}
	src := l.src
	l.wordKey = l.wordKey[:0]
	ascii := true
	j := i
	for j < len(src) {
		r, n := lexkit.Rune(src, j)
		if !lexkit.IsWord(r) {
			break
		}
		if f := lexkit.FoldASCII(r); f < utf8.RuneSelf {
			l.wordKey = append(l.wordKey, byte(f))
		} else {
			ascii = false
		}
		j += n
	}
	l.wordPos, l.wordN, l.wordASCII = i+1, j-i, ascii
	return l.wordN, ascii
}

// entity tries \b(entities)\b (after the \b before it has been checked) and returns the length, or 0. As in
// the regular expression, the first alternative in list order that matches wins.
func (l *lexer) entity(i int) int {
	n, ascii := l.word(i)
	best := len(entities) // larger than any index
	if n > 0 && ascii {
		if k, ok := entityIdx[string(l.wordKey)]; ok {
			best = k
		}
	}
	r, _ := lexkit.Rune(l.src, i)
	if f := lexkit.FoldASCII(r); f < utf8.RuneSelf {
		for _, w := range entityIrr[f] {
			if w.idx > best {
				break
			}
			if p := lexkit.FoldPrefix(l.src, i, w.word); p > 0 && lexkit.WordBoundary(l.src, i+p) {
				return p
			}
		}
	}
	if best < len(entities) {
		return n
	}
	return 0
}

func (l *lexer) value(i int) (int, bool) {
	src := l.src
	c := src[i]
	if c == ' ' || c == '\t' {
		return l.emit(fh.Text, i, i+lexkit.Span(src, i, isBlank))
	}
	if c == '!' {
		for _, w := range [...]string{"!important", "!default", "!global"} {
			if n := lexkit.FoldPrefix(src, i, w); n > 0 {
				return l.emit(fh.Keyword, i, i+n)
			}
		}
	}
	if c == '!' || c == '$' {
		if n := lexkit.Span(src, i+1, isWordOrDash); n > 0 {
			return l.emit(fh.NameVariable, i, i+1+n)
		}
	}
	if n := lexkit.FoldPrefix(src, i, "url("); n > 0 {
		l.push(stringURL)
		return l.emit(fh.LiteralStringOther, i, i+n)
	}
	r, rn := lexkit.Rune(src, i)
	nameStart := isAZ(r) || c == '_' || c == '-'
	nameEnd := 0
	if nameStart { // [a-z_-][\w-]*(?=\()
		nameEnd = i + rn + lexkit.Span(src, i+rn, isWordOrDash)
		if l.at(nameEnd) == '(' {
			return l.emit(fh.NameFunction, i, nameEnd)
		}
	}
	if e := l.property(i, valueProps); e > 0 {
		return l.emit(fh.NameAttribute, i, e)
	}
	if e := l.anyProperty(i); e > 0 {
		return l.emit(fh.NameAttribute, i, e)
	}
	if lexkit.WordBoundary(src, i) {
		if n := l.entity(i); n > 0 {
			return l.emit(fh.NameEntity, i, i+n)
		}
		if n, ascii := l.word(i); n > 0 && ascii && colorSet[string(l.wordKey)] {
			return l.emit(fh.NameConstant, i, i+n)
		}
	}
	for _, w := range [...]string{"true", "false"} {
		if n := lexkit.FoldPrefix(src, i, w); n > 0 {
			return l.emit(fh.NamePseudo, i, i+n)
		}
	}
	for _, w := range [...]string{"and", "or", "not"} {
		if n := lexkit.FoldPrefix(src, i, w); n > 0 {
			return l.emit(fh.OperatorWord, i, i+n)
		}
	}
	if strings.HasPrefix(src[i:], "/*") {
		l.push(inlineComment)
		return l.emit(fh.CommentMultiline, i, i+2)
	}
	if strings.HasPrefix(src[i:], "//") {
		return l.emit(fh.CommentSingle, i, lexkit.LineEnd(src, i))
	}
	if c == '#' { // \#[a-z0-9]{1,6}
		j, count := i+1, 0
		for count < 6 && j < len(src) {
			r, n := lexkit.Rune(src, j)
			if !isAZ(r) && !isASCIIDigit(r) {
				break
			}
			j += n
			count++
		}
		if count > 0 {
			return l.emit(fh.LiteralNumberHex, i, j)
		}
	}
	if n, ok := l.number(i); ok {
		return n, ok
	}
	if l.interpolation(i) {
		return 2, true
	}
	if n := spanBytes(src, i, "~^*!&%<>|+=@:,./?-"); n > 0 {
		return l.emit(fh.Operator, i, i+n)
	}
	if n := spanBytes(src, i, "[]()"); n > 0 {
		return l.emit(fh.Punctuation, i, i+n)
	}
	switch c {
	case '"':
		l.push(stringDouble)
		return l.emit(fh.LiteralStringDouble, i, i+1)
	case '\'':
		l.push(stringSingle)
		return l.emit(fh.LiteralStringSingle, i, i+1)
	}
	if nameStart { // [a-z_-][\w-]*
		return l.emit(fh.Name, i, nameEnd)
	}
	switch c {
	case '\n':
		return l.emit(fh.Text, i, i+1)
	case ';', '{', '}':
		l.pop()
		return l.emit(fh.Punctuation, i, i+1)
	}
	return 0, false
}

// number tries (-)?(\d+)(\%|[a-z]+)? and (-?)(\d*\.\d+)(\%|[a-z]+)?.
func (l *lexer) number(i int) (int, bool) {
	src := l.src
	j := i
	if src[i] == '-' {
		j++
	}
	d := j + lexkit.Span(src, j, lexkit.IsDigit)
	t := fh.LiteralNumberInteger
	if d == j {
		// \d* is empty here: if there were digits, the first rule would have matched.
		if l.at(j) != '.' {
			return 0, false
		}
		d = j + 1 + lexkit.Span(src, j+1, lexkit.IsDigit)
		if d == j+1 {
			return 0, false
		}
		t = fh.LiteralNumberFloat
	}
	u := d
	if l.at(d) == '%' {
		u++
	} else {
		u += lexkit.Span(src, d, isAZ)
	}
	l.x.Emit(fh.Operator, i, j)
	l.x.Emit(t, j, d)
	l.x.Emit(fh.KeywordType, d, u)
	return u - i, true
}

// quoted tries the rules of the string-double and string-single states (q is the quote).
func (l *lexer) quoted(i int, q byte, t fh.TokenType) (int, bool) {
	src := l.src
	j := i
	for j < len(src) { // (\\.|#(?=[^\n{])|[^\n"#])+
		c := src[j]
		if c == '\\' && j+1 < len(src) {
			_, n := lexkit.Rune(src, j+1)
			j += 1 + n
			continue
		}
		if c == '#' {
			if j+1 < len(src) && src[j+1] != '\n' && src[j+1] != '{' {
				j++
				continue
			}
			break
		}
		if c == '\n' || c == q {
			break
		}
		_, n := lexkit.Rune(src, j)
		j += n
	}
	if j > i {
		return l.emit(t, i, j)
	}
	if l.interpolation(i) {
		return 2, true
	}
	if src[i] == q {
		l.pop()
		return l.emit(t, i, i+1)
	}
	return 0, false
}

// url tries the rules of the string-url state.
func (l *lexer) url(i int) (int, bool) {
	src := l.src
	j := i
	for j < len(src) { // (\\#|#(?=[^\n{])|[^\n#)])+
		c := src[j]
		if c == '\\' && j+1 < len(src) && src[j+1] == '#' {
			j += 2
			continue
		}
		if c == '#' {
			if j+1 < len(src) && src[j+1] != '\n' && src[j+1] != '{' {
				j++
				continue
			}
			break
		}
		if c == '\n' || c == ')' {
			break
		}
		_, n := lexkit.Rune(src, j)
		j += n
	}
	if j > i {
		return l.emit(fh.LiteralStringOther, i, j)
	}
	if l.interpolation(i) {
		return 2, true
	}
	if src[i] == ')' {
		l.pop()
		return l.emit(fh.LiteralStringOther, i, i+1)
	}
	return 0, false
}

// comment tries the rules of the inline-comment state.
func (l *lexer) comment(i int) (int, bool) {
	src := l.src
	j := i
loop:
	for j < len(src) { // (\\#|#(?=[^{])|\*(?=[^/])|[^#*])+
		switch c := src[j]; {
		case c == '\\' && j+1 < len(src) && src[j+1] == '#':
			j += 2
		case c == '#':
			if j+1 >= len(src) || src[j+1] == '{' {
				break loop
			}
			j++
		case c == '*':
			if j+1 >= len(src) || src[j+1] == '/' {
				break loop
			}
			j++
		default:
			_, n := lexkit.Rune(src, j)
			j += n
		}
	}
	if j > i {
		return l.emit(fh.CommentMultiline, i, j)
	}
	if l.interpolation(i) {
		return 2, true
	}
	if strings.HasPrefix(src[i:], "*/") {
		l.pop()
		return l.emit(fh.Comment, i, i+2)
	}
	return 0, false
}
