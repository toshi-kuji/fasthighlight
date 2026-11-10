package markdown_test

import (
	"bytes"
	"fmt"
	"os"
	"sync"
	"testing"

	_ "github.com/toshi-kuji/fasthighlight/lang/all"
	. "github.com/toshi-kuji/fasthighlight/markdown"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"
)

// A conversion with Extension added to your own goldmark gives the same HTML as Converter (sequential or parallel).
// One goldmark.Markdown can be used from several goroutines at the same time.
func TestExtensionMatchesConverter(t *testing.T) {
	multi, err := os.ReadFile("../testdata/multilang.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"../testdata/input.md", "../testdata/multilang.md", "multilang.md x2"} {
		var src []byte
		if path == "multilang.md x2" { // the code exceeds 32KB, so it goes parallel even with ParallelAuto
			src = bytes.Repeat(multi, 2)
		} else if src, err = os.ReadFile(path); err != nil {
			t.Fatal(err)
		}
		var want bytes.Buffer
		if err := newSequential().Convert(src, &want); err != nil {
			t.Fatal(err)
		}
		for _, p := range []Parallelism{ParallelAuto, ParallelNever, ParallelAlways} {
			md := goldmark.New(goldmark.WithExtensions(extension.GFM, &Extension{Parallelism: p}))
			var wg sync.WaitGroup
			for range 4 {
				wg.Add(1)
				go func() {
					defer wg.Done()
					var got bytes.Buffer
					if err := md.Convert(src, &got); err != nil {
						t.Error(err)
						return
					}
					if !bytes.Equal(got.Bytes(), want.Bytes()) {
						t.Errorf("%s, parallelism %d: output differs from Converter", path, p)
					}
				}()
			}
			wg.Wait()
		}
	}
}

// It can also be added to a goldmark without GFM.
func ExampleExtension() {
	md := goldmark.New(goldmark.WithExtensions(&Extension{}))
	var out bytes.Buffer
	if err := md.Convert([]byte("Hello\n\n```go\nx := 1\n```\n"), &out); err != nil {
		panic(err)
	}
	fmt.Print(out.String())
	// Output:
	// <p>Hello</p>
	// <pre class="highlight" data-lang="go"><code class="language-go"><span class="nx">x</span> <span class="o">:=</span> <span class="mi">1</span>
	// </code></pre>
}

// The language name is the first word of the info string, up to a space or a tab, with backslash escapes and
// entity references resolved as CommonMark does.
func TestLanguageName(t *testing.T) {
	for _, c := range []struct{ info, want string }{
		{"go", "go"},
		{"go title=x", "go"},
		{"go\ttitle=x", "go"},
		{"  go", "go"},
		{`c\+\+`, "c++"},
		{"c&#43;&#x2B;", "c++"},
		{"f&num;", "f#"},
		{`\&amp;`, "&amp;"},  // the escape makes the & literal, so the entity is not resolved
		{"&#92;&amp;", `\&`}, // a resolved \ does not escape what follows
		{"a&b", "a&b"},       // not an entity
		{"a&nosuch;", "a&nosuch;"},
		{"", ""},
	} {
		src := []byte("```" + c.info + "\nx\n```\n")
		doc := goldmark.New().Parser().Parse(text.NewReader(src))
		b, ok := doc.FirstChild().(*ast.FencedCodeBlock)
		if !ok {
			t.Fatalf("%q: no fenced code block", c.info)
		}
		if got := LanguageName(b, src); got != c.want {
			t.Errorf("%q: got %q, want %q", c.info, got, c.want)
		}
	}
}

// A block whose info string separates the language with a tab is highlighted.
func TestLanguageAfterTab(t *testing.T) {
	var tab, space bytes.Buffer
	md := goldmark.New(goldmark.WithExtensions(&Extension{}))
	if err := md.Convert([]byte("```go\tx\nx := 1\n```\n"), &tab); err != nil {
		t.Fatal(err)
	}
	if err := md.Convert([]byte("```go x\nx := 1\n```\n"), &space); err != nil {
		t.Fatal(err)
	}
	if tab.String() != space.String() {
		t.Errorf("after a tab:\n%s\nafter a space:\n%s", tab.String(), space.String())
	}
}
