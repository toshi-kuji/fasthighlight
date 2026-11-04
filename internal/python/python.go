// Package python is the Python tokenizer. lang/python registers it. Options (the switch for the
// intended differences from Chroma) exists only for the comparison in compat/ and is not exposed.
//
// The token types and token boundaries reproduce the rules of Chroma 2.27.0's python.xml without
// using regular expressions. The string states, which in Chroma are combined (a state that chains
// the rules of two states), are represented by a combination of the quote type and the escape rules
// (frame).
package python

import (
	"strings"
	"unicode"

	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/lexkit"
)

// Indexes of the word lists (lists.go). A value in words is a set of bits for the lists that
// contain the word.
const (
	lKeywords = iota
	lConstants
	lExprKeywords
	lOperatorWords
	lBuiltins
	lPseudo
	lExceptions
	lMagicFuncs
	lMagicVars
	numLists
)

var (
	words map[string]uint16
	// spaced holds the words that contain a space (such as "yield from"), only those that come
	// before the first word ("yield") in the list. Those that come after it never match, because the
	// first word matches first.
	spaced [numLists][]string
)

// Prepare builds the word tables and so on (Prepare of fasthighlight.Language).
func Prepare() {
	words = map[string]uint16{}
	for k, alts := range [numLists]string{keywords, keywordConstants, exprKeywords, operatorWords,
		builtins, builtinPseudo, exceptions, magicFuncs, magicVars} {
		list := strings.Split(alts, "|")
		for idx, w := range list {
			first, _, isSpaced := strings.Cut(w, " ")
			if !isSpaced {
				words[w] |= 1 << k
				continue
			}
			if i := indexOf(list, first); i < 0 || idx < i {
				spaced[k] = append(spaced[k], w)
			}
		}
	}
}

func indexOf(list []string, w string) int {
	for i, x := range list {
		if x == w {
			return i
		}
	}
	return -1
}

// The kinds of state.
type kind uint8

const (
	root       kind = iota
	funcname        // after def
	classname       // after class
	fromimport      // after from
	importName      // after import
	fexpr           // inside {…} of an f-string (expr-inside-fstring)
	fexprInner      // inside brackets within that (expr-inside-fstring-inner)
	str             // string
)

// frame is one state on the stack. For a string, it holds the quote and the rules that Chroma's
// combined prepends.
type frame struct {
	kind   kind
	q      byte // ' or "
	triple bool // ''' or """
	f      bool // f-string (the fstrings-double rules); if false, the strings-double rules
	braces bool // rfstringescape ({{ and }})
	escape bool // stringescape (\n and so on)
}

// Options is the switch for the intended differences from Chroma. The zero value is the production
// setting (the one lang/python registers).
type Options struct {
	// If ChromaDocstring is true, the docstring rule is made the same as Chroma's. In regexp2, the
	// {,2} in Chroma's rule ^(\s*)([rRuUbB]{,2})("""(?:.|\n)*?""") is not a repetition but "one
	// character followed by the literal characters "{,2}"", so an ordinary docstring gets the string
	// color. If false, {,2} is read as a repetition of 0 to 2 times (as in Pygments), and a
	// """…""" at the start of a line gets the docstring color.
	ChromaDocstring bool
}

type lexer struct {
	opt   Options
	x     *lexkit.Tokens
	src   string
	stack []frame
	// wpos, wlen and wmask are the last position examined, the length of the run of word characters
	// from there, and the lists that contain that word (so that listWord does not examine them
	// repeatedly).
	wpos, wlen int
	wmask      uint16
}

// Lex splits src into tokens (the same contract as Lex of fasthighlight.Language). compat/ uses it
// to compare with Chroma while switching o.
func Lex(src string, end int, o Options) []fh.Token {
	l := &lexer{opt: o, x: lexkit.NewTokens(src), src: src, stack: []frame{{kind: root}}, wpos: -1}
	for i := 0; i < end; {
		f := l.stack[len(l.stack)-1]
		n, ok := l.step(f, i)
		if !ok {
			if src[i] == '\n' && f.kind != root {
				l.stack = append(l.stack[:0], frame{kind: root})
				continue
			}
			n = l.x.Error(i)
		}
		i += n
	}
	return l.x.Result()
}

func (l *lexer) emit(t fh.TokenType, s, e int) (int, bool) { l.x.Emit(t, s, e); return e - s, true }
func (l *lexer) push(f frame)                              { l.stack = append(l.stack, f) }
func (l *lexer) pop()                                      { l.stack = l.stack[:len(l.stack)-1] }

