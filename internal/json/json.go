// Package json is the JSON tokenizer. It is used by lang/json (JSON) and by the languages that embed JSON
// (Docker, and the body of HTTP messages in internal/http).
//
// Token types and boundaries reproduce the rules of Chroma 2.27.0's json.xml without regular expressions.
package json

import (
	"strings"

	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/lexkit"
)

// The states of json.xml in Chroma.
type state int

const (
	root            state = iota
	objectValue           // inside {, waiting for a key
	objectAttribute       // after a key, waiting for a value and a separator
	arrayValue            // inside [
)

// Lex splits src into tokens (fasthighlight.Language.Lex).
func Lex(src string, end int) []fh.Token {
	x := lexkit.NewTokens(src)
	LexInto(x, src, end)
	return x.Result()
}

// LexInto splits src into tokens and adds them to x. x's src is either the same as src or was made with
// x.Sub for src (JSON embedded in another language, as with Chroma's using).
func LexInto(x *lexkit.Tokens, src string, end int) {
	stack := []state{root}
	for i := 0; i < end && len(stack) > 0; {
		st := stack[len(stack)-1]
		n, t := 0, fh.TokenType(0)
		push, pop := state(-1), 0
		c := src[i]
		switch st {
		case root:
			n, t, push = value(src, i)
		case objectValue:
			// whitespace, comment, key string, }
			switch {
			case lexkit.Span(src, i, lexkit.IsSpace) > 0:
				n, t = lexkit.Span(src, i, lexkit.IsSpace), fh.Text
			case commentLen(src, i) > 0:
				n, t = commentLen(src, i), fh.CommentSingle
			case c == '"' && stringLen(src, i) > 0:
				n, t, push = stringLen(src, i), fh.NameTag, objectAttribute
			case c == '}':
				n, t, pop = 1, fh.Punctuation, 1
			}
		case objectAttribute:
			if n, t, push = value(src, i); n == 0 {
				switch c {
				case ':':
					n, t = 1, fh.Punctuation
				case ',':
					n, t, pop = 1, fh.Punctuation, 1
				case '}':
					n, t, pop = 1, fh.Punctuation, 2
				}
			}
		case arrayValue:
			if n, t, push = value(src, i); n == 0 {
				switch c {
				case ',':
					n, t = 1, fh.Punctuation
				case ']':
					n, t, pop = 1, fh.Punctuation, 1
				}
			}
		}
		if n == 0 {
			// No rule matches. In a state other than root, Chroma returns to root on reaching a newline.
			if c == '\n' && st != root {
				stack = stack[:1]
				stack[0] = root
				continue
			}
			i += x.Error(i)
			continue
		}
		x.Emit(t, i, i+n)
		i += n
		if push >= 0 {
			stack = append(stack, push)
		}
		if pop > 0 {
			stack = stack[:max(0, len(stack)-pop)]
		}
	}
}

// value tries the rules of the value state in json.xml (whitespace, simplevalue, comment, {, [) in
// this order. If none matches, n is 0. push is the state to enter (-1 if none).
func value(src string, i int) (n int, t fh.TokenType, push state) {
	push = -1
	if n = lexkit.Span(src, i, lexkit.IsSpace); n > 0 {
		return n, fh.Text, push
	}
	if n, t = simpleValue(src, i); n > 0 {
		return n, t, push
	}
	if n = commentLen(src, i); n > 0 {
		return n, fh.CommentSingle, push
	}
	switch src[i] {
	case '{':
		return 1, fh.Punctuation, objectValue
	case '[':
		return 1, fh.Punctuation, arrayValue
	}
	return 0, 0, push
}

// simpleValue tries (true|false|null)\b, a decimal, an integer, and a string, in this order.
func simpleValue(src string, i int) (int, fh.TokenType) {
	for _, w := range [...]string{"true", "false", "null"} {
		if strings.HasPrefix(src[i:], w) && lexkit.WordBoundary(src, i+len(w)) {
			return len(w), fh.KeywordConstant
		}
	}
	if j := intEnd(src, i); j > 0 {
		// -?(0|[1-9]\d*)(\.\d+[eE](\+|-)?\d+|[eE](\+|-)?\d+|\.\d+)
		if src[j:] != "" && src[j] == '.' {
			if d := lexkit.Span(src, j+1, lexkit.IsDigit); d > 0 {
				k := j + 1 + d
				if e := expLen(src, k); e > 0 {
					return k + e - i, fh.LiteralNumberFloat
				}
				return k - i, fh.LiteralNumberFloat
			}
		} else if e := expLen(src, j); e > 0 {
			return j + e - i, fh.LiteralNumberFloat
		}
		return j - i, fh.LiteralNumberInteger
	}
	if src[i] == '"' {
		if n := stringLen(src, i); n > 0 {
			return n, fh.LiteralStringDouble
		}
	}
	return 0, 0
}

// intEnd returns the end position of the part matching -?(0|[1-9]\d*). It returns 0 if there is no match.
func intEnd(src string, i int) int {
	j := i
	if src[j] == '-' {
		j++
	}
	if j >= len(src) {
		return 0
	}
	switch c := src[j]; {
	case c == '0':
		return j + 1
	case c >= '1' && c <= '9':
		return j + 1 + lexkit.Span(src, j+1, lexkit.IsDigit)
	}
	return 0
}

// expLen returns the length matching [eE](\+|-)?\d+. It returns 0 if there is no match.
func expLen(src string, i int) int {
	if i >= len(src) || (src[i] != 'e' && src[i] != 'E') {
		return 0
	}
	j := i + 1
	if j < len(src) && (src[j] == '+' || src[j] == '-') {
		j++
	}
	if d := lexkit.Span(src, j, lexkit.IsDigit); d > 0 {
		return j + d - i
	}
	return 0
}

// commentLen returns the length matching //.*?\n (dot_all). It returns 0 if there is no newline.
func commentLen(src string, i int) int {
	if !strings.HasPrefix(src[i:], "//") {
		return 0
	}
	if k := strings.IndexByte(src[i+2:], '\n'); k >= 0 {
		return 2 + k + 1
	}
	return 0
}

// stringLen returns the length matching "(\\\\|\\"|[^"])*". It returns 0 if there is no match. src[i] is '"'.
func stringLen(src string, i int) int {
	return lexkit.QuotedLen(src, i, '"')
}
