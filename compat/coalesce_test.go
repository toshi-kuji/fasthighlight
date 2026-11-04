package compat

import (
	"slices"
	"strings"
	"testing"

	fh "github.com/toshi-kuji/fasthighlight"
)

// TestCoalesceLimitMatchesChroma checks that, when tokens of the same type run long, the positions where they
// are split at the coalescing length limit (fh.CoalesceLimit) are the same as in Chroma.
func TestCoalesceLimitMatchesChroma(t *testing.T) {
	tests := []struct{ lang, code string }{
		{"diff", strings.Repeat("+an added line of some length\n", 600)},
		{"diff", strings.Repeat("-x\n", 5000)},
		{"json", "[" + strings.Repeat(" ", 20000) + "1]"},
		{"json", strings.Repeat("// comment line\n", 1000) + "1"},
		{"toml", strings.Repeat("# comment\n", 1500)},
		{"toml", strings.Repeat("$", 9000)},
		{"ts", strings.Repeat("// comment line\n", 1000)},
		{"js", strings.Repeat("/* block */", 1000) + "\n"},
		{"js", strings.Repeat("a", 10000) + "\n"},
	}
	for _, tt := range tests {
		l := lookup(t, tt.lang)
		ours, theirs := l.Tokenise(tt.code), chromaTokens(t, l, tt.code)
		if !slices.Equal(ours, theirs) {
			t.Errorf("%s (%d bytes): %d tokens, Chroma %d tokens; lengths ours=%v chroma=%v",
				l.Name, len(tt.code), len(ours), len(theirs), lengths(ours), lengths(theirs))
		}
	}
}

func lengths(toks []fh.Token) []int {
	var n []int
	for _, t := range toks {
		n = append(n, len(t.Value))
	}
	if len(n) > 12 {
		n = append(n[:12], -1)
	}
	return n
}
