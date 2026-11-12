// Package powershell registers PowerShell.
//
//	import _ "github.com/toshi-kuji/fasthighlight/lang/powershell"
//
// Token types and boundaries reproduce the rules of Chroma 2.27.0's powershell.xml without regular
// expressions. powershell.xml is case-insensitive (case_insensitive) and . matches newlines (dot_all). As
// with regexp2, letters are compared with lexkit.FoldEq and lexkit.FoldASCII.
package powershell

import (
	"strings"
	"unicode/utf8"

	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/lexkit"
)

// Language is PowerShell. Its name, aliases, and filename patterns are listed in the catalog of the main
// package (fasthighlight.Builtin).
var Language = fh.Builtin("PowerShell")

func init() {
	Language.Lex = lex
	Language.Prepare = prepare
	fh.Register(Language)
}

// Indexes of the word lists (lists.go).
const (
	lBuiltins = iota
	lAttributes
	lAttributes2
	lKeywords
	lOperators
	lAliases
)

var (
	lists *lexkit.FoldWordLists
	docs  []string
)

func prepare() {
	lists = lexkit.NewFoldWordLists(builtins, attributes, attributes2, keywords, operators, aliases)
	docs = strings.Split(docWords, "|")
}

type state uint8

const (
	root state = iota
	child
	requires
	multline
	str
	heredoc
)

type lexer struct {
	x     *lexkit.Tokens
	src   string
	stack []state
	words lexkit.FoldWordCache
	// runEnd and lastQuote remember, for the heredoc rule [^@\n]+"], the end of the run of characters
	// other than @ and newline, and the position of the last "] in it (-1 if none). Every position before
	// runEnd shares them, so the run is not scanned again for each character.
	runEnd, lastQuote int
	// typeEnd and typeClose do the same for the type rule (\[)([a-z_\[][\w. `,\[\]]*)(\]): the end of the run of
	// type characters, and the position of the last ] in it (-1 if none).
	typeEnd, typeClose int
	// helpRun is the last run of # and whitespace that helpLine measured: it is tried at every # at the start
	// of a line, and the run spans lines, so lines of only # would otherwise be measured again from each line.
	helpRun lexkit.SpanCache
	// hereMiss is the position from which the here-string rule @'\n.*?\n'@ found no closing \n'@ (len(src)+1
	// until one search fails). A later @'\n cannot find one either, so it does not search again.
	hereMiss int
}

