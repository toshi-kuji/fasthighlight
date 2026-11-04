// Package css is the CSS tokenizer. It is used by lang/css (CSS) and internal/html (inside <style>).
//
// The token types and boundaries reproduce the rules of Chroma 2.27.0's css.xml without regular
// expressions. The word lists (lists.go) are copied from css.xml.
package css

import (
	"strings"
	"sync"

	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/lexkit"
)

// wordList holds the alternatives (a|b|…) of a regular expression in the same order, split by first byte.
type wordList struct{ byFirst [256][]string }

func newWordList(alts string) *wordList {
	w := &wordList{}
	for _, s := range strings.Split(alts, "|") {
		w.byFirst[s[0]] = append(w.byFirst[s[0]], s)
	}
	return w
}

// prefix returns the length of the first alternative that matches the start of src[i:]. If boundary is
// true, only alternatives followed by a \b qualify.
func (w *wordList) prefix(src string, i int, boundary bool) int {
	for _, s := range w.byFirst[src[i]] {
		if strings.HasPrefix(src[i:], s) && (!boundary || lexkit.WordBoundary(src, i+len(s))) {
			return len(s)
		}
	}
	return 0
}

func set(alts string) map[string]bool {
	m := map[string]bool{}
	for _, s := range strings.Split(alts, "|") {
		m[s] = true
	}
	return m
}

var (
	vendors, constants1, constants2, colors, keywords, unitList *wordList
	builtins, properties                                        map[string]bool
	prepareOnce                                                 sync.Once
)

// Prepare builds the word lists. From the second call on it does nothing. Lex and LexInto also call it first.
func Prepare() { prepareOnce.Do(prepare) }

func prepare() {
	vendors = newWordList(vendorPrefixes)
	constants1 = newWordList(valueConstants1)
	constants2 = newWordList(valueConstants2)
	colors = newWordList(colorNames)
	keywords = newWordList(valueKeywords)
	unitList = newWordList(units)
	builtins = set(builtinFunctions)
	properties = set(propertyNames)
}

// The states of Chroma's css.xml.
type state int

const (
	root          state = iota
	content             // inside { … } (property names)
	valueStart          // a property value
	functionStart       // inside a function's ( … )
	atParenthesis       // ( … ) inside a function
	atRule              // an @ rule
	atContent           // the { … } of an @ rule
	numericEnd          // the unit after a number
)

type lexer struct {
	x     *lexkit.Tokens
	src   string
	stack []state
}

// Lex splits src into tokens (same contract as Lex in fasthighlight.Language).
func Lex(src string, end int) []fh.Token {
	x := lexkit.NewTokens(src)
	LexInto(x, src, end)
	return x.Result()
}

