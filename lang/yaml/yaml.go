// Package yaml registers YAML.
//
//	import _ "github.com/toshi-kuji/fasthighlight/lang/yaml"
//
// Token types and boundaries reproduce the rules of Chroma 2.27.0's yaml.xml without regular expressions.
// The only state is root, and the whitespace, key, and value rules are included in root (include).
package yaml

import (
	"strings"
	"unicode/utf8"

	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/lexkit"
)

// Language is YAML. Its name, aliases, and filename patterns are listed in the catalog of the main
// package (fasthighlight.Builtin).
var Language = fh.Builtin("YAML")

func init() {
	Language.Lex = lex
	fh.Register(Language)
}

func lex(src string, end int) []fh.Token {
	x := lexkit.NewTokens(src)
	var kc colonCache
	for i := 0; i < end; {
		i = step(x, &kc, src, i)
	}
	return x.Result()
}

// bol returns true if i is at the start of a line ((?m)^ in a regular expression).
func bol(src string, i int) bool { return i == 0 || src[i-1] == '\n' }

// step tries the root rules in order at position i, appends the token of the matching rule, and
// returns the next position.
func step(x *lexkit.Tokens, kc *colonCache, src string, i int) int {
	emit := func(t fh.TokenType, e int) int { x.Emit(t, i, e); return e }

	// whitespace: \s+
	if n := lexkit.Span(src, i, lexkit.IsSpace); n > 0 {
		return emit(fh.TextWhitespace, i+n)
	}
	// ^--- and ^\.\.\.
	if bol(src, i) && (strings.HasPrefix(src[i:], "---") || strings.HasPrefix(src[i:], "...")) {
		return emit(fh.NameNamespace, i+3)
	}
	// [\n?]?\s*- (here src[i] is not whitespace)
	if src[i] == '?' {
		if k := i + 1 + lexkit.Span(src, i+1, lexkit.IsSpace); strings.HasPrefix(src[k:], "- ") {
			return emit(fh.Text, k+2)
		}
	}
	if strings.HasPrefix(src[i:], "- ") {
		return emit(fh.Text, i+2)
	}
	// #.*$ and //.*$
	if src[i] == '#' || strings.HasPrefix(src[i:], "//") {
		return emit(fh.Comment, lexkit.LineEnd(src, i))
	}
	// !![^\s]+, &[^\s]+, \*[^\s]+
	for _, p := range [...]string{"!!", "&", "*"} {
		if strings.HasPrefix(src[i:], p) {
			if n := lexkit.Span(src, i+len(p), notSpace); n > 0 {
				return emit(fh.CommentPreproc, i+len(p)+n)
			}
		}
	}
	// ^%include\s+[^\n\r]+
	if bol(src, i) && strings.HasPrefix(src[i:], "%include") {
		if e := includeEnd(src, i+len("%include")); e > 0 {
			return emit(fh.CommentPreproc, e)
		}
	}
	if e, ok := key(x, kc, src, i); ok {
		return e
	}
	if e, ok := value(x, src, i); ok {
		return e
	}
	// [?:,\[\]]
	if strings.IndexByte("?:,[]", src[i]) >= 0 {
		return emit(fh.Punctuation, i+1)
	}
	// . (one character other than a newline; a newline matches whitespace, so it never gets here)
	_, n := lexkit.Rune(src, i)
	return emit(fh.Text, i+n)
}

func notSpace(r rune) bool { return !lexkit.IsSpace(r) }

// includeEnd returns the end of the part after %include that matches \s+[^\n\r]+. It returns 0 if
// there is no match. \s+ takes all whitespace including newlines; if no character follows, it
// backtracks and starts [^\n\r]+ from whitespace other than a newline.
func includeEnd(src string, j int) int {
	w := lexkit.Span(src, j, lexkit.IsSpace)
	if w == 0 {
		return 0
	}
	untilCR := func(k int) int {
		for k < len(src) && src[k] != '\n' && src[k] != '\r' {
			k++
		}
		return k
	}
	if j+w < len(src) { // after the whitespace there is a non-whitespace character (not \n or \r)
		return untilCR(j + w)
	}
	// whitespace up to the end of the document: shorten \s+ one character at a time and look for a
	// character that matches [^\n\r]
	for k := j + w; k > j; {
		_, n := utf8.DecodeLastRuneInString(src[j:k])
		k -= n
		if k > j && src[k] != '\n' && src[k] != '\r' {
			return untilCR(k)
		}
	}
	return 0
}

