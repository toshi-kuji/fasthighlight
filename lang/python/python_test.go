package python_test

import (
	"testing"

	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/lang/python"
)

// A """…""" at the start of a line gets the docstring color (an intended difference from Chroma).
// The prefix is 0 to 2 characters.
func TestDocstring(t *testing.T) {
	for _, c := range []struct {
		code  string
		affix string
		doc   string // if "", it is not a docstring
	}{
		{"\"\"\"doc\"\"\"\n", "", `"""doc"""`},
		{"  r'''a\nb'''\n", "r", "'''a\nb'''"},
		{"def f():\n    Rb\"\"\"x\"\"\"\n", "Rb", `"""x"""`},
		{"x = \"\"\"not doc\"\"\"\n", "", ""},
		{"rbu\"\"\"x\"\"\"\n", "", ""},
		{"\"\"\"unclosed\n", "", ""},
	} {
		var affix, doc string
		for _, tk := range python.Language.Tokenise(c.code) {
			switch tk.Type {
			case fh.LiteralStringAffix:
				affix += tk.Value
			case fh.LiteralStringDoc:
				doc += tk.Value
			}
		}
		if c.doc == "" {
			if doc != "" {
				t.Errorf("%q: got docstring %q, want none", c.code, doc)
			}
			continue
		}
		if affix != c.affix || doc != c.doc {
			t.Errorf("%q: affix %q doc %q, want %q %q", c.code, affix, doc, c.affix, c.doc)
		}
	}
}