// LexInto splits src into tokens and adds them to x. x's src is either the same as src or was made with
// x.Sub.
func LexInto(x *lexkit.Tokens, src string, end int) {
	Prepare()
	l := &lexer{x: x, src: src, stack: []state{root}}
	for i := 0; i < end && len(l.stack) > 0; {
		st := l.stack[len(l.stack)-1]
		n, ok := l.step(st, i)
		if !ok {
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

func (l *lexer) emit(t fh.TokenType, s, e int) (int, bool) { l.x.Emit(t, s, e); return e - s, true }
func (l *lexer) push(s state)                              { l.stack = append(l.stack, s) }
func (l *lexer) pop(n int)                                 { l.stack = l.stack[:len(l.stack)-n] }

// step tries the rules of state st in order. If one matches, it returns the number of bytes advanced
// (0 is possible) and true.
func (l *lexer) step(st state, i int) (int, bool) {
	src := l.src
	c := src[i]
	switch st {
	case root:
		return l.basics(i)
	case content:
		return l.content(i)
	case valueStart:
		return l.valueStart(i)
	case functionStart, atParenthesis:
		return l.function(i)
	case atRule:
		switch c {
		case '{':
			l.push(atContent)
			return l.emit(fh.Punctuation, i, i+1)
		case ';':
			l.pop(1)
			return l.emit(fh.Punctuation, i, i+1)
		}
		return l.basics(i)
	case atContent:
		if n, ok := l.basics(i); ok {
			return n, ok
		}
		if c == '}' {
			l.pop(2)
			return l.emit(fh.Punctuation, i, i+1)
		}
	case numericEnd:
		if n := unitList.prefix(src, i, true); n > 0 {
			return l.emit(fh.KeywordType, i, i+n)
		}
		if c == '%' {
			return l.emit(fh.KeywordType, i, i+1)
		}
		l.pop(1) // if nothing matches, return without reading anything
		return 0, true
	}
	return 0, false
}

func isWordOrDash(r rune) bool { return lexkit.IsWord(r) || r == '-' }

// basics tries the rules of the basics state.
func (l *lexer) basics(i int) (int, bool) {
	src := l.src
	c := src[i]
	if n := lexkit.Span(src, i, lexkit.IsSpace); n > 0 {
		return l.emit(fh.Text, i, i+n)
	}
	if e := commentEnd(src, i); e > 0 {
		return l.emit(fh.Comment, i, e)
	}
	switch c {
	case '{':
		l.push(content)
		return l.emit(fh.Punctuation, i, i+1)
	case ':': // (\:{1,2})([\w-]+)
		j := i + 1
		if j < len(src) && src[j] == ':' {
			j++
		}
		if n := lexkit.Span(src, j, isWordOrDash); n > 0 {
			l.x.Emit(fh.Punctuation, i, j)
			l.x.Emit(fh.NameDecorator, j, j+n)
			return j + n - i, true
		}
	case '.', '#', '@': // (\.)([\w-]+), (\#)([\w-]+), (@)([\w-]+)
		if n := lexkit.Span(src, i+1, isWordOrDash); n > 0 {
			l.x.Emit(fh.Punctuation, i, i+1)
			t := fh.NameClass
			switch c {
			case '#':
				t = fh.NameNamespace
			case '@':
				t = fh.Keyword
				l.push(atRule)
			}
			l.x.Emit(t, i+1, i+1+n)
			return 1 + n, true
		}
	}
	if n := lexkit.Span(src, i, isWordOrDash); n > 0 { // [\w-]+
		return l.emit(fh.NameTag, i, i+n)
	}
	if strings.IndexByte("~^*!%&$[]()<>|+=@:;,./?-", c) >= 0 {
		return l.emit(fh.Operator, i, i+1)
	}
	return l.strings(i)
}

// strings tries "(\\\\|\\"|[^"])*" and '(\\\\|\\'|[^'])*'.
func (l *lexer) strings(i int) (int, bool) {
	switch c := l.src[i]; c {
	case '"':
		if n := lexkit.QuotedLen(l.src, i, '"'); n > 0 {
			return l.emit(fh.LiteralStringDouble, i, i+n)
		}
	case '\'':
		if n := lexkit.QuotedLen(l.src, i, '\''); n > 0 {
			return l.emit(fh.LiteralStringSingle, i, i+n)
		}
	}
	return 0, false
}

// commentEnd returns the end of the part matched by /\*(?:.|\n)*?\*/, or 0 if it does not match.
func commentEnd(src string, i int) int {
	if !strings.HasPrefix(src[i:], "/*") {
		return 0
	}
	if k := strings.Index(src[i+2:], "*/"); k >= 0 {
		return i + 2 + k + 2
	}
	return 0
}

// content tries the rules of the content state.
func (l *lexer) content(i int) (int, bool) {
	src := l.src
	c := src[i]
	if n := lexkit.Span(src, i, lexkit.IsSpace); n > 0 {
		return l.emit(fh.Text, i, i+n)
	}
	switch {
	case c == '}':
		l.pop(1)
		return l.emit(fh.Punctuation, i, i+1)
	case c == ';':
		return l.emit(fh.Punctuation, i, i+1)
	case c == '@' && (i == 0 || src[i-1] == '\n'): // ^@.*?$
		return l.emit(fh.CommentPreproc, i, lexkit.LineEnd(src, i))
	}
	if n := vendors.prefix(src, i, false); n > 0 {
		return l.emit(fh.KeywordPseudo, i, i+n)
	}
	// (property name)(\s*)(\:). A property name consists only of [a-z-], so the only possible match is
	// the whole run of [a-z-].
	if j := i + lexkit.Span(src, i, isLowerOrDash); j > i && properties[src[i:j]] {
		if n, ok := l.colon(i, j, fh.Keyword); ok {
			return n, ok
		}
	}
	// (--[a-zA-Z_][\w-]*)(\s*)(\:)
	if strings.HasPrefix(src[i:], "--") && i+2 < len(src) && isNameStart(src[i+2]) {
		if n, ok := l.colon(i, i+3+lexkit.Span(src, i+3, isWordOrDash), fh.NameVariable); ok {
			return n, ok
		}
	}
	// ([a-zA-Z_][\w-]*)(\s*)(\:)
	if isNameStart(c) {
		if n, ok := l.colon(i, i+1+lexkit.Span(src, i+1, isWordOrDash), fh.Name); ok {
			return n, ok
		}
	}
	if e := commentEnd(src, i); e > 0 {
		return l.emit(fh.Comment, i, e)
	}
	return 0, false
}

// colon, if the name src[i:j] is followed by \s*:, makes the name type t, the whitespace Text and the :
// Punctuation, and enters value-start.
func (l *lexer) colon(i, j int, t fh.TokenType) (int, bool) {
	s := j + lexkit.Span(l.src, j, lexkit.IsSpace)
	if s >= len(l.src) || l.src[s] != ':' {
		return 0, false
	}
	l.x.Emit(t, i, j)
	l.x.Emit(fh.Text, j, s)
	l.x.Emit(fh.Punctuation, s, s+1)
	l.push(valueStart)
	return s + 1 - i, true
}

func isLowerOrDash(r rune) bool { return r >= 'a' && r <= 'z' || r == '-' }
func isNameStart(c byte) bool   { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_' }
func isFuncChar(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-'
}

// commonValues tries the rules of the common-values state.
func (l *lexer) commonValues(i int) (int, bool) {
	src := l.src
	if n := lexkit.Span(src, i, lexkit.IsSpace); n > 0 {
		return l.emit(fh.Text, i, i+n)
	}
	if n := vendors.prefix(src, i, false); n > 0 {
		return l.emit(fh.KeywordPseudo, i, i+n)
	}
	if n, ok := l.url(i); ok {
		return n, ok
	}
	// (built-in function)(\(). A function name consists only of [A-Za-z0-9-], so the only possible match is
	// the whole run.
	if j := i + lexkit.Span(src, i, isFuncChar); j > i && j < len(src) && src[j] == '(' && builtins[src[i:j]] {
		l.x.Emit(fh.NameBuiltin, i, j)
		l.x.Emit(fh.Punctuation, j, j+1)
		l.push(functionStart)
		return j + 1 - i, true
	}
	// ([a-zA-Z_][\w-]+)(\()
	if isNameStart(src[i]) {
		if n := lexkit.Span(src, i+1, isWordOrDash); n > 0 && i+1+n < len(src) && src[i+1+n] == '(' {
			j := i + 1 + n
			l.x.Emit(fh.NameFunction, i, j)
			l.x.Emit(fh.Punctuation, j, j+1)
			l.push(functionStart)
			return j + 1 - i, true
		}
	}
	for _, w := range [...]*wordList{constants1, constants2, colors} {
		if n := w.prefix(src, i, true); n > 0 {
			return l.emit(fh.KeywordConstant, i, i+n)
		}
	}
	return 0, false
}

// url tries the urls state rule (url)(\()("…"|'…'|…)(\)) (inside the quotes, and the …, contain no
// newline).
func (l *lexer) url(i int) (int, bool) {
	src := l.src
	if !strings.HasPrefix(src[i:], "url(") {
		return 0, false
	}
	o := i + 4
	le := lexkit.LineEnd(src, o)
	emit := func(t fh.TokenType, close int) (int, bool) {
		l.x.Emit(fh.NameBuiltin, i, i+3)
		l.x.Emit(fh.Punctuation, i+3, o)
		l.x.Emit(t, o, close)
		l.x.Emit(fh.Punctuation, close, close+1)
		return close + 1 - i, true
	}
	for _, q := range [...]byte{'"', '\''} {
		if o < le && src[o] == q {
			for k := o + 1; k < le; k++ {
				if src[k] == q && k+1 < len(src) && src[k+1] == ')' {
					t := fh.LiteralStringDouble
					if q == '\'' {
						t = fh.LiteralStringSingle
					}
					return emit(t, k+1)
				}
			}
		}
	}
	if k := strings.IndexByte(src[o:le], ')'); k >= 0 {
		return emit(fh.LiteralStringOther, o+k)
	}
	return 0, false
}

// numeric tries the rules of the numeric-values state.
func (l *lexer) numeric(i int) (int, bool) {
	src := l.src
	c := src[i]
	if c == '#' { // \#[a-zA-Z0-9]{1,6}
		n := 0
		for n < 6 && i+1+n < len(src) && isASCIIAlnum(src[i+1+n]) {
			n++
		}
		if n > 0 {
			return l.emit(fh.LiteralNumberHex, i, i+1+n)
		}
		return 0, false
	}
	j := i
	if c == '+' || c == '-' {
		j++
	}
	d := asciiDigits(src, j)
	if d < len(src) && src[d] == '.' { // [+\-]?[0-9]*[.][0-9]+
		if e := asciiDigits(src, d+1); e > d+1 {
			l.push(numericEnd)
			return l.emit(fh.LiteralNumberFloat, i, e)
		}
	}
	if d > j { // [+\-]?[0-9]+
		l.push(numericEnd)
		return l.emit(fh.LiteralNumberInteger, i, d)
	}
	return 0, false
}

func asciiDigits(src string, j int) int {
	for j < len(src) && src[j] >= '0' && src[j] <= '9' {
		j++
	}
	return j
}

func isASCIIAlnum(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// valueStart tries the rules of the value-start state.
func (l *lexer) valueStart(i int) (int, bool) {
	src := l.src
	if n, ok := l.commonValues(i); ok {
		return n, ok
	}
	if n := keywords.prefix(src, i, true); n > 0 {
		return l.emit(fh.Keyword, i, i+n)
	}
	if strings.HasPrefix(src[i:], "!important") {
		return l.emit(fh.CommentPreproc, i, i+len("!important"))
	}
	if e := commentEnd(src, i); e > 0 {
		return l.emit(fh.Comment, i, e)
	}
	if n, ok := l.numeric(i); ok {
		return n, ok
	}
	if n := spanBytes(src, i, "~^*!%&<>|+=@:./?-"); n > 0 {
		return l.emit(fh.Operator, i, i+n)
	}
	if n := spanBytes(src, i, "[](),"); n > 0 {
		return l.emit(fh.Punctuation, i, i+n)
	}
	if n, ok := l.strings(i); ok {
		return n, ok
	}
	c := src[i]
	if isNameStart(c) { // [a-zA-Z_][\w-]*
		return l.emit(fh.Name, i, i+1+lexkit.Span(src, i+1, isWordOrDash))
	}
	switch c {
	case ';':
		l.pop(1)
		return l.emit(fh.Punctuation, i, i+1)
	case '}':
		l.pop(2)
		return l.emit(fh.Punctuation, i, i+1)
	}
	return 0, false
}

func spanBytes(src string, i int, chars string) int {
	j := i
	for j < len(src) && strings.IndexByte(chars, src[j]) >= 0 {
		j++
	}
	return j - i
}

// function tries the rules of the function-start and atparenthesis states (both have the same rules).
func (l *lexer) function(i int) (int, bool) {
	src := l.src
	if n, ok := l.commonValues(i); ok {
		return n, ok
	}
	if e := commentEnd(src, i); e > 0 {
		return l.emit(fh.Comment, i, e)
	}
	if n, ok := l.numeric(i); ok {
		return n, ok
	}
	c := src[i]
	switch c {
	case '*', '+', '/', '-':
		return l.emit(fh.Operator, i, i+1)
	case ',':
		return l.emit(fh.Punctuation, i, i+1)
	}
	if n, ok := l.strings(i); ok {
		return n, ok
	}
	if isNameStart(c) || c == '-' { // [a-zA-Z_-]\w*
		return l.emit(fh.Name, i, i+1+lexkit.Span(src, i+1, lexkit.IsWord))
	}
	switch c {
	case '(':
		l.push(atParenthesis)
		return l.emit(fh.Punctuation, i, i+1)
	case ')':
		l.pop(1)
		return l.emit(fh.Punctuation, i, i+1)
	}
	return 0, false
}
