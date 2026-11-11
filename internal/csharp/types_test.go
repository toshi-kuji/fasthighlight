package csharp

import (
	"testing"

	fh "github.com/toshi-kuji/fasthighlight"
)

// In production, user-defined names in type positions are types (KeywordType); names in expressions
// stay names. With ChromaTypes set, they are all names, as in Chroma.
func TestTypePositions(t *testing.T) {
	Prepare()
	for _, c := range []struct {
		src          string
		types, names []string
	}{
		{"List<User> xs = new List<User>();", []string{"List", "User"}, []string{"xs"}},
		{"User Get(Dictionary<string, User> m) {}", []string{"User", "Dictionary"}, []string{"Get", "m"}},
		{"private Foo.Bar<Baz[]>? field;", []string{"Foo", "Bar", "Baz"}, []string{"field"}},
		{"public User Name { get; set; }", []string{"User"}, []string{"Name"}},
		{"T Find<T>(IEnumerable<T> xs, int n) where T : class {}", []string{"T", "IEnumerable"}, []string{"Find", "xs", "n"}},
		{"var d = Get<User>(x);", []string{"User"}, []string{"d", "Get", "x"}},
		{"foreach (User u in users) {}", []string{"User"}, []string{"u", "users"}},
		{"catch (IOException e) {}", []string{"IOException"}, []string{"e"}},
		{"var u = o as User;", []string{"User"}, []string{"u", "o"}},
		{"var m = new Dictionary<string, List<int>>();", []string{"Dictionary", "List"}, []string{"m"}},
		{"async Task<User[,]> Load(params User[] us) {}", []string{"Task", "User"}, []string{"Load", "us"}},
		// Comparisons, shifts, member access and expressions are not types.
		{"if (a < b && c > d) {}", nil, []string{"a", "b", "c", "d"}},
		{"F(a < b, c > d);", nil, []string{"F", "a", "b", "c", "d"}},
		{"x = a < b;", nil, []string{"x", "a", "b"}},
		{"x = a << b > c;", nil, []string{"x", "a", "b", "c"}},
		{"if (a < 1 || b > c) {}", nil, []string{"a", "b", "c"}},
		{"x = y.Z(w);", nil, []string{"x", "y", "Z", "w"}},
		{"var r = p with { A = 1 };", nil, []string{"r", "p", "A"}},
		{"return x == y;", nil, []string{"x", "y"}},
		{"a.b = c < d;", nil, []string{"a", "b", "c", "d"}},
		// Pattern combinators are not types (the type after "is" is left as it is).
		{"return x is A or B;", nil, []string{"x", "or"}},
		{"var y = x switch { A or B => 1 };", nil, []string{"y", "x", "or"}},
		{"if (x is not Foo) {}", nil, []string{"x", "not"}},
		{"var w = x is Foo and Bar;", nil, []string{"w", "x", "and"}},
	} {
		got := map[string]fh.TokenType{}
		for _, tok := range Lex(c.src, len(c.src), Options{}) {
			if _, ok := got[tok.Value]; !ok {
				got[tok.Value] = tok.Type
			}
		}
		for _, w := range c.types {
			if got[w] != fh.KeywordType {
				t.Errorf("%q: %q is %v, want KeywordType", c.src, w, got[w])
			}
		}
		for _, w := range c.names {
			if got[w] != fh.Name {
				t.Errorf("%q: %q is %v, want Name", c.src, w, got[w])
			}
		}
		for _, tok := range Lex(c.src, len(c.src), Options{ChromaTypes: true}) {
			if tok.Type == fh.KeywordType && tok.Value != "string" && tok.Value != "int" && tok.Value != "var" {
				t.Errorf("%q with ChromaTypes: %q is KeywordType", c.src, tok.Value)
			}
		}
	}
}
