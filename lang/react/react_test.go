package react_test

import (
	"slices"
	"testing"

	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/lang/react"
)

// JSX tags, attributes and expressions follow Chroma's react.xml.
func TestJSX(t *testing.T) {
	code := "<A.B x={f({})} y=\"s\"/>"
	want := []fh.Token{
		{Type: fh.Punctuation, Value: "<"},
		{Type: fh.NameTag, Value: "A.B"},
		{Type: fh.Text, Value: " "},
		{Type: fh.NameAttribute, Value: "x"},
		{Type: fh.Operator, Value: "="},
		{Type: fh.Punctuation, Value: "{"},
		{Type: fh.NameOther, Value: "f"},
		{Type: fh.Punctuation, Value: "({})}"},
		{Type: fh.Text, Value: " "},
		{Type: fh.NameAttribute, Value: "y"},
		{Type: fh.Operator, Value: "="},
		{Type: fh.LiteralString, Value: "\"s\""},
		{Type: fh.Punctuation, Value: "/>"},
	}
	if got := react.Language.Tokenise(code); !slices.Equal(got, want) {
		t.Errorf("got  %v\nwant %v", got, want)
	}
}

// Intended differences from Chroma: escapes in template literals and { } inside ${ … }.
func TestIntendedDifferences(t *testing.T) {
	code := "`x\\n${f({})}`"
	want := []fh.Token{
		{Type: fh.LiteralStringBacktick, Value: "`x\\n"},
		{Type: fh.LiteralStringInterpol, Value: "${"},
		{Type: fh.NameOther, Value: "f"},
		{Type: fh.Punctuation, Value: "({})"},
		{Type: fh.LiteralStringInterpol, Value: "}"},
		{Type: fh.LiteralStringBacktick, Value: "`"},
	}
	if got := react.Language.Tokenise(code); !slices.Equal(got, want) {
		t.Errorf("got  %v\nwant %v", got, want)
	}
}
