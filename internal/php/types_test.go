package php

import (
	"testing"

	fh "github.com/toshi-kuji/fasthighlight"
)

// In production, user-defined names in parameter types, return types and typed properties are NameClass;
// values in ternaries, case labels, named arguments and constants stay NameOther. With ChromaTypes, all
// of them stay NameOther, as in Chroma.
func TestTypePositions(t *testing.T) {
	Prepare()
	for _, c := range []struct {
		src          string
		types, names []string
	}{
		{"function get(User $u): User {}", []string{"User"}, []string{"get"}},
		{"function f(?User $u, A|B $v, C&D &$w, E ...$xs): ?R {}", []string{"User", "A", "B", "C", "D", "E", "R"}, []string{"f"}},
		{"function f(int $a, string $b): Foo|Bar|null {}", []string{"Foo", "Bar"}, []string{"int", "string"}},
		{"function &f(\\App\\Model $m) : Res {}", []string{"\\App\\Model", "Res"}, []string{"f"}},
		{"$h = fn(Req $r): Resp => $r;", []string{"Req", "Resp"}, nil},
		{"$h = function (Req $r) use ($x): Resp { };", []string{"Req", "Resp"}, nil},
		{"class C { private User $u; public ?Repo $r = null; }", []string{"User", "Repo"}, nil},
		{"try {} catch (MyException $e) {}", []string{"MyException"}, nil},
		{"function f(): void {}", nil, []string{"f", "void"}},
		// Not types.
		{"$a = $b ? Foo : $c;", nil, []string{"Foo"}},
		{"$a = $b ? f($x) : Bar;", nil, []string{"f", "Bar"}},
		{"switch ($v) { case X: $y = 1; }", nil, []string{"X"}},
		{"f(x: 1, y: $z);", nil, []string{"f", "x", "y"}},
		{"$m = FLAG & $x | OTHER;", nil, []string{"FLAG", "OTHER"}},
		{"$m = A || $x && B;", nil, []string{"A", "B"}},
		{"if (f($x) : Bar) {}", nil, []string{"f", "Bar"}},
		{"$a = FOO &$b;", nil, []string{"FOO"}},
		{"$m = FLAG_A &$flags;", nil, []string{"FLAG_A"}},
		{"$a = A|B &$x;", nil, []string{"A", "B"}},
		{"echo X . $y;", nil, []string{"X"}},
		{"if (X && $y) {}", nil, []string{"X"}},
		{"return Y ?: $z;", nil, []string{"Y"}},
		{"f(Z, W $v);", nil, []string{"Z", "W"}},
		// Types in declarations.
		{"function f(User &$u, ?A $a, A|B $v, C&D $w, E ...$xs) {}", []string{"User", "A", "B", "C", "D", "E"}, []string{"f"}},
		{"$h = fn(Foo $f) => 1;", []string{"Foo"}, nil},
		{"$h = function (Foo $f) use ($x): Bar {};", []string{"Foo", "Bar"}, nil},
		{"class C { private ?User $u; public readonly Repo $r; }", []string{"User", "Repo"}, nil},
		{"function f(int $a = g(1, 2), Foo $b) {}", []string{"Foo"}, nil},
	} {
		for _, chroma := range []bool{false, true} {
			got := map[string]fh.TokenType{}
			for _, tok := range Lex(c.src+"\n", len(c.src)+1, Options{ChromaTypes: chroma}) {
				got[tok.Value] = tok.Type
			}
			for _, w := range c.types {
				want := fh.NameClass
				if chroma {
					want = fh.NameOther
				}
				if got[w] != want {
					t.Errorf("%s (ChromaTypes=%v): %s is %v, want %v", c.src, chroma, w, got[w], want)
				}
			}
			for _, w := range c.names {
				if got[w] == fh.NameClass {
					t.Errorf("%s (ChromaTypes=%v): %s is NameClass", c.src, chroma, w)
				}
			}
		}
	}
}
