package markdown_test

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/toshi-kuji/fasthighlight/internal/testcases"
	_ "github.com/toshi-kuji/fasthighlight/lang/all"
	md2 "github.com/toshi-kuji/fasthighlight/markdown/v2"
	"github.com/yuin/goldmark/v2/extension"
	"github.com/yuin/goldmark/v2/parser"
	"github.com/yuin/goldmark/v2/renderer/html"
)

// The output for testdata/input.md is the golden file the v1 package is checked against.
func TestConvertMatchesGolden(t *testing.T) {
	src, err := os.ReadFile("../../testdata/input.md")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("../../testdata/input.html")
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := md2.NewConverter().Convert(src, &out); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out.Bytes(), want) {
		t.Errorf("output differs from testdata/input.html\n%s", testcases.FirstDiff(string(want), out.String()))
	}
}

// The empty block at the end of testcases.MarkdownEdge has its line in v2.
func TestEmptyBlockLine(t *testing.T) {
	src := []byte("text\n\n```\n```\n")
	c := md2.NewConverter()
	bs := c.CodeBlocks(c.Parse("", src))
	if len(bs) != 1 || bs[0].Line != 3 {
		t.Fatalf("blocks = %+v, want one block on line 3", bs)
	}
}

// Extension in your own goldmark v2 configuration gives the same HTML as Converter, highlights only fenced
// blocks, and lets NewDocument and CodeBlocks work on what your parser returned.
func TestExtensionInOwnConfig(t *testing.T) {
	src := []byte(testcases.MarkdownEdge)
	ext := &md2.Extension{Parallelism: md2.ParallelAlways}
	p := parser.New(parser.WithExtensions(extension.NewGFMParser(), ext))
	r := html.New(html.WithExtensions(extension.NewGFMHTMLRenderer(), ext))
	root := p.Parse(src)
	var got, want bytes.Buffer
	if err := r.Render(&got, src, root); err != nil {
		t.Fatal(err)
	}
	if err := md2.NewConverter().Convert(src, &want); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Bytes(), want.Bytes()) {
		t.Errorf("own configuration differs from Converter\n%s", testcases.FirstDiff(want.String(), got.String()))
	}
	if !strings.Contains(got.String(), "<pre><code>indented &lt;code&gt; &amp; stuff\n</code></pre>") {
		t.Errorf("indented code block was not left to goldmark:\n%s", got.String())
	}
	c := md2.NewConverter()
	bs := c.CodeBlocks(md2.NewDocument("own", src, root))
	if len(bs) != 11 || bs[0].Lang != "go" || !bs[0].Colored || bs[0].Node == nil {
		t.Errorf("CodeBlocks on NewDocument = %d blocks, first %+v", len(bs), bs[0])
	}
}

// Parallel and sequential give the same output, also when several goroutines share one renderer.
func TestParallelEqualsSequentialConcurrent(t *testing.T) {
	src, err := os.ReadFile("../../testdata/multilang.md")
	if err != nil {
		t.Fatal(err)
	}
	seq := md2.NewConverter()
	seq.Parallelism = md2.ParallelNever
	var want bytes.Buffer
	if err := seq.Convert(src, &want); err != nil {
		t.Fatal(err)
	}
	c := md2.NewConverter()
	c.Parallelism = md2.ParallelAlways
	d := c.Parse("", src)
	errs := make(chan error, 8)
	for range 8 {
		go func() {
			var out bytes.Buffer
			if err := c.Render(&out, d); err != nil {
				errs <- err
				return
			}
			if !bytes.Equal(out.Bytes(), want.Bytes()) {
				errs <- fmt.Errorf("concurrent render differs from sequential")
				return
			}
			errs <- nil
		}()
	}
	for range 8 {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
}
