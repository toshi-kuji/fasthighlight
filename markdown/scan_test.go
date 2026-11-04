package markdown_test

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"

	_ "github.com/toshi-kuji/fasthighlight/lang/all"
	. "github.com/toshi-kuji/fasthighlight/markdown"
)

// Language names of several documents are checked together, and the languages to highlight are separated from the blocks that are not highlighted.
func TestScan(t *testing.T) {
	srcA := []byte("# A\n\n```ts\nx\n```\n\n```TS title=\"b.ts\"\ny\n```\n\n```fortran\nprogram p\n```\n\n```\nplain\n```\n")
	srcB := []byte("text\n\n- item\n\n  ```cobol\n  DISPLAY 1.\n  ```\n\n~~~mjs\nz\n~~~\n")
	c := NewConverter()
	a, b := c.Parse("a.md", srcA), c.Parse("b.md", srcB)
	r := Scan(a, b)

	var used []string
	for _, l := range r.Used {
		used = append(used, l.Name)
	}
	if want := []string{"JavaScript", "TypeScript"}; !reflect.DeepEqual(used, want) {
		t.Errorf("Used = %v, want %v", used, want)
	}
	wantUnsupported := []CodeBlock{{Document: "a.md", Line: 11, Lang: "fortran"}, {Document: "b.md", Line: 5, Lang: "cobol"}}
	if !reflect.DeepEqual(r.Unsupported, wantUnsupported) {
		t.Errorf("Unsupported = %+v, want %+v", r.Unsupported, wantUnsupported)
	}
	if want := []CodeBlock{{Document: "a.md", Line: 15}}; !reflect.DeepEqual(r.NoLanguage, want) {
		t.Errorf("NoLanguage = %+v, want %+v", r.NoLanguage, want)
	}
	if len(r.NotBuiltIn) != 0 {
		t.Errorf("NotBuiltIn = %+v, want none (lang/all is imported)", r.NotBuiltIn)
	}
	if !r.Uncolored() {
		t.Errorf("Uncolored() = false, want true")
	}

	// Even after Scan, the output of Render is the same as Convert.
	for d, src := range map[*Document][]byte{a: srcA, b: srcB} {
		var got, want bytes.Buffer
		if err := c.Render(&got, d); err != nil {
			t.Fatal(err)
		}
		if err := NewConverter().Convert(src, &want); err != nil {
			t.Fatal(err)
		}
		if got.String() != want.String() {
			t.Errorf("%s: Render after Scan differs from Convert", d.Name)
		}
	}
}

// When all documents can be highlighted, Uncolored is false.
func TestScanAllColored(t *testing.T) {
	c := NewConverter()
	r := Scan(c.Parse("x.md", []byte("```js\nx\n```\n")))
	if r.Uncolored() || len(r.Used) != 1 || r.Used[0].Name != "JavaScript" {
		t.Errorf("Scan = %+v", r)
	}
}

func ExampleScan() {
	c := NewConverter()
	doc := c.Parse("guide.md", []byte("```ts\nconst x = 1\n```\n\n```cobol\nDISPLAY 1.\n```\n\n```\nplain\n```\n"))

	// Report the blocks that are not highlighted before conversion.
	r := Scan(doc)
	for _, b := range r.Unsupported {
		fmt.Printf("%s:%d: %q は対象外の言語なので、色付けしない\n", b.Document, b.Line, b.Lang)
	}
	for _, b := range r.NoLanguage {
		fmt.Printf("%s:%d: 言語名がないので、色付けしない\n", b.Document, b.Line)
	}

	var out bytes.Buffer
	_ = c.Render(&out, doc)
	// Output:
	// guide.md:5: "cobol" は対象外の言語なので、色付けしない
	// guide.md:9: 言語名がないので、色付けしない
}
