// Package ini is the INI tokenizer. It is used by lang/ini (INI) and internal/http (message bodies).
//
// Token types and boundaries reproduce the rules of Chroma 2.27.0's ini.xml without regular expressions.
// ini.xml has only the root state, and its last rule matches the rest of any line, so no character
// becomes an Error.
package ini

import (
	"strings"

	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/lexkit"
)

// Lex splits src into tokens (the same contract as Lex of fasthighlight.Language).
func Lex(src string, end int) []fh.Token {
	x := lexkit.NewTokens(src)
	LexInto(x, src, end)
	return x.Result()
}

// LexInto is the same as Lex but adds the tokens to x (for embedding INI in another language, such as the
// body of an HTTP message).
func LexInto(x *lexkit.Tokens, src string, end int) {
	for i := 0; i < end; {
		i = step(x, src, i)
	}
}

// step tries the rules of the root state in order at i and returns the position after the match.
func step(x *lexkit.Tokens, src string, i int) int {
	if n := lexkit.Span(src, i, lexkit.IsSpace); n > 0 { // \s+
		x.Emit(fh.Text, i, i+n)
		return i + n
	}
	eol := lexkit.LineEnd(src, i)
	switch src[i] {
	case ';', '#': // [;#].*
		x.Emit(fh.CommentSingle, i, eol)
		return eol
	case '[': // \[.*?\]$ (the first ] on the line that is followed by the end of the line)
		if eol > i+1 && src[eol-1] == ']' {
			x.Emit(fh.Keyword, i, eol)
			return eol
		}
	}
	// (.*?)([ \t]*)(=)([ \t]*)(.*(?:\n[ \t].+)*): the first = on the line, with the spaces and tabs just
	// before it.
	if k := strings.IndexByte(src[i:eol], '='); k >= 0 {
		eq := i + k
		ws := eq
		for ws > i && isBlank(src[ws-1]) {
			ws--
		}
		v := eq + 1
		for v < len(src) && isBlank(src[v]) {
			v++
		}
		e := lexkit.LineEnd(src, v)
		// (?:\n[ \t].+)*: a following line that starts with a space or a tab and has at least one more
		// character.
		for e+2 < len(src) && isBlank(src[e+1]) && src[e+2] != '\n' {
			e = lexkit.LineEnd(src, e+2)
		}
		x.Emit(fh.NameAttribute, i, ws)
		x.Emit(fh.Text, ws, eq)
		x.Emit(fh.Operator, eq, eq+1)
		x.Emit(fh.Text, eq+1, v)
		x.Emit(fh.LiteralString, v, e)
		return e
	}
	// (.+?)$ (src[i] is not a newline, so the rest of the line is not empty)
	x.Emit(fh.NameAttribute, i, eol)
	return eol
}

func isBlank(c byte) bool { return c == ' ' || c == '\t' }
