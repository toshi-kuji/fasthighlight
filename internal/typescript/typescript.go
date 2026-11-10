// Package typescript is the TypeScript tokenizer. lang/typescript registers it. Options (the switch
// for the intended differences from Chroma) exists only for the comparison in compat/ and is not
// exposed.
//
// The token types and token boundaries reproduce the rules of Chroma 2.27.0's typescript.xml
// without using regular expressions.
package typescript

import (
	"strings"
	"unicode"

	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/lexkit"
)

// Options is the switch for the intended differences from Chroma. The zero value is the production
// setting (the one lang/typescript registers).
type Options struct {
	// If Chroma is true, the following Chroma behaviors are reproduced. If false, they are fixed.
	//
	//  - The JSX rule (<tag …>) is used. If an alphanumeric character follows <, it is read as a tag,
	//    so the type arguments in a<b or Array<T> also become tags, and what follows may become Error.
	//  - ${ … } in a template literal is closed at the first } (the depth of the { } inside is not
	//    counted).
	//  - A name (NameOther) can start only with [$a-zA-Z_] (a name that starts with a non-ASCII
	//    letter becomes Error).
	//  - In a template literal, the \ of an escape such as \n (other than \\ and \`) becomes Error.
	Chroma bool
}

// Indexes of the word lists (lists.go).
const (
	lKeywords = iota
	lDeclarations
	lReserved
	lConstants
	lBuiltins
	lTypes
	lReserved2
)

var lists *lexkit.WordLists

// Prepare builds the word tables (Prepare of fasthighlight.Language).
func Prepare() {
	lists = lexkit.NewWordLists(keywords, declarations, reserved, constants, builtins, types, reserved2)
}

// The states of Chroma's typescript.xml.
type state int

const (
	root         state = iota
	interp             // inside `…`
	interpInside       // inside ${ … }
	slashRegex         // slashstartsregex: the next / can be read as a regular expression
	badRegex           // an unclosed regular expression, up to the end of the line
	comment            // <!-- … -->
	tag                // inside a JSX tag (Chroma only)
	attr               // a JSX attribute value (Chroma only)
	expression         // JSX { … } (Chroma only)
)

// frame is one level of the stack. depth, for interpInside, is the number of { opened inside ${.
type frame struct {
	st    state
	depth int
}

type lexer struct {
	x     *lexkit.Tokens
	src   string
	o     Options
	stack []frame
	words lexkit.WordCache
	mem   lexkit.SpanCache // the last run of [\w?.$]
}

// Lex splits src into tokens (the same contract as Lex of fasthighlight.Language). compat/ uses it
// to compare with Chroma while varying o.
func Lex(src string, end int, o Options) []fh.Token {
	l := &lexer{x: lexkit.NewTokens(src), src: src, o: o, stack: []frame{{st: root}}, words: lexkit.WordCache{Lists: lists}}
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
	return l.x.Result()
}

func (l *lexer) emit(t fh.TokenType, s, e int) (int, bool) { l.x.Emit(t, s, e); return e - s, true }
func (l *lexer) push(s state)                              { l.stack = append(l.stack, frame{st: s}) }
func (l *lexer) pop()                                      { l.stack = l.stack[:len(l.stack)-1] }