// key tries the rules of the key state in yaml.xml.
func key(x *lexkit.Tokens, kc *colonCache, src string, i int) (int, bool) {
	// "[^"\n#].*": (within a line, up to the last ": ")
	if src[i] == '"' && i+1 < len(src) && strings.IndexByte("\"\n#", src[i+1]) < 0 {
		_, w := lexkit.Rune(src, i+1)
		start, le := i+1+w, lexkit.LineEnd(src, i)
		if start <= le {
			if p := strings.LastIndex(src[start:le], "\": "); p >= 0 {
				e := start + p + 3
				x.Emit(fh.NameTag, i, e)
				return e, true
			}
		}
	}
	// (-)( )((?:(?!//)[^"\n{#])*?)(:)( ) never gets here, because the earlier [\n?]?\s*- always matches first.
	// ((?:(?!//)[^"\n{#])*?)(:)( ) and ((?:(?!//)[^"\n{#])*?)(:)(\n)
	for a, after := range [...]byte{' ', '\n'} {
		if j := kc[a].find(src, i, after); j >= 0 {
			x.Emit(fh.NameTag, i, j)
			x.Emit(fh.Punctuation, j, j+1)
			x.Emit(fh.TextWhitespace, j+1, j+2)
			return j + 2, true
		}
	}
	return 0, false
}

// colonCache remembers the last keyColon scan for each of the two characters after the colon.
type colonCache [2]colonScan

// colonScan is one keyColon scan: it started at from and looked at every position up to upto, where
// it found the colon (res >= 0) or stopped (res == -1). Every start in [from, upto] gives the same
// result, because keyColon only stops at ASCII bytes, which never occur inside a multi-byte rune.
// Remembering it keeps a long line from being scanned again at every position.
type colonScan struct{ from, upto, res int }

// find returns the first result of keyColon(src, i, after), from the remembered scan when i is inside it.
func (c *colonScan) find(src string, i int, after byte) int {
	if c.upto > 0 && c.from <= i && i <= c.upto {
		return c.res
	}
	res, upto := keyColon(src, i, after)
	*c = colonScan{from: i, upto: upto, res: res}
	return res
}

// keyColon returns the position of the first ":" that follows (?:(?!//)[^"\n{#])*? from i and is
// followed by after, or -1 if there is none, and the position where the scan ended.
func keyColon(src string, i int, after byte) (int, int) {
	for j := i; j < len(src); {
		if src[j] == ':' && j+1 < len(src) && src[j+1] == after {
			return j, j
		}
		c := src[j]
		if c == '"' || c == '\n' || c == '{' || c == '#' || strings.HasPrefix(src[j:], "//") {
			return -1, j
		}
		_, n := lexkit.Rune(src, j)
		j += n
	}
	return -1, len(src)
}

// constants lists the candidates of (false|False|…|Y|y)\b in the same order as the regular
// expression.
var constants = [...]string{
	"false", "False", "FALSE", "true", "True", "TRUE", "null", "Off", "off", "yes", "Yes", "YES",
	"OFF", "On", "ON", "no", "No", "on", "NO", "n", "N", "Y", "y",
}

// literalExcluded holds the characters excluded by [^\{\}\[\]\?,\:\!\-\*&\@].
const literalExcluded = "{}[]?,:!-*&@"

