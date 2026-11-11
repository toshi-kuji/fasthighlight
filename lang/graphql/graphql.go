// Package graphql registers GraphQL.
//
//	import _ "github.com/toshi-kuji/fasthighlight/lang/graphql"
//
// Token types and boundaries reproduce the rules of Chroma 2.27.0's graphql.xml without regular expressions.
package graphql

import (
	"strings"

	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/lexkit"
)

// Language is GraphQL. Its name, aliases, and filename patterns are listed in the catalog of the main
// package (fasthighlight.Builtin).
var Language = fh.Builtin("GraphQL")

func init() {
	Language.Lex = lex
	Language.Prepare = prepare
	fh.Register(Language)
}

type state uint8

const (
	root state = iota
	typ
)

// declarations are the words of (query|mutation|…|type), which push the type state. They have no \b, so
// they also match the start of a longer word.
var declarations = [...]string{
	"query", "mutation", "subscription", "fragment", "scalar", "implements", "interface", "union", "enum", "input", "type",
}

// keywords are the words of (on|extend|schema|directive|\.\.\.), also without \b.
var keywords = [...]string{"on", "extend", "schema", "directive", "..."}

// constants are the words of (QUERY|MUTATION|…)\b (directive locations). prepare builds the set from
// constantWords, on first use.
var constants map[string]bool

var constantWords = [...]string{
	"QUERY", "MUTATION", "SUBSCRIPTION", "FIELD", "FRAGMENT_DEFINITION", "FRAGMENT_SPREAD", "INLINE_FRAGMENT",
	"SCHEMA", "SCALAR", "OBJECT", "FIELD_DEFINITION", "ARGUMENT_DEFINITION", "INTERFACE", "UNION", "ENUM",
	"ENUM_VALUE", "INPUT_OBJECT", "INPUT_FIELD_DEFINITION",
}

// wordStart marks the first bytes of declarations and keywords: at any other byte none of them can match,
// so they are not compared one by one.
var wordStart [256]bool

func prepare() {
	for _, w := range declarations {
		wordStart[w[0]] = true
	}
	for _, w := range keywords {
		wordStart[w[0]] = true
	}
	constants = make(map[string]bool, len(constantWords))
	for _, w := range constantWords {
		constants[w] = true
	}
}

func lex(src string, end int) []fh.Token {
	x := lexkit.NewTokens(src)
	stack := []state{root}
	for i := 0; i < end; {
		st := stack[len(stack)-1]
		n, t, push, pop := match(src, i, st)
		if n == 0 {
			if src[i] == '\n' && st != root {
				stack = append(stack[:0], root)
				continue
			}
			i += x.Error(i)
			continue
		}
		x.Emit(t, i, i+n)
		if pop {
			stack = stack[:len(stack)-1]
		}
		if push {
			stack = append(stack, typ)
		}
		i += n
	}
	return x.Result()
}

