package fasthighlight_test

import (
	"os"
	"sort"
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
	for _, c := range append(slowCases, rescanCases...) {
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

// TestLinearTimeLarge checks rescanCases at 1 MB, which must take less than 2 seconds. A linear tokenizer
// takes well under half a second there, while a quadratic scan that is still fast at 64 KB (and passes
// TestLinearTime) takes 256 times as long as at 64 KB: seconds, or minutes. It is skipped with -race, which
// makes every tokenizer about ten times slower; CI runs it on its own without -race.
func TestLinearTimeLarge(t *testing.T) {
	if testing.Short() {
		t.Skip("slow when it fails")
	}
	if raceEnabled {
		t.Skip("too slow with -race; run without it")
	}
	const size = 1 << 20
	for _, c := range rescanCases {
		l := fasthighlight.LookupRegistered(c.lang)
		src := strings.Repeat(c.unit, size/len(c.unit))
		start := time.Now()
		l.Tokenise(src)
		if d := time.Since(start); d > 2*time.Second {
			t.Errorf("%s, %q repeated to %d bytes: %v", c.lang, c.unit, len(src), d)
		}
	}
}

// slowCases are inputs (a unit repeated) that once made a tokenizer rescan the same text from many positions,
// or that make it scan a bounded distance from each position.
var slowCases = []struct{ lang, unit string }{
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
	// C#: the scans for a type (type arguments after each <, a declaration after each name) are bounded.
	{"csharp", "a<"},
	{"csharp", "a "},
	{"csharp", "a.<"},
	{"csharp", "new a<"},
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
	// Kotlin: each "<" or ":" scans ahead for a type (bounded).
	{"kotlin", "a<"},
	{"kotlin", "a<b"},
	{"kotlin", "a:"},
	{"kotlin", "a<b<c>  "},
	// Java: each capitalized name looks for [] pairs and a variable name after it (bounded scans).
	{"java", "A "},
	{"java", "A [] "},
	// Java: the method-name rule measured A<A<… to its end again from each position, and rescanned a
	// failed word sequence from the middle of a word (the B of A<B).
	{"java", "A<"},
	{"java", "A<B "},
	// PHP: each $ rescanned the run of $.
	{"php", "$"},
	// PHP: type scans (a name before a variable, a return type after "):") look at most 512 bytes.
	{"php", "A|"},
	{"php", "A &"},
	{"php", "(a):"},
	{"php", "function("},
	// Python: type hints scan a bounded distance forward and backward from each ":" and "->".
	{"python", "a:"},
	{"python", "a.:"},
	{"python", "->a"},
	{"python", "def f(a:"},
	// Swift: each type position or "<" after a name scans at most 512 bytes.
	{"swift", "a<"},
	{"swift", "f(a:"},
	{"swift", "let a: ["},
	// Rust: each ":" or "::<" scans the type after it (bounded by maxType).
	{"rust", "a: "},
	{"rust", "x::<"},
	{"rust", ": Vec<"},
}

// rescanCases are inputs that once made a tokenizer look for the same thing again from each of many
// positions (a closing delimiter that is not there, or the same run of whitespace). They were found by
// repeating every one- and two-character combination of a small alphabet in every language and measuring how
// the time grew with the size (see AGENTS.md). TestLinearTime checks them with the others at 64 KB, and
// TestLinearTimeLarge at 1 MB, where the quadratic ones that were fast at 64 KB take seconds.
var rescanCases = []struct{ lang, unit string }{
	// Python: each line start measured the whitespace after it again (for a docstring), across lines.
	{"python", "\n "},
	{"python", "\t\n"},
	// PowerShell: each # at a line start measured the run of # and whitespace again (for a help keyword).
	{"powershell", "#\n"},
	// R: a string or `name` that never closes was scanned to the end again from each opening.
	{"r", "`\\"},
	{"r", "\"\\"},
	{"r", "'\\"},
	// PHP: the same for '…' and `…`.
	{"php", "\\'"},
	{"php", "`\\"},
	// YAML: each " looked for the last ": " of its line, and for the line end, again.
	{"yaml", "}\""},
	{"yaml", "\t\""},
	// XML and HTML: each <?, <! or <![CDATA[ looked for its end again.
	{"xml", "<?"},
	{"xml", "!<"},
	{"xml", "<![CDATA["},
	{"html", "<?"},
	{"html", "!<"},
	{"html", "<![CDATA["},
	// C and C++: a comment at a line start checked the text up to each */ for a newline again.
	{"c", "/*"},
	{"cpp", "/*"},
	// Java and JSON: each // looked for the end of the line again.
	{"java", "/"},
	{"json", "/"},
	// Markdown: each [ looked for ] again.
	{"markdown", "["},
	{"markdown", "!["},
	// Kotlin and C#: each [ at a line start looked for ] again.
	{"kotlin", "[\n"},
	{"csharp", "[\n"},
}

// TestFindSlowInputs looks for inputs that may make a tokenizer take more than linear time, in every language:
// each one- and two-character combination of a small alphabet, repeated to 64 KB. It lists those that take at
// least 30 ms, the slowest first, with the time at 32 KB for comparison. It takes about 6 minutes, so it runs
// only when FASTHIGHLIGHT_SCAN is set:
//
//	FASTHIGHLIGHT_SCAN=1 go test -run TestFindSlowInputs -v -timeout 30m .
//
// A listed input is only a candidate: many are linear but slow (bounded scans), and single measurements are
// noisy. Confirm one by timing it at several sizes, best of three (64 KB to 4 MB: a linear tokenizer takes
// roughly 64 to 150 times as long, a quadratic one about 4000 times), then fix it and add it to rescanCases.
func TestFindSlowInputs(t *testing.T) {
	if os.Getenv("FASTHIGHLIGHT_SCAN") == "" {
		t.Skip("set FASTHIGHLIGHT_SCAN=1 to run it (about 6 minutes)")
	}
	alpha := []string{"a", "A", "1", " ", "\n", "\t", "é", "<", ">", "(", ")", "[", "]", "{", "}", "\"", "'", "`", "/",
		"\\", "*", "#", "$", "@", ":", ";", ",", ".", "-", "+", "=", "&", "|", "!", "?", "%", "^", "~", "_"}
	units := append([]string{}, alpha...)
	for _, a := range alpha {
		for _, b := range alpha {
			if a != b {
				units = append(units, a+b)
			}
		}
	}
	timed := func(l *fasthighlight.Language, u string, size int) time.Duration {
		src := strings.Repeat(u, size/len(u))
		start := time.Now()
		l.Tokenise(src)
		return time.Since(start)
	}
	type hit struct {
		lang, unit string
		half, full time.Duration
	}
	var hits []hit
	for _, l := range fasthighlight.Languages() {
		for _, u := range units {
			if d := timed(l, u, 64<<10); d >= 30*time.Millisecond {
				hits = append(hits, hit{l.Name, u, timed(l, u, 32<<10), d})
			}
		}
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].full > hits[j].full })
	for _, h := range hits {
		t.Logf("%-12s %-8q 32 KB %-10v 64 KB %v", h.lang, h.unit, h.half.Round(time.Millisecond), h.full.Round(time.Millisecond))
	}
	t.Logf("%d inputs took 30 ms or more at 64 KB", len(hits))
}
