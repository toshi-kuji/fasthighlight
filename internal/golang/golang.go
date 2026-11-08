// Package golang is the Go tokenizer. lang/golang registers it. Options (the switches for the intended
// differences from Chroma) exist only for the comparison in compat/ and are not exposed.
//
// The token types and boundaries reproduce Chroma 2.27.0's Go rules (lexers/go.go) without regular expressions.
// It does not use the standard library's go/scanner: Chroma's rules differ from the Go grammar in details (comments
// such as //go:build, reading the inside of a raw string as a template, and so on), so the result would
// not be the same as Chroma's.
package golang

import (
	"strings"

	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/lexkit"
)

// Indexes of the word lists (lists.go).
const (
	lDeclarations = iota
	lKeywords
	lConstants
	lBuiltins
	lTypes
	lTemplateKeywords
)

var lists *lexkit.WordLists

// Prepare builds the word tables and the like (the Prepare of fasthighlight.Language).
func Prepare() {
	lists = lexkit.NewWordLists(declarations, keywords, constants, builtins, types, templateKeywords)
}

// Options holds the switches for the intended differences from Chroma. The zero value is the production
// setting (the one lang/golang registers).
type Options struct {
	// ChromaIdent, if true, makes the name-start character [^\W\d] the same as Chroma (in regexp2 it is
	// the same as \w, so a name can also start with a digit). If false, it is a word character other than
	// digits, as the rule intends.
	ChromaIdent bool
	// ChromaTypeNames, if true, leaves the names in type positions (struct fields, declarations,
	// parameters and results) as NameOther, as Chroma does. If false, they get the type color (types.go).
	ChromaTypeNames bool
}

type lexer struct {
	o     Options
	x     *lexkit.Tokens
	src   string
	words lexkit.WordCache
	raws  [][2]int // the [start, end) of the raw strings read
}

// Lex splits src into tokens (with the same contract as the Lex of fasthighlight.Language). compat/ uses it
// to compare with Chroma, switching o.
func Lex(src string, end int, o Options) []fh.Token {
	x := lexkit.NewTokens(src)
	raws := lexInto(x, src, end, o)
	toks := x.Result()
	if !o.ChromaTypeNames {
		markTypes(toks, raws)
	}
	return toks
}

// lexInto reads with the Go root rules (the only state is root). It returns the ranges of the raw strings.
func lexInto(x *lexkit.Tokens, src string, end int, o Options) [][2]int {
	l := &lexer{o: o, x: x, src: src, words: lexkit.WordCache{Lists: lists}}
	for i := 0; i < end; {
		n, ok := l.root(i)
		if !ok {
			n = l.x.Error(i)
		}
		i += n
	}
	return l.raws
}

func (l *lexer) emit(t fh.TokenType, s, e int) (int, bool) { l.x.Emit(t, s, e); return e - s, true }

func (l *lexer) at(i int) byte {
	if i < len(l.src) {
		return l.src[i]
	}
	return 0
}

func (l *lexer) digits(i int) int { return lexkit.Span(l.src, i, lexkit.IsDigit) }

// signedExp returns the length of [Ee][-+]\d+ (a sign is required).
func (l *lexer) signedExp(j int) int {
	if c := l.at(j); c != 'e' && c != 'E' {
		return 0
	}
	if c := l.at(j + 1); c != '-' && c != '+' {
		return 0
	}
	if d := l.digits(j + 2); d > 0 {
		return 2 + d
	}
	return 0
}

// optExp returns the length of [eE][+\-]?\d+ (the sign is optional).
func (l *lexer) optExp(j int) int {
	if c := l.at(j); c != 'e' && c != 'E' {
		return 0
	}
	k := j + 1
	if c := l.at(k); (c == '-' || c == '+') && l.digits(k+1) > 0 {
		k++
	}
	if d := l.digits(k); d > 0 {
		return k + d - j
	}
	return 0
}

func isHexU(c byte) bool {
	return '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F' || c == '_'
}
func isHex(c byte) bool { return c != '_' && isHexU(c) }

