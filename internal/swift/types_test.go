package swift

import (
	"strings"
	"testing"

	fh "github.com/toshi-kuji/fasthighlight"
)

// typesOf returns the names that got NameClass, joined by spaces.
func typesOf(src string, o Options) string {
	var out []string
	for _, t := range Lex(src, len(src), o) {
		if t.Type == fh.NameClass {
			out = append(out, t.Value)
		}
	}
	return strings.Join(out, " ")
}

func TestTypePositions(t *testing.T) {
	Prepare()
	cases := []struct{ src, want string }{
		// Positive cases.
		{"let xs: [User] = []", "User"},
		{"func get(m: [String: User]) -> User {}", "User User"},
		{"var o: User?", "User"},
		{"let a = Array<User>()", "User"},
		{"let b: Box<User> = make()", "Box User"},
		{"func f(to name: Foo.Bar, _ x: Baz = .init()) {}", "Foo Bar Baz"},
		{"init?(user: User) {}", "User"},
		{"func g<T: Model>(x: T) -> [T]? {}", "T Model T T"},
		{"class A: Base, Proto {}", "A Base Proto"},
		{"let u = x as? User", "User"},
		{"if x is User {}", "User"},
		{"let f: (User) -> Item", "User Item"},
		{"var body: some View {", "View"},
		{"func f(y: any Proto) -> any Bar {}", "Proto Bar"},
		{"func f(y: inout User, z: borrowing Item, w: consuming Box) {}", "User Item Box"},
		{"func f(y: sending User, z: isolated Worker) {}", "User Worker"},
		{"let g: (inout User) -> some View", "User View"},
		{"class A: B, C where T: D {}", "A B C"},
		// Negative cases: values stay names.
		{"let v = c ? a : b", ""},
		{"let d = [k: v]", ""},
		{"f(x: value)", ""},
		{"if a < b && c > d {}", ""},
		{"if a<b {}", ""},
		{"switch x { case let y: print(y) }", ""},
		{"func f(a: Int = c ? x : y) {}", ""},
	}
	for _, c := range cases {
		if got := typesOf(c.src, Options{}); got != c.want {
			t.Errorf("%q: types %q, want %q", c.src, got, c.want)
		}
		// The Chroma switch keeps the old result: only the name after class and so on.
		want := ""
		if strings.HasPrefix(c.src, "class A") {
			want = "A"
		}
		if got := typesOf(c.src, Options{ChromaTypes: true}); got != want {
			t.Errorf("%q with ChromaTypes: types %q, want %q", c.src, got, want)
		}
	}
}

// Standard library names of current Swift (Codable, Identifiable) are built-ins like Hashable and Int; a
// user type of the same shape (Task) is not. With ChromaBuiltins set, only Chroma's list counts.
func TestModernBuiltins(t *testing.T) {
	Prepare()
	src := "struct Task: Identifiable, Codable, Hashable, Sendable {\n    let id: Int\n    var e: (any Error)?\n}\n"
	types := func(o Options) map[string]fh.TokenType {
		got := map[string]fh.TokenType{}
		for _, tok := range Lex(src, len(src), o) {
			got[tok.Value] = tok.Type
		}
		return got
	}
	got := types(Options{})
	for _, w := range []string{"Identifiable", "Codable", "Hashable", "Sendable", "Int", "Error"} {
		if got[w] != fh.NameBuiltin {
			t.Errorf("%s is %v, want NameBuiltin", w, got[w])
		}
	}
	if got["Task"] != fh.NameClass {
		t.Errorf("Task is %v, want NameClass", got["Task"])
	}
	chroma := types(Options{ChromaBuiltins: true})
	for _, w := range []string{"Identifiable", "Codable", "Sendable"} {
		if chroma[w] == fh.NameBuiltin {
			t.Errorf("with ChromaBuiltins, %s is NameBuiltin", w)
		}
	}
}
