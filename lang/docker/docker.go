// Package docker registers Docker (Dockerfile and Containerfile).
//
//	import _ "github.com/toshi-kuji/fasthighlight/lang/docker"
//
// Token types and boundaries reproduce the rules of Chroma 2.27.0's docker.xml without regular expressions.
// Commands are read as Bash (internal/bash), and the [ … ] form of VOLUME, ENTRYPOINT, CMD and SHELL as
// JSON (internal/json).
package docker

import (
	"strings"

	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/bash"
	"github.com/toshi-kuji/fasthighlight/internal/json"
	"github.com/toshi-kuji/fasthighlight/internal/lexkit"
)

// Language is Docker. Its name, aliases, and filename patterns are listed in the catalog of the main
// package (fasthighlight.Builtin).
var Language = fh.Builtin("Docker")

func init() {
	Language.Lex = lex
	fh.Register(Language)
}

// The alternatives of the keyword groups of docker.xml, in the same order (lowercase; docker.xml is
// case-insensitive, so they are compared with lexkit.FoldPrefix).
var (
	jsonForms = [...]string{"volume", "entrypoint", "cmd", "shell"}
	assigns   = [...]string{"label", "env", "arg"}
	arguments = [...]string{"from", "maintainer", "expose", "workdir", "user", "stopsignal", "volume"}
	commands  = [...]string{"run", "cmd", "entrypoint", "env", "arg", "label", "add", "copy"}
)

type lexer struct {
	x   *lexkit.Tokens
	src string
}

// lex tries the rules of the root state of docker.xml (the only state) in this order. A newline matches
// none of them, so it is an Error (as in Chroma).
func lex(src string, end int) []fh.Token {
	l := &lexer{x: lexkit.NewTokens(src), src: src}
	for i := 0; i < end; {
		n := l.step(i)
		if n == 0 {
			n = l.x.Error(i)
		}
		i += n
	}
	return l.x.Result()
}

// bash reads src[s:e] as Bash (using Bash).
func (l *lexer) bash(s, e int) {
	bash.LexInto(l.x.Sub(l.src[s:e], s), l.src[s:e], e-s)
}

func (l *lexer) step(i int) int {
	src := l.src
	if src[i] == '#' { // #.*
		e := lexkit.LineEnd(src, i)
		l.x.Emit(fh.Comment, i, e)
		return e - i
	}
	// (ONBUILD)((?:\s*\\?\s*))
	if n := lexkit.FoldPrefix(src, i, "onbuild"); n > 0 {
		e := gap(src, i+n)
		l.x.Emit(fh.Keyword, i, i+n)
		l.bash(i+n, e)
		return e - i
	}
	// (HEALTHCHECK)((?:(?:\s*\\?\s*)--\w+=\w+(?:\s*\\?\s*))*)
	if n := lexkit.FoldPrefix(src, i, "healthcheck"); n > 0 {
		e := options(src, i+n, true)
		l.x.Emit(fh.Keyword, i, i+n)
		l.bash(i+n, e)
		return e - i
	}
	// (VOLUME|ENTRYPOINT|CMD|SHELL)((?:\s*\\?\s*))(\[.*?\])
	if n := foldAny(src, i, jsonForms[:]); n > 0 {
		g := gap(src, i+n)
		if g < len(src) && src[g] == '[' {
			if k := strings.IndexByte(src[g:lexkit.LineEnd(src, g)], ']'); k >= 0 {
				e := g + k + 1
				l.x.Emit(fh.Keyword, i, i+n)
				l.bash(i+n, g)
				json.LexInto(l.x.Sub(src[g:e], g), src[g:e], e-g)
				return e - i
			}
		}
	}
	// (LABEL|ENV|ARG)((?:(?:\s*\\?\s*)\w+=\w+(?:\s*\\?\s*))*)
	if n := foldAny(src, i, assigns[:]); n > 0 {
		e := options(src, i+n, false)
		l.x.Emit(fh.Keyword, i, i+n)
		l.bash(i+n, e)
		return e - i
	}
	if n := l.argument(i); n > 0 {
		return n
	}
	// ((?:RUN|CMD|ENTRYPOINT|ENV|ARG|LABEL|ADD|COPY))
	if n := foldAny(src, i, commands[:]); n > 0 {
		l.x.Emit(fh.Keyword, i, i+n)
		return n
	}
	// (.*\\\n)*.+
	if e := continuedLine(src, i); e > i {
		l.bash(i, e)
		return e - i
	}
	return 0
}

