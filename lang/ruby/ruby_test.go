package ruby_test

import (
	"strings"
	"testing"

	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/lang/ruby"
)

// Intended differences from Chroma: no characters are added, defined? is a reserved word, and
// names may start with a non-letter.
func TestIntendedDifferences(t *testing.T) {
	for _, c := range []struct {
		code, value string
		want        fh.TokenType
	}{
		{"defined? x\n", "defined?", fh.Keyword},
		{"defined?(y)\n", "defined?", fh.Keyword},
		{"日本 = 1\n", "日本", fh.Name},
		{"@名前 = 1\n", "@名前", fh.NameVariableInstance},
		{"x = :記号\n", ":記号", fh.LiteralStringSymbol},
	} {
		toks := ruby.Language.Tokenise(c.code)
		found := false
		for _, tk := range toks {
			if tk.Value == c.value {
				found = true
				if tk.Type != c.want {
					t.Errorf("%q: %q is %s, want %s", c.code, c.value, tk.Type, c.want)
				}
			}
		}
		if !found {
			t.Errorf("%q: no token %q in %v", c.code, c.value, toks)
		}
	}
	// Do not emit the delimiter character of "% abc " a second time (Chroma emits it as None).
	code := "x =% abc ;y\n"
	var b strings.Builder
	for _, tk := range ruby.Language.Tokenise(code) {
		b.WriteString(tk.Value)
		if tk.Type == fh.None {
			t.Errorf("unexpected None token %q", tk.Value)
		}
	}
	if b.String() != code {
		t.Errorf("tokens = %q, want %q", b.String(), code)
	}
}
