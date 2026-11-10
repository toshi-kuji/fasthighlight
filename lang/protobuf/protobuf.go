// Package protobuf registers Protocol Buffer.
//
//	import _ "github.com/toshi-kuji/fasthighlight/lang/protobuf"
//
// Token types and boundaries reproduce the rules of Chroma 2.27.0's protocol_buffer.xml without regular
// expressions.
package protobuf

import (
	"strings"

	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/lexkit"
)

// Language is Protocol Buffer. Its name, aliases, and filename patterns are listed in the catalog of the
// main package (fasthighlight.Builtin).
var Language = fh.Builtin("Protocol Buffer")

func init() {
	Language.Lex = lex
	Language.Prepare = prepare
	fh.Register(Language)
}

// The words of the keyword rules, each followed by \b. prepare builds them, on first use.
var (
	// keywords is \b(ctype|default|…|weak)\b (Keyword).
	keywords map[string]bool
	// types is (sfixed32|…|bool)\b (KeywordType), with no \b before.
	types map[string]bool
)

func prepare() {
	keywords = set("ctype", "default", "edition", "export", "local", "max", "option", "optional", "packed",
		"public", "repeated", "required", "reserved", "returns", "stream", "syntax", "to", "weak")
	types = set("sfixed32", "sfixed64", "fixed32", "fixed64", "sint32", "sint64", "double", "string",
		"uint32", "uint64", "int32", "float", "int64", "bytes", "bool")
}

func set(words ...string) map[string]bool {
	m := make(map[string]bool, len(words))
	for _, w := range words {
		m[w] = true
	}
	return m
}

// misses remembers where the comment rules found no end. A line comment needs a newline not after a \, and
// a block comment a */ (or *\<newline>/); if there is none after a position, there is none after any later
// position either, so a later comment that starts there fails without scanning the rest of the input
// again (which would take time growing with the square of the input).
type misses struct{ line, block int }

// state is the state after a keyword that names something: package, message (and extend), or type (enum,
// group, oneof, rpc, service). Each reads one name, or nothing, and goes back to root.
type state uint8

const (
	root state = iota
	pkg
	message
	typ
)

func lex(src string, end int) []fh.Token {
	x := lexkit.NewTokens(src)
	ms := &misses{line: len(src) + 1, block: len(src) + 1}
	var words lexkit.SpanCache // the last run of word characters, so that a long run is measured once
	st := root
	for i := 0; i < end; {
		if st != root {
			// [a-zA-Z_][\w.]* (package, message) or [a-zA-Z_]\w* (type); otherwise pop without a token.
			if isIdentStart(src[i]) {
				f := isNameChar
				t := fh.NameNamespace
				switch st {
				case message:
					t = fh.NameClass
				case typ:
					f, t = lexkit.IsWord, fh.Name
				}
				n := 1 + lexkit.Span(src, i+1, f)
				x.Emit(t, i, i+n)
				i += n
			}
			st = root
			continue
		}
		n, next := match(x, ms, &words, src, i)
		if n == 0 {
			i += x.Error(i)
			continue
		}
		st = next
		i += n
	}
	return x.Result()
}

