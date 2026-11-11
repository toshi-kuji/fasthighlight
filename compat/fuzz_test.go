package compat

import (
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/c"
	"github.com/toshi-kuji/fasthighlight/internal/csharp"
	"github.com/toshi-kuji/fasthighlight/internal/golang"
	"github.com/toshi-kuji/fasthighlight/internal/html"
	"github.com/toshi-kuji/fasthighlight/internal/http"
	"github.com/toshi-kuji/fasthighlight/internal/java"
	"github.com/toshi-kuji/fasthighlight/internal/js"
	"github.com/toshi-kuji/fasthighlight/internal/kotlin"
	"github.com/toshi-kuji/fasthighlight/internal/lexkit"
	"github.com/toshi-kuji/fasthighlight/internal/markdown"
	"github.com/toshi-kuji/fasthighlight/internal/php"
	"github.com/toshi-kuji/fasthighlight/internal/python"
	"github.com/toshi-kuji/fasthighlight/internal/r"
	"github.com/toshi-kuji/fasthighlight/internal/ruby"
	"github.com/toshi-kuji/fasthighlight/internal/rust"
	"github.com/toshi-kuji/fasthighlight/internal/swift"
	"github.com/toshi-kuji/fasthighlight/internal/testcases"
	"github.com/toshi-kuji/fasthighlight/internal/typescript"
	_ "github.com/toshi-kuji/fasthighlight/lang/all"

	"github.com/alecthomas/chroma/v2/lexers"
)

// FuzzHTMLMatchesChroma checks that any token sequence returned by a tokenizer produces the HTML that
// chromaHTML builds from Chroma's output (Chroma's class names and escaping, in fasthighlight's shape). It covers all registered languages and the case of an
// unsupported language (not highlighted).
//
//	go test -run='^$' -fuzz=FuzzHTMLMatchesChroma -fuzztime=1m .
func FuzzHTMLMatchesChroma(f *testing.F) {
	bs, err := testcases.InputBlocks("../testdata/input.md")
	if err != nil {
		f.Fatal(err)
	}
	langs := fh.Languages()
	index := func(l *fh.Language) uint8 {
		for i, x := range langs {
			if x == l {
				return uint8(i)
			}
		}
		panic("not registered: " + l.Name)
	}
	for _, b := range append(bs, testcases.EdgeCases...) {
		f.Add(index(fh.LookupRegistered(b.Lang)), b.Code)
	}
	f.Add(uint8(len(langs)), "plain <text> & 'quotes'\n")
	f.Fuzz(func(t *testing.T, lang uint8, code string) {
		// 0 to len(langs)-1 are registered languages, len(langs) is an unsupported language (not highlighted).
		var toks []fh.Token
		if i := int(lang) % (len(langs) + 1); i < len(langs) {
			toks = langs[i].Tokenise(code)
		} else {
			toks = []fh.Token{{Type: fh.Text, Value: code}}
		}
		if got, want := ourHTML(t, toks), chromaHTML(t, toks); got != want {
			t.Fatalf("HTML differs from Chroma\n got %q\nwant %q", got, want)
		}
	})
}

// exactLanguages are the languages for which the fuzz test checks that any input gives the same token
// sequence as Chroma. Languages that intentionally differ from Chroma are compared using the switch that
// matches Chroma (chromaMode).
var exactLanguages = []string{"Bash", "C", "C++", "C#", "CSS", "Diff", "Go", "HTML", "Java", "JavaScript", "JSON", "Kotlin", "markdown", "PHP", "Python", "R", "Ruby", "Rust", "SQL", "Swift", "TOML", "TypeScript", "XML", "YAML",
	// Added after the first 24: new languages go at the end, so the saved fuzz inputs keep their language.
	"react", "SCSS", "Bash Session", "Docker", "Makefile", "HTTP", "INI", "GraphQL", "PowerShell", "Protocol Buffer"}

