// Package js is the JavaScript tokenizer. It is used by lang/javascript (JavaScript) and internal/html
// (inside <script>).
//
// The token types and boundaries reproduce the rules of Chroma 2.27.0's javascript.xml without regular
// expressions. There is only one intended difference (see the "What was changed" section of website/compatibility.md):
// the { } inside the ${ … } of a template literal are closed by counting depth. Chroma closes the ${ at
// the first }, so the text after ${f({ a: 1 })} is colored as a string. Setting Options.ChromaBraces to
// true makes it behave like Chroma (used by the tests that compare with Chroma).
package js

import (
	"strings"
	"unicode"

	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/lexkit"
)

// Options switches the differences from Chroma.
type Options struct {
	// If ChromaBraces is true, ${ … } is closed at the first }, as in Chroma.
	ChromaBraces bool
}

// The states of Chroma's javascript.xml.
type state int

const (
	root         state = iota
	interp             // inside `…`
	interpInside       // inside ${ … }
	slashRegex         // slashstartsregex: the next / can be read as a regular expression
	badRegex           // an unclosed regular expression; up to the end of the line
)

// frame is one level of the stack. depth is used in interpInside: the number of { opened inside the ${.
type frame struct {
	st    state
	depth int
}

type lexer struct {
	x     *lexkit.Tokens
	src   string
	o     Options
	stack []frame
}

// Lex splits src into tokens (same contract as Lex in fasthighlight.Language).
func Lex(src string, end int, o Options) []fh.Token {
	x := lexkit.NewTokens(src)
	LexInto(x, src, end, o)
	return x.Result()
}

// LexInto splits src into tokens and adds them to x. x's src is either the same as src or was made with
// x.Sub.
func LexInto(x *lexkit.Tokens, src string, end int, o Options) {
	l := &lexer{x: x, src: src, o: o, stack: []frame{{st: root}}}
	for i := 0; i < end && len(l.stack) > 0; {
		top := &l.stack[len(l.stack)-1]
		n, ok := l.step(top, i)
		if !ok {
			if src[i] == '\n' && top.st != root {
				l.stack = append(l.stack[:0], frame{st: root})
				continue
			}
			n = l.x.Error(i)
		}
		i += n
	}
}

func (l *lexer) emit(t fh.TokenType, s, e int) (int, bool) { l.x.Emit(t, s, e); return e - s, true }
func (l *lexer) push(s state)                              { l.stack = append(l.stack, frame{st: s}) }
func (l *lexer) pop()                                      { l.stack = l.stack[:len(l.stack)-1] }

// step tries the rules of the top state in order. If one matches, it returns the number of bytes
// advanced (0 is possible) and true.
func (l *lexer) step(top *frame, i int) (int, bool) {
	src := l.src
	c := src[i]
	switch top.st {
	case root:
		return l.root(i, nil)
	case interpInside:
		if c == '}' {
			if !l.o.ChromaBraces && top.depth > 0 {
				top.depth--
				return l.emit(fh.Punctuation, i, i+1) // the root [})\].]
			}
			l.pop()
			return l.emit(fh.LiteralStringInterpol, i, i+1)
		}
		return l.root(i, top)
	case interp:
		switch {
		case c == '`':
			l.pop()
			return l.emit(fh.LiteralStringBacktick, i, i+1)
		case c == '\\': // \\\\, \\`, \\[^`\\]
			if i+1 < len(src) {
				_, n := lexkit.Rune(src, i+1)
				return l.emit(fh.LiteralStringBacktick, i, i+1+n)
			}
			return 0, false
		case strings.HasPrefix(src[i:], "${"):
			l.push(interpInside)
			return l.emit(fh.LiteralStringInterpol, i, i+2)
		case c == '$':
			return l.emit(fh.LiteralStringBacktick, i, i+1)
		}
		// [^`\\$]+
		j := i
		for j < len(src) && src[j] != '`' && src[j] != '\\' && src[j] != '$' {
			j++
		}
		return l.emit(fh.LiteralStringBacktick, i, j)
	case slashRegex:
		if n, ok := l.commentsAndWhitespace(i); ok {
			return n, ok
		}
		if e := regexEnd(src, i); e > 0 {
			l.pop()
			return l.emit(fh.LiteralStringRegex, i, e)
		}
		l.pop()
		if c == '/' { // (?=/) enters badregex
			l.push(badRegex)
		}
		return 0, true
	case badRegex:
		if c == '\n' {
			l.pop()
			return l.emit(fh.Text, i, i+1)
		}
	}
	return 0, false
}