func (l *lexer) run(j int, f func(byte) bool) int {
	for j < len(l.src) && f(l.src[j]) {
		j++
	}
	return j
}

// number tries the number rules. If one matches, it returns the end and the kind.
func (l *lexer) number(i int) (int, fh.TokenType) {
	src := l.src
	d := l.digits(i)
	if d > 0 {
		if l.at(i+d) == 'i' { // \d+i
			return i + d + 1, fh.LiteralNumber
		}
		if l.at(i+d) == '.' { // \d+\.\d*([Ee][-+]\d+)?i
			j := i + d + 1
			j += l.digits(j)
			if x := l.signedExp(j); x > 0 && l.at(j+x) == 'i' {
				return j + x + 1, fh.LiteralNumber
			}
			if l.at(j) == 'i' {
				return j + 1, fh.LiteralNumber
			}
		}
	}
	if src[i] == '.' { // \.\d+([Ee][-+]\d+)?i
		if d2 := l.digits(i + 1); d2 > 0 {
			j := i + 1 + d2
			if x := l.signedExp(j); x > 0 && l.at(j+x) == 'i' {
				return j + x + 1, fh.LiteralNumber
			}
			if l.at(j) == 'i' {
				return j + 1, fh.LiteralNumber
			}
		}
	}
	if d > 0 {
		if x := l.signedExp(i + d); x > 0 && l.at(i+d+x) == 'i' { // \d+[Ee][-+]\d+i
			return i + d + x + 1, fh.LiteralNumber
		}
		// \d+(\.\d+[eE][+\-]?\d+|\.\d*|[eE][+\-]?\d+)
		j := i + d
		if l.at(j) == '.' {
			if d2 := l.digits(j + 1); d2 > 0 {
				if x := l.optExp(j + 1 + d2); x > 0 {
					return j + 1 + d2 + x, fh.LiteralNumberFloat
				}
			}
			return j + 1 + l.digits(j+1), fh.LiteralNumberFloat
		}
		if x := l.optExp(j); x > 0 {
			return j + x, fh.LiteralNumberFloat
		}
	}
	if src[i] == '.' { // \.\d+([eE][+\-]?\d+)?
		if d2 := l.digits(i + 1); d2 > 0 {
			return i + 1 + d2 + l.optExp(i+1+d2), fh.LiteralNumberFloat
		}
	}
	if src[i] == '0' {
		if c := l.at(i + 1); '0' <= c && c <= '7' { // 0[0-7]+
			return l.run(i+1, func(c byte) bool { return '0' <= c && c <= '7' }), fh.LiteralNumberOct
		}
		if c := l.at(i + 1); (c == 'x' || c == 'X') && isHexU(l.at(i+2)) { // 0[xX][0-9a-fA-F_]+
			return l.run(i+2, isHexU), fh.LiteralNumberHex
		}
		if l.at(i+1) == 'b' { // 0b[01_]+
			isBin := func(c byte) bool { return c == '0' || c == '1' || c == '_' }
			if isBin(l.at(i + 2)) {
				return l.run(i+2, isBin), fh.LiteralNumberBin
			}
		}
		return i + 1, fh.LiteralNumberInteger // 0
	}
	if c := src[i]; '1' <= c && c <= '9' { // [1-9][0-9_]*
		return l.run(i+1, func(c byte) bool { return '0' <= c && c <= '9' || c == '_' }), fh.LiteralNumberInteger
	}
	return 0, 0
}

