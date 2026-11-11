package kotlin

import (
	"testing"

	fh "github.com/toshi-kuji/fasthighlight"
)

// In production, names after a ":" and in type arguments are types (NameClass); comparisons, the
// elvis operator, references and annotation targets keep their names. With ChromaTypes set, only the
// names Chroma marks are types.
func TestTypePositions(t *testing.T) {
	Prepare()
	for _, c := range []struct {
		src          string
		types, names []string
	}{
		{"val xs: List<User> = listOf()", []string{"List", "User"}, []string{"listOf"}},
		{"fun get(m: Map<String, User>): User {}", []string{"Map", "String", "User"}, []string{"m"}},
		{"fun f(a: kotlin.collections.List<out T?>, b: Array<*>) {}", []string{"kotlin", "collections", "List", "T", "Array"}, []string{"a", "b"}},
		{"val u = listOf<User>()", []string{"User"}, []string{"listOf"}},
		{"class A<T : Base<T>> : Parent()", []string{"T", "Base", "Parent"}, nil},
		{"val m: Map<K, List<V>> by lazy { x }", []string{"Map", "K", "List", "V"}, []string{"x"}},
		{"object : Runnable by impl", []string{"Runnable"}, []string{"impl"}},
		{"xs.forEach { u: User -> u }", []string{"User"}, []string{"xs", "forEach"}},
		// Not types.
		{"if (a<b && c>d) run()", nil, []string{"a", "b", "c", "d", "run"}},
		{"if (a < b || c > d) run()", nil, []string{"a", "b", "c", "d", "run"}},
		{"f(a<b, c>d)", nil, []string{"f", "a", "b", "c", "d"}},
		{"val z = a<b", nil, []string{"a", "b"}},
		{"val z = a<1", nil, []string{"a"}},
		{"val z = a ?: b", nil, []string{"a", "b"}},
		{"val r = String::length", nil, []string{"length"}},
		{"@file:JvmName(name)", nil, []string{"name"}},
		{"loop@ for (i in xs) break@loop", nil, []string{"i", "xs"}},
		{"when (x) { is Int -> y }", nil, []string{"x", "y"}},
	} {
		got := map[string]fh.TokenType{}
		for _, tok := range Lex(c.src+"\n", len(c.src)+1, Options{}) {
			got[tok.Value] = tok.Type
		}
		for _, w := range c.types {
			if got[w] != fh.NameClass {
				t.Errorf("%s: %s is %v, want NameClass", c.src, w, got[w])
			}
		}
		for _, w := range c.names {
			if got[w] != fh.Name {
				t.Errorf("%s: %s is %v, want Name", c.src, w, got[w])
			}
		}
	}
	// Chroma's rules: names in type positions stay Name.
	src := "val xs: List<User> = listOf()\n"
	for _, tok := range Lex(src, len(src), Options{ChromaTypes: true}) {
		if tok.Value == "List" || tok.Value == "User" {
			if tok.Type != fh.Name {
				t.Errorf("with ChromaTypes set, %s is %v, want Name", tok.Value, tok.Type)
			}
		}
	}
}