func isIdentStart(c byte) bool { return c == '_' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' }

func isNameChar(r rune) bool { return r == '.' || lexkit.IsWord(r) }

func at(src string, i int) byte {
	if i < len(src) {
		return src[i]
	}
	return 0
}

// match tries the rules of root at i, emits the tokens, and returns the length (0 if no rule matches) and
// the state to push.
func match(x *lexkit.Tokens, ms *misses, words *lexkit.SpanCache, src string, i int) (int, state) {
	emit := func(t fh.TokenType, n int) (int, state) { x.Emit(t, i, i+n); return n, root }
	c := src[i]
	if n := lexkit.Span(src, i, lexkit.IsSpace); n > 0 { // \s+
		return emit(fh.Text, n)
	}
	if strings.IndexByte(",;{}[]()<>", c) >= 0 { // [,;{}\[\]()<>]
		return emit(fh.Punctuation, 1)
	}
	if c == '/' {
		if n, t := comment(ms, src, i); n > 0 {
			return emit(t, n)
		}
	}
	// A run longer than the longest word ("extensions") is none of them, and is not hashed.
	if w := words.Span(src, i, lexkit.IsWord); 0 < w && w <= len("extensions") {
		word := src[i : i+w]
		if lexkit.WordBoundary(src, i) {
			if keywords[word] { // \b(ctype|…)\b
				return emit(fh.Keyword, w)
			}
			if word == "extensions" || word == "map" { // \b(extensions|map)\b
				return emit(fh.KeywordDeclaration, w)
			}
		}
		if types[word] { // (sfixed32|…)\b
			return emit(fh.KeywordType, w)
		}
		if word == "true" || word == "false" { // (true|false)\b
			return emit(fh.KeywordConstant, w)
		}
	}
	for _, k := range [...]struct {
		word string
		t    fh.TokenType
		next state
	}{
		{"package", fh.KeywordNamespace, pkg},       // (package)(\s+)
		{"import", fh.KeywordNamespace, root},       // import\b
		{"message", fh.KeywordDeclaration, message}, // (message|extend)(\s+)
		{"extend", fh.KeywordDeclaration, message},
		{"enum", fh.KeywordDeclaration, typ}, // (enum|group|oneof|rpc|service)(\s+)
		{"group", fh.KeywordDeclaration, typ},
		{"oneof", fh.KeywordDeclaration, typ},
		{"rpc", fh.KeywordDeclaration, typ},
		{"service", fh.KeywordDeclaration, typ},
	} {
		if !strings.HasPrefix(src[i:], k.word) {
			continue
		}
		e := i + len(k.word)
		if k.word == "import" {
			if lexkit.WordBoundary(src, e) {
				return emit(k.t, len(k.word))
			}
			continue
		}
		if s := lexkit.Span(src, e, lexkit.IsSpace); s > 0 {
			x.Emit(k.t, i, e)
			x.Emit(fh.Text, e, e+s)
			return e + s - i, k.next
		}
	}
	if c == '"' || c == '\'' { // \".*?\" and \'.*?\'
		for j := i + 1; j < len(src) && src[j] != '\n'; j++ {
			if src[j] == c {
				return emit(fh.LiteralString, j+1-i)
			}
		}
	}
	if n, t := number(src, i); n > 0 {
		return emit(t, n)
	}
	if '+' <= c && c <= '=' { // [+-=] is the range from + to =
		return emit(fh.Operator, 1)
	}
	if isIdentStart(c) {
		n := 1 + lexkit.Span(src, i+1, isNameChar)
		// ([a-zA-Z_][\w.]*)([ \t]*)(=): both runs are maximal, since neither can end before the =.
		j := i + n
		for at(src, j) == ' ' || at(src, j) == '\t' {
			j++
		}
		if at(src, j) == '=' {
			x.Emit(fh.Name, i, i+n)
			x.Emit(fh.Text, i+n, j)
			x.Emit(fh.Operator, j, j+1)
			return j + 1 - i, root
		}
		return emit(fh.Name, n) // [a-zA-Z_][\w.]*
	}
	return 0, root
}

// comment returns the length and type of /(\\\n)?/(\n|(.|\n)*?[^\\]\n) (CommentSingle) or
// /(\\\n)?\*(.|\n)*?\*(\\\n)?/ (CommentMultiline) at the / at i, or 0. If the optional \<newline> after the
// first / leads to no match, giving it back does not help: the / or * would have to match the \.
func comment(ms *misses, src string, i int) (int, fh.TokenType) {
	j := i + 1
	if strings.HasPrefix(src[j:], "\\\n") {
		j += 2
	}
	switch at(src, j) {
	case '/':
		// A line comment ends at the first newline after its start that does not follow a \ (a newline
		// right after the // ends it too). Without such a newline, it does not match.
		s := j + 1
		if at(src, s) == '\n' {
			return s + 1 - i, fh.CommentSingle
		}
		if s+1 >= ms.line {
			return 0, 0
		}
		for k := s + 1; k < len(src); k++ {
			if src[k] == '\n' && src[k-1] != '\\' {
				return k + 1 - i, fh.CommentSingle
			}
		}
		ms.line = s + 1
	case '*':
		if j+1 >= ms.block {
			return 0, 0
		}
		for p := j + 1; p < len(src); p++ {
			if src[p] != '*' {
				continue
			}
			if at(src, p+1) == '/' {
				return p + 2 - i, fh.CommentMultiline
			}
			if strings.HasPrefix(src[p+1:], "\\\n/") {
				return p + 4 - i, fh.CommentMultiline
			}
		}
		ms.block = j + 1
	}
	return 0, 0
}

func digitsLen(src string, i int) int { return lexkit.Span(src, i, lexkit.IsDigit) }

// suffixLen returns the length of [LlUu]* at i.
func suffixLen(src string, i int) int {
	j := i
	for j < len(src) && strings.IndexByte("LlUu", src[j]) >= 0 {
		j++
	}
	return j - i
}

func isHex(c byte) bool {
	return '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F'
}

// number returns the length and type of the number rules at i, tried in order (0 if none matches). Every
// \d+ and \d* is maximal: a shorter run is followed by a digit, which no later part accepts.
func number(src string, i int) (int, fh.TokenType) {
	d := digitsLen(src, i)
	// The ends of the mantissas \d+\.\d* and \.\d+ (0 when they do not match).
	var dot, frac int
	if d > 0 && at(src, i+d) == '.' {
		dot = i + d + 1 + digitsLen(src, i+d+1)
	}
	if d == 0 && at(src, i) == '.' {
		if f := digitsLen(src, i+1); f > 0 {
			frac = i + 1 + f
		}
	}
	// (\d+\.\d*|\.\d+|\d+)[eE][+-]?\d+[LlUu]*
	plain := 0
	if d > 0 {
		plain = i + d
	}
	for _, q := range [...]int{dot, frac, plain} {
		if q == 0 {
			continue
		}
		if c := at(src, q); c == 'e' || c == 'E' {
			k := q + 1
			if c := at(src, k); c == '+' || c == '-' {
				k++
			}
			if n := digitsLen(src, k); n > 0 {
				return k + n + suffixLen(src, k+n) - i, fh.LiteralNumberFloat
			}
		}
	}
	// (\d+\.\d*|\.\d+|\d+[fF])[fF]?
	q := dot
	if q == 0 {
		q = frac
	}
	if q == 0 && d > 0 && (at(src, i+d) == 'f' || at(src, i+d) == 'F') {
		q = i + d + 1
	}
	if q > 0 {
		if c := at(src, q); c == 'f' || c == 'F' {
			q++
		}
		return q - i, fh.LiteralNumberFloat
	}
	// (\-?(inf|nan))\b
	j := i
	if at(src, j) == '-' {
		j++
	}
	if (strings.HasPrefix(src[j:], "inf") || strings.HasPrefix(src[j:], "nan")) && lexkit.WordBoundary(src, j+3) {
		return j + 3 - i, fh.LiteralNumberFloat
	}
	if strings.HasPrefix(src[i:], "0x") && isHex(at(src, i+2)) { // 0x[0-9a-fA-F]+[LlUu]*
		k := i + 2
		for isHex(at(src, k)) {
			k++
		}
		return k + suffixLen(src, k) - i, fh.LiteralNumberHex
	}
	if at(src, i) == '0' && '0' <= at(src, i+1) && at(src, i+1) <= '7' { // 0[0-7]+[LlUu]*
		k := i + 1
		for '0' <= at(src, k) && at(src, k) <= '7' {
			k++
		}
		return k + suffixLen(src, k) - i, fh.LiteralNumberOct
	}
	if d > 0 { // \d+[LlUu]*
		return d + suffixLen(src, i+d), fh.LiteralNumberInteger
	}
	return 0, 0
}