// value tries the rules of the value state in yaml.xml.
func value(x *lexkit.Tokens, src string, i int) (int, bool) {
	c := src[i]
	// ([>|](?:[+-])?)(\n(^ {1,})(?:(?:.*\n*(?:^\3 *).*)+|.*))
	if c == '>' || c == '|' {
		if j, e := blockScalar(src, i); e > 0 {
			x.Emit(fh.Punctuation, i, j)
			x.Emit(fh.LiteralStringDoc, j, e)
			return e, true
		}
	}
	for _, w := range constants {
		if strings.HasPrefix(src[i:], w) && lexkit.WordBoundary(src, i+len(w)) {
			x.Emit(fh.KeywordConstant, i, i+len(w))
			return i + len(w), true
		}
	}
	// "(?:\\.|[^"])*" and '(?:\\.|[^'])*'
	if c == '"' || c == '\'' {
		if n := quotedLen(src, i, c); n > 0 {
			t := fh.LiteralStringDouble
			if c == '\'' {
				t = fh.LiteralStringSingle
			}
			x.Emit(t, i, i+n)
			return i + n, true
		}
	}
	if n := dateLen(src, i); n > 0 {
		x.Emit(fh.LiteralDate, i, i+n)
		return i + n, true
	}
	if n := numberLen(src, i); n > 0 {
		x.Emit(fh.LiteralNumber, i, i+n)
		return i + n, true
	}
	// ([^…].*)( )+(#.*), ([^…].*)( )+(//.*), [^…].*
	r, w := lexkit.Rune(src, i)
	if r == '\n' || strings.ContainsRune(literalExcluded, r) {
		return 0, false
	}
	le := lexkit.LineEnd(src, i)
	for _, mark := range [...]string{" #", " //"} {
		if p := strings.LastIndex(src[i+w:le], mark); p >= 0 {
			j := i + w + p
			x.Emit(fh.Literal, i, j)
			x.Emit(fh.TextWhitespace, j, j+1)
			x.Emit(fh.Comment, j+1, le)
			return le, true
		}
	}
	x.Emit(fh.Literal, i, le)
	return le, true
}

// blockScalar returns the end j of the indicator and the end e of the whole thing if src matches a
// block string (one starting with | or >). If there is no match, e is 0.
//
// When the indicator is followed directly by a newline and the next line starts with one or more
// spaces, it takes in following lines for as long as they start with that same indentation (blank
// lines in between are included). The newline of the last line is not included.
func blockScalar(src string, i int) (j, e int) {
	j = i + 1
	if j < len(src) && (src[j] == '+' || src[j] == '-') {
		j++
	}
	if j >= len(src) || src[j] != '\n' {
		return j, 0
	}
	k := j + 1
	indent := 0
	for k+indent < len(src) && src[k+indent] == ' ' {
		indent++
	}
	if indent == 0 {
		return j, 0
	}
	prefix := src[k : k+indent]
	e = lexkit.LineEnd(src, k+indent) // first line
	for {
		q := e
		for q < len(src) && src[q] == '\n' {
			q++
		}
		if q == e || !strings.HasPrefix(src[q:], prefix) {
			return j, e
		}
		e = lexkit.LineEnd(src, q)
	}
}

// quotedLen returns the length matching q(?:\\.|[^q])*q (. excludes a newline). It returns 0 if
// there is no match. It closes at a q that has no backslash before it. If there is no such q, it
// closes at the last q after a backslash, because of regular-expression backtracking.
func quotedLen(src string, i int, q byte) int {
	last := -1
	for j := i + 1; j < len(src); {
		switch c := src[j]; {
		case c == '\\' && j+1 < len(src) && src[j+1] != '\n':
			if src[j+1] == q {
				last = j + 1
			}
			_, n := lexkit.Rune(src, j+1)
			j += 1 + n
		case c == q:
			return j + 1 - i
		default:
			j++
		}
	}
	if last >= 0 {
		return last + 1 - i
	}
	return 0
}

// digits returns the end position of the run of \d (Unicode digits) at the start of src[j:].
func digits(src string, j int) int { return j + lexkit.Span(src, j, lexkit.IsDigit) }

// nDigits returns the end position if src[j:] starts with exactly n \d. It returns -1 otherwise.
func nDigits(src string, j, n int) int {
	for range n {
		if j >= len(src) {
			return -1
		}
		r, w := lexkit.Rune(src, j)
		if !lexkit.IsDigit(r) {
			return -1
		}
		j += w
	}
	return j
}

