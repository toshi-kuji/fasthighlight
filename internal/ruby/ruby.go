// Package ruby is the Ruby tokenizer. lang/ruby registers it. Options (the switch for the intended
// differences from Chroma) exists only for the comparison in compat/ and is not exposed.
//
// The token types and token boundaries reproduce the rules of Chroma 2.27.0's ruby.xml, used as the
// reference, without using regular expressions. ruby.xml is dot_all, so . also matches a newline.
//
// Intended differences from Chroma (setting Options.Chroma to true makes it the same as Chroma):
//   - In string rules such as "% abc ", Chroma emits the delimiter character a second time (as None),
//     so characters increase. The hand-written version does not emit it
//   - defined? requires a \b after it in Chroma, so it almost never becomes a keyword. The
//     hand-written version makes it a keyword
//   - Names (variables, methods, symbols and so on) can start only with an ASCII letter in Chroma, so
//     a name such as 日本 becomes Error. The hand-written version can also start with a Unicode letter
package ruby

import (
	"strings"
	"unicode"
	"unicode/utf8"

	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/lexkit"
)

// Options is the switch for the intended differences from Chroma. The zero value is the production
// setting (the one lang/ruby registers).
type Options struct {
	// If Chroma is true, the three points above are made the same as Chroma.
	Chroma bool
}

// Indexes of the word lists (lists.go).
const (
	lKeywords = iota
	lPseudo
	lOperatorWords
	lBuiltinQuestions
	lBuiltins2
)

var lists *lexkit.WordLists

// Prepare builds the word tables (Prepare of fasthighlight.Language).
func Prepare() {
	lists = lexkit.NewWordLists(keywords, pseudo, operatorWords, builtinQuestions, builtins2)
}

type kind uint8

const (
	root kind = iota
	simpleString
	simpleSym
	simpleBacktick
	bracket // bracketed strings and regular expressions such as %Q{ … }
	multilineRegex
	inIntp
	classname
	defexpr
	funcname
	endPart
)

// The kinds of bracketed string.
const (
	intpString  = iota // %Q{ and so on (interpolation and escapes)
	plainString        // %q{ and so on
	regex              // %r{ and so on (interpolation)
)

type frame struct {
	kind        kind
	style       uint8 // for bracket: intpString, plainString or regex
	open, close byte  // for bracket: the brackets
}

type lexer struct {
	o     Options
	x     *lexkit.Tokens
	src   string
	stack []frame
	words lexkit.WordCache
}

