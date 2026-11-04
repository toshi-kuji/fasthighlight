package fasthighlight_test

import (
	"strings"
	"testing"

	. "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/testcases"
	_ "github.com/toshi-kuji/fasthighlight/lang/all"
)

// allBlocks are the ts and js blocks of the sample document, plus the edge cases.
func allBlocks(t *testing.T) []testcases.Block {
	t.Helper()
	bs, err := testcases.InputBlocks("testdata/input.md")
	if err != nil {
		t.Fatal(err)
	}
	return append(bs, testcases.EdgeCases...)
}

func join(toks []Token) string {
	var b strings.Builder
	for _, tk := range toks {
		b.WriteString(tk.Value)
	}
	return b.String()
}

// 1. Joining the tokens gives back the input (newlines normalized to \n; it may include an appended trailing
// newline), and there are no empty tokens or adjacent tokens of the same type.
func TestRoundTrip(t *testing.T) {
	for _, c := range allBlocks(t) {
		l := LookupRegistered(c.Lang)
		checkTokens(t, l, c.Code, l.Tokenise(c.Code))
	}
}

// checkTokens checks the tokenizer contract.
func checkTokens(t *testing.T, l *Language, code string, toks []Token) {
	t.Helper()
	var b strings.Builder
	for k, tk := range toks {
		if tk.Value == "" {
			t.Fatalf("%s: empty token at %d in %q", l.Name, k, code)
		}
		if k > 0 && toks[k-1].Type == tk.Type && len(toks[k-1].Value) < CoalesceLimit {
			t.Fatalf("%s: adjacent tokens of the same type %v at %d in %q", l.Name, tk.Type, k, code)
		}
		b.WriteString(tk.Value)
	}
	got := b.String()
	code = NormalizeNewlines(code)
	ok := got == code
	if !ok && l.EnsureNewline && !strings.HasSuffix(code, "\n") {
		ok = got == code+"\n"
	}
	if !ok {
		t.Fatalf("%s: tokens do not join back to the input\n got %q\nwant %q", l.Name, got, code)
	}
}

// 2. The types are as expected (checks the main rules).
func TestBasics(t *testing.T) {
	tests := []struct {
		lang, code string
		want       []Token
	}{
		{"ts", "const x = 1", []Token{
			{Type: KeywordReserved, Value: "const"}, {Type: Text, Value: " "},
			{Type: NameOther, Value: "x"}, {Type: Text, Value: " "},
			{Type: Operator, Value: "="}, {Type: Text, Value: " "},
			{Type: LiteralNumberInteger, Value: "1"},
		}},
		{"ts", "a / b", []Token{
			{Type: NameOther, Value: "a"}, {Type: Text, Value: " "},
			{Type: Operator, Value: "/"}, {Type: Text, Value: " "},
			{Type: NameOther, Value: "b"},
		}},
		{"ts", "x = /a+/g", []Token{
			{Type: NameOther, Value: "x"}, {Type: Text, Value: " "},
			{Type: Operator, Value: "="}, {Type: Text, Value: " "},
			{Type: LiteralStringRegex, Value: "/a+/g"},
		}},
		{"ts", "`a${f({ k: 1 })}b`", []Token{
			{Type: LiteralStringBacktick, Value: "`a"}, {Type: LiteralStringInterpol, Value: "${"},
			{Type: NameOther, Value: "f"}, {Type: Punctuation, Value: "({"},
			// Because of Chroma's "name: type" rule, 1 also becomes KeywordType
			{Type: Text, Value: " "}, {Type: NameOther, Value: "k"},
			{Type: Text, Value: ": "}, {Type: KeywordType, Value: "1"},
			{Type: Text, Value: " "},
			{Type: Punctuation, Value: "})"}, {Type: LiteralStringInterpol, Value: "}"},
			{Type: LiteralStringBacktick, Value: "b`"},
		}},
		{"ts", "p: string", []Token{
			{Type: NameOther, Value: "p"}, {Type: Text, Value: ": "},
			{Type: KeywordType, Value: "string"},
		}},
		{"ts", "// c\n'it\\'s'", []Token{
			{Type: CommentSingle, Value: "// c\n"}, {Type: LiteralStringSingle, Value: "'it\\'s'"},
		}},
		{"js", "f(...a) => 0xFF", []Token{
			{Type: NameOther, Value: "f"}, {Type: Punctuation, Value: "(..."},
			{Type: NameOther, Value: "a"}, {Type: Punctuation, Value: ")"},
			{Type: Text, Value: " "}, {Type: Punctuation, Value: "=>"},
			{Type: Text, Value: " "}, {Type: LiteralNumberHex, Value: "0xFF"},
		}},
	}
	for _, tt := range tests {
		got := LookupRegistered(tt.lang).Tokenise(tt.code)
		if len(got) != len(tt.want) {
			t.Errorf("%q: got %v\nwant %v", tt.code, got, tt.want)
			continue
		}
		for k := range got {
			if got[k] != tt.want[k] {
				t.Errorf("%q: token %d = %v, want %v", tt.code, k, got[k], tt.want[k])
			}
		}
	}
}
