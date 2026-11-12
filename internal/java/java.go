// Package java is the Java tokenizer. lang/java registers it. Options (the switches for the intended
// differences from Chroma) exist only for the comparison in compat/ and are not exposed.
//
// The token types and boundaries reproduce the rules of Chroma 2.27.0's java.xml without regular expressions.
// java.xml is dot_all, so . also matches a newline.
package java

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
	lDeclarations
	lTypes
	lConstants
)

var lists *lexkit.WordLists

// Prepare builds the word tables and the like (the Prepare of fasthighlight.Language).
func Prepare() { lists = lexkit.NewWordLists(keywords, declarations, types, constants) }

type state uint8

const (
	root state = iota
	class
	module
	varName
	importName
	multiline
	str
)

// Options holds the switches for the intended differences from Chroma. The zero value is the production
// setting (the one lang/java registers).
type Options struct {
	// ChromaIdent, if true, makes the name-start character [^\W\d] the same as Chroma. In regexp2,
	// the \d in [^\W\d] has no effect and it is the same as \w, so a part that starts with a digit also becomes
	// a name (a number does not get the number color). If false, it is a word character other than digits,
	// as the rule intends.
	ChromaIdent bool
	// ChromaTypes, if true, makes a user-defined type name a plain name (Name), as in Chroma, where only
	// the built-in types (int, void, ...) are types. If false (production), a name in a type position is
	// KeywordType too: a capitalized name with type arguments (List<User>, new ArrayList<User>()) and the
	// names inside them, a capitalized name followed by a variable name (User u = ..., User u) as a
	// field, local variable or parameter, and the return type right before a method name (User get().
	ChromaTypes bool
}

type lexer struct {
	o     Options
	x     *lexkit.Tokens
	src   string
	stack []state
	words lexkit.WordCache
	// failFrom and failEnd are the position where the function rule did not match and the end of the
	// word sequence examined then.
	failFrom, failEnd int
	sp                lexkit.SpanCache // the last run of whitespace
	// tc is the last run of type characters measured by the function rule, so that a long run that is not
	// followed by whitespace (such as A<A<A<…) is measured once, not again from each of its positions.
	tc lexkit.SpanCache
	// lineEnd and commentEnd remember failed searches for the newline after // and for */.
	lineEnd, commentEnd lexkit.Finder
	// typeEnd is the end of the type arguments being read (production only): before it, a name is a type.
	typeEnd int
	// prefix is set for the text before a method name that the function rule reads again (its last word
	// is the return type).
	prefix bool
}

// Lex splits src into tokens (with the same contract as the Lex of fasthighlight.Language). compat/ uses it
// to compare with Chroma, switching o.
func Lex(src string, end int, o Options) []fh.Token {
	x := lexkit.NewTokens(src)
	lexInto(x, src, end, o, false)
	return x.Result()
}

