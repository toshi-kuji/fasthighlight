package fasthighlight_test

import (
	"testing"

	. "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/testcases"
	_ "github.com/toshi-kuji/fasthighlight/lang/all"
)

// Fuzz tests. The seeds are the sample document's code blocks and the edge cases.
// Inputs that are found are put in testdata/fuzz/<function name>/ and also run as regression tests in the ordinary go test.
//
//	go test -run='^$' -fuzz=FuzzTokenise -fuzztime=1m .

func fuzzSeeds(f *testing.F) []testcases.Block {
	f.Helper()
	bs, err := testcases.InputBlocks("testdata/input.md")
	if err != nil {
		f.Fatal(err)
	}
	return append(bs, testcases.EdgeCases...)
}

// langIndex returns the position of l among the registered languages (in name order).
func langIndex(l *Language) uint8 {
	for i, x := range Languages() {
		if x == l {
			return uint8(i)
		}
	}
	panic("language not registered: " + l.Name)
}

// Tokenizer: for all registered languages, no panic. Joining the tokens matches the input (it may include
// the newline appended by EnsureNewline). There are no empty tokens or adjacent tokens of the same type.
func FuzzTokenise(f *testing.F) {
	for _, b := range fuzzSeeds(f) {
		f.Add(langIndex(LookupRegistered(b.Lang)), b.Code)
	}
	langs := Languages()
	f.Fuzz(func(t *testing.T, lang uint8, code string) {
		l := langs[int(lang)%len(langs)]
		checkTokens(t, l, code, l.Tokenise(code))
	})
}