// step tries the rules of the top state in order. If one matches, it returns the number of bytes
// advanced (which may be 0) and true.
func (l *lexer) step(top *frame, i int) (int, bool) {
	src := l.src
	c := src[i]
	switch top.st {
	case root:
		return l.root(i, nil)
	case interpInside:
		if c == '}' {
			if !l.o.Chroma && top.depth > 0 {
				top.depth--
				return l.emit(fh.Punctuation, i, i+1) // root's [})\].]
			}
			l.pop()
			return l.emit(fh.LiteralStringInterpol, i, i+1)
		}
		return l.root(i, top)
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
	case interp:
		switch {
		case c == '`':
			l.pop()
			return l.emit(fh.LiteralStringBacktick, i, i+1)
		case strings.HasPrefix(src[i:], "\\\\"), strings.HasPrefix(src[i:], "\\`"):
			return l.emit(fh.LiteralStringBacktick, i, i+2)
		case c == '\\':
			// Chroma's rules have no other escape (JavaScript's \\[^`\\]). In production, it is made the same
			// as JavaScript.
			if !l.o.Chroma && i+1 < len(src) {
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
	case comment:
		switch {
		case strings.HasPrefix(src[i:], "-->"):
			l.pop()
			return l.emit(fh.Comment, i, i+3)
		case c == '-':
			return l.emit(fh.Comment, i, i+1)
		}
		j := strings.IndexByte(src[i:], '-') // [^-]+
		if j < 0 {
			j = len(src) - i
		}
		return l.emit(fh.Comment, i, i+j)
	case tag:
		return l.tag(i)
	case attr:
		switch c {
		case '{':
			l.push(expression)
			return l.emit(fh.Punctuation, i, i+1)
		case '"', '\'': // ".*?" and '.*?' (dot_all, so this includes newlines)
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

// commentsAndWhitespace tries the rules of the commentsandwhitespace state.
func (l *lexer) commentsAndWhitespace(i int) (int, bool) {
	src := l.src
	if n := lexkit.Span(src, i, lexkit.IsSpace); n > 0 {
		return l.emit(fh.Text, i, i+n)
	}
	rest := src[i:]
	switch {
	case strings.HasPrefix(rest, "<!--"):
		l.push(comment)
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

func isTagName(r rune) bool { return r == '.' || lexkit.IsWord(r) }

// jsx tries the rules of the jsx state (Chroma only).
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
	// (<)(/)([\w\.]*)(>)
	if strings.HasPrefix(src[i:], "</") {
		n := lexkit.Span(src, i+2, isTagName)
		if e := i + 2 + n; e < len(src) && src[e] == '>' {
			l.x.Emit(fh.Punctuation, i, i+1)
			l.x.Emit(fh.Punctuation, i+1, i+2)
			l.x.Emit(fh.NameTag, i+2, e)
			l.x.Emit(fh.Punctuation, e, e+1)
			return e + 1 - i, true
		}
	}
	return 0, false
}

func isAttrName(r rune) bool { return r == '-' || lexkit.IsWord(r) }

// tag tries the rules of the tag state (Chroma only).
func (l *lexer) tag(i int) (int, bool) {
	src := l.src
	if n, ok := l.jsx(i); ok {
		return n, ok
	}
	switch src[i] {
	case ',':
		return l.emit(fh.Punctuation, i, i+1)
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
	if n, ok := l.commentsAndWhitespace(i); ok {
		return n, ok
	}
	// ([\w-]+\s*)(=)(\s*)
	if n := lexkit.Span(src, i, isAttrName); n > 0 {
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

// regexEnd returns the end of the part matching /(\\.|[^[/\\\n]|\[(\\.|[^\]\\\n])*])+/([gim]+\b|\B).
// It returns 0 if there is no match (dot_all, so the . in \\. also matches a newline).
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
					return 0 // an unclosed [ matches none of the alternatives
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
	if f := lexkit.Span(src, q, isFlag); f > 0 && lexkit.WordBoundary(src, q+f) { // [gim]+\b
		return q + f
	}
	if !lexkit.WordBoundary(src, q) { // \B
		return q
	}
	return 0
}

func isFlag(r rune) bool { return r == 'g' || r == 'i' || r == 'm' }

// root tries the rules of the root state. in is, when used from interpInside, that level (whose
// depth is counted).
func (l *lexer) root(i int, in *frame) (int, bool) {
	src := l.src
	c := src[i]
	if l.o.Chroma {
		if n, ok := l.jsx(i); ok {
			return n, ok
		}
	}
	// ^(?=\s|/|<!--) enters slashstartsregex (consumes nothing)
	if i == 0 || src[i-1] == '\n' {
		if r, _ := lexkit.Rune(src, i); lexkit.IsSpace(r) || c == '/' || strings.HasPrefix(src[i:], "<!--") {
			l.push(slashRegex)
			return 0, true
		}
	}
	if n, ok := l.commentsAndWhitespace(i); ok {
		return n, ok
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
	if n := l.words.Match(src, i, lKeywords); n > 0 {
		l.push(slashRegex)
		return l.emit(fh.Keyword, i, i+n)
	}
	if n := l.words.Match(src, i, lDeclarations); n > 0 {
		l.push(slashRegex)
		return l.emit(fh.KeywordDeclaration, i, i+n)
	}
	if n := l.words.Match(src, i, lReserved); n > 0 {
		return l.emit(fh.KeywordReserved, i, i+n)
	}
	if n := l.words.Match(src, i, lConstants); n > 0 {
		return l.emit(fh.KeywordConstant, i, i+n)
	}
	if n := l.words.Match(src, i, lBuiltins); n > 0 {
		return l.emit(fh.NameBuiltin, i, i+n)
	}
	b := lexkit.WordBoundary(src, i)
	if b {
		if n, ok := l.module(i); ok {
			return n, ok
		}
		if n := l.words.Match(src, i, lTypes); n > 0 {
			return l.emit(fh.KeywordType, i, i+n)
		}
		if n := l.words.Match(src, i, lReserved2); n > 0 {
			return l.emit(fh.KeywordReserved, i, i+n)
		}
	}
	// (super)(\s*)(\([\w,?.$\s]+\s*\)) never matches, because the earlier rule super\b matches first.
	// ([a-zA-Z_?.$][\w?.$]*)\(\) \{. The ? and . are matched first by earlier rules.
	if c == '_' || c == '$' || c < 0x80 && unicode.IsLetter(rune(c)) {
		e := i + 1 + l.members(i+1)
		if strings.HasPrefix(src[e:], "() {") {
			l.push(slashRegex)
			return l.emit(fh.NameOther, i, e+4)
		}
	}
	// ([\w?.$][\w?.$]*)(\s*:\s*)([\w?.$][\w?.$]*)
	if n := l.members(i); n > 0 {
		e := i + n
		k := e + lexkit.Span(src, e, lexkit.IsSpace)
		if k < len(src) && src[k] == ':' {
			k++
			k += lexkit.Span(src, k, lexkit.IsSpace)
			if m := l.members(k); m > 0 {
				l.x.Emit(fh.NameOther, i, e)
				l.x.Emit(fh.Text, e, k)
				l.x.Emit(fh.KeywordType, k, k+m)
				return k + m - i, true
			}
		}
	}
	// [$a-zA-Z_]\w*. In production, it can also start with a Unicode letter.
	if r, n := lexkit.Rune(src, i); r == '$' || r == '_' || r < 0x80 && unicode.IsLetter(r) || !l.o.Chroma && unicode.IsLetter(r) {
		return l.emit(fh.NameOther, i, i+n+lexkit.Span(src, i+n, lexkit.IsWord))
	}
	if e := numberEnd(src, i); e > 0 {
		return l.emit(numberType(src, i, e), i, e)
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
	case '@': // @\w+
		if n := lexkit.Span(src, i+1, lexkit.IsWord); n > 0 {
			return l.emit(fh.KeywordDeclaration, i, i+1+n)
		}
	}
	return 0, false
}

func isMember(r rune) bool { return r == '?' || r == '.' || r == '$' || lexkit.IsWord(r) }

// members returns the length of the run of [\w?.$] at i.
func (l *lexer) members(i int) int { return l.mem.Span(l.src, i, isMember) }

// module tries \b(module)(\s+)("[\w\./@]+")(\s+) (the caller checks the preceding \b).
func (l *lexer) module(i int) (int, bool) {
	src := l.src
	if !strings.HasPrefix(src[i:], "module") {
		return 0, false
	}
	a := i + 6
	b := a + lexkit.Span(src, a, lexkit.IsSpace)
	if b == a || b >= len(src) || src[b] != '"' {
		return 0, false
	}
	n := lexkit.Span(src, b+1, func(r rune) bool { return r == '.' || r == '/' || r == '@' || lexkit.IsWord(r) })
	q := b + 1 + n
	if n == 0 || q >= len(src) || src[q] != '"' {
		return 0, false
	}
	e := q + 1 + lexkit.Span(src, q+1, lexkit.IsSpace)
	if e == q+1 {
		return 0, false
	}
	l.x.Emit(fh.KeywordReserved, i, a)
	l.x.Emit(fh.Text, a, b)
	l.x.Emit(fh.NameOther, b, q+1)
	l.x.Emit(fh.Text, q+1, e)
	l.push(slashRegex)
	return e - i, true
}

func isASCIIDigit(c byte) bool { return '0' <= c && c <= '9' }

func asciiDigits(src string, i int) int {
	n := 0
	for i+n < len(src) && isASCIIDigit(src[i+n]) {
		n++
	}
	return n
}

// numberEnd returns the end of the part matching any of the number rules (decimal, hexadecimal,
// integer). It returns 0 if there is no match.
func numberEnd(src string, i int) int {
	d := asciiDigits(src, i)
	if d == 0 {
		return 0
	}
	// [0-9][0-9]*\.[0-9]+([eE][0-9]+)?[fd]?
	if p := i + d; p < len(src) && src[p] == '.' {
		if f := asciiDigits(src, p+1); f > 0 {
			e := p + 1 + f
			if e < len(src) && (src[e] == 'e' || src[e] == 'E') {
				if x := asciiDigits(src, e+1); x > 0 {
					e += 1 + x
				}
			}
			if e < len(src) && (src[e] == 'f' || src[e] == 'd') {
				e++
			}
			return e
		}
	}
	// 0x[0-9a-fA-F]+
	if strings.HasPrefix(src[i:], "0x") {
		j := i + 2
		for j < len(src) && (isASCIIDigit(src[j]) || 'a' <= src[j] && src[j] <= 'f' || 'A' <= src[j] && src[j] <= 'F') {
			j++
		}
		if j > i+2 {
			return j
		}
	}
	return i + d // [0-9]+
}

// numberType re-determines, in rule order, the type of the src[i:e] that numberEnd matched.
func numberType(src string, i, e int) fh.TokenType {
	if d := asciiDigits(src, i); i+d < e && src[i+d] == '.' {
		return fh.LiteralNumberFloat
	}
	if strings.HasPrefix(src[i:e], "0x") {
		return fh.LiteralNumberHex
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