// match tries the rules of state st at i. It returns the length (0 if no rule matches), the token type,
// and whether to push the type state or pop.
func match(src string, i int, st state) (n int, t fh.TokenType, push, pop bool) {
	c := src[i]
	if st == typ {
		if strings.IndexByte("{}()[]", c) >= 0 { // [\{\}\(\)\[\]]
			return 1, fh.Punctuation, false, true
		}
		if n := lexkit.Span(src, i, lexkit.IsWord); n > 0 { // [^\W\d]\w* ([^\W\d] is the same as \w in regexp2)
			return n, fh.NameClass, false, true
		}
	}
	if wordStart[c] {
		for _, w := range declarations {
			if strings.HasPrefix(src[i:], w) {
				return len(w), fh.KeywordDeclaration, true, false
			}
		}
		for _, w := range keywords {
			if strings.HasPrefix(src[i:], w) {
				return len(w), fh.KeywordDeclaration, false, false
			}
		}
	}
	if n := lexkit.Span(src, i, lexkit.IsWord); n > 0 {
		if constants[src[i:i+n]] { // (QUERY|…)\b
			return n, fh.KeywordConstant, false, false
		}
		// [^\W\d]\w*. Because it is the same as \w+, it also takes every word that starts with a digit, so
		// the number rules that start with \d never match.
		return n, fh.NameProperty, false, false
	}
	switch c {
	case '@': // \@\w+
		if n := lexkit.Span(src, i+1, lexkit.IsWord); n > 0 {
			return 1 + n, fh.NameDecorator, false, false
		}
	case ':':
		return 1, fh.Punctuation, true, false
	case '$': // \$\w+
		if n := lexkit.Span(src, i+1, lexkit.IsWord); n > 0 {
			return 1 + n, fh.NameVariable, false, false
		}
	case '.':
		if n := lexkit.Span(src, i+1, lexkit.IsDigit); n > 0 {
			j := i + 1 + n
			// \.\d+([Ee][-+]\d+)?i: every \d+ is maximal, because a digit cannot stand for the i or [Ee].
			e := expEnd(src, j, false)
			if e > 0 && e < len(src) && src[e] == 'i' {
				return e + 1 - i, fh.LiteralNumber, false, false
			}
			if j < len(src) && src[j] == 'i' {
				return j + 1 - i, fh.LiteralNumber, false, false
			}
			// \.\d+([eE][+\-]?\d+)?
			if e := expEnd(src, j, true); e > 0 {
				return e - i, fh.LiteralNumberFloat, false, false
			}
			return j - i, fh.LiteralNumberFloat, false, false
		}
	case '"':
		if strings.HasPrefix(src[i:], `"""`) { // """[\x00-\x7F]*?"""
			for j := i + 3; j < len(src) && src[j] < 0x80; j++ {
				if strings.HasPrefix(src[j:], `"""`) {
					return j + 3 - i, fh.LiteralString, false, false
				}
			}
		}
		if n := charLen(src, i); n > 0 {
			return n, fh.LiteralStringChar, false, false
		}
		// "(\\\\|\\"|[^"])*". The rule "(true|false|null)*" after it never matches: it needs a closing ",
		// and with one this rule matches.
		if n := lexkit.QuotedLen(src, i, '"'); n > 0 {
			return n, fh.LiteralString, false, false
		}
	case '#': // #[^\r\n]*
		j := i
		for j < len(src) && src[j] != '\n' && src[j] != '\r' {
			j++
		}
		return j - i, fh.Comment, false, false
	}
	if strings.IndexByte("(){}[],!|=", c) >= 0 { // [\(\)\{\}\[\],!\|=]
		return 1, fh.Punctuation, false, false
	}
	if n := lexkit.Span(src, i, lexkit.IsSpace); n > 0 { // [\r\n\s]+
		return n, fh.TextWhitespace, false, false
	}
	return 0, 0, false, false
}

// expEnd returns the end of [Ee][-+]\d+ (or [eE][+\-]?\d+ if optSign is set) at j, or 0.
func expEnd(src string, j int, optSign bool) int {
	if j >= len(src) || src[j] != 'e' && src[j] != 'E' {
		return 0
	}
	k := j + 1
	if k < len(src) && (src[k] == '+' || src[k] == '-') {
		k++
	} else if !optSign {
		return 0
	}
	n := lexkit.Span(src, k, lexkit.IsDigit)
	if n == 0 {
		return 0
	}
	return k + n
}

func isHex(c byte) bool {
	return '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F'
}

// charLen returns the length of
// "(\\["\\abfnrtv]|\\x[0-9a-fA-F]{2}|\\[0-7]{1,3}|\\u[0-9a-fA-F]{4}|\\U[0-9a-fA-F]{8}|[^\\])" at i, or 0.
func charLen(src string, i int) int {
	at := func(k int) byte {
		if k < len(src) {
			return src[k]
		}
		return 0
	}
	closes := func(k int) int {
		if at(k) == '"' {
			return k + 1 - i
		}
		return 0
	}
	hex := func(k, n int) bool {
		for d := range n {
			if !isHex(at(k + d)) {
				return false
			}
		}
		return true
	}
	j := i + 1
	if j >= len(src) {
		return 0
	}
	if src[j] != '\\' {
		_, w := lexkit.Rune(src, j) // [^\\]
		return closes(j + w)
	}
	d := at(j + 1)
	if d != 0 && strings.IndexByte("\"\\abfnrtv", d) >= 0 {
		if n := closes(j + 2); n > 0 {
			return n
		}
	}
	if d == 'x' && hex(j+2, 2) {
		if n := closes(j + 4); n > 0 {
			return n
		}
	}
	if '0' <= d && d <= '7' {
		// [0-7]{1,3}: a shorter run is followed by an octal digit, not by ".
		k := j + 1
		for k < j+4 && '0' <= at(k) && at(k) <= '7' {
			k++
		}
		if n := closes(k); n > 0 {
			return n
		}
	}
	if d == 'u' && hex(j+2, 4) {
		if n := closes(j + 6); n > 0 {
			return n
		}
	}
	if d == 'U' && hex(j+2, 8) {
		if n := closes(j + 10); n > 0 {
			return n
		}
	}
	return 0
}