// charLen returns the length of '(\\['"\\abfnrtv]|\\x[0-9a-fA-F]{2}|\\[0-7]{1,3}|\\u[0-9a-fA-F]{4}|\\U[0-9a-fA-F]{8}|[^\\])'
// (0 if it does not match). src[i] is '.
func charLen(src string, i int) int {
	at := func(j int) byte {
		if j < len(src) {
			return src[j]
		}
		return 0
	}
	count := func(j, max int, f func(byte) bool) int {
		n := 0
		for n < max && j+n < len(src) && f(src[j+n]) {
			n++
		}
		return n
	}
	isOct := func(c byte) bool { return '0' <= c && c <= '7' }
	k := i + 1
	if k >= len(src) {
		return 0
	}
	if src[k] != '\\' {
		_, n := lexkit.Rune(src, k)
		if at(k+n) == '\'' {
			return 2 + n
		}
		return 0
	}
	c := at(k + 1)
	var bodies []int // lengths to try after \\ (in the order of the regular expression's alternatives)
	if c != 0 && strings.IndexByte("'\"\\abfnrtv", c) >= 0 {
		bodies = append(bodies, 1)
	}
	if c == 'x' && count(k+2, 2, isHex) == 2 {
		bodies = append(bodies, 3)
	}
	if o := count(k+1, 3, isOct); o > 0 {
		bodies = append(bodies, o)
	}
	if c == 'u' && count(k+2, 4, isHex) == 4 {
		bodies = append(bodies, 5)
	}
	if c == 'U' && count(k+2, 8, isHex) == 8 {
		bodies = append(bodies, 9)
	}
	for _, b := range bodies {
		if at(k+1+b) == '\'' {
			return 1 + 1 + b + 1
		}
	}
	return 0
}

// quotedLen returns the length of "(\\\\|\\"|[^"])*" (0 if it does not match). src[i] is ".
//
// If a closing " is found, that decides it. If the scan reaches the end without finding one, then, as
// with regular expression backtracking, it starts over from the last choice (re-reading \\\\ or \\" as
// a single \ character). It remembers the positions known not to match.
func quotedLen(src string, i int) int {
	// Fast path: look for the closing " without backtracking.
	for j := i + 1; j < len(src); {
		switch src[j] {
		case '"':
			return j + 1 - i
		case '\\':
			if j+1 < len(src) && (src[j+1] == '\\' || src[j+1] == '"') {
				j += 2
				continue
			}
		}
		j++
	}
	failed := map[int]bool{}
	var try func(p int) int
	try = func(p int) int {
		for p < len(src) {
			if failed[p] {
				return -1
			}
			switch src[p] {
			case '"':
				return p + 1
			case '\\':
				if p+1 < len(src) && (src[p+1] == '\\' || src[p+1] == '"') {
					if e := try(p + 2); e >= 0 {
						return e
					}
					failed[p+2] = true
				}
			}
			p++ // [^"] (a \ is also read as one character)
		}
		return -1
	}
	if e := try(i + 1); e >= 0 {
		return e - i
	}
	return 0
}

// ops are the alternatives of the operator rule (longest first, the same order as the regular expression).
var ops = [...]string{"<<=", ">>=", "<<", ">>", "<=", ">=", "&^=", "&^", "+=", "-=", "*=", "/=", "%=", "&=", "|=",
	"&&", "||", "<-", "++", "--", "==", "!=", ":=", "..."}