// commentsAndWhitespace tries the rules of the commentsandwhitespace state.
func (l *lexer) commentsAndWhitespace(i int) (int, bool) {
	src := l.src
	if n := lexkit.Span(src, i, lexkit.IsSpace); n > 0 {
		return l.emit(fh.Text, i, i+n)
	}
	rest := src[i:]
	switch {
	case strings.HasPrefix(rest, "<!--"):
		return l.emit(fh.Comment, i, i+4)
	case strings.HasPrefix(rest, "//"): // //.*?\n
		if k := strings.IndexByte(src[i+2:], '\n'); k >= 0 {
			return l.emit(fh.CommentSingle, i, i+2+k+1)
		}
	case strings.HasPrefix(rest, "/*"): // /\*.*?\*/
		if k := strings.Index(src[i+2:], "*/"); k >= 0 {
			return l.emit(fh.CommentMultiline, i, i+2+k+2)
		}
	}
	return 0, false
}

// regexEnd returns the end of the part matched by /(\\.|[^[/\\\n]|\[(\\.|[^\]\\\n])*])+/([gimuy]+\b|\B), or 0
// if it does not match.
func regexEnd(src string, i int) int {
	if src[i] != '/' {
		return 0
	}
	j, items := i+1, 0
	for j < len(src) {
		c := src[j]
		if c == '/' || c == '\n' {
			break
		}
		switch c {
		case '\\':
			if j+1 >= len(src) {
				return 0
			}
			_, n := lexkit.Rune(src, j+1)
			j += 1 + n
		case '[':
			k := j + 1
			for {
				if k >= len(src) || src[k] == '\n' {
					return 0 // an unclosed [ matches no item
				}
				if src[k] == ']' {
					break
				}
				if src[k] == '\\' {
					if k+1 >= len(src) {
						return 0
					}
					_, n := lexkit.Rune(src, k+1)
					k += 1 + n
					continue
				}
				_, n := lexkit.Rune(src, k)
				k += n
			}
			j = k + 1
		default:
			_, n := lexkit.Rune(src, j)
			j += n
		}
		items++
	}
	if items == 0 || j >= len(src) || src[j] != '/' {
		return 0
	}
	q := j + 1
	if f := lexkit.Span(src, q, isFlag); f > 0 && lexkit.WordBoundary(src, q+f) { // [gimuy]+\b
		return q + f
	}
	if !lexkit.WordBoundary(src, q) { // \B
		return q
	}
	return 0
}

func isFlag(r rune) bool { return r == 'g' || r == 'i' || r == 'm' || r == 'u' || r == 'y' }

// The word lists. They are in the same order as the regular-expression alternatives (they end with \b, so the
// order changes the result).
var (
	keywords = [...]string{
		"for", "in", "while", "do", "break", "return", "continue", "switch", "case", "default", "if", "else",
		"throw", "try", "catch", "finally", "new", "delete", "typeof", "instanceof", "void", "yield", "this", "of",
	}
	declarations = [...]string{"var", "let", "with", "function"}
	reserved     = [...]string{
		"abstract", "async", "await", "boolean", "byte", "char", "class", "const", "debugger", "double", "enum",
		"export", "extends", "final", "float", "goto", "implements", "import", "int", "interface", "long",
		"native", "package", "private", "protected", "public", "short", "static", "super", "synchronized",
		"throws", "transient", "using", "volatile",
	}
	constants = [...]string{"true", "false", "null", "NaN", "Infinity", "undefined"}
	builtins  = [...]string{
		"Array", "Boolean", "Date", "Error", "Function", "Math", "netscape", "Number", "Object", "Packages",
		"RegExp", "String", "Promise", "Proxy", "sun", "decodeURI", "decodeURIComponent", "encodeURI",
		"encodeURIComponent", "Error", "eval", "isFinite", "isNaN", "isSafeInteger", "parseFloat", "parseInt",
		"document", "this", "window",
	}
)

