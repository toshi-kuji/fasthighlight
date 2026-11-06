package compat

import (
	"slices"
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

// TestSCSSWordListsMatchChroma checks that, for every word in the SCSS word lists (lang/scss/lists.go), the
// token sequence is the same as Chroma's in several contexts, in lower case, upper case and as written.
// To find copying mistakes in the lists (missing words, different order), it extracts the words from
// Chroma's rules and tries them.
func TestSCSSWordListsMatchChroma(t *testing.T) {
	rl, ok := lexers.Get("SCSS").(*chroma.RegexLexer)
	if !ok {
		t.Fatal("Chroma's SCSS lexer is not a RegexLexer")
	}
	rules, err := rl.Rules()
	if err != nil {
		t.Fatal(err)
	}
	// The same lists as genscsslists (state, rule index, the parts before and after the alternatives). The
	// contexts have the word in %s.
	property := []string{"%s: 1;\n", "%s :1;\n", "%s\n: 1;\n", "%sx: 1;\n", "x%s: 1;\n", "%s: 1\n", "a { %s: b; }\n",
		"a { b: %s: c; }\n", "a { b: c %s: d; }\n", "a { b: %s; }\n"}
	value := []string{"a { b: %s; }\n", "a { b: %sx; }\n", "a { b: %s-y; }\n", "a { b: x%s; }\n", "a { b: %s(1); }\n",
		"a { b: %s_; }\n", "$x: %s %s;\n", "a { b: 1 %s, #{%s} }\n", "@include m(%s);\n"}
	for _, c := range []struct {
		state          string
		idx            int
		prefix, suffix string
		contexts       []string
	}{
		{"root", 16, `\b(`, `)(?=\s*:)`, property},
		{"value", 5, `\b(`, `)(?=\s*:)`, property},
		{"value", 7, `\b(`, `)\b`, value},
		{"value", 8, `(?i)\b(`, `)\b`, value},
	} {
		p := rules[c.state][c.idx].Pattern
		inner, ok := strings.CutPrefix(p, c.prefix)
		if ok {
			inner, ok = strings.CutSuffix(inner, c.suffix)
		}
		if !ok {
			t.Fatalf("%s[%d]: unexpected pattern %q", c.state, c.idx, p)
		}
		l := lookup(t, "scss")
		n := 0
		for _, w := range strings.Split(inner, "|") {
			for _, w := range []string{w, strings.ToLower(w), strings.ToUpper(w)} {
				for _, ctx := range c.contexts {
					code := strings.ReplaceAll(ctx, "%s", w)
					if ours, theirs := l.Tokenise(code), chromaTokens(t, l, code); !slices.Equal(ours, theirs) {
						t.Errorf("%s[%d] %q in %q: tokens differ\n ours %v\nchroma %v", c.state, c.idx, w, code, ours, theirs)
					}
					n++
				}
			}
		}
		t.Logf("%s[%d]: %d documents compared", c.state, c.idx, n)
	}
}