// at returns src[i] (0 if out of range).
func (l *lexer) at(i int) byte {
	if i < len(l.src) {
		return l.src[i]
	}
	return 0
}

func (l *lexer) step(f frame, i int) (int, bool) {
	switch f.kind {
	case root:
		return l.root(i)
	case funcname:
		if n, ok := l.listWord(i, lMagicFuncs); ok {
			return l.emit(fh.NameFunctionMagic, i, i+n)
		}
		if n := nameLen(l.src, i); n > 0 {
			l.pop()
			return l.emit(fh.NameFunction, i, i+n)
		}
		l.pop()
		return 0, true
	case classname:
		if n := nameLen(l.src, i); n > 0 {
			l.pop()
			return l.emit(fh.NameClass, i, i+n)
		}
		return 0, false
	case fromimport:
		if n := lexkit.Span(l.src, i, lexkit.IsSpace); n > 0 && strings.HasPrefix(l.src[i+n:], "import") && lexkit.WordBoundary(l.src, i+n+6) {
			l.x.Emit(fh.Text, i, i+n)
			l.x.Emit(fh.KeywordNamespace, i+n, i+n+6)
			l.pop()
			return n + 6, true
		}
		if l.src[i] == '.' {
			return l.emit(fh.NameNamespace, i, i+1)
		}
		if strings.HasPrefix(l.src[i:], "None") && lexkit.WordBoundary(l.src, i+4) {
			l.pop()
			return l.emit(fh.NameBuiltinPseudo, i, i+4)
		}
		if n := nameLen(l.src, i); n > 0 {
			return l.emit(fh.NameNamespace, i, i+n)
		}
		l.pop()
		return 0, true
	case importName:
		// (\s+)(as)(\s+)
		if a := lexkit.Span(l.src, i, lexkit.IsSpace); a > 0 && strings.HasPrefix(l.src[i+a:], "as") {
			if b := lexkit.Span(l.src, i+a+2, lexkit.IsSpace); b > 0 {
				l.x.Emit(fh.Text, i, i+a)
				l.x.Emit(fh.Keyword, i+a, i+a+2)
				l.x.Emit(fh.Text, i+a+2, i+a+2+b)
				return a + 2 + b, true
			}
		}
		if l.src[i] == '.' {
			return l.emit(fh.NameNamespace, i, i+1)
		}
		if n := nameLen(l.src, i); n > 0 {
			return l.emit(fh.NameNamespace, i, i+n)
		}
		// (\s*)(,)(\s*)
		if a := lexkit.Span(l.src, i, lexkit.IsSpace); l.at(i+a) == ',' {
			b := lexkit.Span(l.src, i+a+1, lexkit.IsSpace)
			l.x.Emit(fh.Text, i, i+a)
			l.x.Emit(fh.Operator, i+a, i+a+1)
			l.x.Emit(fh.Text, i+a+1, i+a+1+b)
			return a + 1 + b, true
		}
		l.pop()
		return 0, true
	case fexpr:
		switch l.src[i] {
		case '{', '(', '[':
			l.push(frame{kind: fexprInner})
			return l.emit(fh.Punctuation, i, i+1)
		}
		// (=\s*)?(\![sraf])?\} and (=\s*)?(\![sraf])?:
		j := i
		if l.src[j] == '=' {
			j += 1 + lexkit.Span(l.src, j+1, lexkit.IsSpace)
		}
		if l.at(j) == '!' && strings.IndexByte("sraf", l.at(j+1)) >= 0 && l.at(j+1) != 0 {
			j += 2
		}
		if c := l.at(j); c == '}' || c == ':' {
			l.pop()
			return l.emit(fh.LiteralStringInterpol, i, j+1)
		}
		if n := lexkit.Span(l.src, i, lexkit.IsSpace); n > 0 {
			return l.emit(fh.Text, i, i+n)
		}
		return l.expr(i)
	case fexprInner:
		switch l.src[i] {
		case '{', '(', '[':
			l.push(frame{kind: fexprInner})
			return l.emit(fh.Punctuation, i, i+1)
		case ']', ')', '}':
			l.pop()
			return l.emit(fh.Punctuation, i, i+1)
		}
		if n := lexkit.Span(l.src, i, lexkit.IsSpace); n > 0 {
			return l.emit(fh.Text, i, i+n)
		}
		return l.expr(i)
	case str:
		return l.str(f, i)
	}
	return 0, false
}

