// Package bashsession registers Bash Session (a shell session: prompts with commands, and their output).
//
//	import _ "github.com/toshi-kuji/fasthighlight/lang/bashsession"
//
// Token types and boundaries reproduce the rules of Chroma 2.27.0's bash_session.xml without regular
// expressions. The commands after a prompt are read as Bash (internal/bash).
package bashsession

import (
	"strings"

	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/bash"
	"github.com/toshi-kuji/fasthighlight/internal/lexkit"
)

// Language is Bash Session. Its name, aliases, and filename patterns are listed in the catalog of the main
// package (fasthighlight.Builtin).
var Language = fh.Builtin("Bash Session")

func init() {
	Language.Lex = lex
	fh.Register(Language)
}

// lex tries the rules of the root state of bash_session.xml (the only state) in this order:
//
//	^((?:\[[^]]+@[^]]+\]\s?)?[#$%>])(\s*)(.*\n?)   prompt, space, and a command read as Bash
//	^.+\n?                                          output
//
// Both rules start at the start of a line (^ in multiline mode), and every match ends at the start of a
// line, so an empty line matches neither and is an Error (as in Chroma).
func lex(src string, end int) []fh.Token {
	x := lexkit.NewTokens(src)
	var b brackets
	for i := 0; i < end; {
		if i > 0 && src[i-1] != '\n' {
			i += x.Error(i)
			continue
		}
		if p := b.promptEnd(src, i); p > 0 {
			s := p + lexkit.Span(src, p, lexkit.IsSpace) // \s*
			e := lineEnd(src, s)                         // .*\n?
			x.Emit(fh.GenericPrompt, i, p)
			x.Emit(fh.Text, p, s)
			bash.LexInto(x.Sub(src[s:e], s), src[s:e], e-s)
			i = e
			continue
		}
		if le := lexkit.LineEnd(src, i); le > i { // ^.+\n?
			e := lineEnd(src, i)
			x.Emit(fh.GenericOutput, i, e)
			i = e
			continue
		}
		i += x.Error(i)
	}
	return x.Result()
}

// lineEnd returns the end of .*\n? from i (after the newline, if there is one).
func lineEnd(src string, i int) int {
	e := lexkit.LineEnd(src, i)
	if e < len(src) {
		e++
	}
	return e
}

// brackets remembers the ] found last, so that lines starting with [ are not scanned again up to the
// same ] (or to the end, when there is none).
type brackets struct {
	from  int // the position + 1 the ] was looked up from (0: not looked up yet)
	close int // the position of the first ] after from (-1: none)
	at    int // the last @ in [from+1, close-1) (-1: none)
}

// promptEnd returns the end of (?:\[[^]]+@[^]]+\]\s?)?[#$%>] from i, or 0 if it does not match.
func (b *brackets) promptEnd(src string, i int) int {
	if strings.IndexByte("#$%>", src[i]) >= 0 {
		return i + 1
	}
	if src[i] != '[' {
		return 0
	}
	// [^]]+ also matches newlines. The ] is the first one after the [, and an @ must have at least one
	// character on each side before it.
	if b.from == 0 || b.close >= 0 && i >= b.close {
		b.from, b.close, b.at = i+1, -1, -1
		if k := strings.IndexByte(src[i+1:], ']'); k >= 0 {
			b.close = i + 1 + k
			if b.close-1 > i+2 {
				if k := strings.LastIndexByte(src[i+2:b.close-1], '@'); k >= 0 {
					b.at = i + 2 + k
				}
			}
		}
	}
	r := b.close
	if r < 0 || b.at < i+2 || b.at > r-2 {
		return 0
	}
	q := r + 1
	if q >= len(src) {
		return 0
	}
	// \s? is tried with a space first, then without.
	if c, n := lexkit.Rune(src, q); lexkit.IsSpace(c) && q+n < len(src) && strings.IndexByte("#$%>", src[q+n]) >= 0 {
		return q + n + 1
	}
	if strings.IndexByte("#$%>", src[q]) >= 0 {
		return q + 1
	}
	return 0
}
