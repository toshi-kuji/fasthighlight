package compat

import (
	"slices"
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

// TestPythonWordListsMatchChroma checks that, for every word in the Python word lists
// (internal/python/lists.go), the token sequence is the same as Chroma's in several contexts.
// To find copying mistakes in the lists (missing words, different order), it extracts the words from
// Chroma's rules and tries them.
func TestPythonWordListsMatchChroma(t *testing.T) {
	rl, ok := lexers.Get("Python").(*chroma.RegexLexer)
	if !ok {
		t.Fatal("Chroma's Python lexer is not a RegexLexer")
	}
	rules, err := rl.Rules()
	if err != nil {
		t.Fatal(err)
	}
	// The same lists as genpythonlists.
	lists := []struct {
		state string
		idx   int
	}{
		{"keywords", 0}, {"keywords", 1}, {"expr-keywords", 0}, {"expr", 20},
		{"builtins", 0}, {"builtins", 1}, {"builtins", 2}, {"magicfuncs", 0}, {"magicvars", 0},
	}
	// Contexts. The word goes in %s. Start of a line (root rule), inside an expression, after a dot, inside {…}
	// of an f-string, after def, continuing a word.
	contexts := []string{"%s\n", "x = %s\n", "x.%s\n", "f'{%s}'\n", "def %s(): pass\n", "%sx\n", "%s_\n", "%s(1)\n", "(%s)\n"}
	l := lookup(t, "python")
	n := 0
	for _, list := range lists {
		p := rules[list.state][list.idx].Pattern
		start, end := strings.Index(p, "(")+1, strings.LastIndex(p, ")")
		if strings.HasPrefix(p, `(?<!\.)`) {
			start = len(`(?<!\.)(`)
		}
		if start <= 0 || end < start || !strings.HasSuffix(p, `)\b`) {
			t.Fatalf("%s[%d]: unexpected pattern %q", list.state, list.idx, p)
		}
		for _, w := range strings.Split(p[start:end], "|") {
			for _, c := range contexts {
				code := strings.ReplaceAll(c, "%s", w)
				if ours, theirs := l.Tokenise(code), chromaTokens(t, l, code); !slices.Equal(ours, theirs) {
					t.Errorf("%q in %q: tokens differ\n ours %v\nchroma %v", w, code, ours, theirs)
				}
				n++
			}
		}
	}
	t.Logf("%d cases", n)
}