// root tries the rules of root.
func (l *lexer) root(i int) (int, bool) {
	src := l.src
	c := src[i]
	if c == '\n' {
		return l.emit(fh.Text, i, i+1)
	}
	// ^(\s*)([rRuUbB]{,2})("""(?:.|\n)*?""") and its ''' version (docstring). See Options.ChromaDocstring.
	if i == 0 || src[i-1] == '\n' {
		if n, ok := l.docstring(i); ok {
			return n, true
		}
	}
	switch c {
	case '#':
		e := lexkit.LineEnd(src, i)
		if i == 0 && l.at(1) == '!' && e > 2 { // \A#!.+$
			return l.emit(fh.CommentHashbang, i, e)
		}
		return l.emit(fh.CommentSingle, i, e) // #.*$
	case '\\':
		if l.at(i+1) == '\n' {
			return l.emit(fh.Text, i, i+2)
		}
		return l.emit(fh.Text, i, i+1)
	}
	// keywords
	if n, ok := l.listWord(i, lKeywords); ok {
		return l.emit(fh.Keyword, i, i+n)
	}
	if n, ok := l.listWord(i, lConstants); ok {
		return l.emit(fh.KeywordConstant, i, i+n)
	}
	// (def)((?:\s|\\\s)+) and so on
	for _, r := range [...]struct {
		word string
		t    fh.TokenType
		next kind
	}{{"def", fh.Keyword, funcname}, {"class", fh.Keyword, classname}, {"from", fh.KeywordNamespace, fromimport}, {"import", fh.KeywordNamespace, importName}} {
		if strings.HasPrefix(src[i:], r.word) {
			if n := l.spaces(i + len(r.word)); n > 0 {
				l.x.Emit(r.t, i, i+len(r.word))
				l.x.Emit(fh.Text, i+len(r.word), i+len(r.word)+n)
				l.push(frame{kind: r.next})
				return len(r.word) + n, true
			}
		}
	}
	return l.expr(i)
}

// docstring tries the docstring rule from i, the start of a line.
func (l *lexer) docstring(i int) (int, bool) {
	src := l.src
	j := i + lexkit.Span(src, i, lexkit.IsSpace)
	a := j // end of the prefix
	if l.opt.ChromaDocstring {
		// one character of [rRuUbB], and the literal characters "{,2}"
		if j >= len(src) || strings.IndexByte("rRuUbB", src[j]) < 0 || !strings.HasPrefix(src[j+1:], "{,2}") {
			return 0, false
		}
		a = j + 5
	} else {
		// [rRuUbB]{0,2}. If taking greedily is not followed by a quote, taking fewer does not match
		// either (because the next character would be a prefix character).
		for a < len(src) && a < j+2 && strings.IndexByte("rRuUbB", src[a]) >= 0 {
			a++
		}
	}
	for _, q := range [...]string{`"""`, `'''`} {
		if strings.HasPrefix(src[a:], q) {
			if k := strings.Index(src[a+3:], q); k >= 0 {
				e := a + 3 + k + 3
				l.x.Emit(fh.Text, i, j)
				l.x.Emit(fh.LiteralStringAffix, j, a)
				l.x.Emit(fh.LiteralStringDoc, a, e)
				return e - i, true
			}
		}
	}
	return 0, false
}

// spaces returns the length matched by (?:\s|\\\s)+.
func (l *lexer) spaces(i int) int {
	j := i
	for j < len(l.src) {
		r, n := lexkit.Rune(l.src, j)
		if lexkit.IsSpace(r) {
			j += n
			continue
		}
		if r == '\\' && j+1 < len(l.src) {
			if r2, n2 := lexkit.Rune(l.src, j+1); lexkit.IsSpace(r2) {
				j += 1 + n2
				continue
			}
		}
		break
	}
	return j - i
}

// listWord returns the length and true if a word ((a|b|…)\b) of list k matches at position i.
func (l *lexer) listWord(i, k int) (int, bool) {
	for _, w := range spaced[k] {
		if strings.HasPrefix(l.src[i:], w) && lexkit.WordBoundary(l.src, i+len(w)) {
			return len(w), true
		}
	}
	// Every word in the lists consists only of word characters, so the \b after a word is the same as
	// the run of word characters ending there.
	if l.wpos != i {
		l.wpos, l.wlen, l.wmask = i, lexkit.Span(l.src, i, lexkit.IsWord), 0
		if l.wlen > 0 {
			l.wmask = words[l.src[i:i+l.wlen]]
		}
	}
	if l.wmask&(1<<k) != 0 {
		return l.wlen, true
	}
	return 0, false
}

