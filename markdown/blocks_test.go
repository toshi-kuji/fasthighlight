package markdown_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	_ "github.com/toshi-kuji/fasthighlight/lang/all"
	"github.com/toshi-kuji/fasthighlight/markdown"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"
)

// TestCodeBlocks checks CodeBlocks: the fields of each block, and that Open + Code + Close is the HTML Render
// writes, sequentially and in parallel.
func TestCodeBlocks(t *testing.T) {
	src := "# Doc\n\n```go\nx := \"<a>\"\n```\n\n- item\n\n  ```rust\n  fn main() {}\n  ```\n\n" +
		"```\nno language\n```\n\n```text\nlog\n```\n\n~~~cobol\nDISPLAY 1.\n~~~\n\n```go\n```\n\n    indented, not fenced\n"
	type want struct {
		line    int
		lang    string
		colored bool
		reason  string
	}
	wants := []want{{3, "go", true, ""}, {9, "rust", true, ""}, {13, "", false, "no-language"}, {17, "text", false, "plain"},
		{21, "cobol", false, "unsupported"}, {25, "go", true, ""}}
	for _, p := range []markdown.Parallelism{markdown.ParallelNever, markdown.ParallelAlways} {
		c := markdown.NewConverter()
		c.Parallelism = p
		d := c.Parse("doc.md", []byte(src))
		bs := c.CodeBlocks(d)
		if len(bs) != len(wants) {
			t.Fatalf("parallelism %d: %d blocks, want %d", p, len(bs), len(wants))
		}
		var html bytes.Buffer
		if err := c.Render(&html, d); err != nil {
			t.Fatal(err)
		}
		rest := html.String()
		for i, w := range wants {
			b := bs[i]
			if b.Document != "doc.md" || b.Line != w.line || b.Lang != w.lang || b.Colored != w.colored || b.Reason != w.reason ||
				!strings.HasPrefix(b.Open, `<pre class="highlight"`) || b.Close != "</code></pre>" || strings.Contains(b.Code, "<code") {
				t.Errorf("parallelism %d, block %d: %+v", p, i, b)
			}
			// The blocks appear in Render's output in the same order.
			k := strings.Index(rest, b.Open+b.Code+b.Close)
			if k < 0 {
				t.Errorf("parallelism %d, block %d is not in Render's output: %q", p, i, b.Open+b.Code+b.Close)
				continue
			}
			rest = rest[k+len(b.Open+b.Code+b.Close):]
		}
	}
	if bs := markdown.NewConverter().CodeBlocks(markdown.NewConverter().Parse("", []byte("no code\n"))); len(bs) != 0 {
		t.Errorf("blocks in a document without code: %v", bs)
	}
}

// TestNewDocument checks that CodeBlocks on a document parsed with your own goldmark (with or without
// Extension) gives the same blocks as on Parse's, sequentially and in parallel.
func TestNewDocument(t *testing.T) {
	var src bytes.Buffer
	src.WriteString("# Doc\n\n")
	for i := range 40 { // more than ParallelMinBytes of code, so that ParallelAuto goes parallel
		fmt.Fprintf(&src, "```go\nfunc f%d() { return \"%s\" }\n```\n\n```cobol\nDISPLAY %d.\n```\n\n", i, strings.Repeat("x", 1000), i)
	}
	mds := map[string]goldmark.Markdown{
		"plain":     goldmark.New(goldmark.WithExtensions(extension.GFM)),
		"extension": goldmark.New(goldmark.WithExtensions(extension.GFM, &markdown.Extension{})),
	}
	for _, p := range []markdown.Parallelism{markdown.ParallelAuto, markdown.ParallelNever, markdown.ParallelAlways} {
		c := markdown.NewConverter()
		c.Parallelism = p
		want := c.CodeBlocks(c.Parse("doc.md", src.Bytes()))
		for name, md := range mds {
			root := md.Parser().Parse(text.NewReader(src.Bytes()))
			got := c.CodeBlocks(markdown.NewDocument("doc.md", src.Bytes(), root))
			if fmt.Sprint(got) != fmt.Sprint(want) {
				t.Errorf("parallelism %d, %s goldmark: the blocks differ from Parse's", p, name)
			}
			if r := markdown.Scan(markdown.NewDocument("doc.md", src.Bytes(), root)); len(r.Unsupported) != 40 {
				t.Errorf("parallelism %d, %s goldmark: Scan found %d unsupported blocks, want 40", p, name, len(r.Unsupported))
			}
		}
	}
}

// NewDocument lets CodeBlocks work on a document parsed with your own goldmark, so it is parsed once.
func ExampleNewDocument() {
	src := []byte("# Title\n\n```go\nx := 1\n```\n")
	md := goldmark.New(goldmark.WithExtensions(extension.GFM)) // your own goldmark, with your own settings
	root := md.Parser().Parse(text.NewReader(src))
	// ... render root with md as you do now ...
	for _, b := range markdown.NewConverter().CodeBlocks(markdown.NewDocument("page.md", src, root)) {
		fmt.Println(b.Line, b.Lang, b.Code)
	}
	// Output:
	// 3 go <span class="nx">x</span> <span class="o">:=</span> <span class="mi">1</span>
}

// CodeBlocks gives the highlighted code blocks of a document, for a site that converts the Markdown itself.
func ExampleConverter_CodeBlocks() {
	c := markdown.NewConverter()
	d := c.Parse("page.md", []byte("# Title\n\n```go\nx := 1\n```\n\n```cobol\nDISPLAY 1.\n```\n"))
	for _, b := range c.CodeBlocks(d) {
		fmt.Printf("line %d, %s, colored: %v", b.Line, b.Lang, b.Colored)
		if b.Reason != "" {
			fmt.Printf(" (%s)", b.Reason)
		}
		fmt.Println()
		fmt.Print(b.Code)
	}
	// Output:
	// line 3, go, colored: true
	// <span class="nx">x</span> <span class="o">:=</span> <span class="mi">1</span>
	// line 7, cobol, colored: false (unsupported)
	// DISPLAY 1.
}