// argument tries
//
//	((?:FROM|MAINTAINER|EXPOSE|WORKDIR|USER|STOPSIGNAL)|VOLUME)\b(\s)([\S]*)\b(?:(\s)(AS)\b(\s)([\S]*))?\b
//
// and returns the length of the match (0 if it does not match).
func (l *lexer) argument(i int) int {
	src := l.src
	n := foldAny(src, i, arguments[:])
	if n == 0 {
		return 0
	}
	k := i + n // \b holds here when a space follows
	if k >= len(src) {
		return 0
	}
	r, w := lexkit.Rune(src, k)
	if !lexkit.IsSpace(r) {
		return 0
	}
	v := k + w
	ve := nonSpaceWord(src, v)
	if ve < 0 {
		return 0
	}
	l.x.Emit(fh.Keyword, i, k)
	l.x.Emit(fh.TextWhitespace, k, v)
	l.x.Emit(fh.LiteralString, v, ve)
	e := ve
	// (?:(\s)(AS)\b(\s)([\S]*))? is only possible when [\S]* took every non-space character.
	if ve == v+lexkit.Span(src, v, notSpace) && ve < len(src) {
		if r, w := lexkit.Rune(src, ve); lexkit.IsSpace(r) {
			a := ve + w
			if m := lexkit.FoldPrefix(src, a, "as"); m > 0 && a+m < len(src) {
				if r, w2 := lexkit.Rune(src, a+m); lexkit.IsSpace(r) {
					s := a + m + w2
					if se := nonSpaceWord(src, s); se >= 0 {
						l.x.Emit(fh.TextWhitespace, ve, a)
						l.x.Emit(fh.Keyword, a, a+m)
						l.x.Emit(fh.TextWhitespace, a+m, s)
						l.x.Emit(fh.LiteralString, s, se)
						e = se
					}
				}
			}
		}
	}
	return e - i
}

// nonSpaceWord returns the end of [\S]*\b from i: the longest run of non-space characters that ends at a
// word boundary. It returns -1 if there is none (not even the empty run).
func nonSpaceWord(src string, i int) int {
	e := -1
	lexkit.Run(src, i, 0, notSpace, func(k int) bool {
		if lexkit.WordBoundary(src, k) {
			e = k
			return true
		}
		return false
	})
	return e
}

func notSpace(r rune) bool { return !lexkit.IsSpace(r) }

// gap returns the end of \s*\\?\s* from i.
func gap(src string, i int) int {
	i += lexkit.Span(src, i, lexkit.IsSpace)
	if i < len(src) && src[i] == '\\' {
		i++
		i += lexkit.Span(src, i, lexkit.IsSpace)
	}
	return i
}

// options returns the end of ((?:\s*\\?\s*)--\w+=\w+(?:\s*\\?\s*))* from i (without the -- when dashes is
// false). Each repetition either matches as a whole or ends the loop, so no backtracking is needed.
func options(src string, i int, dashes bool) int {
	for {
		j := gap(src, i)
		if dashes {
			if !strings.HasPrefix(src[j:], "--") {
				return i
			}
			j += 2
		}
		n := lexkit.Span(src, j, lexkit.IsWord)
		if n == 0 || j+n >= len(src) || src[j+n] != '=' {
			return i
		}
		j += n + 1
		n = lexkit.Span(src, j, lexkit.IsWord)
		if n == 0 {
			return i
		}
		i = gap(src, j+n)
	}
}

// continuedLine returns the end of (.*\\\n)*.+ from i, or i if it does not match. Lines ending with a
// backslash are taken whole; if the line after them is empty, the regular expression backtracks and the
// match ends before the newline of the last one.
func continuedLine(src string, i int) int {
	last := -1 // the start of the last line taken by (.*\\\n)
	p := i
	for {
		le := lexkit.LineEnd(src, p)
		if le >= len(src) || le == p || src[le-1] != '\\' {
			break
		}
		last, p = p, le+1
	}
	if le := lexkit.LineEnd(src, p); le > p {
		return le
	}
	if last >= 0 {
		return lexkit.LineEnd(src, last)
	}
	return i
}

// foldAny returns the length of the first of words that matches the start of src[i:] case-insensitively,
// or 0 if none does.
func foldAny(src string, i int, words []string) int {
	for _, w := range words {
		if n := lexkit.FoldPrefix(src, i, w); n > 0 {
			return n
		}
	}
	return 0
}