// word returns the length of the first word in words that matches the start of src[i:] and is followed by a \b.
func word(src string, i int, words []string) int {
	for _, w := range words {
		if strings.HasPrefix(src[i:], w) && lexkit.WordBoundary(src, i+len(w)) {
			return len(w)
		}
	}
	return 0
}

// root tries the rules of the root state. in is the stack level when called from interpInside (it counts
// depth).
func (l *lexer) root(i int, in *frame) (int, bool) {
	src := l.src
	c := src[i]
	// \A#! ?/.*?\n
	if i == 0 && strings.HasPrefix(src, "#!") {
		j := 2
		if j < len(src) && src[j] == ' ' {
			j++
		}
		if j < len(src) && src[j] == '/' {
			if k := strings.IndexByte(src[j+1:], '\n'); k >= 0 {
				return l.emit(fh.CommentHashbang, 0, j+1+k+1)
			}
		}
	}
	// ^(?=\s|/|<!--) enters slashstartsregex (reads nothing)
	if i == 0 || src[i-1] == '\n' {
		if r, _ := lexkit.Rune(src, i); lexkit.IsSpace(r) || c == '/' || strings.HasPrefix(src[i:], "<!--") {
			l.push(slashRegex)
			return 0, true
		}
	}
	if n, ok := l.commentsAndWhitespace(i); ok {
		return n, ok
	}
	if e := numberEnd(src, i); e > 0 {
		return l.emit(numberType(src, i, e), i, e)
	}
	if strings.HasPrefix(src[i:], "...") {
		return l.emit(fh.Punctuation, i, i+3)
	}
	if strings.HasPrefix(src[i:], "=>") {
		return l.emit(fh.Punctuation, i, i+2)
	}
	if n := operatorLen(src, i); n > 0 {
		l.push(slashRegex)
		return l.emit(fh.Operator, i, i+n)
	}
	switch c {
	case '{', '(', '[', ';', ',':
		if c == '{' && in != nil {
			in.depth++
		}
		l.push(slashRegex)
		return l.emit(fh.Punctuation, i, i+1)
	case '}', ')', ']', '.':
		return l.emit(fh.Punctuation, i, i+1)
	}
	if n := word(src, i, keywords[:]); n > 0 {
		l.push(slashRegex)
		return l.emit(fh.Keyword, i, i+n)
	}
	if n := word(src, i, declarations[:]); n > 0 {
		l.push(slashRegex)
		return l.emit(fh.KeywordDeclaration, i, i+n)
	}
	if n := word(src, i, reserved[:]); n > 0 {
		return l.emit(fh.KeywordReserved, i, i+n)
	}
	if n := word(src, i, constants[:]); n > 0 {
		return l.emit(fh.KeywordConstant, i, i+n)
	}
	if n := word(src, i, builtins[:]); n > 0 {
		return l.emit(fh.NameBuiltin, i, i+n)
	}
	if e := nameEnd(src, i); e > i {
		return l.emit(fh.NameOther, i, e)
	}
	switch c {
	case '"':
		if n := lexkit.QuotedLen(src, i, '"'); n > 0 {
			return l.emit(fh.LiteralStringDouble, i, i+n)
		}
	case '\'':
		if n := lexkit.QuotedLen(src, i, '\''); n > 0 {
			return l.emit(fh.LiteralStringSingle, i, i+n)
		}
	case '`':
		l.push(interp)
		return l.emit(fh.LiteralStringBacktick, i, i+1)
	}
	return 0, false
}

