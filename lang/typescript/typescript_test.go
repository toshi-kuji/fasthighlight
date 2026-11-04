package typescript_test

import (
	"slices"
	"testing"

	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/lang/typescript"
)

// Intended differences from Chroma: type arguments are not treated as tags, escapes in template
// literals, { } inside ${ … }, and names that start with a non-letter.
func TestIntendedDifferences(t *testing.T) {
	code := "a<b>(名前, `x\\n${f({})}`)\n"
	want := []fh.Token{
		{Type: fh.NameOther, Value: "a"},
		{Type: fh.Operator, Value: "<"},
		{Type: fh.NameOther, Value: "b"},
		{Type: fh.Operator, Value: ">"},
		{Type: fh.Punctuation, Value: "("},
		{Type: fh.NameOther, Value: "名前"},
		{Type: fh.Punctuation, Value: ","},
		{Type: fh.Text, Value: " "},
		{Type: fh.LiteralStringBacktick, Value: "`x\\n"},
		{Type: fh.LiteralStringInterpol, Value: "${"},
		{Type: fh.NameOther, Value: "f"},
		{Type: fh.Punctuation, Value: "({})"},
		{Type: fh.LiteralStringInterpol, Value: "}"},
		{Type: fh.LiteralStringBacktick, Value: "`"},
		{Type: fh.Punctuation, Value: ")"},
		{Type: fh.Text, Value: "\n"},
	}
	if got := typescript.Language.Tokenise(code); !slices.Equal(got, want) {
		t.Errorf("got  %v\nwant %v", got, want)
	}
}
