package java_test

import (
	"testing"

	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/lang/java"
)

// Numbers get the number color (an intended difference from Chroma, where they get the name color).
func TestNumbers(t *testing.T) {
	for _, c := range []struct {
		code string
		want fh.TokenType
	}{
		{"0", fh.LiteralNumberInteger}, {"42", fh.LiteralNumberInteger}, {"1_000L", fh.LiteralNumberInteger},
		{"0x7fff_ffffL", fh.LiteralNumberHex}, {"0b1010", fh.LiteralNumberBin}, {"017", fh.LiteralNumberOct},
		{"1.5", fh.LiteralNumberFloat}, {"1.5e-3d", fh.LiteralNumberFloat}, {".5f", fh.LiteralNumberFloat},
		{"3f", fh.LiteralNumberFloat}, {"1e5", fh.LiteralNumberFloat}, {"0x1.8p3", fh.LiteralNumberFloat},
		{"0x1p-2f", fh.LiteralNumberFloat},
	} {
		toks := java.Language.Tokenise("x = " + c.code + ";")
		if len(toks) != 6 || toks[4].Value != c.code || toks[4].Type != c.want {
			t.Errorf("%q: tokens %v, want %s", c.code, toks, c.want)
		}
	}
	// No name starts with a digit.
	if toks := java.Language.Tokenise("1abc"); len(toks) != 2 || toks[0].Type != fh.LiteralNumberInteger || toks[1].Type != fh.Name {
		t.Errorf("1abc: tokens %v", toks)
	}
}
