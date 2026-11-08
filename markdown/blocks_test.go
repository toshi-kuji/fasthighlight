package markdown_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	_ "github.com/toshi-kuji/fasthighlight/lang/all"
	"github.com/toshi-kuji/fasthighlight/markdown"
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
