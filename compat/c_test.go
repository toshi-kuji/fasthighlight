package compat

import (
	"regexp/syntax"
	"slices"
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

// TestCWordListsMatchChroma checks that, for every word in the C and C++ word lists (internal/c/lists.go),
// the token sequence is the same as Chroma's in several contexts. To find mistakes in building the lists
// (such as missing words), it extracts the words from Chroma's rules and tries them.
func TestCWordListsMatchChroma(t *testing.T) {
	// The same lists as genclists (language, index of the rule in statements).
	lists := []struct {
		lang string
		idx  int
	}{
		{"C", 10}, {"C", 11}, {"C", 12}, {"C", 13}, {"C", 14}, {"C", 15},
		{"C++", 0}, {"C++", 17}, {"C++", 18}, {"C++", 19}, {"C++", 20}, {"C++", 21}, {"C++", 22},
	}
	// Contexts. The word goes in %s. Start of a line (root rule), inside a statement, inside a function,
	// continuing a word, preceded by a word character.
	contexts := []string{"%s\n", "x = %s;\n", "int f() { %s; }\n", "%sx\n", "%s_\n", "1%s\n", "%s:\n", "%s(1);\n", "a.%s\n"}
	n := 0
	for _, list := range lists {
		rl, ok := lexers.Get(list.lang).(*chroma.RegexLexer)
		if !ok {
			t.Fatalf("Chroma's %s lexer is not a RegexLexer", list.lang)
		}
		rules, err := rl.Rules()
		if err != nil {
			t.Fatal(err)
		}
		p := strings.TrimSuffix(rules["statements"][list.idx].Pattern, `|\b[a-z]\w*_t\b`)
		x, ok := strings.CutSuffix(p, `\b`)
		if !ok {
			t.Fatalf("%s[%d]: unexpected pattern %q", list.lang, list.idx, p)
		}
		re, err := syntax.Parse(x, syntax.Perl)
		if err != nil {
			t.Fatal(err)
		}
		l := lookup(t, strings.ToLower(list.lang))
		for _, w := range finiteWords(t, re) {
			for _, c := range contexts {
				code := strings.ReplaceAll(c, "%s", w)
				if ours, theirs := l.Tokenise(code), chromaTokens(t, l, code); !slices.Equal(ours, theirs) {
					t.Errorf("%s %q in %q: tokens differ\n ours %v\nchroma %v", list.lang, w, code, ours, theirs)
				}
				n++
			}
		}
	}
	t.Logf("%d cases", n)
}

// finiteWords returns every string that re (made only of characters, alternatives and ?) matches.
func finiteWords(t *testing.T, re *syntax.Regexp) []string {
	t.Helper()
	switch re.Op {
	case syntax.OpLiteral:
		return []string{string(re.Rune)}
	case syntax.OpEmptyMatch:
		return []string{""}
	case syntax.OpCapture:
		return finiteWords(t, re.Sub[0])
	case syntax.OpCharClass:
		var out []string
		for i := 0; i < len(re.Rune); i += 2 {
			for r := re.Rune[i]; r <= re.Rune[i+1]; r++ {
				out = append(out, string(r))
			}
		}
		return out
	case syntax.OpQuest:
		return append(finiteWords(t, re.Sub[0]), "")
	case syntax.OpAlternate:
		var out []string
		for _, s := range re.Sub {
			out = append(out, finiteWords(t, s)...)
		}
		return out
	case syntax.OpConcat:
		out := []string{""}
		for _, s := range re.Sub {
			var next []string
			for _, a := range out {
				for _, b := range finiteWords(t, s) {
					next = append(next, a+b)
				}
			}
			out = next
		}
		return out
	}
	t.Fatalf("unsupported regexp %v", re)
	return nil
}
