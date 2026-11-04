package markdown_test

import (
	"slices"
	"testing"

	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/lang/markdown"
)

// Intended differences from Chroma (emphasis at the start of a line and at the start of a list
// item, and a newline right after a marker), and highlighting inside code blocks (in this test,
// YAML is registered and JSON is not, so JSON is not highlighted).
func TestIntendedDifferences(t *testing.T) {
	code := "# T\n**Note** x\n- **a** b\n#\nx\n>\ny\n```yaml\na: 1\n```\n```json\n[1]\n```\n"
	want := []fh.Token{
		{Type: fh.GenericHeading, Value: "# T\n"},
		{Type: fh.GenericStrong, Value: "**Note**"},
		{Type: fh.Text, Value: " x\n"},
		{Type: fh.Keyword, Value: "-"},
		{Type: fh.Text, Value: " "},
		{Type: fh.GenericStrong, Value: "**a**"},
		{Type: fh.Text, Value: " b\n#\nx\n>\ny\n"},
		{Type: fh.LiteralString, Value: "```yaml\n"},
		{Type: fh.NameTag, Value: "a"},
		{Type: fh.Punctuation, Value: ":"},
		{Type: fh.TextWhitespace, Value: " "},
		{Type: fh.LiteralNumber, Value: "1"},
		{Type: fh.TextWhitespace, Value: "\n"},
		{Type: fh.LiteralString, Value: "```"},
		{Type: fh.Text, Value: "\n"},
		{Type: fh.LiteralString, Value: "```json\n"},
		{Type: fh.Text, Value: "[1]\n"},
		{Type: fh.LiteralString, Value: "```"},
		{Type: fh.Text, Value: "\n"},
	}
	if got := markdown.Language.Tokenise(code); !slices.Equal(got, want) {
		t.Errorf("got  %v\nwant %v", got, want)
	}
}