func lex(src string, end int) []fh.Token {
	l := &lexer{x: lexkit.NewTokens(src), src: src, stack: []state{root}, words: lexkit.FoldWordCache{Lists: lists},
		hereMiss: len(src) + 1}
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

// fold returns the length if src[i:] starts with word (lowercase) under (?i), or 0.
func (l *lexer) fold(i int, word string) int { return lexkit.FoldPrefix(l.src, i, word) }

// letter returns the size of the character at i if it matches [a-z] under (?i) (A-Z, the Kelvin sign, ſ and
// so on), or 0.
func (l *lexer) letter(i int) int {
	if i >= len(l.src) {
		return 0
	}
	r, n := lexkit.Rune(l.src, i)
	if f := lexkit.FoldASCII(r); 'a' <= f && f <= 'z' {
		return n
	}
	return 0
}

// spaceBefore reports whether (?<!\S) holds at i (the start of the text, or a \s before it).
func (l *lexer) spaceBefore(i int) bool {
	if i == 0 {
		return true
	}
	r, _ := utf8.DecodeLastRuneInString(l.src[:i])
	return lexkit.IsSpace(r)
}

func notSpace(r rune) bool { return !lexkit.IsSpace(r) }

func (l *lexer) step(st state, i int) (int, bool) {
	switch st {
	case child:
		if l.src[i] == ')' { // \)
			l.pop()
			return l.emit(fh.Punctuation, i, i+1)
		}
		return l.root(i)
	case requires:
		return l.requires(i)
	case multline:
		return l.multline(i)
	case str:
		return l.str(i)
	case heredoc:
		return l.heredoc(i)
	}
	return l.root(i)
}

func (l *lexer) root(i int) (int, bool) {
	src := l.src
	c := src[i]
	if c == '(' { // \(
		l.push(child)
		return l.emit(fh.Punctuation, i, i+1)
	}
	if n := lexkit.Span(src, i, lexkit.IsSpace); n > 0 { // \s+
		return l.emit(fh.Text, i, i+n)
	}
	if c == '#' {
		// (\s*)(#)(requires)(\s+): \s* is always empty here, because \s+ above matched first.
		if n := l.fold(i+1, "requires"); n > 0 {
			if w := lexkit.Span(src, i+1+n, lexkit.IsSpace); w > 0 {
				l.x.Emit(fh.Comment, i, i+1)
				l.x.Emit(fh.Keyword, i+1, i+1+n)
				l.push(requires)
				l.x.Emit(fh.TextWhitespace, i+1+n, i+1+n+w)
				return i + 1 + n + w - i, true
			}
		}
		if n := l.helpLine(i); n > 0 {
			return n, true
		}
		return l.emit(fh.Comment, i, lexkit.LineEnd(src, i)) // #[^\n]*?$
	}
	if c == '<' && l.at(i+1) == '#' || c == '&' && l.fold(i, "&lt;#") > 0 { // (&lt;|<)#
		l.push(multline)
		if c == '<' {
			return l.emit(fh.CommentMultiline, i, i+2)
		}
		return l.emit(fh.CommentMultiline, i, i+5)
	}
	if n := l.letter(i); n > 0 && l.at(i+n) == ':' { // (?i)([A-Z]:)
		return l.emit(fh.Name, i, i+n+1)
	}
	switch c {
	case '@':
		switch {
		case l.at(i+1) == '"' && l.at(i+2) == '\n': // @"\n
			l.push(heredoc)
			return l.emit(fh.LiteralStringHeredoc, i, i+3)
		case l.at(i+1) == '\'' && l.at(i+2) == '\n' && i+3 < l.hereMiss: // @'\n.*?\n'@
			if k := strings.Index(src[i+3:], "\n'@"); k >= 0 {
				return l.emit(fh.LiteralStringHeredoc, i, i+3+k+3)
			}
			l.hereMiss = i + 3
		}
		if d := l.at(i + 1); d == '(' || d == '{' { // @(?=\(|{)
			return l.emit(fh.NameVariableMagic, i, i+1)
		}
	case '$':
		if l.at(i+1) == '(' { // \$(?=\()
			return l.emit(fh.NameVariableMagic, i, i+1)
		}
	case '`':
		if d := l.at(i + 1); d != 0 && strings.IndexByte("'\"$@-", d) >= 0 { // `[\'"$@-]
			return l.emit(fh.Punctuation, i, i+2)
		}
	case '"':
		l.push(str)
		return l.emit(fh.LiteralStringDouble, i, i+1)
	case '\'':
		if n := l.single(i); n > 0 {
			return l.emit(fh.LiteralStringSingle, i, i+n)
		}
	}
	if n, ok := l.function(i); ok {
		return n, true
	}
	if n, ok := l.class(i); ok {
		return n, true
	}
	if c == '$' { // \$false|\$null|\$true(?=\b)
		if n := l.fold(i, "$false"); n > 0 {
			return l.emit(fh.NameVariableMagic, i, i+n)
		}
		if n := l.fold(i, "$null"); n > 0 {
			return l.emit(fh.NameVariableMagic, i, i+n)
		}
		if n := l.fold(i, "$true"); n > 0 && lexkit.WordBoundary(src, i+n) {
			return l.emit(fh.NameVariableMagic, i, i+n)
		}
	}
	if c == '$' || c == '@' { // (\$|@@|@)((global|script|private|env):)?\w+
		p := i + 1
		if c == '@' && l.at(i+1) == '@' {
			p = i + 2 // if @@ fails, @ is followed by @, which is not \w, so it fails too
		}
		if n := l.varName(p); n > 0 {
			return l.emit(fh.NameVariable, i, p+n)
		}
	}
	if n := l.words.Match(src, i, lBuiltins); n > 0 {
		return l.emit(fh.NameBuiltin, i, i+n)
	}
	if n := l.dashed(i); n > 0 { // [a-z]\w*-[a-z]\w*\b
		return l.emit(fh.NameBuiltin, i, i+n)
	}
	if n := l.words.Match(src, i, lAttributes); n > 0 {
		return l.emit(fh.NameAttribute, i, i+n)
	}
	if n := l.words.Match(src, i, lAttributes2); n > 0 {
		return l.emit(fh.NameAttribute, i, i+n)
	}
	if n := l.words.Match(src, i, lKeywords); n > 0 {
		return l.emit(fh.Keyword, i, i+n)
	}
	if c == '-' { // -(and|as|…)\b
		if n := l.words.Match(src, i+1, lOperators); n > 0 {
			return l.emit(fh.Operator, i, i+1+n)
		}
	}
	if n, mask := l.words.Word(src, i); mask&(1<<lAliases) != 0 && i+n < len(src) { // (ac|asnp|…)\s
		if r, w := lexkit.Rune(src, i+n); lexkit.IsSpace(r) {
			return l.emit(fh.NameBuiltin, i, i+n+w)
		}
	}
	if c == '[' { // (\[)([a-z_\[][\w. `,\[\]]*)(\])
		if n := l.letter(i + 1); n > 0 || l.at(i+1) == '_' || l.at(i+1) == '[' {
			if n == 0 {
				n = 1
			}
			p := i + 1 + n
			if p >= l.typeEnd { // a new run; positions inside the last run share its end and last ]
				e := p + lexkit.Span(src, p, isTypeChar)
				l.typeEnd, l.typeClose = e, strings.LastIndexByte(src[p:e], ']')
				if l.typeClose >= 0 {
					l.typeClose += p
				}
			}
			if k := l.typeClose; k >= p {
				l.x.Emit(fh.Punctuation, i, i+1)
				l.x.Emit(fh.NameConstant, i+1, k)
				l.x.Emit(fh.Punctuation, k, k+1)
				return k + 1 - i, true
			}
		}
	}
	if n := l.property(i); n > 0 {
		return l.emit(fh.NameProperty, i, i+n)
	}
	if num, e := l.number(i); e > 0 {
		l.x.Emit(fh.LiteralNumberFloat, i, num)
		l.x.Emit(fh.Punctuation, num, e)
		return e - i, true
	}
	if c == '-' { // -[a-z_]\w*:*
		n := l.letter(i + 1)
		if n == 0 && l.at(i+1) == '_' {
			n = 1
		}
		if n > 0 {
			e := i + 1 + n + lexkit.Span(src, i+1+n, lexkit.IsWord)
			for l.at(e) == ':' {
				e++
			}
			return l.emit(fh.Name, i, e)
		}
	}
	if n := lexkit.Span(src, i, lexkit.IsWord); n > 0 { // \w+
		return l.emit(fh.Name, i, i+n)
	}
	if strings.IndexByte(".,;@{}[]$()=+*/\\&%!~?^`|<>-", c) >= 0 { // [.,;@{}\[\]$()=+*/\\&%!~?^\x60|<>-]
		return l.emit(fh.Punctuation, i, i+1)
	}
	if c == ':' && l.at(i+1) == ':' { // ::
		return l.emit(fh.Punctuation, i, i+2)
	}
	return 0, false
}

// helpLine matches ^(\s*#[#\s]*)(\.(?:component|…))([^\n]*$) at the # at i. \s* is always empty (\s+ matched
// first), and [#\s]* cannot give back characters, because \. must follow it.
func (l *lexer) helpLine(i int) int {
	src := l.src
	if i > 0 && src[i-1] != '\n' {
		return 0
	}
	j := i + 1 + l.helpRun.Span(src, i+1, isHashOrSpace)
	if l.at(j) != '.' {
		return 0
	}
	for _, w := range docs {
		if n := l.fold(j+1, w); n > 0 {
			e := lexkit.LineEnd(src, j+1+n)
			l.x.Emit(fh.Comment, i, j)
			l.x.Emit(fh.LiteralStringDoc, j, j+1+n)
			l.x.Emit(fh.Comment, j+1+n, e)
			return e - i
		}
	}
	return 0
}

// isHashOrSpace reports whether r is # or whitespace (the characters of helpLine's [#\s]*).
func isHashOrSpace(r rune) bool { return r == '#' || lexkit.IsSpace(r) }

// single returns the length of '([^']|”)*' at i (0 if it does not match). A ” is read as a pair; if no
// lone ' closes the string, the regular expression backtracks to the first ' of the last pair.
func (l *lexer) single(i int) int {
	src := l.src
	lastPair := -1
	for j := i + 1; j < len(src); j++ {
		if src[j] != '\'' {
			continue
		}
		if j+1 < len(src) && src[j+1] == '\'' {
			lastPair = j
			j++
			continue
		}
		return j + 1 - i
	}
	if lastPair >= 0 {
		return lastPair + 1 - i
	}
	return 0
}

// nameEnd returns the end of \w\S*\b at p (-1 if it does not match). \S* gives back characters until the
// end is a word boundary.
func (l *lexer) nameEnd(p int) int {
	src := l.src
	if p >= len(src) {
		return -1
	}
	r, n := lexkit.Rune(src, p)
	if !lexkit.IsWord(r) {
		return -1
	}
	q := p + n
	for e := q + lexkit.Span(src, q, notSpace); e >= q; {
		if lexkit.WordBoundary(src, e) {
			return e
		}
		if e == q {
			break
		}
		_, w := utf8.DecodeLastRuneInString(src[:e])
		e -= w
	}
	return -1
}

var scopes = [...]string{"global:", "script:", "private:", "env:"}

// function matches (?<!\S)(function|filter|workflow)(\s*)(global:|script:|private:|env:)?(\w\S*\b).
func (l *lexer) function(i int) (int, bool) {
	if !l.spaceBefore(i) {
		return 0, false
	}
	var j int
	for _, w := range [...]string{"function", "filter", "workflow"} {
		if n := l.fold(i, w); n > 0 {
			j = i + n
			break
		}
	}
	if j == 0 {
		return 0, false
	}
	// \s* cannot give back spaces: neither the scope nor \w matches a space.
	k := j + lexkit.Span(l.src, j, lexkit.IsSpace)
	scope := 0
	for _, s := range scopes {
		if n := l.fold(k, s); n > 0 {
			scope = n
			break
		}
	}
	e := -1
	if scope > 0 {
		e = l.nameEnd(k + scope)
	}
	if e < 0 {
		scope = 0
		e = l.nameEnd(k)
	}
	if e < 0 {
		return 0, false
	}
	l.x.Emit(fh.KeywordDeclaration, i, j)
	l.x.Emit(fh.TextWhitespace, j, k)
	l.x.Emit(fh.NameVariableMagic, k, k+scope)
	l.x.Emit(fh.NameBuiltin, k+scope, e)
	return e - i, true
}

// class matches (?<!\S)(class|configuration)(\s+)(\w\S*)(\s*)(:*).
func (l *lexer) class(i int) (int, bool) {
	if !l.spaceBefore(i) {
		return 0, false
	}
	j := i + l.fold(i, "class")
	if j == i {
		j = i + l.fold(i, "configuration")
	}
	if j == i {
		return 0, false
	}
	src := l.src
	k := j + lexkit.Span(src, j, lexkit.IsSpace)
	if k == j || k >= len(src) {
		return 0, false
	}
	r, n := lexkit.Rune(src, k)
	if !lexkit.IsWord(r) {
		return 0, false
	}
	p := k + n + lexkit.Span(src, k+n, notSpace)
	q := p + lexkit.Span(src, p, lexkit.IsSpace)
	e := q
	for l.at(e) == ':' {
		e++
	}
	l.x.Emit(fh.KeywordDeclaration, i, j)
	l.x.Emit(fh.TextWhitespace, j, k)
	l.x.Emit(fh.NameBuiltin, k, p)
	l.x.Emit(fh.NameBuiltin, p, q)
	l.x.Emit(fh.NameBuiltin, q, e)
	return e - i, true
}

// varName returns the length of ((global|script|private|env):)?\w+ at p (0 if it does not match). If \w+
// does not follow the scope, the scope is given back and \w+ reads its word.
func (l *lexer) varName(p int) int {
	src := l.src
	for _, s := range scopes {
		if n := l.fold(p, s); n > 0 {
			if w := lexkit.Span(src, p+n, lexkit.IsWord); w > 0 {
				return n + w
			}
			break
		}
	}
	return lexkit.Span(src, p, lexkit.IsWord)
}

// strVar returns the length of ((\$)((global|script|private|env):)?\w+)|((\$){((global|script|private|env):)?\w+})
// at i (0 if it does not match).
func (l *lexer) strVar(i int) int {
	if l.src[i] != '$' {
		return 0
	}
	if n := l.varName(i + 1); n > 0 {
		return 1 + n
	}
	if l.at(i+1) != '{' {
		return 0
	}
	p := i + 2
	for _, s := range scopes {
		if n := l.fold(p, s); n > 0 {
			if w := lexkit.Span(l.src, p+n, lexkit.IsWord); w > 0 && l.at(p+n+w) == '}' {
				return p + n + w + 1 - i
			}
			break
		}
	}
	if w := lexkit.Span(l.src, p, lexkit.IsWord); w > 0 && l.at(p+w) == '}' {
		return p + w + 1 - i
	}
	return 0
}

// dashed returns the length of [a-z]\w*-[a-z]\w*\b at i (0 if it does not match). Both \w* are maximal: the
// - cannot follow a shorter run, and \b holds after the longest one.
func (l *lexer) dashed(i int) int {
	n := l.letter(i)
	if n == 0 {
		return 0
	}
	j := i + n + lexkit.Span(l.src, i+n, lexkit.IsWord)
	if l.at(j) != '-' {
		return 0
	}
	m := l.letter(j + 1)
	if m == 0 {
		return 0
	}
	return j + 1 + m + lexkit.Span(l.src, j+1+m, lexkit.IsWord) - i
}

func isTypeChar(r rune) bool {
	return lexkit.IsWord(r) || r == '.' || r == ' ' || r == '`' || r == ',' || r == '[' || r == ']'
}

// property returns the length of (?<!\[)(?<=\S[^\*|\n]\.)\w+(?=\s+|\(|\{|\.) at i (0 if it does not match).
// The character before i is the ., so (?<!\[) always holds. \w+ is maximal, because a word character is
// none of the characters the lookahead accepts.
func (l *lexer) property(i int) int {
	src := l.src
	if i == 0 || src[i-1] != '.' || i-1 == 0 {
		return 0
	}
	r2, n2 := utf8.DecodeLastRuneInString(src[:i-1])
	if r2 == '*' || r2 == '|' || r2 == '\n' || i-1-n2 == 0 {
		return 0
	}
	if r3, _ := utf8.DecodeLastRuneInString(src[:i-1-n2]); lexkit.IsSpace(r3) {
		return 0
	}
	n := lexkit.Span(src, i, lexkit.IsWord)
	if n == 0 || i+n >= len(src) {
		return 0
	}
	r, _ := lexkit.Rune(src, i+n)
	if lexkit.IsSpace(r) || r == '(' || r == '{' || r == '.' {
		return n
	}
	return 0
}

func (l *lexer) digits(i int) int {
	j := i
	for j < len(l.src) && '0' <= l.src[j] && l.src[j] <= '9' {
		j++
	}
	return j - i
}

// number matches (?<!\w)([-+]?(?:[0-9]+)?\.?[0-9]+(?:(?:e|E)[0-9]+)?(?:F|f|D|d|M|m)?)((?i:[kmgtp]b)?)\b at i.
// It returns the end of the number (group 1) and of the whole match (end is 0 if it does not match).
//
// A run of digits that is cut short is followed by a digit, which none of the later parts accept (and
// which is no word boundary), so every [0-9]+ is maximal. That leaves two readings: digits.digits, and
// (if that fails) the digits alone, which both (?:[0-9]+)? empty and the shorter (?:[0-9]+)? reach.
func (l *lexer) number(i int) (int, int) {
	src := l.src
	if i > 0 {
		if r, _ := utf8.DecodeLastRuneInString(src[:i]); lexkit.IsWord(r) {
			return 0, 0
		}
	}
	p := i
	if c := src[p]; c == '-' || c == '+' {
		p++
	}
	n := l.digits(p)
	if l.at(p+n) == '.' {
		if m := l.digits(p + n + 1); m > 0 {
			if num, e := l.afterDigits(p + n + 1 + m); e > 0 {
				return num, e
			}
		}
	}
	if n > 0 {
		return l.afterDigits(p + n)
	}
	return 0, 0
}

// afterDigits matches (?:(?:e|E)[0-9]+)?(?:F|f|D|d|M|m)?((?i:[kmgtp]b)?)\b at q, trying the longer choices
// first.
func (l *lexer) afterDigits(q int) (int, int) {
	if lexkit.FoldASCII(rune(l.at(q))) == 'e' {
		if m := l.digits(q + 1); m > 0 {
			if num, e := l.afterExp(q + 1 + m); e > 0 {
				return num, e
			}
		}
	}
	return l.afterExp(q)
}

func (l *lexer) afterExp(q int) (int, int) {
	if c := l.at(q); c != 0 && strings.IndexByte("fdm", byte(lexkit.FoldASCII(rune(c)))) >= 0 {
		if e := l.unit(q + 1); e > 0 {
			return q + 1, e
		}
	}
	if e := l.unit(q); e > 0 {
		return q, e
	}
	return 0, 0
}

// unit matches ((?i:[kmgtp]b)?)\b at q and returns the end (0 if it does not match).
func (l *lexer) unit(q int) int {
	if q < len(l.src) {
		r, n := lexkit.Rune(l.src, q)
		if f := lexkit.FoldASCII(r); f < 0x80 && strings.IndexByte("kmgtp", byte(f)) >= 0 &&
			lexkit.FoldASCII(rune(l.at(q+n))) == 'b' && lexkit.WordBoundary(l.src, q+n+1) {
			return q + n + 1
		}
	}
	if lexkit.WordBoundary(l.src, q) {
		return q
	}
	return 0
}

var requiresWords = [...]string{"modules", "pssnapin", "runasadministrator", "ahellid", "version", "assembly", "psedition"}

func (l *lexer) requires(i int) (int, bool) {
	src := l.src
	// \s*\n|\s*$: up to the last newline of the spaces, or the spaces up to the end of the text.
	if w := lexkit.Span(src, i, lexkit.IsSpace); w > 0 {
		if k := strings.LastIndexByte(src[i:i+w], '\n'); k >= 0 {
			l.pop()
			return l.emit(fh.TextWhitespace, i, i+k+1)
		}
		if i+w == len(src) {
			l.pop()
			return l.emit(fh.TextWhitespace, i, i+w)
		}
		// \s+(\S+)
		if n := lexkit.Span(src, i+w, notSpace); n > 0 {
			return l.emit(fh.NameAttribute, i, i+w+n)
		}
		return 0, false
	}
	if src[i] != '-' {
		return 0, false
	}
	for _, w := range requiresWords { // -(?i:modules|pssnapin|…)
		if n := l.fold(i+1, w); n > 0 {
			return l.emit(fh.KeywordDeclaration, i, i+1+n)
		}
	}
	// -\S*\b
	q := i + 1
	for e := q + lexkit.Span(src, q, notSpace); e >= q; {
		if lexkit.WordBoundary(src, e) {
			return l.emit(fh.Comment, i, e)
		}
		if e == q {
			break
		}
		_, w := utf8.DecodeLastRuneInString(src[:e])
		e -= w
	}
	return 0, false
}

func (l *lexer) multline(i int) (int, bool) {
	src := l.src
	j := i
	for j < len(src) && src[j] != '#' && src[j] != '&' && src[j] != '.' {
		j++
	}
	if j > i { // [^#&.]+
		return l.emit(fh.CommentMultiline, i, j)
	}
	if src[i] == '#' && (l.at(i+1) == '>' || l.fold(i+1, "&gt;") > 0) { // #(>|&gt;)
		l.pop()
		if l.at(i+1) == '>' {
			return l.emit(fh.CommentMultiline, i, i+2)
		}
		return l.emit(fh.CommentMultiline, i, i+5)
	}
	if src[i] == '.' { // (\s*\.)(component|…)(\s*$); the first \s* is always empty here
		for _, w := range docs {
			n := l.fold(i+1, w)
			if n == 0 {
				continue
			}
			p := i + 1 + n
			s := lexkit.Span(src, p, lexkit.IsSpace)
			e := p + s
			if e != len(src) {
				k := strings.LastIndexByte(src[p:e], '\n')
				if k < 0 {
					break
				}
				e = p + k
			}
			l.x.Emit(fh.CommentMultiline, i, i+1)
			l.x.Emit(fh.LiteralStringDoc, i+1, p)
			l.x.Emit(fh.CommentMultiline, p, e)
			return e - i, true
		}
	}
	return l.emit(fh.CommentMultiline, i, i+1) // [#&.]
}

func (l *lexer) str(i int) (int, bool) {
	src := l.src
	switch src[i] {
	case '`':
		if i+1 < len(src) { // `[0abfnrtv'"$`]
			r, n := lexkit.Rune(src, i+1)
			if f := lexkit.FoldASCII(r); f < 0x80 && strings.IndexByte("0abfnrtv'\"$`", byte(f)) >= 0 {
				return l.emit(fh.LiteralStringEscape, i, i+1+n)
			}
		}
		return l.emit(fh.LiteralStringDouble, i, i+1) // [`$]
	case '$':
		if l.at(i+1) == '(' { // \$\(
			l.push(child)
			return l.emit(fh.Punctuation, i, i+2)
		}
		if n := l.strVar(i); n > 0 {
			return l.emit(fh.NameVariable, i, i+n)
		}
		return l.emit(fh.LiteralStringDouble, i, i+1) // [`$]
	case '"':
		if l.at(i+1) == '"' { // ""
			return l.emit(fh.LiteralStringDouble, i, i+2)
		}
		l.pop()
		return l.emit(fh.LiteralStringDouble, i, i+1)
	}
	j := i
	for j < len(src) && src[j] != '$' && src[j] != '`' && src[j] != '"' {
		j++
	}
	return l.emit(fh.LiteralStringDouble, i, j) // [^$`"]+
}

func (l *lexer) heredoc(i int) (int, bool) {
	src := l.src
	if strings.HasPrefix(src[i:], "\n\"@") { // \n"@
		l.pop()
		return l.emit(fh.LiteralStringHeredoc, i, i+3)
	}
	if src[i] == '$' {
		if l.at(i+1) == '(' { // \$\(
			l.push(child)
			return l.emit(fh.Punctuation, i, i+2)
		}
		if n := l.strVar(i); n > 0 {
			return l.emit(fh.NameVariable, i, i+n)
		}
	}
	// [^@\n]+"]: the last "] inside the run of characters other than @ and newline.
	if i >= l.runEnd {
		j := i
		for j < len(src) && src[j] != '@' && src[j] != '\n' {
			j++
		}
		l.runEnd, l.lastQuote = j, strings.LastIndex(src[i:j], "\"]")
		if l.lastQuote >= 0 {
			l.lastQuote += i
		}
	}
	if l.lastQuote > i {
		return l.emit(fh.LiteralStringHeredoc, i, l.lastQuote+2)
	}
	_, n := lexkit.Rune(src, i) // .
	return l.emit(fh.LiteralStringHeredoc, i, i+n)
}
