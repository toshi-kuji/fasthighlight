package rust

import (
	"fmt"
	"strings"
	"testing"

	fh "github.com/toshi-kuji/fasthighlight"
)

// In production, the names of a whole type after ":" or "->" and in turbofish type arguments are
// NameClass (NameNamespace before "::"); what does not look like a type keeps Chroma's coloring. With
// ChromaTypes set, only the one name Chroma marks is NameClass.
func TestTypePositions(t *testing.T) {
	Prepare()
	for _, c := range []struct {
		src                string
		classes, ns, names []string
	}{
		{"let xs: Vec<User> = vec![];", []string{"User"}, nil, []string{"xs"}},
		{"fn get(m: HashMap<String, User>) -> User {}", []string{"HashMap", "User"}, nil, []string{"m"}},
		{"fn f(p: &mut Point, q: &'a [Item; 4]) {}", []string{"Point", "Item"}, nil, []string{"p", "q"}},
		{"let m: std::collections::HashMap<K, V>;", []string{"HashMap", "K", "V"}, []string{"std", "collections"}, []string{"m"}},
		{"fn f() -> Box<dyn Fn(Event) -> Reply + Send> {}", []string{"Event", "Reply"}, nil, nil},
		{"let it: Wrapper<Item = Node> = w;", []string{"Wrapper", "Item", "Node"}, nil, []string{"it", "w"}},
		{"let v = it.collect::<Vec<User>>();", []string{"User"}, nil, []string{"v", "it", "collect"}},
		{"let t: (A, B) = p;", []string{"A", "B"}, nil, []string{"t", "p"}},
		{"impl<T: Display + Debug> X {}", []string{"Display", "Debug"}, nil, []string{"T", "X"}},
		// Not types: comparisons, shifts, calls, fields, paths in values, macros, characters.
		{"S { x: a < b, y: c > d };", []string{"a", "c"}, nil, []string{"b", "d", "S"}}, // a and c: Chroma too
		{"S { x: a << 2 };", []string{"a"}, nil, []string{"S"}},
		{"S { x: a.b, y: f(z) };", []string{"a", "f"}, nil, []string{"b", "z", "S"}},
		{"S { x: Foo::new(v) };", []string{"Foo"}, nil, []string{"new", "v", "S"}},
		{"S { x: m!(v) };", nil, nil, []string{"v", "S"}},
		{"if a < b && c > d {}", nil, nil, []string{"a", "b", "c", "d"}},
		{"let n = a << b >> c;", nil, nil, []string{"n", "a", "b", "c"}},
	} {
		got := map[string]fh.TokenType{}
		for _, tok := range Lex(c.src+"\n", len(c.src)+1, Options{}) {
			if _, ok := got[tok.Value]; !ok {
				got[tok.Value] = tok.Type
			}
		}
		check := func(words []string, want fh.TokenType) {
			for _, w := range words {
				if got[w] != want {
					t.Errorf("%s: %s is %v, want %v", c.src, w, got[w], want)
				}
			}
		}
		check(c.classes, fh.NameClass)
		check(c.ns, fh.NameNamespace)
		check(c.names, fh.Name)
		// Keywords, built-ins and lifetimes keep their types.
		for _, k := range []struct {
			w string
			t fh.TokenType
		}{{"mut", fh.Keyword}, {"dyn", fh.Keyword}, {"Vec", fh.NameBuiltin}, {"String", fh.NameBuiltin}, {"a", fh.NameAttribute}} {
			if tt, ok := got[k.w]; ok && strings.Contains(c.src, "'"+k.w) && tt != k.t {
				t.Errorf("%s: %s is %v, want %v", c.src, k.w, tt, k.t)
			}
		}
		if tt, ok := got["mut"]; ok && tt != fh.Keyword {
			t.Errorf("%s: mut is %v", c.src, tt)
		}
	}
	// ChromaTypes: Chroma's coloring (User in type arguments is Name, mut after & is NameClass).
	src := "let xs: Vec<User> = f(p: &mut T);\n"
	var b strings.Builder
	for _, tok := range Lex(src, len(src), Options{ChromaTypes: true}) {
		fmt.Fprintf(&b, "%v %q;", tok.Type, tok.Value)
	}
	for _, want := range []string{fmt.Sprintf("%v %q;", fh.Name, "User"), fmt.Sprintf("%v %q;", fh.NameClass, "mut")} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("with ChromaTypes: want %s in %s", want, b.String())
		}
	}
}