// chromaMode returns the tokenizer of l with the switch that matches Chroma turned on (it does the same
// preprocessing as Language.Tokenise). For a language without such a switch, it returns l.Tokenise as is.
func chromaMode(l *fh.Language) func(string) []fh.Token {
	var lex func(text string, end int) []fh.Token
	switch l.Name {
	case "JavaScript":
		lex = func(text string, end int) []fh.Token { return js.Lex(text, end, js.Options{ChromaBraces: true}) }
	case "C++":
		l.Tokenise("") // preparation (word lists)
		lex = func(text string, end int) []fh.Token {
			return c.Lex(text, end, c.Options{CPP: true, ChromaRawString: true})
		}
	case "C#":
		l.Tokenise("") // preparation (word lists)
		lex = func(text string, end int) []fh.Token { return csharp.Lex(text, end, csharp.Options{ChromaTypes: true}) }
	case "Java":
		l.Tokenise("") // preparation (word lists)
		lex = func(text string, end int) []fh.Token { return java.Lex(text, end, java.Options{ChromaIdent: true}) }
	case "Kotlin":
		l.Tokenise("") // preparation (word lists)
		lex = func(text string, end int) []fh.Token { return kotlin.Lex(text, end, kotlin.Options{ChromaTypes: true}) }
	case "PHP":
		l.Tokenise("") // preparation (word lists)
		lex = func(text string, end int) []fh.Token {
			return php.Lex(text, end, php.Options{ChromaTags: true, ChromaTypes: true})
		}
	case "Swift":
		l.Tokenise("")
		lex = func(text string, end int) []fh.Token {
			return swift.Lex(text, end, swift.Options{ChromaNames: true, ChromaTypes: true})
		}
	case "R":
		l.Tokenise("")
		lex = func(text string, end int) []fh.Token { return r.Lex(text, end, r.Options{ChromaNames: true}) }
	case "Go":
		l.Tokenise("")
		lex = func(text string, end int) []fh.Token {
			return golang.Lex(text, end, golang.Options{ChromaIdent: true, ChromaTypeNames: true})
		}
	case "Ruby":
		l.Tokenise("")
		lex = func(text string, end int) []fh.Token { return ruby.Lex(text, end, ruby.Options{Chroma: true}) }
	case "Python":
		l.Tokenise("") // preparation (word lists)
		lex = func(text string, end int) []fh.Token {
			return python.Lex(text, end, python.Options{ChromaDocstring: true, ChromaTypes: true})
		}
	case "TypeScript":
		l.Tokenise("") // preparation (word lists)
		lex = func(text string, end int) []fh.Token {
			return typescript.Lex(text, end, typescript.Options{Chroma: true})
		}
	case "Rust":
		l.Tokenise("") // preparation (word lists)
		lex = func(text string, end int) []fh.Token { return rust.Lex(text, end, rust.Options{ChromaTypes: true}) }
	case "markdown":
		// Code blocks and front matter are also read with the version that matches Chroma (whether the language is
		// looked up the same way as in Chroma is checked by comparable).
		lex = func(text string, end int) []fh.Token {
			return markdown.Lex(text, end, markdown.Options{Chroma: true, Lookup: func(name string) func(string) []fh.Token {
				if sub := fh.LookupRegistered(name); sub != nil {
					return chromaMode(sub)
				}
				return nil
			}})
		}
	case "react":
		lex = func(text string, end int) []fh.Token { return js.LexReact(text, end, true) }
	case "HTTP":
		l.Tokenise("") // preparation (CSS tables)
		lex = func(text string, end int) []fh.Token {
			return http.Lex(text, end, http.Options{JS: js.Options{ChromaBraces: true}, Lookup: func(name string) func(string) []fh.Token {
				if sub := fh.LookupRegistered(name); sub != nil {
					return chromaMode(sub)
				}
				return nil
			}})
		}
	case "HTML":
		l.Tokenise("") // preparation (CSS tables)
		lex = func(text string, end int) []fh.Token { return html.Lex(text, end, js.Options{ChromaBraces: true}) }
	default:
		return l.Tokenise
	}
	return func(code string) []fh.Token {
		code = fh.NormalizeNewlines(code)
		end := len(code)
		if l.EnsureNewline && !strings.HasSuffix(code, "\n") {
			code += "\n"
		}
		return lex(code, end)
	}
}

