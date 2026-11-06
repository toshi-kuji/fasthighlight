package compat

import (
	"slices"
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"

	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/http"
)

// TestHTTPMimeTypesMatchChroma checks that, for every MIME type of Chroma's lexers (and the same types
// with a +suffix), the language internal/http reads an HTTP body in is the one Chroma's
// httpBodyContentTyper chooses, when fasthighlight supports that language (and none otherwise).
func TestHTTPMimeTypesMatchChroma(t *testing.T) {
	var mimes []string
	for _, l := range lexers.GlobalLexerRegistry.Lexers {
		for _, m := range l.Config().MimeTypes {
			mimes = append(mimes, m)
			if i := strings.IndexByte(m, '/'); i >= 0 {
				mimes = append(mimes, m[:i+1]+"vnd.example+"+m[i+1:])
			}
		}
	}
	mimes = append(mimes, "application/ld+json", "application/soap+xml", "text/plain", "", "+", "/+", "json+", "a+b/json")
	for _, m := range mimes {
		want := ""
		if cl := chromaMimeLexer(m); cl != nil && builtin(cl.Config().Name) {
			want = cl.Config().Name
		}
		if got := http.MimeLanguage(m); got != want {
			t.Errorf("MimeLanguage(%q) = %q, Chroma chooses %q", m, got, want)
		}
	}
}

// chromaMimeLexer returns the lexer Chroma's httpBodyContentTyper chooses for the content type ct (nil if
// none).
func chromaMimeLexer(ct string) chroma.Lexer {
	l := lexers.MatchMimeType(ct)
	if l == nil && strings.Contains(ct, "+") {
		slash := strings.Index(ct, "/")
		plus := strings.LastIndex(ct, "+")
		l = lexers.MatchMimeType(ct[:slash+1] + ct[plus+1:])
	}
	return l
}

// httpIncomparable returns the reason why an HTTP message cannot be compared with Chroma ("" if it can
// be): its body is in a language that fasthighlight does not compare exactly (it is plain text then), or
// in Markdown with a code block that cannot be compared.
func httpIncomparable(code string) string {
	ct := http.ContentType(fh.NormalizeNewlines(code))
	if ct == "" {
		return ""
	}
	cl := chromaMimeLexer(ct)
	switch {
	case cl == nil:
		return ""
	case !slices.Contains(exactLanguages, cl.Config().Name):
		return "HTTP: Chroma reads the body as " + cl.Config().Name + ", which is not compared exactly"
	case cl.Config().Name == "markdown":
		return incomparable(fh.LookupRegistered("markdown"), code)
	}
	return ""
}

// dropHTTPBodyNewline removes, from Chroma's tokens of an HTTP message, the newline that Chroma appends to
// a body read in a language with ensure_nl when the message does not end with a newline. fasthighlight
// does not append it, so that the tokens joined are the input (see internal/http).
func dropHTTPBodyNewline(l *fh.Language, code string, toks []fh.Token) []fh.Token {
	if l.Name != "HTTP" || len(toks) == 0 || joinTokens(toks) != fh.NormalizeNewlines(code)+"\n" {
		return toks
	}
	last := &toks[len(toks)-1]
	if last.Value = last.Value[:len(last.Value)-1]; last.Value == "" {
		toks = toks[:len(toks)-1]
	}
	return toks
}
