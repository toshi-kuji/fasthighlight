// Package toml registers TOML.
//
//	import _ "github.com/toshi-kuji/fasthighlight/lang/toml"
//
// Token types and boundaries reproduce the rules of Chroma 2.27.0's toml.xml without regular expressions.
package toml

import (
	"strings"

	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/lexkit"
)

// Language is TOML. Its name, aliases, and filename patterns are listed in the catalog of the main
// package (fasthighlight.Builtin).
var Language = fh.Builtin("TOML")

func init() {
	Language.Lex = lex
	fh.Register(Language)
}

// lex tries the rules of the root state in toml.xml in this order (root is the only state).
func lex(src string, end int) []fh.Token {
	x := lexkit.NewTokens(src)
	for i := 0; i < end; {
		n, t := match(src, i)
		if n == 0 {
			i += x.Error(i)
			continue
		}
		x.Emit(t, i, i+n)
		i += n
	}
	return x.Result()
}

func match(src string, i int) (int, fh.TokenType) {
	if n := lexkit.Span(src, i, lexkit.IsSpace); n > 0 { // \s+
		return n, fh.Text
	}
	c := src[i]
	if c == '#' { // #.*
		return lexkit.LineEnd(src, i) - i, fh.Comment
	}
	for _, w := range [...]string{"false", "true"} { // (false|true)\b
		if strings.HasPrefix(src[i:], w) && lexkit.WordBoundary(src, i+len(w)) {
			return len(w), fh.KeywordConstant
		}
	}
	if n := dateLen(src, i); n > 0 {
		return n, fh.LiteralDate
	}
	if j := intEnd(src, i); j > 0 {
		// [+-]?[0-9](_?\d)*\.\d+ is a decimal, [+-]?[0-9](_?\d)* is an integer
		if j < len(src) && src[j] == '.' {
			if d := lexkit.Span(src, j+1, lexkit.IsDigit); d > 0 {
				return j + 1 + d - i, fh.LiteralNumberFloat
			}
		}
		return j - i, fh.LiteralNumberInteger
	}
	switch c {
	case '"':
		if n := lexkit.QuotedLen(src, i, '"'); n > 0 {
			return n, fh.LiteralStringDouble
		}
	case '\'':
		if n := lexkit.QuotedLen(src, i, '\''); n > 0 {
			return n, fh.LiteralStringSingle
		}
	}
	if strings.IndexByte(".,=[]{}", c) >= 0 { // [.,=\[\]{}]
		return 1, fh.Punctuation
	}
	if n := lexkit.Span(src, i, isNameChar); n > 0 { // [A-Za-z0-9_-]+
		return n, fh.NameOther
	}
	return 0, 0
}

func isNameChar(r rune) bool {
	return r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_' || r == '-'
}

// intEnd returns the end position of the part matching [+-]?[0-9](_?\d)*. It returns 0 if there is no match.
func intEnd(src string, i int) int {
	j := i
	if src[j] == '+' || src[j] == '-' {
		j++
	}
	if j >= len(src) || src[j] < '0' || src[j] > '9' {
		return 0
	}
	j++
	for j < len(src) {
		k := j
		if src[k] == '_' {
			k++
		}
		if k >= len(src) {
			break
		}
		r, n := lexkit.Rune(src, k)
		if !lexkit.IsDigit(r) {
			break
		}
		j = k + n
	}
	return j
}

// dateLen returns the length matching
// \d\d\d\d-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d\+)?(Z|[+-]\d{2}:\d{2}).
// As in the Chroma rule, (\.\d\+) matches three characters: a dot, one digit, and a +.
func dateLen(src string, i int) int {
	j := i
	digits := func(n int) bool {
		for range n {
			if j >= len(src) {
				return false
			}
			r, w := lexkit.Rune(src, j)
			if !lexkit.IsDigit(r) {
				return false
			}
			j += w
		}
		return true
	}
	lit := func(c byte) bool {
		if j < len(src) && src[j] == c {
			j++
			return true
		}
		return false
	}
	if !(digits(4) && lit('-') && digits(2) && lit('-') && digits(2) && lit('T') &&
		digits(2) && lit(':') && digits(2) && lit(':') && digits(2)) {
		return 0
	}
	// (\.\d\+)? is undone if it does not match.
	if save := j; !(lit('.') && digits(1) && lit('+')) {
		j = save
	}
	switch {
	case lit('Z'):
	case lit('+') || lit('-'):
		if !(digits(2) && lit(':') && digits(2)) {
			return 0
		}
	default:
		return 0
	}
	return j - i
}
