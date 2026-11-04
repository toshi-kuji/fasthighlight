package markdown

import (
	"testing"

	fh "github.com/toshi-kuji/fasthighlight"
)

// A language's Prepare is not called at registration or by Scan, and is called once only for the languages
// that are highlighted.
func TestPrepareOnlyUsedLanguages(t *testing.T) {
	calls := map[string]int{}
	mk := func(name string) *fh.Language {
		return &fh.Language{
			Name:    name,
			Lex:     func(code string, _ int) []fh.Token { return []fh.Token{{Type: fh.Text, Value: code}} },
			Prepare: func() { calls[name]++ },
		}
	}
	// Registered languages cannot be removed, so the names are unique to this test.
	fh.Register(mk("Mdusedlang"))
	fh.Register(mk("Mdunusedlang"))
	if len(calls) != 0 {
		t.Fatalf("Prepare was called at registration: %v", calls)
	}
	c := NewConverter()
	d := c.Parse("", []byte("```mdusedlang\nx\n```\n"))
	Scan(d)
	Scan(d)
	if len(calls) != 0 {
		t.Fatalf("Prepare was called by Scan: %v", calls)
	}
	if err := c.Render(new(discard), d); err != nil {
		t.Fatal(err)
	}
	if calls["Mdusedlang"] != 1 || calls["Mdunusedlang"] != 0 {
		t.Errorf("Prepare calls = %v, want Mdusedlang once and Mdunusedlang never", calls)
	}
}

type discard struct{}

func (discard) Write(p []byte) (int, error)       { return len(p), nil }
func (discard) WriteString(s string) (int, error) { return len(s), nil }