// FuzzTokensMatchChroma checks that, for any input (if it is valid UTF-8), the token sequence is exactly
// the same as Chroma's.
//
//	go test -run='^$' -fuzz=FuzzTokensMatchChroma -fuzztime=1m .
func FuzzTokensMatchChroma(f *testing.F) {
	langs := make([]*fh.Language, len(exactLanguages))
	for i, name := range exactLanguages {
		langs[i] = fh.LookupRegistered(name)
		if langs[i] == nil {
			f.Fatalf("language %q is not registered", name)
		}
	}
	for _, b := range testcases.EdgeCases {
		if l := fh.LookupRegistered(b.Lang); l != nil {
			for i, x := range langs {
				if x == l {
					f.Add(uint8(i), b.Code)
				}
			}
		}
	}
	f.Fuzz(func(t *testing.T, lang uint8, code string) {
		if !utf8.ValidString(code) {
			t.Skip("Chroma replaces invalid UTF-8, so the token values cannot be compared")
		}
		l := langs[int(lang)%len(langs)]
		if why := incomparable(l, code); why != "" {
			t.Skip(why)
		}
		ours := chromaMode(l)(code)
		theirs, slow := chromaTokensTimed(t, l, code)
		if !slices.Equal(ours, theirs) {
			if slow {
				t.Skipf("%s: tokens differ, but Chroma may have hit its regexp timeout", l.Name)
			}
			t.Fatalf("%s: tokens differ from Chroma for %q\n ours %v\nchroma %v", l.Name, code, ours, theirs)
		}
	})
}

// incomparable returns the reason why code cannot be compared with Chroma for l ("" if it can be compared).
// Chroma looks up the language of a Markdown code block among all of its languages, so the comparison is made
// only when the language name resolves to the same language as a fasthighlight language (one that splits
// exactly like Chroma). Also, Chroma looks for front matter in the input before newlines are normalized, so
// when front matter is likely, the comparison is skipped if there is a lone \r (not part of \r\n)
// (fasthighlight normalizes newlines first, as in the other cases; see the "What was changed"
// section of website/compatibility.md).
func incomparable(l *fh.Language, code string) string {
	if l.Name == "HTTP" {
		return httpIncomparable(code)
	}
	if l.Name != "markdown" {
		return ""
	}
	if strings.HasPrefix(fh.NormalizeNewlines(code), "---\n") && strings.Contains(strings.ReplaceAll(code, "\r\n", ""), "\r") {
		return "markdown: a lone \\r"
	}
	for i := 0; ; {
		n := strings.Index(code[i:], "```")
		if n < 0 {
			return ""
		}
		i += n + 3
		w := i
		for w < len(code) {
			r, size := utf8.DecodeRuneInString(code[w:])
			if !lexkit.IsWord(r) {
				break
			}
			w += size
		}
		if w == i {
			continue
		}
		name := code[i:w]
		ours, theirs := fh.LookupRegistered(name), lexers.Get(name)
		switch {
		case theirs == nil && ours == nil:
		case theirs == nil || ours == nil || theirs.Config().Name != ours.Name:
			return "markdown: code block language " + name + " differs from Chroma's"
		case !slices.Contains(exactLanguages, ours.Name):
			return "markdown: code block language " + name + " is not compared exactly"
		}
	}
}

// chromaMatchTimeout is the timeout of a single match of a Chroma regular expression (MatchTimeout in
// regexp.go). When it times out, Chroma treats the rule as "not matched" and goes on to the next rule.
const chromaMatchTimeout = 250 * time.Millisecond

// chromaTokensTimed is the same as chromaTokens. It sets slow to true if Chroma took chromaMatchTimeout or
// longer. In that case a regular expression may have timed out, and Chroma's result may differ from the
// true result of the regular expression (this happens when the CPU is busy, and also with long inputs that
// cause a lot of backtracking). Chroma does not tell whether a timeout happened, so this is judged by the
// elapsed time. Without a timeout the whole run can still take 250ms or more, but with a timeout it always
// takes 250ms or more.
func chromaTokensTimed(t *testing.T, l *fh.Language, code string) (toks []fh.Token, slow bool) {
	t.Helper()
	start := time.Now()
	toks = chromaTokens(t, l, code)
	return toks, time.Since(start) >= chromaMatchTimeout
}
