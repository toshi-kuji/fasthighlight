package golang

import (
	"fmt"
	"strings"
	"testing"
	"time"

	fh "github.com/toshi-kuji/fasthighlight"
)

// TestTypeNames checks the colors of the names in type positions. want lists the names and their types in
// order; each case is a struct body unless it starts with "!".
func TestTypeNames(t *testing.T) {
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
		{"fn func(x Arg) Result", []name{{"fn", other}, {"x", other}, {"Arg", class}, {"Result", class}}},
		{"in struct{ X Y }", []name{{"in", other}, {"X", other}, {"Y", class}}},
		{"in struct {\n\t\tX Y\n\t}\n\tZ W", []name{{"in", other}, {"X", other}, {"Y", class}, {"Z", other}, {"W", class}}},
		{"i interface{ M(a A) B }", []name{{"i", other}, {"a", other}, {"A", class}, {"B", class}}},
		{"a A; b B", []name{{"a", other}, {"A", class}, {"b", other}, {"B", class}}},
		{"t Tag `x:\"{{ .Name }}\"`\n\tu U", []name{{"t", other}, {"Tag", class}, {"u", other}, {"U", class}}},
		{"a /* c */ Foo", []name{{"a", other}, {"Foo", class}}},
		{"a func(\n\tb Foo\n\tc Bar", []name{{"a", other}, {"b", other}, {"Foo", class}, {"c", other}, {"Bar", other}}},
		{"a [\n\tb Foo", []name{{"a", other}, {"b", other}, {"Foo", class}}},
		{"x int `a:\"b\"`\n\ty Foo", []name{{"x", other}, {"y", other}, {"Foo", class}}},
		// Declarations, parameters, results and receivers (the examples of the feedback first).
		{"!var c *http.Client\nvar d time.Duration\nfunc f(r *http.Request) error { return nil }\n", []name{{"c", other},
			{"http", ns}, {"Client", class}, {"d", other}, {"time", ns}, {"Duration", class}, {"r", other}, {"http", ns}, {"Request", class}}},
		{"!var x Foo\nfunc f(a Bar) Baz { return Baz{A: b} }\n", []name{{"x", other}, {"Foo", class}, {"a", other},
			{"Bar", class}, {"Baz", class}, {"Baz", other}, {"A", other}, {"b", other}}},
		{"!var a, b pkg.T = x, y\nvar n = m\nconst k Kind = 1\n", []name{{"a", other}, {"b", other}, {"pkg", ns}, {"T", class},
			{"x", other}, {"y", other}, {"n", other}, {"m", other}, {"k", other}, {"Kind", class}}},
		{"!var (\n\tc *http.Client\n\tn = m\n)\nconst (\n\tA Kind = iota\n\tB\n)\n", []name{{"c", other}, {"http", ns},
			{"Client", class}, {"n", other}, {"m", other}, {"A", other}, {"Kind", class}, {"B", other}}},
		{"!type D time.Duration\ntype A = pkg.B\ntype L[T any] []T\ntype (\n\tX Y\n\tZ map[K]V\n)\n", []name{{"D", other},
			{"time", ns}, {"Duration", class}, {"A", other}, {"pkg", ns}, {"B", class}, {"L", other}, {"T", other}, {"T", class},
			{"X", other}, {"Y", class}, {"Z", other}, {"K", class}, {"V", class}}},
		{"!func (e *AppEnv) Do(req *http.Request) (*http.Response, error) {}\n", []name{{"e", other}, {"AppEnv", class},
			{"req", other}, {"http", ns}, {"Request", class}, {"http", ns}, {"Response", class}}},
		{"!func Map[T any](xs []T, f func(T) U) (out []U) {}\n", []name{{"Map", other}, {"T", other}, {"xs", other}, {"T", class}, {"f", other},
			{"T", other}, {"U", class}, {"out", other}, {"U", class}}},
		{"!h := func(w http.ResponseWriter, _ *http.Request) {}\n", []name{{"h", other}, {"w", other}, {"http", ns},
			{"ResponseWriter", class}, {"_", other}, {"http", ns}, {"Request", class}}},
		// A single name in a parameter list is left (a name or a type); a qualified name is a type.
		{"!func f(a, b T, c ...pkg.U) {}\nfunc g(T) {}\nfunc h(pkg.T) {}\n", []name{{"a", other}, {"b", other}, {"T", class},
			{"c", other}, {"pkg", ns}, {"U", class}, {"T", other}, {"pkg", ns}, {"T", class}}},
		// Expressions are left as they are.
		{"!x := http.Get(u)\nt := time.Now()\ny := Foo(z)\nv := pkg.T{A: b}\n", []name{{"x", other}, {"http", other},
			{"u", other}, {"t", other}, {"time", other}, {"y", other}, {"z", other}, {"v", other}, {"pkg", other}, {"T", other},
			{"A", other}, {"b", other}}},
		{"!switch v := x.(type) {\ncase T:\n}\n", []name{{"v", other}, {"x", other}, {"T", other}}},
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
		// With ChromaTypeNames, every name stays NameOther.
		for _, tk := range Lex(src, len(src), Options{ChromaTypeNames: true}) {
			if tk.Type == class || tk.Type == ns {
				t.Errorf("%q with ChromaTypeNames: %q is %v", src, tk.Value, tk.Type)
			}
		}
	}
}

// TestTypeNamesDeepBrackets checks that a long run of unclosed brackets in a field type does not take
// quadratic time.
func TestTypeNamesDeepBrackets(t *testing.T) {
	Prepare()
	for _, src := range []string{
		"type T struct {\n\ta func" + strings.Repeat("(", 200000),
		"type T struct {\n\ta func" + strings.Repeat("[", 200000),
		"type T struct {\n\ta func" + strings.Repeat("[[", 200000),
		"func f" + strings.Repeat("(", 200000),
		"var x " + strings.Repeat("[]", 200000),
		"var x " + strings.Repeat("map[", 200000),
		"var x " + strings.Repeat("chan ", 200000),
		"var x " + strings.Repeat("func(", 200000),
		"type T " + strings.Repeat("struct{a ", 200000),
		strings.Repeat("var (\n", 100000),
		strings.Repeat("func(a A) ", 100000),
	} {
		start := time.Now()
		Lex(src, len(src), Options{})
		if d := time.Since(start); d > 2*time.Second {
			t.Errorf("%.30q…: %v", src, d)
		}
	}
}