// isNameStart and isNameChar are [_\p{L}] and [_\p{L}\p{N}].
func isNameStart(r rune) bool { return r == '_' || unicode.IsLetter(r) }
func isNameChar(r rune) bool  { return r == '_' || unicode.IsLetter(r) || unicode.IsNumber(r) }

// nameLen returns the length matched by [_\p{L}][_\p{L}\p{N}]*.
func nameLen(src string, i int) int {
	if i >= len(src) {
		return 0
	}
	if r, n := lexkit.Rune(src, i); isNameStart(r) {
		return n + lexkit.Span(src, i+n, isNameChar)
	}
	return 0
}

func isQuote(c byte) bool { return c == '"' || c == '\'' }

func lower(c byte) byte {
	if 'A' <= c && c <= 'Z' {
		return c + 'a' - 'A'
	}
	return c
}

// expr tries the rules of expr.
func (l *lexer) expr(i int) (int, bool) {
	src := l.src
	if n, ok := l.stringStart(i); ok {
		return n, true
	}
	c := src[i]
	// [^\S\n]+
	if n := lexkit.Span(src, i, func(r rune) bool { return r != '\n' && lexkit.IsSpace(r) }); n > 0 {
		return l.emit(fh.Text, i, i+n)
	}
	if n, t := l.number(i); n > 0 {
		return l.emit(t, i, i+n)
	}
	// !=|==|<<|>>|:=|[-~+/*%=<>&^|.]
	if i+1 < len(src) {
		switch src[i : i+2] {
		case "!=", "==", "<<", ">>", ":=":
			return l.emit(fh.Operator, i, i+2)
		}
	}
	if strings.IndexByte("-~+/*%=<>&^|.", c) >= 0 {
		return l.emit(fh.Operator, i, i+1)
	}
	if strings.IndexByte("]{}:(),;[", c) >= 0 {
		return l.emit(fh.Punctuation, i, i+1)
	}
	if n, ok := l.listWord(i, lOperatorWords); ok {
		return l.emit(fh.OperatorWord, i, i+n)
	}
	if n, ok := l.listWord(i, lExprKeywords); ok {
		return l.emit(fh.Keyword, i, i+n)
	}
	if n, ok := l.listWord(i, lConstants); ok {
		return l.emit(fh.KeywordConstant, i, i+n)
	}
	if i == 0 || src[i-1] != '.' { // (?<!\.)
		for _, b := range [...]struct {
			k int
			t fh.TokenType
		}{{lBuiltins, fh.NameBuiltin}, {lPseudo, fh.NameBuiltinPseudo}, {lExceptions, fh.NameException}} {
			if n, ok := l.listWord(i, b.k); ok {
				return l.emit(b.t, i, i+n)
			}
		}
	}
	if n, ok := l.listWord(i, lMagicFuncs); ok {
		return l.emit(fh.NameFunctionMagic, i, i+n)
	}
	if n, ok := l.listWord(i, lMagicVars); ok {
		return l.emit(fh.NameVariableMagic, i, i+n)
	}
	// name
	if c == '@' {
		if n := nameLen(src, i+1); n > 0 {
			// @name(\s*\.\s*name)*
			e := i + 1 + n
			for {
				j := e + lexkit.Span(src, e, lexkit.IsSpace)
				if l.at(j) != '.' {
					break
				}
				j++
				j += lexkit.Span(src, j, lexkit.IsSpace)
				m := nameLen(src, j)
				if m == 0 {
					break
				}
				e = j + m
			}
			return l.emit(fh.NameDecorator, i, e)
		}
		return l.emit(fh.Operator, i, i+1)
	}
	if n := nameLen(src, i); n > 0 {
		return l.emit(fh.Name, i, i+n)
	}
	return 0, false
}

