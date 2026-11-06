package compat

import (
	"regexp/syntax"
	"slices"
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

// TestWordListsMatchChroma checks that, for every word in the word lists of Java, C#, Kotlin, Swift and the
// other languages (lang/<language>/lists.go), the token sequence is the same as Chroma's in several contexts.
// To find copying mistakes in the lists, it extracts the words from Chroma's rules and tries them.
func TestWordListsMatchChroma(t *testing.T) {
	// The same lists as genlists (language, index of the rule in root, the parts before and after the
	// alternatives). The contexts have the word in %s.
	for _, c := range []struct {
		lang     string
		rules    string // the Chroma language to take the rules from (lang if omitted)
		state    []string
		idx      []int
		prefix   []string
		suffix   []string
		contexts []string
	}{
		{"Java", "", []string{"root", "root", "root", "root"}, []int{4, 7, 8, 10}, []string{"", "", "", ""}, []string{`\b`, `\b`, `\b`, `\b`},
			[]string{"%s\n", "x = %s;\n", "  %s x;\n", "%sx\n", "%s$\n", "a.%s\n", "%s(1);\n", "%s:\n", "@%s\n"}},
		{"C#", "", []string{"root", "root", "root", "root", "root"}, []int{14, 16, 18, 19, 20}, []string{`#[ \t]*`, "", "", "", ""}, []string{`\b`, `\b`, `\b`, `\b\??`, `\b`},
			[]string{"%s\n", "#%s\n", "# %s x\n", "x = %s;\n", "%s?\n", "%sx\n", "%s_\n", "a.%s\n", "@%s\n"}},
		{"Kotlin", "", []string{"root"}, []int{23}, []string{""}, []string{`\b`},
			[]string{"%s\n", "x = %s\n", "%s x\n", "%sx\n", "%s_\n", "%s?\n", "a.%s\n", "@%s\n", "\"$%s\"\n"}},
		{"Swift", "", []string{"keywords", "keywords", "keywords", "keywords", "root", "root"}, []int{0, 2, 3, 8, 6, 7},
			[]string{"", "", "", "", "", ""}, []string{`\b`, `\b`, `\b`, `\b`, `\b`, `\b`},
			[]string{"%s\n", "x = %s\n", "%s x\n", "%sx\n", "%s_\n", "%s(1)\n", "a.%s\n", "#if %s\n", "import %s\n"}},
		{"Go", "", []string{"root", "root", "root", "root", "root"}, []int{6, 7, 8, 9, 10}, []string{"", "", "", "", ""},
			[]string{`\b`, `\b`, `\b`, `\b(\()`, `\b`},
			[]string{"%s\n", "x = %s\n", "%s(1)\n", "%s (1)\n", "%sx\n", "%s_\n", "a.%s\n", "1%s\n", "`{{%s}}`\n"}},
		{"Go", "Go Text Template", []string{"expression"}, []int{2}, []string{""}, []string{`\b`},
			[]string{"`{{%s}}`\n", "`{{ %s .x }}`\n", "`{{(%s)}}`\n", "`{{%sx}}`\n", "`{{$%s}}`\n", "`{{.%s}}`\n"}},
		{"SQL", "", []string{"root", "root"}, []int{5, 6}, []string{"", ""}, []string{`\b`, `\b`},
			[]string{"%s\n", "x %s y\n", "%s(1)\n", "%sx\n", "%s_\n", "%s$\n", "a.%s\n", "1%s\n", "'%s'\n"}},
		{"R", "", []string{"keywords", "builtin_symbols"}, []int{0, 0}, []string{"", ""}, []string{`(?![\w.])`, `(?![\w.])`},
			[]string{"%s\n", "x <- %s\n", "%s(1)\n", "%s (1)\n", "%sx\n", "%s.x\n", "%s_\n", "a$%s\n", "1%s\n"}},
		{"PHP", "", []string{"root", "root", "magicconstants", "magicfuncs"}, []int{18, 19, 0, 0}, []string{"", "", "", ""},
			[]string{`\b`, `\b`, `\b`, `\b`},
			[]string{"%s\n", "$x = %s;\n", "%s(1);\n", "%sx\n", "%s_\n", "$a->%s\n", "function %s() {}\n", "\"{$%s}\"\n", "1%s\n"}},
		{"Ruby", "", []string{"root", "root", "root", "root", "root"}, []int{3, 8, 9, 10, 12}, []string{"", "", "", "", `(?<!\.)`},
			[]string{`\b`, `\b`, `\b`, `\?`, `\b`},
			[]string{"%s\n", "x = %s\n", "%s(1)\n", "%sx\n", "%s_\n", "a.%s\n", "%s?\n", "%s!\n", "\"#{%s}\"\n"}},
		{"TypeScript", "", []string{"root", "root", "root", "root", "root", "root", "root"}, []int{6, 7, 8, 9, 10, 12, 13},
			[]string{"", "", "", "", "", `\b`, `\b`}, []string{`\b`, `\b`, `\b`, `\b`, `\b`, `\b`, `\b`},
			[]string{"%s\n", "x = %s\n", "%s(1)\n", "%sx\n", "%s_\n", "a.%s\n", "1%s\n", "%s: x\n", "`${%s}`\n"}},
		{"Rust", "", []string{"base", "base", "base", "base", "base"}, []int{10, 11, 12, 19, 21}, []string{"", "", "", "", ""},
			[]string{`\b`, `\b`, `\b`, `\b`, `\b`},
			[]string{"%s\n", "let x = %s;\n", "fn f() -> %s {}\n", "%sx\n", "%s_\n", "a.%s\n", "x: &%s\n", "1%s\n", "%s!(1)\n"}},
		{"PowerShell", "", []string{"root", "root", "root", "root", "root", "root", "multline"}, []int{17, 19, 20, 21, 22, 23, 2},
			[]string{"", "", "", "", "-", "", `(\s*\.)`}, []string{`\b`, `\b`, `\b`, `\b`, `\b`, `\s`, `(\s*$)`},
			[]string{"%s\n", "%s x\n", "x -%s y\n", "[%s()]\n", "%sx\n", "%s-x\n", "%s:\n", "$a.%s(\n", "<#\n.%s\n#>\n", "#\n# .%s x\n"}},
	} {
		from := c.lang
		if c.rules != "" {
			from = c.rules
		}
		rl, ok := lexers.Get(from).(*chroma.RegexLexer)
		if !ok {
			t.Fatalf("Chroma's %s lexer is not a RegexLexer", c.lang)
		}
		rules, err := rl.Rules()
		if err != nil {
			t.Fatal(err)
		}
		l := lookup(t, strings.ToLower(c.lang))
		n := 0
		for k, idx := range c.idx {
			p := rules[c.state[k]][idx].Pattern
			inner, ok := strings.CutPrefix(p, c.prefix[k]+"(")
			if ok {
				inner, ok = strings.CutSuffix(inner, ")"+c.suffix[k])
			}
			if !ok {
				t.Fatalf("%s root[%d]: unexpected pattern %q", c.lang, idx, p)
			}
			// The last alternative of the R constants, \.\.(\.|[0-9]+), is not a word list (lang/r handles it separately).
			inner = strings.TrimSuffix(inner, `|\.\.(\.|[0-9]+)`)
			re, err := syntax.Parse(inner, syntax.Perl)
			if err != nil {
				t.Fatal(err)
			}
			words := finiteWords(t, re)
			if c.lang == "SQL" || c.lang == "PHP" || c.lang == "PowerShell" { // matching is case-insensitive, so also try the lower-case and upper-case words
				for _, w := range words {
					words = append(words, strings.ToLower(w), strings.ToUpper(w))
				}
			}
			for _, w := range words {
				for _, ctx := range c.contexts {
					code := strings.ReplaceAll(ctx, "%s", w)
					if ours, theirs := chromaMode(l)(code), chromaTokens(t, l, code); !slices.Equal(ours, theirs) {
						t.Errorf("%s %q in %q: tokens differ\n ours %v\nchroma %v", c.lang, w, code, ours, theirs)
					}
					n++
				}
			}
		}
		t.Logf("%s: %d cases", c.lang, n)
	}
}
