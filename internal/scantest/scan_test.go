// Package scantest checks markdown.Scan in a program that imports only some languages, so that a language
// fasthighlight supports but the program does not import really is "not built in".
package scantest

import (
	"reflect"
	"testing"

	_ "github.com/toshi-kuji/fasthighlight/lang/golang"
	"github.com/toshi-kuji/fasthighlight/markdown"
)

// A language fasthighlight supports but that is not imported goes to NotBuiltIn with the package to import,
// and a language fasthighlight does not support goes to Unsupported.
func TestScanNotBuiltIn(t *testing.T) {
	c := markdown.NewConverter()
	r := markdown.Scan(c.Parse("d.md", []byte("```go\nx\n```\n\n```rs\ny\n```\n\n```main.rs\nz\n```\n\n```hs\nw\n```\n")))
	if len(r.Used) != 1 || r.Used[0].Name != "Go" {
		t.Errorf("Used = %v, want Go only", r.Used)
	}
	const rust = "github.com/toshi-kuji/fasthighlight/lang/rust"
	want := []markdown.CodeBlock{
		{Document: "d.md", Line: 5, Lang: "rs", Package: rust},
		{Document: "d.md", Line: 9, Lang: "main.rs", Package: rust},
	}
	if !reflect.DeepEqual(r.NotBuiltIn, want) {
		t.Errorf("NotBuiltIn = %+v, want %+v", r.NotBuiltIn, want)
	}
	if want := []markdown.CodeBlock{{Document: "d.md", Line: 13, Lang: "hs"}}; !reflect.DeepEqual(r.Unsupported, want) {
		t.Errorf("Unsupported = %+v, want %+v", r.Unsupported, want)
	}
}