// dateLen returns the length matching \d\d\d\d-\d\d-\d\d([T ]\d\d:\d\d:\d\d(\.\d+)?(Z|\s+[-+]\d+)?)?.
func dateLen(src string, i int) int {
	j := nDigits(src, i, 4)
	for _, n := range [...]int{2, 2} {
		if j < 0 || j >= len(src) || src[j] != '-' {
			return 0
		}
		j = nDigits(src, j+1, n)
	}
	if j < 0 {
		return 0
	}
	// time (if it does not match, the date only)
	t := -1
	if j < len(src) && (src[j] == 'T' || src[j] == ' ') {
		t = nDigits(src, j+1, 2)
		for range 2 {
			if t < 0 || t >= len(src) || src[t] != ':' {
				t = -1
				break
			}
			t = nDigits(src, t+1, 2)
		}
	}
	if t < 0 {
		return j - i
	}
	if t < len(src) && src[t] == '.' {
		if d := digits(src, t+1); d > t+1 {
			t = d
		}
	}
	switch {
	case t < len(src) && src[t] == 'Z':
		t++
	default:
		if s := t + lexkit.Span(src, t, lexkit.IsSpace); s > t && s < len(src) && (src[s] == '-' || src[s] == '+') {
			if d := digits(src, s+1); d > s+1 {
				t = d
			}
		}
	}
	return t - i
}

// numberLen returns the length matching
// \b[+\-]?(0x[\da-f]+|0o[0-7]+|(\d+\.?\d*|\.?\d+)(e[\+\-]?\d+)?|\.inf|\.nan)\b.
// The final \b requires backtracking, so the candidates are tried in the same order as in the
// regular expression (continuation passing).
func numberLen(src string, i int) int {
	if !lexkit.WordBoundary(src, i) {
		return 0
	}
	res := -1
	final := func(e int) bool {
		if lexkit.WordBoundary(src, e) {
			res = e
			return true
		}
		return false
	}
	// expo tries (e[\+\-]?\d+)? and then passes to final.
	expo := func(e int) bool {
		if e < len(src) && src[e] == 'e' {
			q := e + 1
			if q < len(src) && (src[q] == '+' || src[q] == '-') && lexkit.Run(src, q+1, 1, lexkit.IsDigit, final) {
				return true
			}
			if lexkit.Run(src, q, 1, lexkit.IsDigit, final) {
				return true
			}
		}
		return final(e)
	}
	body := func(p int) bool {
		if strings.HasPrefix(src[p:], "0x") && lexkit.Run(src, p+2, 1, isHexDigit, final) {
			return true
		}
		if strings.HasPrefix(src[p:], "0o") && lexkit.Run(src, p+2, 1, isOctal, final) {
			return true
		}
		// \d+\.?\d*
		if lexkit.Run(src, p, 1, lexkit.IsDigit, func(a int) bool {
			if a < len(src) && src[a] == '.' && lexkit.Run(src, a+1, 0, lexkit.IsDigit, expo) {
				return true
			}
			return lexkit.Run(src, a, 0, lexkit.IsDigit, expo)
		}) {
			return true
		}
		// \.?\d+
		if p < len(src) && src[p] == '.' && lexkit.Run(src, p+1, 1, lexkit.IsDigit, expo) {
			return true
		}
		if lexkit.Run(src, p, 1, lexkit.IsDigit, expo) {
			return true
		}
		for _, w := range [...]string{".inf", ".nan"} {
			if strings.HasPrefix(src[p:], w) && final(p+len(w)) {
				return true
			}
		}
		return false
	}
	if (src[i] == '+' || src[i] == '-') && body(i+1) {
		return res - i
	}
	if body(i) {
		return res - i
	}
	return 0
}

func isHexDigit(r rune) bool { return lexkit.IsDigit(r) || r >= 'a' && r <= 'f' }
func isOctal(r rune) bool    { return r >= '0' && r <= '7' }
