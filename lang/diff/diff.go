// Package diff registers diff.
//
//	import _ "github.com/toshi-kuji/fasthighlight/lang/diff"
//
// Token types and boundaries reproduce the rules of Chroma 2.27.0's diff.xml without regular expressions.
package diff

import (
	"strings"

	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/lexkit"
)

// Language is Diff. Its name, aliases, and filename patterns are listed in the catalog of the main
// package (fasthighlight.Builtin).
var Language = fh.Builtin("Diff")

func init() {
	Language.Lex = lex
	fh.Register(Language)
}

// lex tries the rules of diff.xml one line at a time. Every rule makes the text up to the end-of-line
// newline a single token (because of EnsureNewline, the last line also has a newline).
func lex(src string, end int) []fh.Token {
	x := lexkit.NewTokens(src)
	for i := 0; i < end; {
		le := lexkit.LineEnd(src, i)
		if le == len(src) { // no newline (normally does not happen because of EnsureNewline)
			i += x.Error(i)
			continue
		}
		x.Emit(lineType(src[i:le]), i, le+1)
		i = le + 1
	}
	return x.Result()
}

// lineType decides the type of a line, excluding its newline, in the order of the rules in diff.xml.
func lineType(line string) fh.TokenType {
	switch {
	case strings.HasPrefix(line, " "):
		return fh.Text
	case isNormalHunk(line):
		return fh.GenericSubheading
	case line == "---":
		return fh.GenericStrong
	case strings.HasPrefix(line, "< "):
		return fh.GenericDeleted
	case strings.HasPrefix(line, "> "):
		return fh.GenericInserted
	case strings.HasPrefix(line, "+"):
		return fh.GenericInserted
	case strings.HasPrefix(line, "-"):
		return fh.GenericDeleted
	case strings.HasPrefix(line, "!"):
		return fh.GenericStrong
	case strings.HasPrefix(line, "@"):
		return fh.GenericSubheading
	case strings.HasPrefix(line, "Index"), strings.HasPrefix(line, "index"), strings.HasPrefix(line, "diff"):
		return fh.GenericHeading
	case strings.HasPrefix(line, "="):
		return fh.GenericHeading
	}
	return fh.Text
}

// isNormalHunk returns true if the whole line matches \d+(,\d+)?(a|c|d)\d+(,\d+)? (example: "12,14c12").
func isNormalHunk(line string) bool {
	j := 0
	// numRange reads \d+(,\d+)?.
	numRange := func() bool {
		n := lexkit.Span(line, j, lexkit.IsDigit)
		if n == 0 {
			return false
		}
		j += n
		if j < len(line) && line[j] == ',' {
			if m := lexkit.Span(line, j+1, lexkit.IsDigit); m > 0 {
				j += 1 + m
			}
		}
		return true
	}
	if !numRange() || j >= len(line) || strings.IndexByte("acd", line[j]) < 0 {
		return false
	}
	j++
	return numRange() && j == len(line)
}
