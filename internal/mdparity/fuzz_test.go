package mdparity_test

import (
	"os"
	"testing"

	"github.com/toshi-kuji/fasthighlight/internal/testcases"
	md1 "github.com/toshi-kuji/fasthighlight/markdown"
	md2 "github.com/toshi-kuji/fasthighlight/markdown/v2"
)

// Fuzz test of the whole conversion. Inputs that are found are put in testdata/fuzz/FuzzSameAsV1/ and also run
// as regression tests in the ordinary go test.
//
//	go test -run='^$' -fuzz=FuzzSameAsV1 -fuzztime=1m ./internal/mdparity

// When goldmark v1 and v2 find the same code blocks with the same contents, the two packages read the same
// language names and highlight them the same way. (Parallel against sequential is markdown/v2's FuzzConvert.)
func FuzzSameAsV1(f *testing.F) {
	src, err := os.ReadFile("../../testdata/input.md")
	if err != nil {
		f.Fatal(err)
	}
	f.Add(src)
	seeds, err := testcases.InputBlocks("../../testdata/input.md")
	if err != nil {
		f.Fatal(err)
	}
	for _, b := range append(seeds, testcases.EdgeCases...) {
		f.Add([]byte("# t\n\n```" + b.Lang + "\n" + b.Code + "```\n\ntext `code` *em*\n"))
	}
	f.Add([]byte("```ts\nconst x = 1\n"))           // an unclosed code block
	f.Add([]byte("~~~js title=\"a.js\"\nx\n~~~\n")) // info after the language name
	f.Add([]byte("```\nno lang\n```\n```go\nfunc f() {}\n```\n"))
	f.Add([]byte(testcases.MarkdownEdge))
	f.Add([]byte("```&#x0000063;\nx\n```\n"))
	v1, v2 := md1.NewConverter(), md2.NewConverter()
	f.Fuzz(func(t *testing.T, src []byte) {
		h1, h2 := v1.CodeBlocks(v1.Parse("", src)), v2.CodeBlocks(v2.Parse("", src))
		if len(h1) != len(h2) {
			return // goldmark v1 and v2 parse this differently; not this package's concern
		}
		for k := range h1 {
			if string(h1[k].Node.Lines().Value(src)) != h2[k].Node.Value.Str(src) {
				continue // goldmark v1 and v2 cut this block's contents differently
			}
			// The language name is read by this package's own code in each, so it must agree.
			if h1[k].Lang != h2[k].Lang || h1[k].Open+h1[k].Code+h1[k].Close != h2[k].Open+h2[k].Code+h2[k].Close {
				t.Fatalf("block %d: v1 %q %q, v2 %q %q", k, h1[k].Lang, h1[k].Code, h2[k].Lang, h2[k].Code)
			}
		}
	})
}