// numberEnd returns the end of the part matched by any of the number rules (decimal, binary, octal,
// hexadecimal, integer), or 0 if none matches.
func numberEnd(src string, i int) int {
	// \d+(\.\d*|[eE][+\-]?\d+)
	if d := i + lexkit.Span(src, i, lexkit.IsDigit); d > i && d < len(src) {
		if src[d] == '.' {
			return d + 1 + lexkit.Span(src, d+1, lexkit.IsDigit)
		}
		if src[d] == 'e' || src[d] == 'E' {
			k := d + 1
			if k < len(src) && (src[k] == '+' || src[k] == '-') {
				k++
			}
			if n := lexkit.Span(src, k, lexkit.IsDigit); n > 0 {
				return k + n
			}
		}
	}
	if src[i] == '0' && i+1 < len(src) {
		var f func(byte) bool
		switch src[i+1] {
		case 'b', 'B':
			f = func(c byte) bool { return c == '0' || c == '1' }
		case 'o', 'O':
			f = func(c byte) bool { return c >= '0' && c <= '7' }
		case 'x', 'X':
			f = func(c byte) bool { return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F' }
		}
		if f != nil {
			j := i + 2
			for j < len(src) && f(src[j]) {
				j++
			}
			if j > i+2 {
				return j
			}
		}
	}
	// [0-9][0-9_]*
	if src[i] >= '0' && src[i] <= '9' {
		j := i + 1
		for j < len(src) && (src[j] >= '0' && src[j] <= '9' || src[j] == '_') {
			j++
		}
		return j
	}
	return 0
}

// numberType re-decides the type of the src[i:e] matched by numberEnd, following the order of the rules.
func numberType(src string, i, e int) fh.TokenType {
	if d := i + lexkit.Span(src, i, lexkit.IsDigit); d > i && d < e {
		if c := src[d]; c == '.' || c == 'e' || c == 'E' {
			return fh.LiteralNumberFloat
		}
	}
	if src[i] == '0' && i+1 < e {
		switch src[i+1] {
		case 'b', 'B':
			return fh.LiteralNumberBin
		case 'o', 'O':
			return fh.LiteralNumberOct
		case 'x', 'X':
			return fh.LiteralNumberHex
		}
	}
	return fh.LiteralNumberInteger
}

// operatorLen returns the length matched by the operator rule
// \+\+|--|~|&&|\?|:|\|\||\\(?=\n)|(<<|>>>?|==?|!=?|[-<>+*%&|^/])=?.
func operatorLen(src string, i int) int {
	rest := src[i:]
	for _, op := range [...]string{"++", "--", "~", "&&", "?", ":", "||"} {
		if strings.HasPrefix(rest, op) {
			return len(op)
		}
	}
	if strings.HasPrefix(rest, "\\\n") {
		return 1
	}
	n := 0
	switch {
	case strings.HasPrefix(rest, "<<"):
		n = 2
	case strings.HasPrefix(rest, ">>"):
		n = 2
		if strings.HasPrefix(rest[2:], ">") {
			n = 3
		}
	case rest[0] == '=' || rest[0] == '!':
		n = 1
		if strings.HasPrefix(rest[1:], "=") {
			n = 2
		}
	case strings.IndexByte("-<>+*%&|^/", rest[0]) >= 0:
		n = 1
	default:
		return 0
	}
	if strings.HasPrefix(rest[n:], "=") {
		n++
	}
	return n
}

// nameEnd returns the end of the part matched by (?:[$_\p{L}\p{N}]|\\u[a-fA-F0-9]{4})(?:(?:[$\p{L}\p{N}]|\\u[a-fA-F0-9]{4}))*
// (or i if it does not match). From the second character on, _ is not included.
func nameEnd(src string, i int) int {
	j := i
	for first := true; j < len(src); first = false {
		if n := uEscape(src, j); n > 0 {
			j += n
			continue
		}
		r, n := lexkit.Rune(src, j)
		if r == '$' || (first && r == '_') || unicode.IsLetter(r) || unicode.IsNumber(r) {
			j += n
			continue
		}
		break
	}
	return j
}

// uEscape returns 6 if src[j:] starts with \u and 4 hexadecimal digits.
func uEscape(src string, j int) int {
	if !strings.HasPrefix(src[j:], "\\u") || j+6 > len(src) {
		return 0
	}
	for _, c := range []byte(src[j+2 : j+6]) {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return 0
		}
	}
	return 6
}
