package fasthighlight_test

import (
	"strings"
	"testing"
	"time"

	"github.com/toshi-kuji/fasthighlight"
	_ "github.com/toshi-kuji/fasthighlight/lang/all"
)

// TestLinearTime checks that inputs which once made a tokenizer rescan the same text from every position
// (taking time that grows with the square of the input) are tokenized in about linear time: 64 KB of each
// must take well under a second, where the quadratic behavior took tens of seconds.
func TestLinearTime(t *testing.T) {
	if testing.Short() {
		t.Skip("slow when it fails")
	}
	const size = 64 << 10
	cases := []struct{ lang, unit string }{
		// Java: the memo of a failed method-name scan was lost at each newline.
		{"java", "a\n"},
		// C and C++: the whitespace run was rescanned at each line start.
		{"c", "\n"},
		{"cpp", "\n"},
		// Keyword lookups rescanned a long run of word characters at each position.
		{"sql", "é"},
		{"rust", "_"},
		{"r", "_"},
		{"kotlin", "_"},
		{"c", "_"},
		{"csharp", "_"},
		{"cpp", "_"},
		{"protobuf", "_"},
		// The whitespace run was rescanned at each line start.
		{"java", "\n"},
		{"csharp", "\n"},
		{"kotlin", "\n"},
		{"markdown", "\n"},
		// A run of word characters that is not one token was rescanned from each of its characters.
		{"c", "é"},
		{"cpp", "é"},
		{"protobuf", "é"},
		// C++: each [[ looked for the last ]] of the line again.
		{"cpp", "["},
		// TypeScript: each position rescanned the run of [\w?.$].
		{"typescript", "$"},
		{"typescript", "a."},
		// YAML: each position looked again for the ": " of a key to the line end.
		{"yaml", "@"},
		{"yaml", "-"},
		{"yaml", "["},
		// HTML and XML: each & looked again for the ; of an entity.
		{"html", "&"},
		{"xml", "&"},
		// PHP: each $ rescanned the run of $.
		{"php", "$"},
	}
	for _, c := range cases {
		l := fasthighlight.LookupRegistered(c.lang)
		if l == nil {
			t.Fatalf("%s is not registered", c.lang)
		}
		src := strings.Repeat(c.unit, size/len(c.unit))
		start := time.Now()
		l.Tokenise(src)
		if d := time.Since(start); d > time.Second {
			t.Errorf("%s, %q repeated to %d bytes: %v", c.lang, c.unit, len(src), d)
		}
	}
}
