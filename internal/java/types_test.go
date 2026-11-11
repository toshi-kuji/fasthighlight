package java

import (
	"testing"

	fh "github.com/toshi-kuji/fasthighlight"
)

// In production, user-defined type names in type positions are types (KeywordType); comparisons, shifts
// and other names stay names. With ChromaTypes set, they are names as in Chroma.
func TestTypePositions(t *testing.T) {
	Prepare()
	for _, c := range []struct {
		src          string
		types, names []string
	}{
		{"List<User> xs = new ArrayList<User>();", []string{"List", "User", "ArrayList"}, []string{"xs"}},
		{"User get(Map<String, User> m) {}", []string{"User", "Map", "String"}, []string{"m"}},
		{"public static User find(String id, int n) {}", []string{"User", "String"}, []string{"id", "n"}},
		{"private final Map<String, List<User>> cache;", []string{"Map", "String", "List", "User"}, []string{"cache"}},
		{"User[] users = load();", []string{"User"}, []string{"users"}},
		{"void run(User... us) {}", []string{"User"}, []string{"us"}},
		{"for (User u : users) {}", []string{"User"}, []string{"u", "users"}},
		{"catch (IOException e) {}", []string{"IOException"}, []string{"e"}},
		{"Map<? extends Key, Value[]> m;", []string{"Map", "Key", "Value"}, []string{"m"}},
		// Comparisons, shifts and other expressions are not types.
		{"if (a < b && c > d) {}", nil, []string{"a", "b", "c", "d"}},
		{"if (A < B && C > D) {}", nil, []string{"A", "B", "C", "D"}},
		{"x = Max < 1 || Y > z;", nil, []string{"x", "Max", "Y", "z"}},
		{"x = A << B;", nil, []string{"x", "A", "B"}},
		{"x = Foo.bar(y);", nil, []string{"x", "Foo", "y"}},
		{"return Count;", nil, []string{"Count"}},
		{"x = cond ? A : b;", nil, []string{"x", "cond", "A", "b"}},
		{"user.save(other);", nil, []string{"user", "other"}},
	} {
		got := map[string]fh.TokenType{}
		for _, tok := range Lex(c.src+"\n", len(c.src)+1, Options{}) {
			got[tok.Value] = tok.Type
		}
		for _, w := range c.types {
			if got[w] != fh.KeywordType {
				t.Errorf("%s: %s is %v, want a type", c.src, w, got[w])
			}
		}
		for _, w := range c.names {
			if got[w] != fh.Name {
				t.Errorf("%s: %s is %v, want a name", c.src, w, got[w])
			}
		}
		// With ChromaTypes, no name is a type.
		for _, tok := range Lex(c.src+"\n", len(c.src)+1, Options{ChromaTypes: true}) {
			if tok.Type == fh.KeywordType && tok.Value != "int" && tok.Value != "void" {
				t.Errorf("%s: with ChromaTypes, %s is a type", c.src, tok.Value)
			}
		}
	}
}
