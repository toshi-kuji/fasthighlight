// Package js is the JavaScript tokenizer. It is used by lang/javascript (JavaScript), internal/html
// (inside <script>), and lang/react (JavaScript with JSX, through LexReact).
//
// The token types and boundaries reproduce the rules of Chroma 2.27.0's javascript.xml (react.xml for
// LexReact) without regular expressions. react.xml is javascript.xml with the JSX states (jsx, tag, attr,
// expression) added, other number rules, no "using" keyword, and no rule for escapes such as \n in a
// template literal.
//
// The intended differences (see the "What was changed" section of website/compatibility.md): the { } inside
// the ${ … } of a template literal are closed by counting depth. Chroma closes the ${ at the first }, so
// the text after ${f({ a: 1 })} is colored as a string. Setting Options.ChromaBraces to true makes it
// behave like Chroma (used by the tests that compare with Chroma). In react, an escape such as \n in a
// template literal is part of the string, as in JavaScript (Chroma's react.xml makes the \ an Error);
// LexReact with chroma set to true behaves like Chroma in both points.
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

	// react selects the rules of Chroma's react.xml (JavaScript with JSX) instead of javascript.xml. It is
	// set only by LexReact.
	react bool
	// chromaEscape (only with react) makes the \ of an escape in a template literal other than \\ and \`
	// an Error, as in Chroma's react.xml. It is set only by LexReact.
	chromaEscape bool
}

// LexReact splits src into tokens with the rules of Chroma's react.xml (same contract as Lex in
// fasthighlight.Language). If chroma is true, the intended differences from Chroma (template-literal
// braces and escapes) are turned off; compat/ uses it to compare with Chroma.
func LexReact(src string, end int, chroma bool) []fh.Token {
	return Lex(src, end, Options{ChromaBraces: chroma, react: true, chromaEscape: chroma})
}

// The states of Chroma's javascript.xml.
type state int

const (
	root         state = iota
	interp             // inside `…`
	interpInside       // inside ${ … }
	slashRegex         // slashstartsregex: the next / can be read as a regular expression
	badRegex           // an unclosed regular expression; up to the end of the line
	tag                // inside a JSX tag (react only)
	attr               // a JSX attribute value (react only)
	expression         // JSX { … } (react only)
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
		case c == '\\': // \\\\, \\`, \\[^`\\] (react.xml has only the first two)
			if l.o.chromaEscape && !strings.HasPrefix(src[i:], `\\`) && !strings.HasPrefix(src[i:], "\\`") {
				return 0, false
			}
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
	case expression:
		switch c {
		case '{':
			l.push(expression)
			return l.emit(fh.Punctuation, i, i+1)
		case '}':
			l.pop()
			return l.emit(fh.Punctuation, i, i+1)
		}
		return l.root(i, nil)
	case tag:
		return l.tag(i)
	case attr:
		switch c {
		case '{':
			l.push(expression)
			return l.emit(fh.Punctuation, i, i+1)
		case '"', '\'': // ".*?" and '.*?' (dot_all, so they include newlines)
			if j := strings.IndexByte(src[i+1:], c); j >= 0 {
				l.pop()
				return l.emit(fh.LiteralString, i, i+1+j+1)
			}
		}
		l.pop()
		return 0, true
	}
	return 0, false
}

func isTagName(r rune) bool { return r == '.' || lexkit.IsWord(r) }

// jsx tries the rules of react.xml's jsx state.
func (l *lexer) jsx(i int) (int, bool) {
	src := l.src
	if src[i] != '<' {
		return 0, false
	}
	// (<)(/?)(>)
	if strings.HasPrefix(src[i:], "<>") {
		l.x.Emit(fh.Punctuation, i, i+1)
		l.x.Emit(fh.Punctuation, i+1, i+2)
		return 2, true
	}
	if strings.HasPrefix(src[i:], "</>") {
		l.x.Emit(fh.Punctuation, i, i+1)
		l.x.Emit(fh.Punctuation, i+1, i+2)
		l.x.Emit(fh.Punctuation, i+2, i+3)
		return 3, true
	}
	// (<)([\w\.]+)
	if n := lexkit.Span(src, i+1, isTagName); n > 0 {
		l.x.Emit(fh.Punctuation, i, i+1)
		l.x.Emit(fh.NameTag, i+1, i+1+n)
		l.push(tag)
		return 1 + n, true
	}
	// (<)(/)([\w\.]+)(>)
	if strings.HasPrefix(src[i:], "</") {
		if n := lexkit.Span(src, i+2, isTagName); n > 0 {
			if e := i + 2 + n; e < len(src) && src[e] == '>' {
				l.x.Emit(fh.Punctuation, i, i+1)
				l.x.Emit(fh.Punctuation, i+1, i+2)
				l.x.Emit(fh.NameTag, i+2, e)
				l.x.Emit(fh.Punctuation, e, e+1)
				return e + 1 - i, true
			}
		}
	}
	return 0, false
}