// Lex splits src into tokens (the same contract as Lex of fasthighlight.Language).
func Lex(src string, end int, o Options) []fh.Token {
	l := &lexer{o: o, x: lexkit.NewTokens(src), src: src, stack: []frame{{kind: root}}, words: lexkit.WordCache{Lists: lists}}
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

func (l *lexer) spaces(i int) int { return lexkit.Span(l.src, i, lexkit.IsSpace) }

func (l *lexer) bol(i int) bool { return i == 0 || l.src[i-1] == '\n' }

func isASCIILetter(c byte) bool { return 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' }
func isDec(c byte) bool         { return '0' <= c && c <= '9' }
func isOct(c byte) bool         { return '0' <= c && c <= '7' }
func isHex(c byte) bool {
	return isDec(c) || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F'
}

// ident returns the length matched by [a-zA-Z_]\w* (if Options.Chroma is false, it can also start
// with a Unicode letter).
func (l *lexer) ident(i int) int {
	r, n := l.runeAt(i)
	if n == 0 {
		return 0
	}
	if !(r == '_' || r < utf8.RuneSelf && isASCIILetter(byte(r)) || !l.o.Chroma && r >= utf8.RuneSelf && unicode.IsLetter(r)) {
		return 0
	}
	return n + lexkit.Span(l.src, i+n, lexkit.IsWord)
}

// asciiIdent returns the length matched by [a-zA-Z_]\w* (the same as Chroma; used for heredoc names
// and so on).
func (l *lexer) asciiIdent(i int) int {
	if c := l.at(i); c == '_' || isASCIILetter(c) {
		return 1 + lexkit.Span(l.src, i+1, lexkit.IsWord)
	}
	return 0
}

// bang returns e+1 if there is a [!?] at position e (otherwise e).
func (l *lexer) bang(e int) int {
	if c := l.at(e); c == '!' || c == '?' {
		return e + 1
	}
	return e
}

func (l *lexer) step(f frame, i int) (int, bool) {
	src := l.src
	switch f.kind {
	case root:
		return l.root(i)
	case simpleString, simpleSym, simpleBacktick:
		t, q := fh.LiteralStringDouble, byte('"')
		switch f.kind {
		case simpleSym:
			t = fh.LiteralStringSymbol
		case simpleBacktick:
			t, q = fh.LiteralStringBacktick, '`'
		}
		if n, ok := l.intpEscaped(i); ok {
			return n, true
		}
		switch c := src[i]; c {
		case '\\', '#':
			return l.emit(t, i, i+1)
		case q:
			l.pop()
			return l.emit(t, i, i+1)
		}
		j := i + 1
		for j < len(src) && src[j] != '\\' && src[j] != '#' && src[j] != q {
			j++
		}
		return l.emit(t, i, j)
	case bracket:
		return l.bracket(f, i)
	case multilineRegex:
		if n, ok := l.intp(i); ok {
			return n, true
		}
		switch c := src[i]; c {
		case '\\':
			if d := l.at(i + 1); d == '\\' || d == '/' { // \\\\ and \\/
				return l.emit(fh.LiteralStringRegex, i, i+2)
			}
			return l.emit(fh.LiteralStringRegex, i, i+1)
		case '#':
			return l.emit(fh.LiteralStringRegex, i, i+1)
		case '/': // /[mixounse]*
			l.pop()
			return l.emit(fh.LiteralStringRegex, i, l.flags(i+1))
		}
		j := i + 1
		for j < len(src) && src[j] != '\\' && src[j] != '/' && src[j] != '#' {
			j++
		}
		return l.emit(fh.LiteralStringRegex, i, j)
	case inIntp:
		switch src[i] {
		case '{':
			l.push(frame{kind: inIntp})
			return l.emit(fh.LiteralStringInterpol, i, i+1)
		case '}':
			l.pop()
			return l.emit(fh.LiteralStringInterpol, i, i+1)
		}
		return l.root(i)
	case classname:
		if src[i] == '(' {
			l.push(frame{kind: defexpr})
			return l.emit(fh.Punctuation, i, i+1)
		}
		if strings.HasPrefix(src[i:], "<<") {
			l.pop()
			return l.emit(fh.Operator, i, i+2)
		}
		l.pop()
		if c := src[i]; c == '_' || 'A' <= c && c <= 'Z' { // [A-Z_]\w*
			return l.emit(fh.NameClass, i, i+1+lexkit.Span(src, i+1, lexkit.IsWord))
		}
		return 0, true
	case defexpr:
		if src[i] == ')' { // (\))(\.|::)?
			l.pop()
			l.x.Emit(fh.Punctuation, i, i+1)
			switch {
			case l.at(i+1) == '.':
				l.x.Emit(fh.Operator, i+1, i+2)
				return 2, true
			case strings.HasPrefix(src[i+1:], "::"):
				l.x.Emit(fh.Operator, i+1, i+3)
				return 3, true
			}
			return 1, true
		}
		if src[i] == '(' {
			l.push(frame{kind: defexpr})
			return l.emit(fh.Operator, i, i+1)
		}
		return l.root(i)
	case funcname:
		if src[i] == '(' {
			l.push(frame{kind: defexpr})
			return l.emit(fh.Punctuation, i, i+1)
		}
		l.pop()
		// (?:([a-zA-Z_]\w*)(\.))?(method name)
		if n := l.ident(i); n > 0 && l.at(i+n) == '.' {
			if m := l.methodName(i + n + 1); m > 0 {
				l.x.Emit(fh.NameClass, i, i+n)
				l.x.Emit(fh.Operator, i+n, i+n+1)
				l.x.Emit(fh.NameFunction, i+n+1, i+n+1+m)
				return n + 1 + m, true
			}
		}
		if m := l.methodName(i); m > 0 {
			return l.emit(fh.NameFunction, i, i+m)
		}
		return 0, true
	case endPart:
		l.pop()
		return l.emit(fh.CommentPreproc, i, len(src)) // .+
	}
	return 0, false
}

// methodName returns the length matched by funcname's
// [a-zA-Z_]\w*[!?]?|\*\*?|[-+]@?|[/%&|^`~]|\[\]=?|<<|>>|<=?>|>=?|===?.
func (l *lexer) methodName(i int) int {
	src := l.src
	if n := l.ident(i); n > 0 {
		return l.bang(i+n) - i
	}
	switch c := l.at(i); c {
	case '*':
		if l.at(i+1) == '*' {
			return 2
		}
		return 1
	case '-', '+':
		if l.at(i+1) == '@' {
			return 2
		}
		return 1
	case '/', '%', '&', '|', '^', '`', '~':
		return 1
	case '[':
		if l.at(i+1) == ']' {
			if l.at(i+2) == '=' {
				return 3
			}
			return 2
		}
	}
	for _, op := range [...]string{"<<", ">>", "<=>", "<>", ">=", ">", "===", "=="} {
		if strings.HasPrefix(src[i:], op) {
			return len(op)
		}
	}
	return 0
}

// flags returns the end of [mixounse]*.
func (l *lexer) flags(j int) int {
	for j < len(l.src) && strings.IndexByte("mixounse", l.src[j]) >= 0 {
		j++
	}
	return j
}

// intp tries the rules of string-intp (#{, #@x, #@@x, #$x).
func (l *lexer) intp(i int) (int, bool) {
	if l.src[i] != '#' {
		return 0, false
	}
	switch l.at(i + 1) {
	case '{':
		l.push(frame{kind: inIntp})
		return l.emit(fh.LiteralStringInterpol, i, i+2)
	case '@':
		j := i + 2
		if l.at(j) == '@' {
			j++
		}
		if n := l.ident(j); n > 0 {
			return l.emit(fh.LiteralStringInterpol, i, j+n)
		}
	case '$':
		if n := l.ident(i + 2); n > 0 {
			return l.emit(fh.LiteralStringInterpol, i, i+2+n)
		}
	}
	return 0, false
}

// intpEscaped tries the rules of string-intp-escaped (string-intp, and the escape \\(…)).
func (l *lexer) intpEscaped(i int) (int, bool) {
	if n, ok := l.intp(i); ok {
		return n, true
	}
	if n := l.escape(i); n > 0 {
		return l.emit(fh.LiteralStringEscape, i, i+n)
	}
	return 0, false
}

// escape returns the length matched by \\([\\abefnrstv#"\']|x[a-fA-F0-9]{1,2}|[0-7]{1,3}) (0 if it
// does not match).
func (l *lexer) escape(i int) int {
	if l.at(i) != '\\' {
		return 0
	}
	switch c := l.at(i + 1); {
	case c != 0 && strings.IndexByte("\\abefnrstv#\"'", c) >= 0:
		return 2
	case c == 'x' && isHex(l.at(i+2)):
		if isHex(l.at(i + 3)) {
			return 4
		}
		return 3
	case isOct(c):
		n := 2
		for n < 4 && isOct(l.at(i+n)) {
			n++
		}
		return n
	}
	return 0
}

// bracket tries the rules of the bracket states such as %Q{ … }.
func (l *lexer) bracket(f frame, i int) (int, bool) {
	src := l.src
	t := fh.LiteralStringOther
	if f.style == regex {
		t = fh.LiteralStringRegex
	}
	c := src[i]
	if c == '\\' {
		if d := l.at(i + 1); d == '\\' || d == f.open || d == f.close {
			return l.emit(t, i, i+2)
		}
	}
	switch c {
	case f.open:
		l.push(f)
		return l.emit(t, i, i+1)
	case f.close:
		l.pop()
		e := i + 1
		if f.style == regex {
			e = l.flags(e)
		}
		return l.emit(t, i, e)
	}
	switch f.style {
	case intpString:
		if n, ok := l.intpEscaped(i); ok {
			return n, true
		}
	case regex:
		if n, ok := l.intp(i); ok {
			return n, true
		}
	}
	if c == '\\' || c == '#' {
		return l.emit(t, i, i+1)
	}
	j := i + 1
	for j < len(src) && src[j] != '\\' && src[j] != '#' && src[j] != f.open && src[j] != f.close {
		j++
	}
	return l.emit(t, i, j)
}

// quoted returns the end of q(\\\\|\\q|[^q])*q (-1 if it does not match). src[p] is just after the
// opening q. If there is no closing q, then, as with regular-expression backtracking, \q is re-read
// as the single character \ and the string closes.
func (l *lexer) quoted(p int, q byte) int {
	src := l.src
	for j := p; j < len(src); {
		switch src[j] {
		case q:
			return j + 1
		case '\\':
			if d := l.at(j + 1); d == '\\' || d == q {
				j += 2
				continue
			}
		}
		j++
	}
	failed := map[int]bool{}
	var try func(j int) int
	try = func(j int) int {
		for j < len(src) {
			if failed[j] {
				return -1
			}
			switch src[j] {
			case q:
				return j + 1
			case '\\':
				if d := l.at(j + 1); d == '\\' || d == q {
					if e := try(j + 2); e >= 0 {
						return e
					}
					failed[j+2] = true
				}
			}
			j++
		}
		return -1
	}
	return try(p)
}

// delimited returns the position just after the closing d of ((?:\\d|(?!d).)*)(d) (-1 if it does not
// match). p is just after the opening d. It reproduces the regular expression's backtracking too
// (when there is no closing d, it closes at the d of \d).
func (l *lexer) delimited(p int, d string) int {
	src := l.src
	failed := map[int]bool{}
	var try func(j int) int
	try = func(j int) int {
		for {
			if failed[j] {
				return -1
			}
			esc := src[j:] != "" && src[j] == '\\' && strings.HasPrefix(src[j+1:], d)
			isD := strings.HasPrefix(src[j:], d)
			if esc { // \\d
				if e := try(j + 1 + len(d)); e >= 0 {
					return e
				}
				failed[j+1+len(d)] = true
			}
			if j < len(src) && !isD { // (?!d).
				_, n := lexkit.Rune(src, j)
				j += n
				continue
			}
			if isD { // end the repetition; this is the closing d
				return j + len(d)
			}
			return -1
		}
	}
	return try(p)
}

// percent tries the rules of strings that start with % (8 to 25). src[i] is %.
func (l *lexer) percent(i int) (int, bool) {
	src := l.src
	c1 := l.at(i + 1)
	// %[QWx]?\{ and so on (brackets)
	k, letter := i+1, byte(0)
	if c1 != 0 && strings.IndexByte("QWxqswr", c1) >= 0 {
		letter, k = c1, i+2
	}
	if b := l.at(k); b == '{' || b == '[' || b == '(' || b == '<' {
		closeOf := map[byte]byte{'{': '}', '[': ']', '(': ')', '<': '>'}
		style := uint8(intpString)
		t := fh.LiteralStringOther
		switch {
		case letter == 'r':
			style, t = regex, fh.LiteralStringRegex
		case letter == 'q' || letter == 's' || letter == 'w':
			style = plainString
		}
		l.push(frame{kind: bracket, style: style, open: b, close: closeOf[b]})
		return l.emit(t, i, k+1)
	}
	delim := func(j int) (string, bool) { // [\W_]
		r, n := l.runeAt(j)
		if n == 0 || lexkit.IsWord(r) && r != '_' {
			return "", false
		}
		return src[j : j+n], true
	}
	switch c1 {
	case 'r': // (%r([\W_]))((?:\\\2|(?!\2).)*)(\2[mixounse]*)
		if d, ok := delim(i + 2); ok {
			if e := l.delimited(i+2+len(d), d); e >= 0 {
				return l.emit(fh.LiteralString, i, l.flags(e))
			}
		}
	case 'q', 's', 'w': // %[qsw]([\W_])((?:\\\1|(?!\1).)*)\1
		if d, ok := delim(i + 2); ok {
			if e := l.delimited(i+2+len(d), d); e >= 0 {
				return l.emit(fh.LiteralStringOther, i, e)
			}
		}
	case 'Q', 'W', 'x': // (%[QWx]([\W_]))((?:\\\2|(?!\2).)*)(\2)
		if d, ok := delim(i + 2); ok {
			if e := l.delimited(i+2+len(d), d); e >= 0 {
				return l.emit(fh.LiteralString, i, e)
			}
		}
	}
	return 0, false
}

// spacePercent tries (\s*)(%([\t ])(?:(?:\\\3|(?!\3).)*)\3) from position i (the caller checks the
// preceding condition).
func (l *lexer) spacePercent(i int) (int, bool) {
	src := l.src
	s := i + l.spaces(i)
	if l.at(s) != '%' {
		return 0, false
	}
	d := l.at(s + 1)
	if d != ' ' && d != '\t' {
		return 0, false
	}
	e := l.delimited(s+2, src[s+1:s+2])
	if e < 0 {
		return 0, false
	}
	l.x.Emit(fh.Text, i, s)
	l.x.Emit(fh.LiteralStringOther, s, e)
	if l.o.Chroma {
		// Chroma emits the third part (the delimiter character) a second time as None.
		l.x.Emit(fh.None, s+1, s+2)
	}
	return e - i, true
}

// percentGeneric tries (%([^a-zA-Z0-9\s]))((?:\\\2|(?!\2).)*)(\2). src[i] is %.
func (l *lexer) percentGeneric(i int) (int, bool) {
	r, n := l.runeAt(i + 1)
	if n == 0 || r < utf8.RuneSelf && (isASCIILetter(byte(r)) || isDec(byte(r))) || lexkit.IsSpace(r) {
		return 0, false
	}
	if e := l.delimited(i+1+n, l.src[i+1:i+1+n]); e >= 0 {
		return l.emit(fh.LiteralString, i, e)
	}
	return 0, false
}

// symOps are the operator names for symbols and method calls (in the same order as the regular
// expression).
var symOps = [...]string{"===", "[]=", "<=>", "**", "==", ">=", "+@", "<>", ">>", "<<", "-@", "[]", "~", "`", "^",
	"|", "&", "<", "%", "/", ">", "+", "-", "*"}

func (l *lexer) symOp(i int) int {
	for _, op := range symOps {
		if strings.HasPrefix(l.src[i:], op) {
			return len(op)
		}
	}
	return 0
}

// strings tries the rules of strings.
func (l *lexer) strings(i int) (int, bool) {
	src := l.src
	c := src[i]
	if c == ':' {
		j := i + 1
		for k := 0; k < 2 && l.at(j) == '@'; k++ { // @{0,2}
			j++
		}
		// Backtracking: even with fewer @, it does not match, because names and operators do not start
		// with @.
		if n := l.ident(j); n > 0 { // \:@{0,2}[a-zA-Z_]\w*[!?]?
			return l.emit(fh.LiteralStringSymbol, i, l.bang(j+n))
		}
		if n := l.symOp(j); n > 0 {
			return l.emit(fh.LiteralStringSymbol, i, j+n)
		}
		if l.at(i+1) == '\'' { // :'(\\\\|\\'|[^'])*'
			if e := l.quoted(i+2, '\''); e >= 0 {
				return l.emit(fh.LiteralStringSymbol, i, e)
			}
		}
	}
	if c == '\'' {
		if e := l.quoted(i+1, '\''); e >= 0 {
			return l.emit(fh.LiteralStringSingle, i, e)
		}
	}
	if c == ':' && l.at(i+1) == '"' {
		l.push(frame{kind: simpleSym})
		return l.emit(fh.LiteralStringSymbol, i, i+2)
	}
	// ([a-zA-Z_]\w*)(:)(?!:)
	if n := l.ident(i); n > 0 && l.at(i+n) == ':' && l.at(i+n+1) != ':' {
		l.x.Emit(fh.LiteralStringSymbol, i, i+n)
		l.x.Emit(fh.Punctuation, i+n, i+n+1)
		return n + 1, true
	}
	if c == '"' {
		l.push(frame{kind: simpleString})
		return l.emit(fh.LiteralStringDouble, i, i+1)
	}
	if c == '`' && (i == 0 || src[i-1] != '.') {
		l.push(frame{kind: simpleBacktick})
		return l.emit(fh.LiteralStringBacktick, i, i+1)
	}
	if c == '%' {
		if n, ok := l.percent(i); ok {
			return n, true
		}
	}
	// (?<=[-+/*%=<>&!^|~,(])(\s*)(%…) and ^(\s*)(%…)
	if i > 0 && strings.IndexByte("-+/*%=<>&!^|~,(", src[i-1]) >= 0 || l.bol(i) {
		if n, ok := l.spacePercent(i); ok {
			return n, true
		}
	}
	if c == '%' {
		if n, ok := l.percentGeneric(i); ok {
			return n, true
		}
	}
	return 0, false
}

// lastRune returns the last character of src[:i] and where it starts (-1 if there is none).
func (l *lexer) lastRune(i int) (rune, int) {
	if i <= 0 {
		return -1, -1
	}
	r, n := utf8.DecodeLastRuneInString(l.src[:i])
	return r, i - n
}

// regexStart checks the condition (lookbehind) before the 17th rule of root.
func (l *lexer) regexStart(i int) bool {
	src := l.src
	if l.bol(i) {
		return true
	}
	if strings.IndexByte("=<>~!:", src[i-1]) >= 0 {
		return true
	}
	r, p := l.lastRune(i) // the final \s of the lookbehind
	if p < 0 || !lexkit.IsSpace(r) {
		return false
	}
	before := src[:p]
	for _, w := range [...]string{"when", "or", "and", "if", "elsif"} { // (?<=(?:\s|;)w\s)
		if strings.HasSuffix(before, w) {
			q := p - len(w)
			if r2, _ := l.lastRune(q); r2 == ';' || r2 >= 0 && lexkit.IsSpace(r2) {
				return true
			}
		}
	}
	for _, w := range [...]string{".index", ".scan", ".sub", ".sub!", ".gsub", ".gsub!", ".match"} { // (?<=\.w\s)
		if strings.HasSuffix(before, w) {
			return true
		}
	}
	for _, w := range [...]string{"when", "index", "scan", "sub", "gsub", "sub!", "gsub!", "match", "if", "elsif"} { // (?<=^w\s)
		if strings.HasSuffix(before, w) && l.bol(p-len(w)) {
			return true
		}
	}
	return false
}

// charLiteral returns the length matched by \?(\\[MC]-)*(\\(…)|\S)(?!\w) (0 if it does not match).
// src[i] is ?.
func (l *lexer) charLiteral(i int) int {
	notWord := func(e int) bool {
		r, n := l.runeAt(e)
		return n == 0 || !lexkit.IsWord(r)
	}
	var prefixes []int // candidate ends of (\\[MC]-)* (longest first)
	j := i + 1
	prefixes = append(prefixes, j)
	for l.at(j) == '\\' && (l.at(j+1) == 'M' || l.at(j+1) == 'C') && l.at(j+2) == '-' {
		j += 3
		prefixes = append(prefixes, j)
	}
	for k := len(prefixes) - 1; k >= 0; k-- {
		p := prefixes[k]
		var cands []int
		if l.at(p) == '\\' {
			switch c := l.at(p + 1); {
			case c != 0 && strings.IndexByte("\\abefnrstv#\"'", c) >= 0:
				cands = append(cands, p+2)
			case c == 'x' && isHex(l.at(p+2)):
				if isHex(l.at(p + 3)) {
					cands = append(cands, p+4)
				}
				cands = append(cands, p+3)
			case isOct(c):
				n := 1
				for n < 3 && isOct(l.at(p+1+n)) {
					n++
				}
				for ; n >= 1; n-- {
					cands = append(cands, p+1+n)
				}
			}
		}
		if r, n := l.runeAt(p); n > 0 && !lexkit.IsSpace(r) { // \S
			cands = append(cands, p+n)
		}
		for _, e := range cands {
			if notWord(e) {
				return e - i
			}
		}
	}
	return 0
}

// numberTail adds (\s*)([/?])? and returns the end of the whole.
func (l *lexer) numberTail(t fh.TokenType, i, e int) (int, bool) {
	s := e + l.spaces(e)
	l.x.Emit(t, i, e)
	l.x.Emit(fh.Text, e, s)
	if c := l.at(s); c == '/' || c == '?' {
		l.x.Emit(fh.Operator, s, s+1)
		s++
	}
	return s - i, true
}

// sepDigits returns the end of X+(?:_X+)* (-1 if src[j] is not X).
func (l *lexer) sepDigits(j int, f func(byte) bool) int {
	if !f(l.at(j)) {
		return -1
	}
	for f(l.at(j)) {
		j++
	}
	for l.at(j) == '_' && f(l.at(j+1)) {
		j++
		for f(l.at(j)) {
			j++
		}
	}
	return j
}

func (l *lexer) root(i int) (int, bool) {
	src := l.src
	c := src[i]
	if c == '#' {
		// \A#!.+?$
		if i == 0 && l.at(1) == '!' && len(src) > 2 {
			e := len(src)
			if k := strings.IndexByte(src[3:], '\n'); k >= 0 {
				e = 3 + k
			}
			return l.emit(fh.CommentHashbang, i, e)
		}
		return l.emit(fh.CommentSingle, i, lexkit.LineEnd(src, i)) // #.*?$
	}
	// =begin\s.*?\n=end.*?$
	if strings.HasPrefix(src[i:], "=begin") {
		if r, n := l.runeAt(i + 6); n > 0 && lexkit.IsSpace(r) {
			if k := strings.Index(src[i+6+n:], "\n=end"); k >= 0 {
				return l.emit(fh.CommentMultiline, i, lexkit.LineEnd(src, i+6+n+k+5))
			}
		}
	}
	// keywords (defined? requires a \b after it in Chroma)
	if !l.o.Chroma && strings.HasPrefix(src[i:], "defined?") {
		if n, _ := l.words.Word(src, i); n == 7 {
			return l.emit(fh.Keyword, i, i+8)
		}
	}
	if n := l.words.Match(src, i, lKeywords); n > 0 {
		return l.emit(fh.Keyword, i, i+n)
	}
	// (module)(\s+)([a-zA-Z_]\w*(?:::[a-zA-Z_]\w*)*)
	if strings.HasPrefix(src[i:], "module") {
		if s := l.spaces(i + 6); s > 0 {
			k := i + 6 + s
			if n := l.ident(k); n > 0 {
				e := k + n
				for strings.HasPrefix(src[e:], "::") {
					m := l.ident(e + 2)
					if m == 0 {
						break
					}
					e += 2 + m
				}
				l.x.Emit(fh.Keyword, i, i+6)
				l.x.Emit(fh.Text, i+6, k)
				l.x.Emit(fh.NameNamespace, k, e)
				return e - i, true
			}
		}
	}
	if strings.HasPrefix(src[i:], "def") {
		if s := l.spaces(i + 3); s > 0 { // (def)(\s+)
			l.x.Emit(fh.Keyword, i, i+3)
			l.x.Emit(fh.Text, i+3, i+3+s)
			l.push(frame{kind: funcname})
			return 3 + s, true
		}
		// def(?=[*%&^`~+-/\[<>=]) (+-/ is the range from + to /)
		if d := l.at(i + 3); d != 0 && strings.IndexByte("*%&^`~+,-./[<>=", d) >= 0 {
			l.push(frame{kind: funcname})
			return l.emit(fh.Keyword, i, i+3)
		}
	}
	if strings.HasPrefix(src[i:], "class") {
		if s := l.spaces(i + 5); s > 0 {
			l.x.Emit(fh.Keyword, i, i+5)
			l.x.Emit(fh.Text, i+5, i+5+s)
			l.push(frame{kind: classname})
			return 5 + s, true
		}
	}
	if n := l.words.Match(src, i, lPseudo); n > 0 {
		return l.emit(fh.KeywordPseudo, i, i+n)
	}
	if n := l.words.Match(src, i, lOperatorWords); n > 0 {
		return l.emit(fh.OperatorWord, i, i+n)
	}
	if n, mask := l.words.Word(src, i); n > 0 && mask&(1<<lBuiltinQuestions) != 0 && l.at(i+n) == '?' { // (…)\?
		return l.emit(fh.NameBuiltin, i, i+n+1)
	}
	for _, w := range [...]string{"chomp", "chop", "exit", "gsub", "sub"} { // (chomp|chop|exit|gsub|sub)!
		if strings.HasPrefix(src[i:], w) && l.at(i+len(w)) == '!' {
			return l.emit(fh.NameBuiltin, i, i+len(w)+1)
		}
	}
	if i == 0 || src[i-1] != '.' { // (?<!\.)(…)\b
		if n := l.words.Match(src, i, lBuiltins2); n > 0 {
			return l.emit(fh.NameBuiltin, i, i+n)
		}
	}
	for _, w := range [...]string{"__FILE__", "__LINE__"} { // __(FILE|LINE)__\b
		if strings.HasPrefix(src[i:], w) && lexkit.WordBoundary(src, i+len(w)) {
			return l.emit(fh.NameBuiltinPseudo, i, i+len(w))
		}
	}
	if strings.HasPrefix(src[i:], "<<") {
		if e := l.heredoc(i); e > 0 {
			return l.emit(fh.LiteralString, i, e)
		}
	}
	if strings.HasPrefix(src[i:], "__END__") {
		l.push(frame{kind: endPart})
		return l.emit(fh.CommentPreproc, i, i+7)
	}
	// (lookbehind)(\s*)(/)
	if l.regexStart(i) {
		if s := l.spaces(i); l.at(i+s) == '/' {
			l.x.Emit(fh.Text, i, i+s)
			l.x.Emit(fh.LiteralStringRegex, i+s, i+s+1)
			l.push(frame{kind: multilineRegex})
			return s + 1, true
		}
	}
	if c == '/' && i > 0 && (src[i-1] == '(' || src[i-1] == ',' || src[i-1] == '[') { // (?<=\(|,|\[)/
		l.push(frame{kind: multilineRegex})
		return l.emit(fh.LiteralStringRegex, i, i+1)
	}
	if s := l.spaces(i); s > 0 && l.at(i+s) == '/' { // (\s+)(/)(?![\s=])
		if r, n := l.runeAt(i + s + 1); n == 0 || !lexkit.IsSpace(r) && r != '=' {
			l.x.Emit(fh.Text, i, i+s)
			l.x.Emit(fh.LiteralStringRegex, i+s, i+s+1)
			l.push(frame{kind: multilineRegex})
			return s + 1, true
		}
	}
	// numbers
	if c == '0' {
		k := i + 1
		if l.at(k) == '_' && isOct(l.at(k+1)) {
			k++
		}
		if e := l.sepDigits(k, isOct); e >= 0 { // 0_?[0-7]+(?:_[0-7]+)*
			return l.numberTail(fh.LiteralNumberOct, i, e)
		}
		if l.at(i+1) == 'x' {
			if e := l.sepDigits(i+2, isHex); e >= 0 {
				return l.numberTail(fh.LiteralNumberHex, i, e)
			}
		}
		if l.at(i+1) == 'b' {
			if e := l.sepDigits(i+2, func(c byte) bool { return c == '0' || c == '1' }); e >= 0 {
				return l.numberTail(fh.LiteralNumberBin, i, e)
			}
		}
	}
	if r, _ := l.runeAt(i); lexkit.IsDigit(r) { // [\d]+(?:[_e]\d+)*
		e := i + lexkit.Span(src, i, lexkit.IsDigit)
		for (l.at(e) == '_' || l.at(e) == 'e') && e+1 < len(src) {
			d := lexkit.Span(src, e+1, lexkit.IsDigit)
			if d == 0 {
				break
			}
			e += 1 + d
		}
		return l.numberTail(fh.LiteralNumberInteger, i, e)
	}
	// variables
	if c == '@' {
		if l.at(i+1) == '@' {
			if n := l.ident(i + 2); n > 0 {
				return l.emit(fh.NameVariableClass, i, i+2+n)
			}
		}
		if n := l.ident(i + 1); n > 0 {
			return l.emit(fh.NameVariableInstance, i, i+1+n)
		}
	}
	if c == '$' {
		if n := lexkit.Span(src, i+1, lexkit.IsWord); n > 0 { // \$\w+
			return l.emit(fh.NameVariableGlobal, i, i+1+n)
		}
		if d := l.at(i + 1); d != 0 && strings.IndexByte("!@&`'+~=/\\,;.<>_*$?:\"^-", d) >= 0 {
			return l.emit(fh.NameVariableGlobal, i, i+2)
		}
	}
	if strings.HasPrefix(src[i:], "::") {
		return l.emit(fh.Operator, i, i+2)
	}
	if n, ok := l.strings(i); ok {
		return n, true
	}
	if c == '?' {
		if n := l.charLiteral(i); n > 0 {
			return l.emit(fh.LiteralStringChar, i, i+n)
		}
	}
	if 'A' <= c && c <= 'Z' { // [A-Z]\w+
		if n := lexkit.Span(src, i+1, lexkit.IsWord); n > 0 {
			return l.emit(fh.NameConstant, i, i+1+n)
		}
	}
	if c == '.' || strings.HasPrefix(src[i:], "::") {
		k := i + 1
		if c == ':' {
			k++
		}
		if n := l.symOp(k); n > 0 { // (\.|::)(operator)
			l.x.Emit(fh.Operator, i, k)
			l.x.Emit(fh.NameOperator, k, k+n)
			return k + n - i, true
		}
		// (\.|::)([a-zA-Z_]\w*[!?]?|[*%&^`~+\-/\[<>=])
		if n := l.ident(k); n > 0 {
			l.x.Emit(fh.Operator, i, k)
			l.x.Emit(fh.Name, k, l.bang(k+n))
			return l.bang(k+n) - i, true
		}
		if d := l.at(k); d != 0 && strings.IndexByte("*%&^`~+-/[<>=", d) >= 0 {
			l.x.Emit(fh.Operator, i, k)
			l.x.Emit(fh.Name, k, k+1)
			return k + 1 - i, true
		}
	}
	if n := l.ident(i); n > 0 { // [a-zA-Z_]\w*[!?]?
		return l.emit(fh.Name, i, l.bang(i+n))
	}
	// (\[|\]|\*\*|<<?|>>?|>=|<=|<=>|=~|={3}|!~|&&?|\|\||\.{1,3})
	switch c {
	case '[', ']':
		return l.emit(fh.Operator, i, i+1)
	case '<', '>':
		if l.at(i+1) == c {
			return l.emit(fh.Operator, i, i+2)
		}
		return l.emit(fh.Operator, i, i+1)
	case '.':
		e := i + 1
		for e < i+3 && l.at(e) == '.' {
			e++
		}
		return l.emit(fh.Operator, i, e)
	}
	for _, op := range [...]string{"**", "=~", "===", "!~", "&&", "&", "||"} {
		if strings.HasPrefix(src[i:], op) {
			return l.emit(fh.Operator, i, i+len(op))
		}
	}
	if strings.IndexByte("-+/*%=<>&!^|~", c) >= 0 { // [-+/*%=<>&!^|~]=?
		e := i + 1
		if l.at(e) == '=' {
			e++
		}
		return l.emit(fh.Operator, i, e)
	}
	if strings.IndexByte("(){};,/?:\\", c) >= 0 {
		return l.emit(fh.Punctuation, i, i+1)
	}
	if n := l.spaces(i); n > 0 {
		return l.emit(fh.Text, i, i+n)
	}
	return 0, false
}

// heredoc returns the end of (?<!\w)(<<-?)(["`\']?)([a-zA-Z_]\w*)(\2)(.*?\n) and
// (<<-?)("|\')()(\2)(.*?\n) (0 if it does not match). src[i:] is <<.
func (l *lexer) heredoc(i int) int {
	src := l.src
	k := i + 2
	if l.at(k) == '-' {
		k++
	}
	lineEnd := func(p int) int {
		if n := strings.IndexByte(src[p:], '\n'); n >= 0 {
			return p + n + 1
		}
		return 0
	}
	if r, _ := l.lastRune(i); r < 0 || !lexkit.IsWord(r) {
		q := l.at(k)
		j := k
		if q == '"' || q == '`' || q == '\'' {
			j++
		} else {
			q = 0
		}
		if n := l.asciiIdent(j); n > 0 {
			e := j + n
			if q == 0 || l.at(e) == q {
				if q != 0 {
					e++
				}
				if end := lineEnd(e); end > 0 {
					return end
				}
			}
		}
	}
	// (<<-?)("|\')()(\2)(.*?\n)
	if q := l.at(k); (q == '"' || q == '\'') && l.at(k+1) == q {
		return lineEnd(k + 2)
	}
	return 0
}