func (l *lexer) root(i int) (int, bool) {
	src := l.src
	c := src[i]
	if c == '\n' {
		return l.emit(fh.TextWhitespace, i, i+1)
	}
	if n := lexkit.Span(src, i, lexkit.IsSpace); n > 0 {
		return l.emit(fh.TextWhitespace, i, i+n)
	}
	if c == '/' {
		if l.at(i+1) == '/' {
			e := i + 2
			for e < len(src) && src[e] != '\n' && src[e] != '\r' {
				e++
			}
			// //[^\s\n\r][^\n\r]* and //[^\n\r]*
			if i+2 < len(src) && !lexkit.IsSpace(runeAt(src, i+2)) {
				return l.emit(fh.CommentPreproc, i, e)
			}
			return l.emit(fh.CommentSingle, i, e)
		}
		// /(\\\n)?[*](.|\n)*?[*](\\\n)?/
		j := i + 1
		if strings.HasPrefix(src[j:], "\\\n") {
			j += 2
		}
		if l.at(j) == '*' {
			for m := j + 1; ; {
				k := strings.IndexByte(src[m:], '*')
				if k < 0 {
					break
				}
				m += k
				if strings.HasPrefix(src[m+1:], "\\\n/") {
					return l.emit(fh.CommentMultiline, i, m+4)
				}
				if l.at(m+1) == '/' {
					return l.emit(fh.CommentMultiline, i, m+2)
				}
				m++
			}
		}
	}
	// (import|package)\b
	if n, _ := l.words.Word(src, i); n > 0 {
		switch src[i : i+n] {
		case "import", "package":
			return l.emit(fh.KeywordNamespace, i, i+n)
		}
	}
	if n := l.words.Match(src, i, lDeclarations); n > 0 {
		return l.emit(fh.KeywordDeclaration, i, i+n)
	}
	if n := l.words.Match(src, i, lKeywords); n > 0 {
		return l.emit(fh.Keyword, i, i+n)
	}
	if n := l.words.Match(src, i, lConstants); n > 0 {
		return l.emit(fh.KeywordConstant, i, i+n)
	}
	if n := l.words.Match(src, i, lBuiltins); n > 0 && l.at(i+n) == '(' { // (…)\b(\()
		l.x.Emit(fh.NameBuiltin, i, i+n)
		l.x.Emit(fh.Punctuation, i+n, i+n+1)
		return n + 1, true
	}
	if n := l.words.Match(src, i, lTypes); n > 0 {
		return l.emit(fh.KeywordType, i, i+n)
	}
	if lexkit.IsDigit(runeAt(src, i)) || c == '.' {
		if e, t := l.number(i); e > 0 {
			return l.emit(t, i, e)
		}
	}
	switch c {
	case '\'':
		if n := charLen(src, i); n > 0 {
			return l.emit(fh.LiteralStringChar, i, i+n)
		}
	case '`': // (`)([^`]*)(`). The inside is read as a Go template.
		if k := strings.IndexByte(src[i+1:], '`'); k >= 0 {
			l.x.Emit(fh.LiteralString, i, i+1)
			lexTemplate(l.x.Sub(src[i+1:i+1+k], i+1), src[i+1:i+1+k], l.o)
			l.x.Emit(fh.LiteralString, i+1+k, i+2+k)
			l.raws = append(l.raws, [2]int{i, i + 2 + k})
			return k + 2, true
		}
	case '"':
		if n := quotedLen(src, i); n > 0 {
			return l.emit(fh.LiteralString, i, i+n)
		}
	}
	for _, op := range ops {
		if strings.HasPrefix(src[i:], op) {
			return l.emit(fh.Operator, i, i+len(op))
		}
	}
	if strings.IndexByte("+-*/%&", c) >= 0 {
		return l.emit(fh.Operator, i, i+1)
	}
	// ([a-zA-Z_]\w*)(\s*)(\()
	if c == '_' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' {
		n, _ := l.words.Word(src, i)
		s := lexkit.Span(src, i+n, lexkit.IsSpace)
		if l.at(i+n+s) == '(' {
			l.x.Emit(fh.NameFunction, i, i+n)
			if s > 0 {
				lexInto(l.x.Sub(src[i+n:i+n+s], i+n), src[i+n:i+n+s], s, l.o)
			}
			l.x.Emit(fh.Punctuation, i+n+s, i+n+s+1)
			return n + s + 1, true
		}
	}
	if strings.IndexByte("|^<>=!()[]{}.,;:~", c) >= 0 {
		return l.emit(fh.Punctuation, i, i+1)
	}
	// [^\W\d]\w* (in regexp2, [^\W\d] is the same as \w. See Options.ChromaIdent)
	if n, _ := l.words.Word(src, i); n > 0 && (l.o.ChromaIdent || !lexkit.IsDigit(runeAt(src, i))) {
		return l.emit(fh.NameOther, i, i+n)
	}
	return 0, false
}

func runeAt(src string, i int) rune {
	r, _ := lexkit.Rune(src, i)
	return r
}