// tag tries the rules of react.xml's tag state.
func (l *lexer) tag(i int) (int, bool) {
	src := l.src
	// \s+
	if n := lexkit.Span(src, i, lexkit.IsSpace); n > 0 {
		return l.emit(fh.Text, i, i+n)
	}
	// ([\w]+\s*)(=)(\s*)
	if n := lexkit.Span(src, i, lexkit.IsWord); n > 0 {
		e := i + n
		e += lexkit.Span(src, e, lexkit.IsSpace)
		if e < len(src) && src[e] == '=' {
			w := lexkit.Span(src, e+1, lexkit.IsSpace)
			l.x.Emit(fh.NameAttribute, i, e)
			l.x.Emit(fh.Operator, e, e+1)
			l.x.Emit(fh.Text, e+1, e+1+w)
			l.push(attr)
			return e + 1 + w - i, true
		}
	}
	// [{}]+
	if j := i; src[j] == '{' || src[j] == '}' {
		for j < len(src) && (src[j] == '{' || src[j] == '}') {
			j++
		}
		return l.emit(fh.Punctuation, i, j)
	}
	// [\w\.]+
	if n := lexkit.Span(src, i, isTagName); n > 0 {
		return l.emit(fh.NameAttribute, i, i+n)
	}
	// (/?)(\s*)(>)
	j := i
	if src[j] == '/' {
		j++
	}
	w := lexkit.Span(src, j, lexkit.IsSpace)
	if e := j + w; e < len(src) && src[e] == '>' {
		l.x.Emit(fh.Punctuation, i, j)
		l.x.Emit(fh.Text, j, e)
		l.x.Emit(fh.Punctuation, e, e+1)
		l.pop()
		return e + 1 - i, true
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
	if l.o.react {
		if n, ok := l.jsx(i); ok {
			return n, ok
		}
	}
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
	if l.o.react {
		if e, t := reactNumber(src, i); e > 0 {
			return l.emit(t, i, e)
		}
	} else if e := numberEnd(src, i); e > 0 {
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
	if n := word(src, i, reserved[:]); n > 0 && !(l.o.react && src[i:i+n] == "using") { // react.xml has no "using"
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

// reactNumber returns the end and type of the part matched by react.xml's number rules
// ((\.\d+|[0-9]+\.[0-9]*)([eE][-+]?[0-9]+)?, 0[bB][01]+, 0[oO][0-7]+, 0[xX][0-9a-fA-F]+, [0-9]+), or 0 if
// none matches.
func reactNumber(src string, i int) (int, fh.TokenType) {
	e := 0
	if src[i] == '.' { // \.\d+ (\d is Unicode)
		if n := lexkit.Span(src, i+1, lexkit.IsDigit); n > 0 {
			e = i + 1 + n
		}
	} else if d := i + asciiDigits(src, i); d > i && d < len(src) && src[d] == '.' { // [0-9]+\.[0-9]*
		e = d + 1 + asciiDigits(src, d+1)
	}
	if e > 0 {
		if e < len(src) && (src[e] == 'e' || src[e] == 'E') { // ([eE][-+]?[0-9]+)?
			k := e + 1
			if k < len(src) && (src[k] == '+' || src[k] == '-') {
				k++
			}
			if n := asciiDigits(src, k); n > 0 {
				e = k + n
			}
		}
		return e, fh.LiteralNumberFloat
	}
	if src[i] == '0' && i+1 < len(src) {
		var f func(byte) bool
		var t fh.TokenType
		switch src[i+1] {
		case 'b', 'B':
			f, t = func(c byte) bool { return c == '0' || c == '1' }, fh.LiteralNumberBin
		case 'o', 'O':
			f, t = func(c byte) bool { return c >= '0' && c <= '7' }, fh.LiteralNumberOct
		case 'x', 'X':
			f, t = func(c byte) bool { return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F' }, fh.LiteralNumberHex
		}
		if f != nil {
			j := i + 2
			for j < len(src) && f(src[j]) {
				j++
			}
			if j > i+2 {
				return j, t
			}
		}
	}
	if n := asciiDigits(src, i); n > 0 { // [0-9]+
		return i + n, fh.LiteralNumberInteger
	}
	return 0, fh.Text
}

// asciiDigits returns the number of bytes of [0-9]* at src[i:].
func asciiDigits(src string, i int) int {
	n := 0
	for i+n < len(src) && src[i+n] >= '0' && src[i+n] <= '9' {
		n++
	}
	return n
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
