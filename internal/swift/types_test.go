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
