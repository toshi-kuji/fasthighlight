package compat

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

// TestCSSWordListsMatchChroma checks that, for every word in the CSS word lists (lang/css/lists.go), the
// token sequence is the same as Chroma's in several contexts.
// To find copying mistakes in the lists (missing words, different order), it extracts the words from
// Chroma's rules and tries them.
func TestCSSWordListsMatchChroma(t *testing.T) {
	rl, ok := lexers.Get("CSS").(*chroma.RegexLexer)
	if !ok {
		t.Fatal("Chroma's CSS lexer is not a RegexLexer")
	}
	rules, err := rl.Rules()
	if err != nil {
		t.Fatal(err)
	}
	group := regexp.MustCompile(`^\(([^()]*)\)`)
	words := func(state string, idx int) []string {
		m := group.FindStringSubmatch(rules[state][idx].Pattern)
		if m == nil {
			t.Fatalf("%s[%d]: unexpected pattern %q", state, idx, rules[state][idx].Pattern)
		}
		return strings.Split(m[1], "|")
	}
	// Contexts. The word goes in %s.
	contexts := map[string][]string{
		"vendor":   {"a{%sx:1}", "a{b:%sx}"},
		"function": {"a{b:%s(1)}", "a{b:%sx(1)}"},
		"constant": {"a{b:%s}", "a{b:%sx}", "a{b:%s-y}", "a{b:x%s}"},
		"property": {"a{%s:1}", "a{%s :1}", "a{%sx:1}"},
		"keyword":  {"a{b:%s}", "a{b:%s_}"},
		"unit":     {"a{b:1%s}", "a{b:1%sx}", "a{b:1%s%%}"},
	}
	lists := []struct {
		state, kind string
		idx         int
	}{
		{"common-values", "vendor", 1}, {"content", "vendor", 4}, {"common-values", "function", 3},
		{"common-values", "constant", 5}, {"common-values", "constant", 6}, {"common-values", "constant", 7},
		{"content", "property", 5}, {"value-start", "keyword", 1}, {"numeric-end", "unit", 0},
	}
	l := lookup(t, "css")
	n := 0
	for _, list := range lists {
		for _, w := range words(list.state, list.idx) {
			for _, c := range contexts[list.kind] {
				code := strings.ReplaceAll(strings.ReplaceAll(c, "%s", w), "%%", "%")
				if ours, theirs := l.Tokenise(code), chromaTokens(t, l, code); !slices.Equal(ours, theirs) {
					t.Errorf("%s %q in %q: tokens differ\n ours %v\nchroma %v", list.kind, w, code, ours, theirs)
				}
				n++
			}
		}
	}
	t.Logf("%d documents compared", n)
}
