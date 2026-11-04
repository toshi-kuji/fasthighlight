package markdown_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/toshi-kuji/fasthighlight/internal/testcases"
	_ "github.com/toshi-kuji/fasthighlight/lang/all"
	. "github.com/toshi-kuji/fasthighlight/markdown"
)

// Fuzz test of the whole conversion. Inputs that are found are put in testdata/fuzz/FuzzConvert/ and also run
// as regression tests in the ordinary go test.
//
//	go test -run='^$' -fuzz=FuzzConvert -fuzztime=1m ./markdown

// Whole conversion: no panic and no error. Parallel and sequential give the same output.
func FuzzConvert(f *testing.F) {
	src, err := os.ReadFile("../testdata/input.md")
	if err != nil {
		f.Fatal(err)
	}
	f.Add(src)
	seeds, err := testcases.InputBlocks("../testdata/input.md")
	if err != nil {
		f.Fatal(err)
	}
	for _, b := range append(seeds, testcases.EdgeCases...) {
		f.Add([]byte("# t\n\n```" + b.Lang + "\n" + b.Code + "```\n\ntext `code` *em*\n"))
	}
	f.Add([]byte("```ts\nconst x = 1\n"))           // an unclosed code block
	f.Add([]byte("~~~js title=\"a.js\"\nx\n~~~\n")) // info after the language name
	f.Add([]byte("```\nno lang\n```\n```go\nfunc f() {}\n```\n"))
	seq, par := newSequential(), NewConverter()
	par.Parallelism = ParallelAlways
	f.Fuzz(func(t *testing.T, src []byte) {
		var a, b bytes.Buffer
		if err := seq.Convert(src, &a); err != nil {
			t.Fatalf("sequential: %v", err)
		}
		if err := par.Convert(src, &b); err != nil {
			t.Fatalf("parallel: %v", err)
		}
		if !bytes.Equal(a.Bytes(), b.Bytes()) {
			t.Fatalf("parallel output differs from sequential")
		}
	})
}
