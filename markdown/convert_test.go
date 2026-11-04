package markdown_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	_ "github.com/toshi-kuji/fasthighlight/lang/all"
	. "github.com/toshi-kuji/fasthighlight/markdown"
)

// 3. Parallel and sequential give the same output.
func TestParallelEqualsSequential(t *testing.T) {
	src, err := os.ReadFile("../testdata/input.md")
	if err != nil {
		t.Fatal(err)
	}
	var seq bytes.Buffer
	if err := newSequential().Convert(src, &seq); err != nil {
		t.Fatal(err)
	}
	par := NewConverter()
	par.Parallelism = ParallelAlways
	for i := 0; i < 30; i++ {
		var out bytes.Buffer
		if err := par.Convert(src, &out); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(out.Bytes(), seq.Bytes()) {
			t.Fatalf("run %d: parallel output differs from sequential", i)
		}
	}
	// A document with many languages (with always-parallel, and with the default).
	multi, err := os.ReadFile("../testdata/multilang.md")
	if err != nil {
		t.Fatal(err)
	}
	var want bytes.Buffer
	if err := newSequential().Convert(multi, &want); err != nil {
		t.Fatal(err)
	}
	for _, c := range []*Converter{par, NewConverter()} {
		var out bytes.Buffer
		if err := c.Convert(multi, &out); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(out.Bytes(), want.Bytes()) {
			t.Fatalf("parallelism %d: multilang.md output differs from sequential", c.Parallelism)
		}
	}
	if !bytes.Contains(seq.Bytes(), []byte(`<span class="kr">const</span>`)) {
		t.Errorf("output does not look highlighted")
	}
}

// 4. The conversion result of the sample document matches the output produced with Chroma's writer
// (testdata/input.html). testdata/input.html was made with Chroma 2.27.0's formatters/html
// (WithClasses(true)). The tests that compare directly with Chroma are in compat/.
func TestConvertMatchesGolden(t *testing.T) {
	src, err := os.ReadFile("../testdata/input.md")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("../testdata/input.html")
	if err != nil {
		t.Fatal(err)
	}
	var got bytes.Buffer
	if err := newSequential().Convert(src, &got); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Bytes(), want) {
		g, w := got.Bytes(), want
		k := 0
		for k < len(g) && k < len(w) && g[k] == w[k] {
			k++
		}
		t.Fatalf("output differs from testdata/input.html at byte %d\n got %q\nwant %q",
			k, g[k:min(len(g), k+80)], w[k:min(len(w), k+80)])
	}
}

// newSequential returns a Converter that always highlights sequentially.
func newSequential() *Converter {
	c := NewConverter()
	c.Parallelism = ParallelNever
	return c
}

// For a code block's language name, only the first word up to the first whitespace is used (the same as
// goldmark's Language). Uppercase language names and extensions are also highlighted. A language that is
// not registered is only escaped.
func TestConvertLanguageName(t *testing.T) {
	const kr = `<span class="kr">const</span>`
	tests := []struct {
		md          string
		highlighted bool
	}{
		{"```ts title=\"app.ts\"\nconst x = 1\n```\n", true},
		{"```TS\nconst x = 1\n```\n", true},
		{"```mts\nconst x = 1\n```\n", true},
		{"```go\nconst x = 1\n```\n", false},
		{"```\nconst x = 1\n```\n", false},
	}
	for _, tt := range tests {
		var b bytes.Buffer
		if err := NewConverter().Convert([]byte(tt.md), &b); err != nil {
			t.Fatal(err)
		}
		if got := strings.Contains(b.String(), kr); got != tt.highlighted {
			t.Errorf("%q: highlighted = %v, want %v\n%s", tt.md, got, tt.highlighted, b.String())
		}
	}
}
