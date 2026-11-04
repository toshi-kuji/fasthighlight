package c

import (
	"testing"

	fh "github.com/toshi-kuji/fasthighlight"
)

// TestRawString checks that a C++ R"delim(…)delim" is a raw string (an intended difference from
// Chroma). The delimiter is 0 to 16 characters.
func TestRawString(t *testing.T) {
	Prepare()
	for _, c := range []struct {
		code, delim, body string // if delim and body are both "", it is not a raw string
	}{
		{`R"(a\nb)"`, "(", `a\nb`},
		{"R\"xy(a)\" b\n)xy\"", "xy(", "a)\" b\n"},
		{`R"0123456789abcdef(x)0123456789abcdef"`, "0123456789abcdef(", "x"},
		{`R"0123456789abcdefg(x)0123456789abcdefg"`, "", ""}, // the delimiter is 17 characters
		{`R"a b(x)a b"`, "", ""},                             // whitespace in the delimiter
		{`R"(unclosed`, "", ""},
	} {
		var delim, body string
		for _, tk := range Lex(c.code+"\n", len(c.code)+1, Options{CPP: true}) {
			switch tk.Type {
			case fh.LiteralStringDelimiter:
				if delim == "" {
					delim = tk.Value
				}
			case fh.LiteralString:
				if delim != "" && body == "" {
					body = tk.Value
				}
			}
		}
		if delim != c.delim || body != c.body {
			t.Errorf("%q: delimiter %q body %q, want %q %q", c.code, delim, body, c.delim, c.body)
		}
	}
}
