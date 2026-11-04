package compat

import (
	"os"
	"regexp"
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"

	fh "github.com/toshi-kuji/fasthighlight"
)

// benchBlocks returns the code blocks of testdata/multilang.md (one per language), without text, which is
// not highlighted.
func benchBlocks(b *testing.B) []struct {
	l    *fh.Language
	code string
} {
	b.Helper()
	src, err := os.ReadFile("../testdata/multilang.md")
	if err != nil {
		b.Fatal(err)
	}
	var out []struct {
		l    *fh.Language
		code string
	}
	for _, m := range regexp.MustCompile("(?ms)^```(\\w+)\\n(.*?)^```$").FindAllStringSubmatch(string(src), -1) {
		if l := fh.LookupRegistered(m[1]); l != nil {
			out = append(out, struct {
				l    *fh.Language
				code string
			}{l, m[2]})
		}
	}
	return out
}

// BenchmarkTokenise compares the speed of tokenising alone with Chroma, on the same code (Markdown parsing and
// HTML output are not included). Both go as far as producing the coalesced (Coalesce) token sequence. The
// preparation (word tables, building Chroma's regular expressions) is done once at the start and is not
// included in the measured time.
//
//	go test -run='^$' -bench=BenchmarkTokenise -benchtime=200ms .
func BenchmarkTokenise(b *testing.B) {
	blocks := benchBlocks(b)
	for _, bl := range blocks {
		l, code := bl.l, bl.code
		cl := chroma.Coalesce(lexers.Get(l.Name))
		chromaTokenise := func() int {
			it, err := cl.Tokenise(nil, code)
			if err != nil {
				b.Fatal(err)
			}
			return len(it.Tokens())
		}
		b.Run("fasthighlight/"+l.Name, func(b *testing.B) {
			l.Tokenise(code)
			b.SetBytes(int64(len(code)))
			b.ReportAllocs()
			for b.Loop() {
				l.Tokenise(code)
			}
		})
		b.Run("chroma/"+l.Name, func(b *testing.B) {
			chromaTokenise()
			b.SetBytes(int64(len(code)))
			b.ReportAllocs()
			for b.Loop() {
				chromaTokenise()
			}
		})
	}
}