func lexInto(x *lexkit.Tokens, src string, end int, o Options, prefix bool) {
	l := &lexer{o: o, x: x, src: src, stack: []state{root}, words: lexkit.WordCache{Lists: lists}, failFrom: -1, prefix: prefix}
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

// nested reads src[s:e] from root as new input, the same as Chroma's usingself. prefix tells that it is
// the text before a method name.
func (l *lexer) nested(s, e int, prefix bool) {
	if s < e {
		lexInto(l.x.Sub(l.src[s:e], s), l.src[s:e], e-s, l.o, prefix)
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

func (l *lexer) runeAt(i int) (rune, int) {
	if i >= len(l.src) {
		return -1, 0
	}
	return lexkit.Rune(l.src, i)
}

func (l *lexer) spaces(i int) int { return l.sp.Span(l.src, i, lexkit.IsSpace) }

// wordStart is [^\W\d] (see Options.ChromaIdent).
func (l *lexer) wordStart(r rune) bool {
	return lexkit.IsWord(r) && (l.o.ChromaIdent || !lexkit.IsDigit(r))
}

// isIdentStart is (?:[^\W\d]|\$), and isNameChar is [\w$].
func (l *lexer) isIdentStart(r rune) bool { return r == '$' || l.wordStart(r) }
func isNameChar(r rune) bool              { return r == '$' || lexkit.IsWord(r) }

// isTypeChar is [\w.\[\]$<>?].
func isTypeChar(r rune) bool {
	return lexkit.IsWord(r) || r == '.' || r == '[' || r == ']' || r == '$' || r == '<' || r == '>' || r == '?'
}

// ident returns the length of (?:[^\W\d]|\$)[\w$]*.
func (l *lexer) ident(i int) int {
	r, n := l.runeAt(i)
	if n == 0 || !l.isIdentStart(r) {
		return 0
	}
	return n + lexkit.Span(l.src, i+n, isNameChar)
}

func (l *lexer) word(i, k int) int { return l.words.Match(l.src, i, k) }

// isWordOf reports whether the word w (word characters only) is at position i and followed by a \b match.
func (l *lexer) isWordOf(i int, w string) bool {
	n, _ := l.words.Word(l.src, i)
	return n == len(w) && l.src[i:i+n] == w
}

func (l *lexer) step(st state, i int) (int, bool) {
	src := l.src
	switch st {
	case root:
		return l.root(i)
	case class, module:
		if n := l.spaces(i); n > 0 {
			return l.emit(fh.Text, i, i+n)
		}
		if n := l.ident(i); n > 0 {
			l.pop()
			return l.emit(fh.NameClass, i, i+n)
		}
	case varName:
		if n := l.ident(i); n > 0 {
			l.pop()
			return l.emit(fh.Name, i, i+n)
		}
	case importName:
		// [\w.]+\*?
		if n := lexkit.Span(src, i, func(r rune) bool { return r == '.' || lexkit.IsWord(r) }); n > 0 {
			if l.at(i+n) == '*' {
				n++
			}
			l.pop()
			return l.emit(fh.NameNamespace, i, i+n)
		}
	case multiline:
		if strings.HasPrefix(src[i:], `"""`) {
			l.pop()
			return l.emit(fh.LiteralString, i, i+3)
		}
		if src[i] == '"' {
			return l.emit(fh.LiteralString, i, i+1)
		}
		return l.str(i)
	case str:
		return l.str(i)
	}
	return 0, false
}

// str tries the rules of the string state.
func (l *lexer) str(i int) (int, bool) {
	src := l.src
	switch src[i] {
	case '"':
		l.pop()
		return l.emit(fh.LiteralString, i, i+1)
	case '\\':
		if c := l.at(i + 1); c == '\\' || c == '"' { // \\\\ and \\"
			return l.emit(fh.LiteralString, i, i+2)
		}
		return l.emit(fh.LiteralString, i, i+1)
	}
	j := i + 1
	for j < len(src) && src[j] != '\\' && src[j] != '"' {
		j++
	}
	return l.emit(fh.LiteralString, i, j)
}

func (l *lexer) bol(i int) bool { return i == 0 || l.src[i-1] == '\n' }

func (l *lexer) root(i int) (int, bool) {
	src := l.src
	c := src[i]
	// (^\s*)((?:(?:public|private|protected|static|strictfp)(?:\s+))*)(record)\b
	if l.bol(i) {
		j := i + l.spaces(i)
		m := j
	mods:
		for {
			for _, w := range [...]string{"public", "private", "protected", "static", "strictfp"} {
				if strings.HasPrefix(src[m:], w) {
					if n := l.spaces(m + len(w)); n > 0 {
						m += len(w) + n
						continue mods
					}
				}
			}
			break
		}
		if strings.HasPrefix(src[m:], "record") && lexkit.WordBoundary(src, m+6) {
			l.x.Emit(fh.TextWhitespace, i, j)
			l.nested(j, m, false)
			l.x.Emit(fh.KeywordDeclaration, m, m+6)
			l.push(class)
			return m + 6 - i, true
		}
	}
	// [^\S\n]+
	if n := lexkit.Span(src, i, func(r rune) bool { return r != '\n' && lexkit.IsSpace(r) }); n > 0 {
		return l.emit(fh.TextWhitespace, i, i+n)
	}
	if c == '/' {
		if l.at(i+1) == '/' { // (//.*?)(\n)
			if k := l.lineEnd.Index(src, i+2, "\n"); k >= 0 {
				k += i + 2
				l.x.Emit(fh.CommentSingle, i, k)
				l.x.Emit(fh.TextWhitespace, k, k+1)
				return k + 1 - i, true
			}
		}
		if l.at(i+1) == '*' { // /\*.*?\*/
			if k := l.commentEnd.Index(src, i+2, "*/"); k >= 0 {
				return l.emit(fh.CommentMultiline, i, i+2+k+2)
			}
		}
	}
	if n := l.word(i, lKeywords); n > 0 {
		return l.emit(fh.Keyword, i, i+n)
	}
	if n, ok := l.function(i); ok {
		return n, true
	}
	// @[^\W\d][\w.]*
	if c == '@' {
		if r, n := l.runeAt(i + 1); n > 0 && l.wordStart(r) {
			e := i + 1 + n + lexkit.Span(src, i+1+n, func(r rune) bool { return r == '.' || lexkit.IsWord(r) })
			return l.emit(fh.NameDecorator, i, e)
		}
	}
	if n := l.word(i, lDeclarations); n > 0 {
		return l.emit(fh.KeywordDeclaration, i, i+n)
	}
	if n := l.word(i, lTypes); n > 0 {
		return l.emit(fh.KeywordType, i, i+n)
	}
	// (package)(\s+)
	if strings.HasPrefix(src[i:], "package") {
		if n := l.spaces(i + 7); n > 0 {
			l.x.Emit(fh.KeywordNamespace, i, i+7)
			l.x.Emit(fh.TextWhitespace, i+7, i+7+n)
			l.push(importName)
			return 7 + n, true
		}
	}
	if n := l.word(i, lConstants); n > 0 {
		return l.emit(fh.KeywordConstant, i, i+n)
	}
	// (class|interface)\b and (module)\b
	for _, w := range [...]string{"class", "interface", "module"} {
		if l.isWordOf(i, w) {
			if w == "module" {
				l.push(module)
			} else {
				l.push(class)
			}
			return l.emit(fh.KeywordDeclaration, i, i+len(w))
		}
	}
	// (var)(\s+)
	if strings.HasPrefix(src[i:], "var") {
		if n := l.spaces(i + 3); n > 0 {
			l.x.Emit(fh.KeywordDeclaration, i, i+3)
			l.x.Emit(fh.TextWhitespace, i+3, i+3+n)
			l.push(varName)
			return 3 + n, true
		}
	}
	// (import(?:\s+(?:static|module))?)(\s+)
	if strings.HasPrefix(src[i:], "import") {
		k := i + 6
		if n := l.spaces(k); n > 0 {
			for _, w := range [...]string{"static", "module"} {
				if strings.HasPrefix(src[k+n:], w) {
					if m := l.spaces(k + n + len(w)); m > 0 {
						e := k + n + len(w)
						l.x.Emit(fh.KeywordNamespace, i, e)
						l.x.Emit(fh.TextWhitespace, e, e+m)
						l.push(importName)
						return e + m - i, true
					}
				}
			}
			l.x.Emit(fh.KeywordNamespace, i, k)
			l.x.Emit(fh.TextWhitespace, k, k+n)
			l.push(importName)
			return 6 + n, true
		}
	}
	if c == '"' {
		if strings.HasPrefix(src[i:], "\"\"\"\n") {
			l.push(multiline)
			return l.emit(fh.LiteralString, i, i+4)
		}
		l.push(str)
		return l.emit(fh.LiteralString, i, i+1)
	}
	// '\\.'|'[^\\]'|'\\u[0-9a-fA-F]{4}'
	if c == '\'' {
		if l.at(i+1) == '\\' {
			if _, n := l.runeAt(i + 2); n > 0 && l.at(i+2+n) == '\'' {
				return l.emit(fh.LiteralStringChar, i, i+3+n)
			}
			if l.at(i+2) == 'u' && hexCount(src, i+3) >= 4 && l.at(i+7) == '\'' {
				return l.emit(fh.LiteralStringChar, i, i+8)
			}
		} else if _, n := l.runeAt(i + 1); n > 0 && l.at(i+1+n) == '\'' {
			return l.emit(fh.LiteralStringChar, i, i+2+n)
		}
	}
	// (\.)((?:[^\W\d]|\$)[\w$]*)
	if c == '.' {
		if n := l.ident(i + 1); n > 0 {
			l.x.Emit(fh.Punctuation, i, i+1)
			l.x.Emit(fh.NameAttribute, i+1, i+1+n)
			return 1 + n, true
		}
	}
	if l.bol(i) {
		j := i + l.spaces(i)
		// ^(\s*)(default)(:)
		if strings.HasPrefix(src[j:], "default:") {
			l.x.Emit(fh.TextWhitespace, i, j)
			l.x.Emit(fh.Keyword, j, j+7)
			l.x.Emit(fh.Punctuation, j+7, j+8)
			return j + 8 - i, true
		}
		// ^(\s*)((?:[^\W\d]|\$)[\w$]*)(:)
		if n := l.ident(j); n > 0 && l.at(j+n) == ':' {
			l.x.Emit(fh.TextWhitespace, i, j)
			l.x.Emit(fh.NameLabel, j, j+n)
			l.x.Emit(fh.Punctuation, j+n, j+n+1)
			return j + n + 1 - i, true
		}
	}
	if n := l.ident(i); n > 0 {
		if !l.o.ChromaTypes && (i < l.typeEnd || l.isType(i, i+n)) {
			return l.emit(fh.KeywordType, i, i+n)
		}
		return l.emit(fh.Name, i, i+n)
	}
	if e, t := l.number(i); e > 0 {
		return l.emit(t, i, e)
	}
	if strings.IndexByte("~^*!%&[]<>|+=/?-", c) >= 0 {
		return l.emit(fh.Operator, i, i+1)
	}
	if strings.IndexByte("{}();:.,", c) >= 0 {
		return l.emit(fh.Punctuation, i, i+1)
	}
	if c == '\n' {
		return l.emit(fh.TextWhitespace, i, i+1)
	}
	return 0, false
}

// function tries ((?:(?:[^\W\d]|\$)[\w.\[\]$<>?]*\s+)+?)((?:[^\W\d]|\$)[\w$]*)(\s*)(\().
//
// One repetition of the first part is fixed as "a word (a run of type characters) and the whitespace
// after it" (backtracking does not make it match). So it advances one word at a time and checks whether
// a name and ( follow. On a mismatch, it remembers the end of the word sequence examined. Trying from any
// position in the middle of the sequence fails for the same reason.
func (l *lexer) function(i int) (int, bool) {
	src := l.src
	// A start inside the failed sequence, after a space or in the middle of a word (such as the B of A<B),
	// reaches the same words: the run of type characters from there ends where that word ends.
	if 0 <= l.failFrom && l.failFrom < i && i <= l.failEnd {
		return 0, false
	}
	p := i
	for {
		r, n := l.runeAt(p)
		if n == 0 || !l.isIdentStart(r) {
			break
		}
		t := p + n + l.tc.Span(src, p+n, isTypeChar)
		w := l.spaces(t)
		if w == 0 {
			break
		}
		p = t + w
		if m := l.ident(p); m > 0 {
			e := p + m
			s := l.spaces(e)
			if l.at(e+s) == '(' {
				l.nested(i, p, true)
				l.x.Emit(fh.NameFunction, p, e)
				l.x.Emit(fh.TextWhitespace, e, e+s)
				l.x.Emit(fh.Punctuation, e+s, e+s+1)
				return e + s + 1 - i, true
			}
		}
	}
	// Keep the memo of a scan that got further: a failure at a position with no word (such as each newline,
	// which the whitespace rule leaves for later) must not replace the memo of a long failed scan, or every
	// word of that scan is scanned again from its own start.
	if p > l.failEnd {
		l.failFrom, l.failEnd = i, p
	}
	return 0, false
}

func isDec(c byte) bool { return '0' <= c && c <= '9' }
func isHex(c byte) bool {
	return '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F'
}

func hexCount(src string, i int) int {
	n := 0
	for i+n < len(src) && isHex(src[i+n]) {
		n++
	}
	return n
}

// run returns the end of the run of bytes satisfying f from the start of src[j:].
func (l *lexer) run(j int, f func(byte) bool) int {
	for j < len(l.src) && f(l.src[j]) {
		j++
	}
	return j
}

func isDecU(c byte) bool { return isDec(c) || c == '_' }
func isHexU(c byte) bool { return isHex(c) || c == '_' }
func isFD(c byte) bool   { return c == 'f' || c == 'F' || c == 'd' || c == 'D' }
func isSign(c byte) bool { return c == '+' || c == '-' }

// signedDigits returns the end of [+\-]?[0-9][0-9_]* (-1 if it does not match).
func (l *lexer) signedDigits(k int) int {
	if isSign(l.at(k)) && isDec(l.at(k+1)) {
		k++
	}
	if !isDec(l.at(k)) {
		return -1
	}
	return l.run(k+1, isDecU)
}

// exponent returns the end of ([eE][+\-]?[0-9][0-9_]*)?.
func (l *lexer) exponent(j int) int {
	if c := l.at(j); c == 'e' || c == 'E' {
		if e := l.signedDigits(j + 1); e >= 0 {
			return e
		}
	}
	return j
}

func (l *lexer) fd(j int) int {
	if isFD(l.at(j)) {
		return j + 1
	}
	return j
}

// number tries the number rules. If one matches, it returns the end and the kind.
func (l *lexer) number(i int) (int, fh.TokenType) {
	c := l.at(i)
	// Decimal: try the 4 alternatives in order.
	if isDec(c) { // [0-9][0-9_]*\.([0-9][0-9_]*)?(exp)?[fFdD]?
		if j := l.run(i+1, isDecU); l.at(j) == '.' {
			j++
			if isDec(l.at(j)) {
				j = l.run(j+1, isDecU)
			}
			return l.fd(l.exponent(j)), fh.LiteralNumberFloat
		}
	}
	if c == '.' && isDec(l.at(i+1)) { // \.[0-9][0-9_]*(exp)?[fFdD]?
		return l.fd(l.exponent(l.run(i+2, isDecU))), fh.LiteralNumberFloat
	}
	if isDec(c) {
		// [0-9][eE][+\-]?[0-9][0-9_]*[fFdD]?
		if d := l.at(i + 1); d == 'e' || d == 'E' {
			if e := l.signedDigits(i + 2); e >= 0 {
				return l.fd(e), fh.LiteralNumberFloat
			}
		}
		// [0-9]([eE][+\-]?[0-9][0-9_]*)?[fFdD]
		if j := l.exponent(i + 1); isFD(l.at(j)) {
			return j + 1, fh.LiteralNumberFloat
		}
	}
	if c == '0' && (l.at(i+1) == 'x' || l.at(i+1) == 'X') {
		// 0[xX]([hex][hex_]*\.?|([hex][hex_]*)?\.[hex][hex_]*)[pP][+\-]?[0-9][0-9_]*[fFdD]?
		k := i + 2
		var cands [3]int
		nc := 0
		if isHex(l.at(k)) {
			h := l.run(k+1, isHexU)
			if l.at(h) == '.' {
				cands[nc] = h + 1
				nc++
			}
			cands[nc] = h
			nc++
			if l.at(h) == '.' && isHex(l.at(h+1)) {
				cands[nc] = l.run(h+2, isHexU)
				nc++
			}
		} else if l.at(k) == '.' && isHex(l.at(k+1)) {
			cands[nc] = l.run(k+2, isHexU)
			nc++
		}
		for _, j := range cands[:nc] {
			if p := l.at(j); p == 'p' || p == 'P' {
				if e := l.signedDigits(j + 1); e >= 0 {
					return l.fd(e), fh.LiteralNumberFloat
				}
			}
		}
		// 0[xX][0-9a-fA-F][0-9a-fA-F_]*[lL]?
		if isHex(l.at(k)) {
			return l.suffixL(l.run(k+1, isHexU)), fh.LiteralNumberHex
		}
	}
	if c == '0' {
		// 0[bB][01][01_]*[lL]?
		if b := l.at(i + 1); (b == 'b' || b == 'B') && (l.at(i+2) == '0' || l.at(i+2) == '1') {
			return l.suffixL(l.run(i+3, func(c byte) bool { return c == '0' || c == '1' || c == '_' })), fh.LiteralNumberBin
		}
		// 0[0-7_]+[lL]?
		isOctU := func(c byte) bool { return '0' <= c && c <= '7' || c == '_' }
		if isOctU(l.at(i + 1)) {
			return l.suffixL(l.run(i+1, isOctU)), fh.LiteralNumberOct
		}
		return i + 1, fh.LiteralNumberInteger // 0
	}
	if '1' <= c && c <= '9' { // [1-9][0-9_]*[lL]?
		return l.suffixL(l.run(i+1, isDecU)), fh.LiteralNumberInteger
	}
	return 0, 0
}

func (l *lexer) suffixL(j int) int {
	if c := l.at(j); c == 'l' || c == 'L' {
		return j + 1
	}
	return j
}

// maxType is how far the type scans look. A type is short; the limit keeps the time linear.
const maxType = 512

// hspaces returns the end of the run of spaces and tabs at j.
func (l *lexer) hspaces(j int) int {
	for j < len(l.src) && (l.src[j] == ' ' || l.src[j] == '\t') {
		j++
	}
	return j
}

// isType reports whether the name src[i:e] is in a type position (production only). Only a capitalized
// name is taken (Java's naming convention), so a comparison of variables (a < b) or a lowercase word is
// never a type. It is a type when it is
//   - followed directly by type arguments (List<User>); it then sets typeEnd so the names inside are types,
//   - the last word before a method name (the return type), or
//   - followed by [] pairs or ... and a variable name, then = ; , ) or : (User u = x;, f(User u)).
func (l *lexer) isType(i, e int) bool {
	src := l.src
	if r, _ := lexkit.Rune(src, i); !unicode.IsUpper(r) {
		return false
	}
	if l.at(e) == '<' {
		if a := l.typeArgsEnd(e); a > 0 && l.followsTypeArgs(a) {
			l.typeEnd = a
			return true
		}
		return false
	}
	if l.prefix && e+l.spaces(e) == len(src) {
		return true
	}
	j := l.hspaces(e)
	for j < e+maxType {
		if l.at(j) == '[' {
			if k := l.hspaces(j + 1); l.at(k) == ']' {
				j = l.hspaces(k + 1)
				continue
			}
			return false
		}
		if strings.HasPrefix(src[j:], "...") {
			j = l.hspaces(j + 3)
		}
		break
	}
	if j == e || j >= e+maxType {
		return false
	}
	n := l.ident(j)
	if n == 0 || l.word(j, lKeywords) > 0 || l.word(j, lDeclarations) > 0 || l.word(j, lTypes) > 0 || l.word(j, lConstants) > 0 {
		return false
	}
	k := l.hspaces(j + n)
	return k < len(src) && strings.IndexByte("=;,):", src[k]) >= 0
}

// followsTypeArgs reports whether what comes at i, after the closing ">" of type arguments, can follow
// them: a declared name (after at least one space) followed by = ; , ) or :, or one of ( . [ ... > , ) {
// :: or the end of input, possibly after spaces. Otherwise (foo(A<B, C>D), A<B>-1) the "<" and ">" are
// comparisons.
func (l *lexer) followsTypeArgs(i int) bool {
	src := l.src
	j := l.hspaces(i)
	if j >= len(src) {
		return true
	}
	if strings.IndexByte("(.[>,){:\n\r", src[j]) >= 0 {
		return src[j] != ':' || l.at(j+1) == ':'
	}
	if j == i {
		return false
	}
	n := l.ident(j)
	if n == 0 {
		return false
	}
	k := l.hspaces(j + n)
	return k < len(src) && strings.IndexByte("=;,):", src[k]) >= 0
}

// typeArgsEnd returns the end of the type arguments that start at the "<" at i, or 0 if what follows
// does not look like type arguments: names, ".", ",", "[", "]", "?", a single "&", whitespace and nested
// "< >", closed by ">" within maxType bytes. Anything else (&&, <<, another operator, a number, a
// parenthesis) means that "<" is a comparison or a shift.
func (l *lexer) typeArgsEnd(i int) int {
	src := l.src
	depth := 0
	for j := i; j < len(src) && j < i+maxType; {
		r, w := lexkit.Rune(src, j)
		switch {
		case r == '<':
			if j > i && src[j-1] == '<' {
				return 0
			}
			depth++
		case r == '>':
			depth--
			if depth == 0 {
				return j + 1
			}
		case r == '&' && l.at(j+1) == '&':
			return 0
		case lexkit.IsDigit(r) && !isNameChar(prevRune(src, j)):
			return 0
		case isNameChar(r), lexkit.IsSpace(r), strings.ContainsRune(".,[]&?", r):
		default:
			return 0
		}
		j += w
	}
	return 0
}

func prevRune(s string, j int) rune {
	r, _ := utf8.DecodeLastRuneInString(s[:j])
	return r
}
