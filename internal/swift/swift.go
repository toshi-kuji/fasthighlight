// Package swift is the Swift tokenizer. lang/swift registers it. Options (the switches for the intended
// differences from Chroma) exist only for the comparison in compat/ and are not exposed.
//
// The token types and boundaries reproduce the rules of Chroma 2.27.0's swift.xml without regular expressions.
package swift

import (
	"strings"
	"unicode"
	"unicode/utf8"

	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/lexkit"
)

// Indexes of the word lists (lists.go).
const (
	lKeywords = iota
	lReserved
	lConstants
	lDeclarations
	lBuiltins
	lBuiltinPseudo
)

var lists *lexkit.WordLists

// Prepare builds the word tables and the like (the Prepare of fasthighlight.Language).
func Prepare() {
	lists = lexkit.NewWordLists(keywords, reserved, constants, declarations, builtins, builtinPseudo)
}

type state uint8

const (
	root state = iota
	commentSingle
	commentMulti
	preproc
	module
	str
	strIntp
)

// Options holds the switches for the intended differences from Chroma. The zero value is the production
// setting (the one lang/swift registers).
type Options struct {
	// ChromaNames, if true, makes the start of a name the same as Chroma's [a-zA-Z_] (a name starting with a
	// non-letter character becomes Error). If false, a name can also start with a Unicode letter.
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
func (l *lexer) push(s state)                              { l.stack = append(l.stack, s) }
func (l *lexer) pop()                                      { l.stack = l.stack[:len(l.stack)-1] }

func (l *lexer) at(i int) byte {
	if i < len(l.src) {
		return l.src[i]
	}
	return 0
}

func isNameStart(c byte) bool { return c == '_' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' }

// name returns the length of [a-zA-Z_]\w*. If Options.ChromaNames is false, it can also start with a
// Unicode letter.
func (l *lexer) name(i int) int {
	if i >= len(l.src) {
		return 0
	}
	r, n := lexkit.Rune(l.src, i)
	if !(r < utf8.RuneSelf && isNameStart(byte(r)) || !l.o.ChromaNames && r >= utf8.RuneSelf && unicode.IsLetter(r)) {
		return 0
	}
	return n + lexkit.Span(l.src, i+n, lexkit.IsWord)
}

func (l *lexer) step(st state, i int) (int, bool) {
	src := l.src
	switch st {
	case root:
		return l.root(i)
	case commentSingle:
		if src[i] == '\n' {
			l.pop()
			return l.emit(fh.Text, i, i+1)
		}
		if n := l.special(i); n > 0 {
			return l.emit(fh.CommentSpecial, i, i+n)
		}
		_, n := lexkit.Rune(src, i) // [^\n]
		return l.emit(fh.CommentSingle, i, i+n)
	case commentMulti:
		if n := l.special(i); n > 0 {
			return l.emit(fh.CommentSpecial, i, i+n)
		}
		switch c := src[i]; {
		case c != '*' && c != '/': // [^*/]
			_, n := lexkit.Rune(src, i)
			return l.emit(fh.CommentMultiline, i, i+n)
		case c == '/' && l.at(i+1) == '*':
			l.push(commentMulti)
			return l.emit(fh.CommentMultiline, i, i+2)
		case c == '*' && l.at(i+1) == '/':
			l.pop()
			return l.emit(fh.CommentMultiline, i, i+2)
		}
		return l.emit(fh.CommentMultiline, i, i+1) // [*/]
	case preproc:
		if src[i] == '\n' {
			l.pop()
			return l.emit(fh.Text, i, i+1)
		}
		if n, ok := l.keywords(i); ok {
			return n, true
		}
		if c := src[i]; 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' { // [A-Za-z]\w*
			return l.emit(fh.CommentPreproc, i, i+1+lexkit.Span(src, i+1, lexkit.IsWord))
		}
		return l.root(i)
	case module:
		if src[i] == '\n' {
			l.pop()
			return l.emit(fh.Text, i, i+1)
		}
		if n := l.name(i); n > 0 {
			return l.emit(fh.NameClass, i, i+n)
		}
		return l.root(i)
	case str:
		return l.str(i)
	case strIntp:
		switch src[i] {
		case '(':
			l.push(strIntp)
			return l.emit(fh.LiteralStringInterpol, i, i+1)
		case ')':
			l.pop()
			return l.emit(fh.LiteralStringInterpol, i, i+1)
		}
		return l.root(i)
	}
	return 0, false
}

// special returns the length of the comment rule :param: [a-zA-Z_]\w*|:returns?:|(FIXME|MARK|TODO):.
func (l *lexer) special(i int) int {
	src := l.src[i:]
	switch {
	case strings.HasPrefix(src, ":param: "):
		if n := l.name(i + 8); n > 0 {
			return 8 + n
		}
	case strings.HasPrefix(src, ":return:"):
		return 8
	case strings.HasPrefix(src, ":returns:"):
		return 9
	}
	for _, w := range [...]string{"FIXME:", "MARK:", "TODO:"} {
		if strings.HasPrefix(src, w) {
			return len(w)
		}
	}
	return 0
}

func isHex(c byte) bool {
	return '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F'
}
func isOct(c byte) bool { return '0' <= c && c <= '7' }
func isDec(c byte) bool { return '0' <= c && c <= '9' }

func (l *lexer) count(j, max int, f func(byte) bool) int {
	n := 0
	for n < max && j+n < len(l.src) && f(l.src[j+n]) {
		n++
	}
	return n
}

// str tries the rules of the string state.
func (l *lexer) str(i int) (int, bool) {
	src := l.src
	switch src[i] {
	case '\\':
		switch c := l.at(i + 1); {
		case c == '(': // \\\(
			l.push(strIntp)
			return l.emit(fh.LiteralStringInterpol, i, i+2)
		case c != 0 && strings.IndexByte("'\"\\nrt", c) >= 0:
			return l.emit(fh.LiteralStringEscape, i, i+2)
		case c == 'x' && l.count(i+2, 2, isHex) == 2:
			return l.emit(fh.LiteralStringEscape, i, i+4)
		case isOct(c):
			return l.emit(fh.LiteralStringEscape, i, i+1+l.count(i+1, 3, isOct))
		case c == 'u' && l.count(i+2, 4, isHex) == 4:
			return l.emit(fh.LiteralStringEscape, i, i+6)
		case c == 'U' && l.count(i+2, 8, isHex) == 8:
			return l.emit(fh.LiteralStringEscape, i, i+10)
		}
		return l.emit(fh.LiteralString, i, i+1)
	case '"':
		l.pop()
		return l.emit(fh.LiteralString, i, i+1)
	}
	j := i + 1
	for j < len(src) && src[j] != '\\' && src[j] != '"' {
		j++
	}
	return l.emit(fh.LiteralString, i, j)
}

// keywords tries the rules of the keywords state.
func (l *lexer) keywords(i int) (int, bool) {
	src := l.src
	if n := l.words.Match(src, i, lKeywords); n > 0 {
		return l.emit(fh.Keyword, i, i+n)
	}
	// @availability\([^)]+\)
	if strings.HasPrefix(src[i:], "@availability(") {
		if k := strings.IndexByte(src[i+14:], ')'); k > 0 {
			return l.emit(fh.KeywordReserved, i, i+14+k+1)
		}
	}
	if n := l.words.Match(src, i, lReserved); n > 0 {
		return l.emit(fh.KeywordReserved, i, i+n)
	}
	if n := l.words.Match(src, i, lConstants); n > 0 {
		return l.emit(fh.KeywordConstant, i, i+n)
	}
	// import\b
	if strings.HasPrefix(src[i:], "import") && lexkit.WordBoundary(src, i+6) {
		l.push(module)
		return l.emit(fh.KeywordDeclaration, i, i+6)
	}
	// (class|enum|extension|struct|protocol)(\s+)([a-zA-Z_]\w*) and so on
	for _, r := range [...]struct {
		w string
		t fh.TokenType
	}{{"class", fh.NameClass}, {"enum", fh.NameClass}, {"extension", fh.NameClass}, {"struct", fh.NameClass},
		{"protocol", fh.NameClass}, {"func", fh.NameFunction}, {"var", fh.NameVariable}, {"let", fh.NameVariable}} {
		if strings.HasPrefix(src[i:], r.w) {
			k := i + len(r.w)
			if n := lexkit.Span(src, k, lexkit.IsSpace); n > 0 {
				if m := l.name(k + n); m > 0 {
					l.x.Emit(fh.KeywordDeclaration, i, k)
					l.x.Emit(fh.Text, k, k+n)
					l.x.Emit(r.t, k+n, k+n+m)
					return k + n + m - i, true
				}
			}
		}
	}
	if n := l.words.Match(src, i, lDeclarations); n > 0 {
		return l.emit(fh.KeywordDeclaration, i, i+n)
	}
	return 0, false
}

func (l *lexer) run(j int, f func(byte) bool) int {
	for j < len(l.src) && f(l.src[j]) {
		j++
	}
	return j
}

func isDecU(c byte) bool { return isDec(c) || c == '_' }

// number tries the number rules. If one matches, it returns the end and the kind.
func (l *lexer) number(i int) (int, fh.TokenType) {
	c, d := l.at(i), l.at(i+1)
	if c == '0' {
		var f func(byte) bool
		var t fh.TokenType
		switch d {
		case 'b':
			f, t = func(c byte) bool { return c == '0' || c == '1' || c == '_' }, fh.LiteralNumberBin
		case 'o':
			f, t = func(c byte) bool { return isOct(c) || c == '_' }, fh.LiteralNumberOct
		case 'x':
			f, t = func(c byte) bool { return isHex(c) || c == '_' }, fh.LiteralNumberHex
		}
		if f != nil && f(l.at(i+2)) {
			return l.run(i+2, f), t
		}
	}
	if !isDec(c) {
		return 0, 0
	}
	j := l.run(i+1, isDecU)
	// (\.[0-9_]+[eE][+\-]?[0-9_]+|\.[0-9_]*|[eE][+\-]?[0-9_]+)
	exp := func(k int) int {
		if e := l.at(k); e != 'e' && e != 'E' {
			return -1
		}
		k++
		if s := l.at(k); (s == '+' || s == '-') && isDecU(l.at(k+1)) {
			k++
		}
		if !isDecU(l.at(k)) {
			return -1
		}
		return l.run(k, isDecU)
	}
	if l.at(j) == '.' {
		if k := l.run(j+1, isDecU); k > j+1 {
			if e := exp(k); e >= 0 {
				return e, fh.LiteralNumberFloat
			}
		}
		return l.run(j+1, isDecU), fh.LiteralNumberFloat
	}
	if e := exp(j); e >= 0 {
		return e, fh.LiteralNumberFloat
	}
	return j, fh.LiteralNumberInteger
}

func (l *lexer) wordBefore(i int) bool {
	if i == 0 {
		return false
	}
	r := lastRune(l.src[:i])
	return lexkit.IsWord(r)
}

func lastRune(s string) rune {
	for k := len(s) - 1; k >= 0; k-- {
		if s[k] < 0x80 || s[k] >= 0xC0 {
			r, _ := lexkit.Rune(s, k)
			return r
		}
	}
	return -1
}

func (l *lexer) wordAfter(i int) bool {
	if i >= len(l.src) {
		return false
	}
	r, _ := lexkit.Rune(l.src, i)
	return lexkit.IsWord(r)
}

func (l *lexer) root(i int) (int, bool) {
	src := l.src
	c := src[i]
	if c == '\n' {
		return l.emit(fh.Text, i, i+1)
	}
	if n := lexkit.Span(src, i, lexkit.IsSpace); n > 0 {
		return l.emit(fh.Text, i, i+n)
	}
	if c == '/' && l.at(i+1) == '/' {
		l.push(commentSingle)
		return l.emit(fh.CommentSingle, i, i+2)
	}
	if c == '/' && l.at(i+1) == '*' {
		l.push(commentMulti)
		return l.emit(fh.CommentMultiline, i, i+2)
	}
	// #(if|elseif|else|endif|available)\b
	if c == '#' {
		n, _ := l.words.Word(src, i+1)
		switch src[i+1 : i+1+n] {
		case "if", "elseif", "else", "endif", "available":
			l.push(preproc)
			return l.emit(fh.CommentPreproc, i, i+1+n)
		}
	}
	if n, ok := l.keywords(i); ok {
		return n, true
	}
	if n := l.words.Match(src, i, lBuiltins); n > 0 {
		return l.emit(fh.NameBuiltin, i, i+n)
	}
	if n := l.words.Match(src, i, lBuiltinPseudo); n > 0 {
		return l.emit(fh.NameBuiltinPseudo, i, i+n)
	}
	// \$\d+
	if c == '$' {
		if n := lexkit.Span(src, i+1, lexkit.IsDigit); n > 0 {
			return l.emit(fh.NameVariable, i, i+1+n)
		}
	}
	if e, t := l.number(i); e > 0 {
		return l.emit(t, i, e)
	}
	if c == '"' {
		l.push(str)
		return l.emit(fh.LiteralString, i, i+1)
	}
	// [(){}\[\].,:;=@#`?]|->|[<&?](?=\w)|(?<=\w)[>!?]
	switch {
	case strings.IndexByte("(){}[].,:;=@#`?", c) >= 0:
		return l.emit(fh.Punctuation, i, i+1)
	case c == '-' && l.at(i+1) == '>':
		return l.emit(fh.Punctuation, i, i+2)
	case (c == '<' || c == '&') && l.wordAfter(i+1):
		return l.emit(fh.Punctuation, i, i+1)
	case (c == '>' || c == '!') && l.wordBefore(i):
		return l.emit(fh.Punctuation, i, i+1)
	}
	// [/=\-+!*%<>&|^?~]+
	if n := lexkit.Span(src, i, func(r rune) bool { return r < 0x80 && strings.IndexByte("/=-+!*%<>&|^?~", byte(r)) >= 0 }); n > 0 {
		return l.emit(fh.Operator, i, i+n)
	}
	if n := l.name(i); n > 0 {
		return l.emit(fh.Name, i, i+n)
	}
	return 0, false
}
