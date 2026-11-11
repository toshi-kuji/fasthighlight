package typescript

import (
	"testing"

	fh "github.com/toshi-kuji/fasthighlight"
)

// In production, names in type arguments and in a return type are types; a "<" or ":" that is not
// one keeps its names as names. With Chroma set, only the names Chroma marks are types.
func TestTypePositions(t *testing.T) {
	Prepare()
	for _, c := range []struct {
		src          string
		types, names []string
	}{
		{"const m: Map<number, User> = x;", []string{"Map", "number", "User"}, []string{"m", "x"}},
		{"m = new Map<number, User>();", []string{"number", "User"}, []string{"m", "Map"}}, // Map is the constructor
		{"g<User>(u);", []string{"User"}, []string{"g", "u"}},
		{"let a: Array<Map<string, User[]>>;", []string{"Array", "Map", "string", "User"}, []string{"a"}},
		{"let p: Promise<A | B & C.D>;", []string{"Promise", "A", "B", "C", "D"}, []string{"p"}},
		{"function f(u: User): Result<User, Err> {}", []string{"User", "Result", "Err"}, []string{"f", "u"}},
		{"const h = (u: User): User => u;", []string{"User"}, []string{"h", "u"}},
		{"interface I { get(id: string): User; }", []string{"string", "User"}, []string{"I", "id"}},
		// Comparisons, conditionals and case are not types.
		{"if (a < b && c > d) {}", nil, []string{"a", "b", "c", "d"}},
		{"let n = a < b, m = c > d;", nil, []string{"n", "a", "b", "m", "c", "d"}},
		{"if (a < 1) { b > c }", nil, []string{"a", "b", "c"}},
		{"const x = cond ? (a) : b;", nil, []string{"x", "cond", "a", "b"}},
		{"const x = cond ? f(a) : b;", nil, []string{"x", "cond", "f", "a", "b"}},
		{"switch (v) { case (k): run(); }", nil, []string{"v", "k", "run"}},
	} {
		got := map[string]fh.TokenType{}
		for _, tok := range Lex(c.src+"\n", len(c.src)+1, Options{}) {
			got[tok.Value] = tok.Type
		}
		for _, w := range c.types {
			if got[w] != fh.KeywordType && got[w] != fh.NameBuiltin {
				t.Errorf("%s: %s is %v, want a type", c.src, w, got[w])
			}
		}
		for _, w := range c.names {
			if got[w] != fh.NameOther {
				t.Errorf("%s: %s is %v, want a name", c.src, w, got[w])
			}
		}
	}
	// Chroma's rules: the User in type arguments is a name.
	for _, tok := range Lex("g<User>(u);\n", 12, Options{Chroma: true}) {
		if tok.Value == "User" && tok.Type == fh.KeywordType {
			t.Error("with Chroma set, User in type arguments is a type")
		}
	}
}
