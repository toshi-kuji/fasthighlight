// Package rust is the tokenizer of Rust. The package lang/rust registers it.
//
// Token types and boundaries reproduce the rules of Chroma 2.27.0's rust.xml without regular expressions.
package rust

import (
	"strings"

	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/lexkit"
)

// Options are the switches for the intended differences from Chroma. The zero value is the production
// behavior; compat/ turns the switches on to compare with Chroma.
type Options struct {
	// ChromaTypes, if true, colors types only as Chroma's rules do: the one name right after ":" or "->"
	// is NameClass (even a keyword such as mut in &mut T), and the names after it (path segments, type
	// arguments) are Name. If false (production), the whole type after ":" or "->" and the type
	// arguments of a turbofish (::<…>) are read: their names are NameClass, a name followed by "::" is
	// NameNamespace, and keywords, built-ins, built-in types and lifetimes keep their own types
	// (types.go).
	ChromaTypes bool
}

// Indexes of the word lists (lists.go).
const (
	lKeywords = iota
	lReserved
	lConstants
	lTypes
	lBuiltins
)

var lists *lexkit.WordLists

// Prepare builds the word lists. Call it once before Lex (Language.Prepare).
func Prepare() { lists = lexkit.NewWordLists(keywords, reserved, constants, types, builtins) }

type state uint8

const (
	root state = iota
	base
	modname
	lifetime
	numberLit
	attribute
	bytestring
	str
	fstring
	comment
	doccomment
	funcname
	typename
)