// The values of fields in struct literals are not types: they keep exactly Chroma's coloring (what
// ChromaTypes gives), at the top level and inside function bodies.
func TestStructLiteralValues(t *testing.T) {
	Prepare()
	for _, lit := range []string{
		"P { a: *x + 1 }",
		"P { a: (lo, hi), b: [lo; N] }",
		"P { a: [a, b, c] }",
		"P { a: (x) }",
		"P { a: &mut buf }",
		"P { a: *ptr }",
		"P { a: A + B }",
		"Config { mode: Mode::Fast }",
		"P { a: Q { b: x }, c: &y }",
	} {
		for _, src := range []string{
			"let p = " + lit + ";\n",
			"fn f() {\n    let p = " + lit + ";\n    g(" + lit + ");\n}\n",
			"static X: P = " + lit + ";\n",
			"fn f() -> P {\n    " + lit + "\n}\n",
			"match v {\n    " + lit + " => 1,\n}\n",
		} {
			// The part after the first type (let p =, X: P =, -> P) must match ChromaTypes.
			got := Lex(src, len(src), Options{})
			want := Lex(src, len(src), Options{ChromaTypes: true})
			k := strings.Index(src, lit)
			var gb, wb strings.Builder
			for _, x := range []struct {
				toks []fh.Token
				b    *strings.Builder
			}{{got, &gb}, {want, &wb}} {
				off := 0
				for _, tok := range x.toks {
					if off >= k {
						fmt.Fprintf(x.b, "%v %q;", tok.Type, tok.Value)
					}
					off += len(tok.Value)
				}
			}
			if gb.String() != wb.String() {
				t.Errorf("%q:\n got %s\nwant %s", src, gb.String(), wb.String())
			}
		}
	}
}

// Real type positions keep the whole type colored: declaration fields, statics, closures, lets in
// function bodies, bounds.
func TestTypePositionsInBodies(t *testing.T) {
	Prepare()
	for _, c := range []struct{ src, class string }{
		{"struct S { a: Vec<User> }", "User"},
		{"pub struct S<T> where T: Into<Name> { pub a: Option<User>, b: T }", "User"},
		{"struct S<T> where T: Into<Name> { a: T }", "Name"},
		{"union U { a: Wrap<User> }", "User"},
		{"enum E { V { a: Vec<User> }, W(X) }", "User"},
		{"trait Tr { type Item: Into<User>; fn f(&self) -> Vec<User>; }", "User"},
		{"impl S { fn f(a: Vec<User>) { let x: Vec<Item> = g(); } }", "Item"},
		{"fn f() { let x: Vec<Item> = P { a: b }; }", "Item"},
		{"fn f() { const N: Wrap<Item> = w; }", "Item"},
		{"fn f() { static N: Wrap<Item> = w; }", "Item"},
		{"fn f() { let g = |x: Vec<Item>, y: Box<User>| x; }", "User"},
		{"fn f() { let g = move |x: Vec<Item>| x; }", "Item"},
		{"static X: Vec<User> = v;", "User"},
		{"mod m { struct S { a: Vec<User> } }", "User"},
		{"fn f<T: Into<User>>(t: T) {}", "User"},
	} {
		src := c.src + "\n"
		found := false
		for _, tok := range Lex(src, len(src), Options{}) {
			if tok.Value == c.class {
				found = true
				if tok.Type != fh.NameClass {
					t.Errorf("%s: %s is %v, want NameClass", c.src, c.class, tok.Type)
				}
			}
		}
		if !found {
			t.Errorf("%s: no %s", c.src, c.class)
		}
	}
}

// Long runs of ":" and "::<" stay linear (each scan is bounded by maxType).
func TestTypeScanBounded(t *testing.T) {
	Prepare()
	for _, unit := range []string{"a: ", "a::<", "x: Vec<", "&"} {
		src := strings.Repeat(unit, 20000) + "\n"
		toks := Lex(src, len(src), Options{})
		var b strings.Builder
		for _, tok := range toks {
			b.WriteString(tok.Value)
		}
		if b.String() != src {
			t.Fatalf("%q: tokens do not join to the input", unit)
		}
	}
}
