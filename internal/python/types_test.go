package python

import (
	"strings"
	"testing"

	fh "github.com/toshi-kuji/fasthighlight"
)

// classes returns the names that got NameClass, space-separated.
func classes(code string, o Options) string {
	var out []string
	for _, tk := range Lex(code, len(code), o) {
		if tk.Type == fh.NameClass {
			out = append(out, tk.Value)
		}
	}
	return strings.Join(out, " ")
}

// Names in type hints get NameClass (an intended difference from Chroma); Options.ChromaTypes keeps
// Chroma's result.
func TestTypeHints(t *testing.T) {
	Prepare()
	for _, c := range []struct{ code, want string }{
		// positive
		{"def get(m: dict[str, User]) -> User:\n    pass\n", "User User"},
		{"xs: list[User] = []\n", "User"},
		{"    self.x: Optional[Node] = None\n", "Optional Node"},
		{"def f(self, a: A, b: B = B(), *c: C, **d: D) -> R | None: ...\n", "A B C D R"},
		{"def f(\n    a: A,\n    b: Callable[[X], Y],\n) -> Z:\n", "A Callable X Y Z"},
		{"def f[T](x: T) -> T: ...\n", "T T"},
		{"x: typing.List[Foo]\n", "List Foo"},
		{"class C:\n    name: Name\n", "C Name"},
		// negative
		{"d = {k: v}\n", ""},
		{"{\n  k: v,\n}\n", ""},
		{"a[1:2]\nb[i]\n", ""},
		{"f = lambda x: x\n", ""},
		{"lambda x: x\n", ""},
		{"f(x=1)\nif x:\n    pass\nelse:\n    pass\n", ""},
		{"def f(x=lambda y: y, z=d[a:b]): pass\n", ""},
		{"def f(x) -> None: pass\n", ""},
		{"while x: y\nfor a in b: c\n", ""},
		{"x := 1\n", ""},
		{"x: \"Foo\" = 1\n", ""},
		{"x: Foo[\n", ""},
		{"print(f\"{a:b}\")\n", ""},
		{"match x:\n    case Foo:\n        pass\n", ""},
	} {
		if got := classes(c.code, Options{}); got != c.want {
			t.Errorf("%q: got %q, want %q", c.code, got, c.want)
		}
		// With ChromaTypes, only class names after "class" are NameClass.
		got := classes(c.code, Options{ChromaTypes: true})
		if strings.HasPrefix(c.code, "class C") {
			if got != "C" {
				t.Errorf("%q ChromaTypes: got %q", c.code, got)
			}
		} else if got != "" {
			t.Errorf("%q ChromaTypes: got %q, want none", c.code, got)
		}
	}
	// Built-ins and None keep their types.
	code := "def f(a: str) -> None:\n"
	for _, tk := range Lex(code, len(code), Options{}) {
		switch tk.Value {
		case "str":
			if tk.Type != fh.NameBuiltin {
				t.Errorf("str: %v", tk.Type)
			}
		case "None":
			if tk.Type != fh.KeywordConstant {
				t.Errorf("None: %v", tk.Type)
			}
		}
	}
}
