package golang

import (
	"fmt"
	"strings"
	"testing"
	"time"

	fh "github.com/toshi-kuji/fasthighlight"
)

// TestFieldTypes checks the colors of the names in struct bodies. want lists the names and their types in
// order; each case is a struct body unless it starts with "!".
func TestFieldTypes(t *testing.T) {
	Prepare()
	type name struct {
		text string
		t    fh.TokenType
	}
	other, class, ns := fh.NameOther, fh.NameClass, fh.NameNamespace
	cases := []struct {
		src  string
		want []name
	}{
		{"reqCtx context.Context", []name{{"reqCtx", other}, {"context", ns}, {"Context", class}}},
		{"Storage *storage.Client", []name{{"Storage", other}, {"storage", ns}, {"Client", class}}},
		{"env AppEnv", []name{{"env", other}, {"AppEnv", class}}},
		{"a, b Point", []name{{"a", other}, {"b", other}, {"Point", class}}},
		{"io.Reader", []name{{"io", ns}, {"Reader", class}}},
		{"*Base", []name{{"Base", class}}},
		{"Base // embedded", []name{{"Base", class}}},
		{"Base `json:\"b\"`", []name{{"Base", class}}},
		{"m map[Key]*pkg.Value `json:\"m\"`", []name{{"m", other}, {"Key", class}, {"pkg", ns}, {"Value", class}}},
		{"xs []Item", []name{{"xs", other}, {"Item", class}}},
		{"arr [N]Item", []name{{"arr", other}, {"N", other}, {"Item", class}}},
		{"l List[T, pkg.U]", []name{{"l", other}, {"List", class}, {"T", class}, {"pkg", ns}, {"U", class}}},
		{"List[T]", []name{{"List", class}, {"T", class}}},
		{"ch <-chan Event", []name{{"ch", other}, {"Event", class}}},
		{"fn func(x Arg) Result", []name{{"fn", other}, {"x", other}, {"Arg", other}, {"Result", class}}},
		{"in struct{ X Y }", []name{{"in", other}, {"X", other}, {"Y", class}}},
		{"in struct {\n\t\tX Y\n\t}\n\tZ W", []name{{"in", other}, {"X", other}, {"Y", class}, {"Z", other}, {"W", class}}},
		{"i interface{ M(a A) B }", []name{{"i", other}, {"a", other}, {"A", other}, {"B", other}}},
		{"a A; b B", []name{{"a", other}, {"A", class}, {"b", other}, {"B", class}}},
		{"t Tag `x:\"{{ .Name }}\"`\n\tu U", []name{{"t", other}, {"Tag", class}, {"u", other}, {"U", class}}},
		{"a /* c */ Foo", []name{{"a", other}, {"Foo", class}}},
		{"a func(\n\tb Foo\n\tc Bar", []name{{"a", other}, {"b", other}, {"Foo", class}, {"c", other}, {"Bar", class}}},
		{"a [\n\tb Foo", []name{{"a", other}, {"b", other}, {"Foo", class}}},
		{"x int `a:\"b\"`\n\ty Foo", []name{{"x", other}, {"y", other}, {"Foo", class}}},
		// Outside struct bodies, nothing changes.
		{"!var x Foo\nfunc f(a Bar) Baz { return Baz{A: b} }\n", []name{{"x", other}, {"Foo", other}, {"a", other},
			{"Bar", other}, {"Baz", other}, {"Baz", other}, {"A", other}, {"b", other}}},
		{"!s := struct{ A B }{A: c}\n", []name{{"s", other}, {"A", other}, {"B", class}, {"A", other}, {"c", other}}},
	}
	for _, c := range cases {
		src := c.src
		if strings.HasPrefix(src, "!") {
			src = src[1:]
		} else {
			src = "type T struct {\n\t" + src + "\n}\n"
		}
		var got []name
		for _, tk := range Lex(src, len(src), Options{}) {
			if tk.Type == other || tk.Type == class || tk.Type == ns {
				got = append(got, name{tk.Value, tk.Type})
			}
		}
		if src != c.src[1:] {
			got = got[1:] // the T of type T struct
		}
		if fmt.Sprint(got) != fmt.Sprint(c.want) {
			t.Errorf("%q:\n got %v\nwant %v", src, got, c.want)
		}
		// With ChromaFieldTypes, every name stays NameOther.
		for _, tk := range Lex(src, len(src), Options{ChromaFieldTypes: true}) {
			if tk.Type == class || tk.Type == ns {
				t.Errorf("%q with ChromaFieldTypes: %q is %v", src, tk.Value, tk.Type)
			}
		}
	}
}

// TestFieldTypesDeepBrackets checks that a long run of unclosed brackets in a field type does not take
// quadratic time.
func TestFieldTypesDeepBrackets(t *testing.T) {
	Prepare()
	for _, open := range []string{"(", "[", "[["} {
		src := "type T struct {\n\ta func" + strings.Repeat(open, 200000)
		start := time.Now()
		Lex(src, len(src), Options{})
		if d := time.Since(start); d > 2*time.Second {
			t.Errorf("%q repeated: %v", open, d)
		}
	}
}