type lexer struct {
	o     Options
	x     *lexkit.Tokens
	src   string
	stack []state
	words lexkit.WordCache
	// typeEnd is the end of the type being read (production only): before it, a name is NameClass, or
	// NameNamespace when "::" follows it.
	typeEnd int
	ctx     typeCtx // where a ":" starts a type (production only, types.go)
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
		if st == base && n > 0 && !o.ChromaTypes {
			l.track(i, n)
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

func (l *lexer) spaces(i int) int { return lexkit.Span(l.src, i, lexkit.IsSpace) }

func isIdentStart(c byte) bool { return c == '_' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' }

// ident returns the length of [a-zA-Z_]\w*.
func (l *lexer) ident(i int) int {
	if !isIdentStart(l.at(i)) {
		return 0
	}
	return 1 + lexkit.Span(l.src, i+1, lexkit.IsWord)
}

func isHex(c byte) bool {
	return '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F'
}

// escape returns the length of \\['"\\nrt]|\\(?=\n)|\\x[0-7][0-9a-fA-F]|\\0|\\u\{[0-9a-fA-F]{1,6}\} (0 if there is
// no match). If byteHex is set, [89a-fA-F] is also allowed as the first character after \\x (the
// bytestring rule).
func (l *lexer) escape(i int, byteHex bool) int {
	if l.at(i) != '\\' {
		return 0
	}
	switch c := l.at(i + 1); {
	case c != 0 && strings.IndexByte("'\"\\nrt", c) >= 0:
		return 2
	case c == '\n':
		return 1 // \\(?=\n)
	case c == 'x':
		if h := l.at(i + 2); (byteHex && isHex(h) || '0' <= h && h <= '7') && isHex(l.at(i+3)) {
			return 4
		}
	case c == '0':
		return 2
	case c == 'u':
		if l.at(i+2) == '{' {
			n := 0
			for n < 6 && isHex(l.at(i+3+n)) {
				n++
			}
			if n > 0 && l.at(i+3+n) == '}' {
				return 3 + n + 1
			}
		}
	}
	return 0
}

func (l *lexer) step(st state, i int) (int, bool) {
	src := l.src
	switch st {
	case root:
		// #![^[\r\n].*$
		if strings.HasPrefix(src[i:], "#!") && i+2 < len(src) && src[i+2] != '[' && src[i+2] != '\r' && src[i+2] != '\n' {
			return l.emit(fh.CommentPreproc, i, lexkit.LineEnd(src, i))
		}
		l.push(base)
		return 0, true
	case base:
		return l.base(i)
	case modname, funcname:
		if n := l.spaces(i); n > 0 {
			return l.emit(fh.Text, i, i+n)
		}
		l.pop()
		if n := l.ident(i); n > 0 {
			t := fh.NameNamespace
			if st == funcname {
				t = fh.NameFunction
			}
			return l.emit(t, i, i+n)
		}
		return 0, true
	case lifetime:
		if isIdentStart(src[i]) { // [a-zA-Z_]+\w*
			return l.emit(fh.NameAttribute, i, i+l.ident(i))
		}
		l.pop()
		return 0, true
	case numberLit:
		l.pop()
		// [ui](8|16|32|64|size) and f(32|64)
		if c := src[i]; c == 'u' || c == 'i' {
			for _, w := range [...]string{"8", "16", "32", "64", "size"} {
				if strings.HasPrefix(src[i+1:], w) {
					return l.emit(fh.Keyword, i, i+1+len(w))
				}
			}
		} else if c == 'f' && (strings.HasPrefix(src[i+1:], "32") || strings.HasPrefix(src[i+1:], "64")) {
			return l.emit(fh.Keyword, i, i+3)
		}
		return 0, true
	case attribute:
		switch src[i] {
		case '"':
			l.push(str)
			return l.emit(fh.LiteralString, i, i+1)
		case '[':
			l.push(attribute)
			return l.emit(fh.CommentPreproc, i, i+1)
		case ']':
			l.pop()
			return l.emit(fh.CommentPreproc, i, i+1)
		}
		j := i + 1
		for j < len(src) && src[j] != '"' && src[j] != ']' && src[j] != '[' {
			j++
		}
		return l.emit(fh.CommentPreproc, i, j)
	case bytestring, str:
		if src[i] == '"' {
			l.pop()
			return l.emit(fh.LiteralString, i, i+1)
		}
		if n := l.escape(i, st == bytestring); n > 0 {
			return l.emit(fh.LiteralStringEscape, i, i+n)
		}
		if src[i] == '\\' {
			return l.emit(fh.LiteralString, i, i+1)
		}
		j := i + 1
		for j < len(src) && src[j] != '\\' && src[j] != '"' {
			j++
		}
		return l.emit(fh.LiteralString, i, j)
	case fstring:
		c := src[i]
		if c == '"' {
			l.pop()
			return l.emit(fh.LiteralString, i, i+1)
		}
		if n := l.escape(i, false); n > 0 {
			return l.emit(fh.LiteralStringEscape, i, i+n)
		}
		if (c == '{' || c == '}') && l.at(i+1) == c { // \{\{|\}\}
			return l.emit(fh.LiteralStringEscape, i, i+2)
		}
		if c == '{' { // \{[^}]*\}
			if k := strings.IndexByte(src[i+1:], '}'); k >= 0 {
				return l.emit(fh.LiteralStringInterpol, i, i+k+2)
			}
			return 0, false
		}
		if c == '}' {
			return 0, false
		}
		if c == '\\' {
			return l.emit(fh.LiteralString, i, i+1)
		}
		j := i + 1
		for j < len(src) && strings.IndexByte("\\\"{}", src[j]) < 0 {
			j++
		}
		return l.emit(fh.LiteralString, i, j)
	case comment, doccomment:
		t := fh.CommentMultiline
		if st == doccomment {
			t = fh.LiteralStringDoc
		}
		c := src[i]
		switch {
		case c != '*' && c != '/':
			j := i + 1
			for j < len(src) && src[j] != '*' && src[j] != '/' {
				j++
			}
			return l.emit(t, i, j)
		case c == '/' && l.at(i+1) == '*':
			l.push(st)
			return l.emit(t, i, i+2)
		case c == '*' && l.at(i+1) == '/':
			l.pop()
			return l.emit(t, i, i+2)
		}
		return l.emit(t, i, i+1)
	case typename:
		if n := l.spaces(i); n > 0 {
			return l.emit(fh.Text, i, i+n)
		}
		if src[i] == '&' {
			return l.emit(fh.KeywordPseudo, i, i+1)
		}
		if n, ok := l.lifetimeStart(i); ok {
			return n, true
		}
		if n := l.words.Match(src, i, lBuiltins); n > 0 {
			return l.emit(fh.NameBuiltin, i, i+n)
		}
		if n := l.words.Match(src, i, lTypes); n > 0 {
			return l.emit(fh.KeywordType, i, i+n)
		}
		l.pop()
		if i < l.typeEnd {
			// Production: a keyword (mut in &mut T, dyn, impl) stays a keyword, and a name is NameClass
			// or NameNamespace (types.go).
			if l.words.Match(src, i, lKeywords) > 0 {
				return 0, true
			}
			if n := l.ident(i); n > 0 {
				return l.emit(l.typeName(i, n), i, i+n)
			}
		}
		if n := l.ident(i); n > 0 {
			return l.emit(fh.NameClass, i, i+n)
		}
		return 0, true
	}
	return 0, false
}

// lifetimeStart tries the '(static|_) and ' rules.
func (l *lexer) lifetimeStart(i int) (int, bool) {
	if l.src[i] != '\'' {
		return 0, false
	}
	if strings.HasPrefix(l.src[i+1:], "static") {
		return l.emit(fh.NameBuiltin, i, i+7)
	}
	if l.at(i+1) == '_' {
		return l.emit(fh.NameBuiltin, i, i+2)
	}
	l.push(lifetime)
	return l.emit(fh.NameAttribute, i, i+1)
}

// charBody returns the length up to the ' when src[k:] matches \\['"\\nrt]|\\x[0-7][0-9a-fA-F]|\\0|\\u\{…\}|.
// (for bytes, \\['"\\nrt]|\\x[0-9a-fA-F]{2}|\\0|.) followed by ' (0 if there is no match).
func (l *lexer) charBody(k int, bytes bool) int {
	src := l.src
	var cands [2]int
	n := 0
	if l.at(k) == '\\' {
		switch c := l.at(k + 1); {
		case c != 0 && strings.IndexByte("'\"\\nrt", c) >= 0, c == '0':
			cands[n] = 2
			n++
		case c == 'x':
			if h := l.at(k + 2); (bytes && isHex(h) || '0' <= h && h <= '7') && isHex(l.at(k+3)) {
				cands[n] = 4
				n++
			}
		case c == 'u' && !bytes:
			if e := l.escape(k, false); e > 0 {
				cands[n] = e
				n++
			}
		}
	}
	if k < len(src) && src[k] != '\n' { // .
		_, w := lexkit.Rune(src, k)
		cands[n] = w
		n++
	}
	for _, c := range cands[:n] {
		if l.at(k+c) == '\'' {
			return c
		}
	}
	return 0
}

// constant returns the length of \b(r#)?_?([A-Z][A-Z0-9_]*){2,}\b (0 if there is no match).
func (l *lexer) constant(i int) int {
	src := l.src
	if !lexkit.WordBoundary(src, i) {
		return 0
	}
	k := i
	if strings.HasPrefix(src[k:], "r#") {
		k += 2
	}
	if l.at(k) == '_' {
		k++
	}
	if c := l.at(k); c < 'A' || c > 'Z' {
		return 0
	}
	upper := 0
	j := k
	for ; j < len(src); j++ {
		c := src[j]
		if 'A' <= c && c <= 'Z' {
			upper++
		} else if !('0' <= c && c <= '9' || c == '_') {
			break
		}
	}
	if upper < 2 || !lexkit.WordBoundary(src, j) {
		return 0
	}
	return j - i
}

// macroStrings holds the names of the rules in which a format string follows NameFunctionMagic.
var macroStrings = [...]string{"eprint!", "eprintln!", "print!", "println!", "format!", "format_args!", "panic!",
	"todo!", "unreachable!", "unimplemented!"}

func (l *lexer) run(j int, f func(byte) bool) int {
	for j < len(l.src) && f(l.src[j]) {
		j++
	}
	return j
}

func isDecU(c byte) bool { return '0' <= c && c <= '9' || c == '_' }

// number tries the number rules. On a match it returns the end and the type (the number_lit state
// follows).
func (l *lexer) number(i int) (int, fh.TokenType) {
	src := l.src
	c := src[i]
	if c == '0' {
		var f func(byte) bool
		var t fh.TokenType
		switch l.at(i + 1) {
		case 'b':
			f, t = func(c byte) bool { return c == '0' || c == '1' || c == '_' }, fh.LiteralNumberBin
		case 'o':
			f, t = func(c byte) bool { return '0' <= c && c <= '7' || c == '_' }, fh.LiteralNumberOct
		case 'x', 'X':
			f, t = func(c byte) bool { return isHex(c) || c == '_' }, fh.LiteralNumberHex
		}
		if f != nil && f(l.at(i+2)) {
			return l.run(i+2, f), t
		}
	}
	if c < '0' || c > '9' {
		return 0, 0
	}
	j := l.run(i+1, isDecU)
	exp := func(k int) int { // [eE][+\-]?[0-9_]+
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
		r := l.run(j+1, isDecU)
		if r > j+1 { // \.[0-9_]+[eE][+\-]?[0-9_]+
			if e := exp(r); e >= 0 {
				return e, fh.LiteralNumberFloat
			}
		}
		// \.[0-9_]*(?!\.). If a . follows, shortening [0-9_]* by one character makes the next character not a .
		if l.at(r) != '.' {
			return r, fh.LiteralNumberFloat
		}
		if r > j+1 {
			return r - 1, fh.LiteralNumberFloat
		}
	}
	if e := exp(j); e >= 0 {
		return e, fh.LiteralNumberFloat
	}
	return j, fh.LiteralNumberInteger
}

func (l *lexer) base(i int) (int, bool) {
	src := l.src
	c := src[i]
	if c == '\n' {
		return l.emit(fh.TextWhitespace, i, i+1)
	}
	if n := l.spaces(i); n > 0 {
		return l.emit(fh.TextWhitespace, i, i+n)
	}
	if c == '/' {
		if strings.HasPrefix(src[i:], "//") {
			nl := strings.IndexByte(src[i+2:], '\n')
			if nl >= 0 {
				e := i + 2 + nl + 1
				// //!.*?\n and ///(\n|[^/].*?\n)
				if l.at(i+2) == '!' || l.at(i+2) == '/' && l.at(i+3) != '/' {
					return l.emit(fh.LiteralStringDoc, i, e)
				}
				return l.emit(fh.CommentSingle, i, e) // //(.*?)\n
			}
		}
		if strings.HasPrefix(src[i:], "/**") {
			if d := l.at(i + 3); d == '\n' || i+3 < len(src) && d != '/' && d != '*' { // /\*\*(\n|[^/*])
				_, n := lexkit.Rune(src, i+3)
				l.push(doccomment)
				return l.emit(fh.LiteralStringDoc, i, i+3+n)
			}
		}
		if strings.HasPrefix(src[i:], "/*!") {
			l.push(doccomment)
			return l.emit(fh.LiteralStringDoc, i, i+3)
		}
		if strings.HasPrefix(src[i:], "/*") {
			l.push(comment)
			return l.emit(fh.CommentMultiline, i, i+2)
		}
	}
	if c == '"' {
		l.push(str)
		return l.emit(fh.LiteralString, i, i+1)
	}
	// \$([a-zA-Z_]\w*|\(,?|\),?|,?)
	if c == '$' {
		n := 1
		switch d := l.at(i + 1); {
		case isIdentStart(d):
			n += l.ident(i + 1)
		case d == '(' || d == ')':
			n++
			if l.at(i+2) == ',' {
				n++
			}
		case d == ',':
			n++
		}
		return l.emit(fh.CommentPreproc, i, i+n)
	}
	if n := l.words.Match(src, i, lKeywords); n > 0 {
		return l.emit(fh.Keyword, i, i+n)
	}
	if n := l.words.Match(src, i, lReserved); n > 0 {
		return l.emit(fh.KeywordReserved, i, i+n)
	}
	if n := l.words.Match(src, i, lConstants); n > 0 {
		return l.emit(fh.KeywordConstant, i, i+n)
	}
	n, _ := l.words.Word(src, i)
	w := src[i : i+n]
	switch w {
	case "self":
		return l.emit(fh.NameBuiltinPseudo, i, i+n)
	case "mod":
		l.push(modname)
		return l.emit(fh.Keyword, i, i+n)
	case "let":
		return l.emit(fh.KeywordDeclaration, i, i+n)
	case "fn":
		l.push(funcname)
		return l.emit(fh.Keyword, i, i+n)
	case "struct", "enum", "type", "union":
		l.push(typename)
		return l.emit(fh.Keyword, i, i+n)
	case "default": // (default)(\s+)(type|fn)\b
		if s := l.spaces(i + n); s > 0 {
			m, _ := l.words.Word(src, i+n+s)
			if v := src[i+n+s : i+n+s+m]; v == "type" || v == "fn" {
				l.x.Emit(fh.Keyword, i, i+n)
				l.x.Emit(fh.Text, i+n, i+n+s)
				l.x.Emit(fh.Keyword, i+n+s, i+n+s+m)
				return n + s + m, true
			}
		}
	}
	if m := l.words.Match(src, i, lTypes); m > 0 {
		return l.emit(fh.KeywordType, i, i+m)
	}
	if w == "Self" { // [sS]elf\b
		return l.emit(fh.NameBuiltinPseudo, i, i+n)
	}
	if m := l.words.Match(src, i, lBuiltins); m > 0 {
		return l.emit(fh.NameBuiltin, i, i+m)
	}
	if c == ':' && l.at(i+1) == ':' && lexkit.WordBoundary(src, i+2) { // ::\b
		return l.emit(fh.Text, i, i+2)
	}
	if c == ':' || c == '-' && l.at(i+1) == '>' { // (?::|->)
		e := i + 1
		if c == '-' {
			e++
		}
		if c == '-' || l.ctx.annotation() {
			l.markType(i, e-i)
		}
		l.push(typename)
		return l.emit(fh.Text, i, e)
	}
	// (break|continue)(\b\s*)(\'[A-Za-z_]\w*)?
	if w == "break" || w == "continue" {
		s := l.spaces(i + n)
		l.x.Emit(fh.Keyword, i, i+n)
		l.x.Emit(fh.TextWhitespace, i+n, i+n+s)
		e := i + n + s
		if l.at(e) == '\'' {
			if m := l.ident(e + 1); m > 0 {
				l.x.Emit(fh.NameLabel, e, e+1+m)
				e += 1 + m
			}
		}
		return e - i, true
	}
	// '(…)' and (b)('(…)')
	if c == '\'' {
		if m := l.charBody(i+1, false); m > 0 {
			return l.emit(fh.LiteralStringChar, i, i+m+2)
		}
	}
	if c == 'b' && l.at(i+1) == '\'' {
		if m := l.charBody(i+2, true); m > 0 {
			l.x.Emit(fh.LiteralStringAffix, i, i+1)
			l.x.Emit(fh.LiteralStringChar, i+1, i+m+3)
			return m + 3, true
		}
	}
	if '0' <= c && c <= '9' {
		if e, t := l.number(i); e > 0 {
			l.push(numberLit)
			return l.emit(t, i, e)
		}
	}
	if c == 'b' && l.at(i+1) == '"' { // (b)(")
		l.x.Emit(fh.LiteralStringAffix, i, i+1)
		l.x.Emit(fh.LiteralString, i+1, i+2)
		l.push(bytestring)
		return 2, true
	}
	// (?s)(b?r)(#*)(".*?"\2)
	if c == 'r' || c == 'b' && l.at(i+1) == 'r' {
		a := i + 1
		if c == 'b' {
			a++
		}
		h := a
		for l.at(h) == '#' {
			h++
		}
		if l.at(h) == '"' {
			closing := `"` + src[a:h]
			if k := strings.Index(src[h+1:], closing); k >= 0 {
				e := h + 1 + k + len(closing)
				l.x.Emit(fh.LiteralStringAffix, i, a)
				l.x.Emit(fh.LiteralString, a, h)
				l.x.Emit(fh.LiteralString, h, e)
				return e - i, true
			}
		}
	}
	if n, ok := l.lifetimeStart(i); ok {
		return n, true
	}
	if c == '.' && l.at(i+1) == '.' { // \.\.=?
		e := i + 2
		if l.at(e) == '=' {
			e++
		}
		return l.emit(fh.Operator, i, e)
	}
	if strings.IndexByte("{}()[],.;", c) >= 0 {
		return l.emit(fh.Punctuation, i, i+1)
	}
	if strings.IndexByte("+-*/%&|<>^!~@=:?", c) >= 0 {
		if c == '<' {
			l.markType(i, 1) // the type arguments of a turbofish
		}
		return l.emit(fh.Operator, i, i+1)
	}
	if m := l.constant(i); m > 0 {
		return l.emit(fh.NameConstant, i, i+m)
	}
	// macros that take a format string (println! and the like)
	for _, name := range macroStrings {
		if strings.HasPrefix(src[i:], name) {
			k := i + len(name)
			s1 := l.spaces(k)
			if l.at(k+s1) == '(' {
				s2 := l.spaces(k + s1 + 1)
				if q := k + s1 + 1 + s2; l.at(q) == '"' {
					l.x.Emit(fh.NameFunctionMagic, i, k)
					l.x.Emit(fh.TextWhitespace, k, k+s1)
					l.x.Emit(fh.Punctuation, k+s1, k+s1+1)
					l.x.Emit(fh.TextWhitespace, k+s1+1, q)
					l.x.Emit(fh.LiteralString, q, q+1)
					l.push(fstring)
					return q + 1 - i, true
				}
			}
		}
	}
	// ([a-zA-Z_]\w*!)(\s*)(\(|\[|\{)
	if m := l.ident(i); m > 0 && l.at(i+m) == '!' {
		s := l.spaces(i + m + 1)
		if d := l.at(i + m + 1 + s); d == '(' || d == '[' || d == '{' {
			e := i + m + 1
			l.x.Emit(fh.NameFunctionMagic, i, e)
			l.x.Emit(fh.TextWhitespace, e, e+s)
			l.x.Emit(fh.Punctuation, e+s, e+s+1)
			return e + s + 1 - i, true
		}
	}
	// (r#)?[a-zA-Z_]\w*
	if strings.HasPrefix(src[i:], "r#") {
		if m := l.ident(i + 2); m > 0 {
			return l.emit(fh.Name, i, i+2+m)
		}
	}
	if m := l.ident(i); m > 0 {
		if i < l.typeEnd && src[i:i+m] != "_" { // production: a name in a type (types.go)
			return l.emit(l.typeName(i, m), i, i+m)
		}
		return l.emit(fh.Name, i, i+m)
	}
	if c == '#' { // #!?\[ and #
		if l.at(i+1) == '[' {
			l.push(attribute)
			return l.emit(fh.CommentPreproc, i, i+2)
		}
		if l.at(i+1) == '!' && l.at(i+2) == '[' {
			l.push(attribute)
			return l.emit(fh.CommentPreproc, i, i+3)
		}
		return l.emit(fh.Text, i, i+1)
	}
	return 0, false
}