// stringStart tries expr's rules for the start of a string (the prefix and the quote).
func (l *lexer) stringStart(i int) (int, bool) {
	src := l.src
	c0, c1 := lower(src[i]), lower(l.at(i+1))
	var affix int
	var f frame
	switch {
	case (c0 == 'r' && c1 == 'f' || c0 == 'f' && c1 == 'r') && isQuote(l.at(i+2)): // (?i)(rf|fr)
		affix, f = 2, frame{f: true, braces: true}
	case c0 == 'f' && isQuote(l.at(i+1)): // [fF]
		affix, f = 1, frame{f: true, braces: true, escape: true}
	case (c0 == 'r' && c1 == 'b' || c0 == 'b' && c1 == 'r') && isQuote(l.at(i+2)): // (?i)(rb|br|r)
		affix = 2
	case c0 == 'r' && isQuote(l.at(i+1)):
		affix = 1
	case strings.IndexByte("uUbB", src[i]) >= 0 && isQuote(l.at(i+1)): // [uUbB]?
		affix, f = 1, frame{escape: true}
	case isQuote(src[i]):
		f = frame{escape: true}
	default:
		return 0, false
	}
	j := i + affix
	f.kind, f.q = str, src[j]
	n := 1
	if l.at(j+1) == f.q && l.at(j+2) == f.q {
		f.triple, n = true, 3
	}
	l.x.Emit(fh.LiteralStringAffix, i, j)
	l.x.Emit(stringType(f.q), j, j+n)
	l.push(f)
	return affix + n, true
}

func stringType(q byte) fh.TokenType {
	if q == '"' {
		return fh.LiteralStringDouble
	}
	return fh.LiteralStringSingle
}

// str tries the rules of the string state. The order is the same as Chroma's combined:
// rfstringescape, stringescape, and then rules such as tdqs.
func (l *lexer) str(f frame, i int) (int, bool) {
	src := l.src
	c := src[i]
	t := stringType(f.q)
	if f.braces && (c == '{' || c == '}') && l.at(i+1) == c { // \{\{ and \}\}
		return l.emit(fh.LiteralStringEscape, i, i+2)
	}
	if f.escape && c == '\\' {
		if n := escapeLen(src, i); n > 0 {
			return l.emit(fh.LiteralStringEscape, i, i+n)
		}
	}
	if f.triple {
		if c == f.q && l.at(i+1) == f.q && l.at(i+2) == f.q {
			l.pop()
			return l.emit(t, i, i+3)
		}
	} else {
		if c == f.q {
			l.pop()
			return l.emit(t, i, i+1)
		}
		// \\\\|\\q|\\\n
		if c == '\\' {
			if d := l.at(i + 1); d == '\\' || d == f.q || d == '\n' {
				return l.emit(fh.LiteralStringEscape, i, i+2)
			}
		}
	}
	if n, ok := l.strContent(f, i, t); ok {
		return n, true
	}
	if f.triple && c == '\n' {
		return l.emit(t, i, i+1)
	}
	return 0, false
}

// strContent tries the rules of fstrings-double or strings-double (and the single versions).
func (l *lexer) strContent(f frame, i int, t fh.TokenType) (int, bool) {
	src := l.src
	c := src[i]
	if f.f {
		switch c {
		case '}':
			return l.emit(fh.LiteralStringInterpol, i, i+1)
		case '{':
			l.push(frame{kind: fexpr})
			return l.emit(fh.LiteralStringInterpol, i, i+1)
		case '\\', '\'', '"':
			return l.emit(t, i, i+1)
		case '\n':
			return 0, false
		}
		j := i + 1
		for j < len(src) && strings.IndexByte("\\'\"{}\n", src[j]) < 0 {
			j++
		}
		return l.emit(t, i, j)
	}
	switch c {
	case '%':
		if n := percentLen(src, i); n > 0 {
			return l.emit(fh.LiteralStringInterpol, i, i+n)
		}
		return l.emit(t, i, i+1)
	case '{':
		if n := braceLen(src, i); n > 0 {
			return l.emit(fh.LiteralStringInterpol, i, i+n)
		}
		if l.at(i+1) == '{' { // \{{1,2}
			return l.emit(t, i, i+2)
		}
		return l.emit(t, i, i+1)
	case '\\', '\'', '"':
		return l.emit(t, i, i+1)
	case '\n':
		return 0, false
	}
	j := i + 1
	for j < len(src) && strings.IndexByte("\\'\"%{\n", src[j]) < 0 {
		j++
	}
	return l.emit(t, i, j)
}

func isHex(c byte) bool {
	return '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F'
}

func isOct(c byte) bool { return '0' <= c && c <= '7' }

