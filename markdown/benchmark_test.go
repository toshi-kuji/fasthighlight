package markdown_test

import (
	"io"
	"os"
	"testing"

	_ "github.com/toshi-kuji/fasthighlight/lang/all"
	. "github.com/toshi-kuji/fasthighlight/markdown"
)

// Compares the speed of Convert with Parse + Scan + Render (the usage that checks the languages before conversion).
//
//	go test -run='^$' -bench=. -benchmem ./markdown
func BenchmarkConvert(b *testing.B) {
	src := readInput(b)
	c := NewConverter()
	for b.Loop() {
		if err := c.Convert(src, io.Discard); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseScanRender(b *testing.B) {
	src := readInput(b)
	c := NewConverter()
	for b.Loop() {
		d := c.Parse("input.md", src)
		Scan(d)
		if err := c.Render(io.Discard, d); err != nil {
			b.Fatal(err)
		}
	}
}

func readInput(b *testing.B) []byte {
	b.Helper()
	src, err := os.ReadFile("../testdata/input.md")
	if err != nil {
		b.Fatal(err)
	}
	return src
}