// countBytes returns the number of bytes at the start of src[i:] that satisfy f (at most max).
func countBytes(src string, i, max int, f func(byte) bool) int {
	n := 0
	for n < max && i+n < len(src) && f(src[i+n]) {
		n++
	}
	return n
}

// escapeLen returns the length matched by the stringescape rule
// \\([\\abfnrtv"']|\n|N\{.*?\}|u[a-fA-F0-9]{4}|U[a-fA-F0-9]{8}|x[a-fA-F0-9]{2}|[0-7]{1,3})
// (0 if it does not match). src[i] is \.
func escapeLen(src string, i int) int {
	if i+1 >= len(src) {
		return 0
	}
	switch c := src[i+1]; {
	case strings.IndexByte("\\abfnrtv\"'\n", c) >= 0:
		return 2
	case c == 'N':
		if i+2 < len(src) && src[i+2] == '{' {
			if k := strings.IndexAny(src[i+3:], "}\n"); k >= 0 && src[i+3+k] == '}' {
				return 3 + k + 1
			}
		}
	case c == 'u':
		if countBytes(src, i+2, 4, isHex) == 4 {
			return 6
		}
	case c == 'U':
		if countBytes(src, i+2, 8, isHex) == 8 {
			return 10
		}
	case c == 'x':
		if countBytes(src, i+2, 2, isHex) == 2 {
			return 4
		}
	case isOct(c):
		return 1 + countBytes(src, i+1, 3, isOct)
	}
	return 0
}

func isASCIIDigit(c byte) bool { return '0' <= c && c <= '9' }

// percentLen returns the length matched by
// %(\(\w+\))?[-#0 +]*([0-9]+|[*])?(\.([0-9]+|[*]))?[hlL]?[E-GXc-giorsaux%]
// (0 if it does not match). src[i] is %. For every part, if taking greedily fails to match what
// follows, backtracking does not make it match either.
func percentLen(src string, i int) int {
	at := func(j int) byte {
		if j < len(src) {
			return src[j]
		}
		return 0
	}
	p := i + 1
	if at(p) == '(' {
		if w := lexkit.Span(src, p+1, lexkit.IsWord); w > 0 && at(p+1+w) == ')' {
			p += 2 + w
		}
	}
	for at(p) != 0 && strings.IndexByte("-#0 +", at(p)) >= 0 {
		p++
	}
	if n := countBytes(src, p, len(src), isASCIIDigit); n > 0 {
		p += n
	} else if at(p) == '*' {
		p++
	}
	if at(p) == '.' {
		if n := countBytes(src, p+1, len(src), isASCIIDigit); n > 0 {
			p += 1 + n
		} else if at(p+1) == '*' {
			p += 2
		}
	}
	if c := at(p); c != 0 && strings.IndexByte("hlL", c) >= 0 {
		p++
	}
	if c := at(p); c != 0 && strings.IndexByte("EFGXcdefgiorsaux%", c) >= 0 {
		return p + 1 - i
	}
	return 0
}

// braceLen returns the length matched by
// \{((\w+)((\.\w+)|(\[[^\]]+\]))*)?(\![sra])?(\:(.?[<>=\^])?[-+ ]?#?0?(\d+)?,?(\.\d+)?[E-GXb-gnosx%]?)?\}
// (0 if it does not match). src[i] is {.
func braceLen(src string, i int) int {
	at := func(j int) byte {
		if j < len(src) {
			return src[j]
		}
		return 0
	}
	p := i + 1
	if w := lexkit.Span(src, p, lexkit.IsWord); w > 0 {
		p += w
		for {
			if at(p) == '.' {
				if w := lexkit.Span(src, p+1, lexkit.IsWord); w > 0 {
					p += 1 + w
					continue
				}
			}
			if at(p) == '[' {
				if k := strings.IndexByte(src[p+1:], ']'); k > 0 {
					p += k + 2
					continue
				}
			}
			break
		}
	}
	if at(p) == '!' && at(p+1) != 0 && strings.IndexByte("sra", at(p+1)) >= 0 {
		p += 2
	}
	if at(p) != ':' {
		if at(p) == '}' {
			return p + 1 - i
		}
		return 0
	}
	isAlign := func(c byte) bool { return c != 0 && strings.IndexByte("<>=^", c) >= 0 }
	// Tries (.?[<>=\^])? in the order (one character plus alignment), (alignment only), (none).
	q := p + 1
	if q < len(src) && src[q] != '\n' {
		_, n := lexkit.Rune(src, q)
		if isAlign(at(q + n)) {
			if e := specTail(src, q+n+1); e > 0 {
				return e - i
			}
		}
	}
	if isAlign(at(q)) {
		if e := specTail(src, q+1); e > 0 {
			return e - i
		}
	}
	if e := specTail(src, q); e > 0 {
		return e - i
	}
	return 0
}

// specTail returns the end if [-+ ]?#?0?(\d+)?,?(\.\d+)?[E-GXb-gnosx%]?\} matches (0 if it does not).
func specTail(src string, p int) int {
	at := func(j int) byte {
		if j < len(src) {
			return src[j]
		}
		return 0
	}
	if c := at(p); c == '-' || c == '+' || c == ' ' {
		p++
	}
	if at(p) == '#' {
		p++
	}
	if at(p) == '0' {
		p++
	}
	p += lexkit.Span(src, p, lexkit.IsDigit)
	if at(p) == ',' {
		p++
	}
	if at(p) == '.' {
		if n := lexkit.Span(src, p+1, lexkit.IsDigit); n > 0 {
			p += 1 + n
		}
	}
	if c := at(p); c != 0 && strings.IndexByte("EFGXbcdefgnosx%", c) >= 0 {
		p++
	}
	if at(p) == '}' {
		return p + 1
	}
	return 0
}

// digits returns the length matched by \d(?:_?\d)*.
func digits(src string, i int) int {
	if i >= len(src) {
		return 0
	}
	r, n := lexkit.Rune(src, i)
	if !lexkit.IsDigit(r) {
		return 0
	}
	j := i + n
	for j < len(src) {
		if src[j] == '_' && j+1 < len(src) {
			if r, n := lexkit.Rune(src, j+1); lexkit.IsDigit(r) {
				j += 1 + n
				continue
			}
		}
		r, n := lexkit.Rune(src, j)
		if !lexkit.IsDigit(r) {
			break
		}
		j += n
	}
	return j - i
}

// exponent returns the length matched by [eE][+-]?\d(?:_?\d)*.
func exponent(src string, i int) int {
	if i >= len(src) || src[i] != 'e' && src[i] != 'E' {
		return 0
	}
	if i+1 < len(src) && (src[i+1] == '+' || src[i+1] == '-') {
		if d := digits(src, i+2); d > 0 {
			return 2 + d
		}
		return 0
	}
	if d := digits(src, i+1); d > 0 {
		return 1 + d
	}
	return 0
}

// number tries the rules of numbers. If one matches, it returns the length and the type.
func (l *lexer) number(i int) (int, fh.TokenType) {
	src := l.src
	d := digits(src, i)
	// (\d(?:_?\d)*\.(?:\d(?:_?\d)*)?|(?:\d(?:_?\d)*)?\.\d(?:_?\d)*)([eE][+-]?\d(?:_?\d)*)?
	if d > 0 && l.at(i+d) == '.' {
		j := i + d + 1
		j += digits(src, j)
		return j + exponent(src, j) - i, fh.LiteralNumberFloat
	}
	if d == 0 && src[i] == '.' {
		if d2 := digits(src, i+1); d2 > 0 {
			j := i + 1 + d2
			return j + exponent(src, j) - i, fh.LiteralNumberFloat
		}
		return 0, 0
	}
	if d == 0 {
		return 0, 0
	}
	// \d(?:_?\d)*[eE][+-]?\d(?:_?\d)*j?
	if e := exponent(src, i+d); e > 0 {
		j := i + d + e
		if l.at(j) == 'j' {
			j++
		}
		return j - i, fh.LiteralNumberFloat
	}
	// 0[oO](?:_?[0-7])+ and so on
	if src[i] == '0' {
		for _, b := range [...]struct {
			c byte
			f func(byte) bool
			t fh.TokenType
		}{{'o', isOct, fh.LiteralNumberOct}, {'b', func(c byte) bool { return c == '0' || c == '1' }, fh.LiteralNumberBin}, {'x', isHex, fh.LiteralNumberHex}} {
			if lower(l.at(i+1)) != b.c {
				continue
			}
			j := i + 2
			for {
				if l.at(j) == '_' && j+1 < len(src) && b.f(src[j+1]) {
					j += 2
				} else if j < len(src) && b.f(src[j]) {
					j++
				} else {
					break
				}
			}
			if j > i+2 {
				return j - i, b.t
			}
		}
	}
	return d, fh.LiteralNumberInteger
}
