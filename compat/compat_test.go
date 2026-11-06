package compat

import (
	"bytes"
	"html"
	"io"
	"math/rand/v2"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"

	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/testcases"
	_ "github.com/toshi-kuji/fasthighlight/lang/all"
	"github.com/toshi-kuji/fasthighlight/markdown"
)

var (
	formatter = chromahtml.New(chromahtml.WithClasses(true))
	style     = styles.Get("github")
)

// lookup finds a registered language from the language name of a code block.
func lookup(t *testing.T, name string) *fh.Language {
	t.Helper()
	l := fh.LookupRegistered(name)
	if l == nil {
		t.Fatalf("language %q is not registered", name)
	}
	return l
}

// chromaLexer returns the Chroma lexer that corresponds to a fasthighlight language.
func chromaLexer(t *testing.T, l *fh.Language) chroma.Lexer {
	t.Helper()
	cl := lexers.Get(l.Name)
	if cl == nil || cl.Config().Name != l.Name {
		t.Fatalf("Chroma has no lexer named %q", l.Name)
	}
	return cl
}

func allBlocks(t *testing.T) []testcases.Block {
	t.Helper()
	bs, err := testcases.InputBlocks("../testdata/input.md")
	if err != nil {
		t.Fatal(err)
	}
	return append(bs, testcases.EdgeCases...)
}

// chromaTokens splits the code into tokens with Chroma and converts them to fasthighlight types.
func chromaTokens(t *testing.T, l *fh.Language, code string) []fh.Token {
	t.Helper()
	it, err := chroma.Coalesce(chromaLexer(t, l)).Tokenise(nil, code)
	if err != nil {
		t.Fatal(err)
	}
	var toks []fh.Token
	for _, c := range it.Tokens() {
		toks = append(toks, fh.Token{Type: fh.TokenType(c.Type), Value: c.Value})
	}
	return dropHTTPBodyNewline(l, code, toks)
}

// chromaHTML turns tokens into HTML with Chroma's output.
func chromaHTML(t *testing.T, toks []fh.Token) string {
	t.Helper()
	cts := make([]chroma.Token, len(toks))
	for i, tk := range toks {
		cts[i] = chroma.Token{Type: chroma.TokenType(tk.Type), Value: tk.Value}
	}
	var b bytes.Buffer
	if err := formatter.Format(&b, style, chroma.Literator(cts...)); err != nil {
		t.Fatal(err)
	}
	// Only the class name of the outer wrapper (PreWrapper) is changed on purpose (Chroma uses chroma, we use
	// highlight). A " in a token value is escaped, so class="chroma" appears only inside a tag.
	return strings.ReplaceAll(b.String(), `class="chroma"`, `class="highlight"`)
}

func ourHTML(t *testing.T, toks []fh.Token) string {
	t.Helper()
	var b bytes.Buffer
	if err := fh.WriteHTML(&b, toks); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

// TestTokenTypesMatchChroma (check 1) checks that the values, names, class names and hierarchy of the
// types are the same as Chroma's.
func TestTokenTypesMatchChroma(t *testing.T) {
	for v := -100; v <= 9000; v++ {
		ct, tt := chroma.TokenType(v), fh.TokenType(v)
		if ct.IsATokenType() {
			if got, want := tt.String(), ct.String(); got != want {
				t.Errorf("name of %d: ours=%q chroma=%q", v, got, want)
			}
		} else if got := tt.String(); got != "TokenType("+itoa(v)+")" {
			t.Errorf("%d is not a Chroma type, but ours is named %q", v, got)
		}
		got, ok := tt.Class()
		want, wantOK := chroma.StandardTypes[ct]
		if ct == chroma.PreWrapper { // only the class name of the outer wrapper is changed on purpose
			want = "highlight"
		}
		if got != want || ok != wantOK {
			t.Errorf("class of %d: ours=%q,%v chroma=%q,%v", v, got, ok, want, wantOK)
		}
		if got, want := int(tt.Parent()), int(ct.Parent()); got != want {
			t.Errorf("parent of %d: ours=%d chroma=%d", v, got, want)
		}
		if got, want := int(tt.Category()), int(ct.Category()); got != want {
			t.Errorf("category of %d: ours=%d chroma=%d", v, got, want)
		}
		if got, want := int(tt.SubCategory()), int(ct.SubCategory()); got != want {
			t.Errorf("subcategory of %d: ours=%d chroma=%d", v, got, want)
		}
	}
}

func itoa(n int) string {
	var b []byte
	b = appendInt(b, n)
	return string(b)
}

func appendInt(b []byte, n int) []byte {
	if n < 0 {
		return appendInt(append(b, '-'), -n)
	}
	if n >= 10 {
		b = appendInt(b, n/10)
	}
	return append(b, byte('0'+n%10))
}

// byteTypes returns the token type of each byte.
func byteTypes(toks []fh.Token) []fh.TokenType {
	var ts []fh.TokenType
	for _, tk := range toks {
		for range len(tk.Value) {
			ts = append(ts, tk.Type)
		}
	}
	return ts
}

// intendedDiff is a place where the result intentionally differs from Chroma (listed in the
// "What was changed" section of website/compatibility.md).
type intendedDiff struct {
	lang         string // language name (Language.Name)
	text         string // the text of the part that differs
	ours, chroma fh.TokenType
	reason       string
}

// templateBraces: for the `})}` in `${f({ k: 1 })}`, Chroma closes the interpolation at the first } and
// makes the remaining `)}` part of the string. fasthighlight counts the depth, makes `})` punctuation and
// the last `}` the end of the interpolation.
const templateBraces = "{ } inside ${ … } of a template literal are closed by counting the depth (Chroma closes at the first })"

// docstring: Chroma colors a """…""" at the start of a line as a string (the [rRuUbB]{,2} in its rule is
// not a repetition in regexp2, so the docstring rule almost never matches). Ours colors it as a docstring.
// javaNumbers: Chroma colors Java numbers as names (the [^\W\d] in the name rule is the same as \w in
// regexp2, so it matches before the number rule). Ours colors them as numbers.
const javaNumbers = `Java numbers are colored as numbers (Chroma's name rule [^\W\d] is the same as \w in regexp2 and also matches numbers)`

// phpTags: Chroma makes the PHP <?php an operator and a name, and makes everything after ?> an Error.
// Ours makes <?php and ?> CommentPreproc and reads what follows ?> as HTML.
const phpTags = "PHP <?php and ?> are CommentPreproc, and what follows ?> is read as HTML (Chroma makes <? an operator and everything after ?> an Error)"

// goIdent: the Go name rule [^\W\d]\w* is the same as \w+ in regexp2, so numbers inside a template also
// become names.
const goIdent = "Go names do not start with a digit (Chroma's [^\\W\\d] is the same as \\w in regexp2)"

// rNames: the [a-zA-z] at the start of an R name is a typo, and [ ] ^ and so on also become names. Ours
// uses [a-zA-Z].
const rNames = "R names start with [a-zA-Z] (Chroma's [a-zA-z] also includes [ \\ ] ^ _ `)"

// unicodeNames: in Chroma, Swift and TypeScript names can start only with a Latin letter, and a name
// starting with any other character becomes an Error.
const unicodeNames = "Names can also start with a Unicode letter (Chroma makes a name starting with a non-Latin character an Error)"

// rawString: Chroma does not make the C++ R"(…)" a raw string (the [^\\()\s]{,16} in its rule is not a
// repetition in regexp2, so it almost never matches). Ours makes it a raw string.
const rawString = `C++ R"(…)" is a raw string (in Chroma's rule {,16} is not a repetition, so it almost never matches)`

const docstring = `"""…""" at the start of a line is colored as a docstring (in Chroma's rule {,2} is not a repetition, so it almost never matches)`

// mdMarkerNewline: even when a newline directly follows the # of a Markdown heading, the - of a list or the
// > of a quote, Chroma extends the heading and so on to the next line (the [^#] after # and the \s after -
// or > match a newline). Ours does not allow a newline directly after the marker.
const mdMarkerNewline = "If a newline directly follows # , - or > in Markdown, it is not a heading and so on (Chroma includes the next line)"

// mdEmphasis: the Markdown emphasis rule needs whitespace before it, so Chroma does not treat emphasis at
// the start of a line, or at the start of the content of a list item (the content is read without the
// whitespace after -), as emphasis.
const mdEmphasis = "Emphasis at the start of a Markdown line or list item is also emphasis (Chroma needs whitespace before it)"

// tsJSX: when a TypeScript < is directly followed by an alphanumeric character, Chroma reads it as a JSX tag
// (the type arguments of Map<string, number> become a tag and attributes). Ours does not handle JSX.
const tsJSX = "A TypeScript < is not a JSX tag (Chroma also makes the type arguments of Map<string> a tag)"

// tsTemplateEscape: Chroma's TypeScript template literal has no rule for escapes such as \n.
const tsTemplateEscape = "\\n and so on in a TypeScript template literal are part of the string (Chroma makes \\ an Error)"

var intendedDiffs = []intendedDiff{
	{"TypeScript", "}", fh.Punctuation, fh.LiteralStringInterpol, templateBraces},
	{"TypeScript", ")", fh.Punctuation, fh.LiteralStringBacktick, templateBraces},
	{"TypeScript", "}", fh.LiteralStringInterpol, fh.LiteralStringBacktick, templateBraces},
	{"JavaScript", "}", fh.Punctuation, fh.LiteralStringInterpol, templateBraces},
	{"JavaScript", ")", fh.Punctuation, fh.LiteralStringBacktick, templateBraces},
	{"JavaScript", "}", fh.LiteralStringInterpol, fh.LiteralStringBacktick, templateBraces},
	// The inside of an HTML <script> is JavaScript, so the same differences appear.
	{"HTML", "}", fh.Punctuation, fh.LiteralStringInterpol, templateBraces},
	{"HTML", ")", fh.Punctuation, fh.LiteralStringBacktick, templateBraces},
	{"HTML", "}", fh.LiteralStringInterpol, fh.LiteralStringBacktick, templateBraces},
	{"C++", "R", fh.LiteralStringAffix, fh.Name, rawString},
	{"C++", "(", fh.LiteralStringDelimiter, fh.LiteralString, rawString},
	{"C++", ")", fh.LiteralStringDelimiter, fh.LiteralString, rawString},
	{"Java", "42", fh.LiteralNumberInteger, fh.Name, javaNumbers},
	{"Java", "1", fh.LiteralNumberFloat, fh.Name, javaNumbers},
	{"Java", ".", fh.LiteralNumberFloat, fh.Punctuation, javaNumbers},
	{"Java", "5", fh.LiteralNumberFloat, fh.NameAttribute, javaNumbers},
	{"PHP", "<?", fh.CommentPreproc, fh.Operator, phpTags},
	{"PHP", "php", fh.CommentPreproc, fh.NameOther, phpTags},
	{"PHP", "\n", fh.Text, fh.Error, phpTags},
	{"Go", "1", fh.LiteralNumberInteger, fh.NameOther, goIdent},
	{"R", "1", fh.LiteralNumber, fh.Name, rNames},
	{"R", "2", fh.LiteralNumber, fh.Name, rNames},
	{"R", "[[", fh.Punctuation, fh.Name, rNames},
	{"R", "]]", fh.Punctuation, fh.Name, rNames},
	{"R", "^", fh.Operator, fh.Name, rNames},
	{"Swift", "日本", fh.Name, fh.Error, unicodeNames},
	{"TypeScript", "名前", fh.NameOther, fh.Error, unicodeNames},
	{"TypeScript", "<", fh.Operator, fh.Punctuation, tsJSX},
	{"TypeScript", "string", fh.KeywordType, fh.NameTag, tsJSX},
	{"TypeScript", "number", fh.KeywordType, fh.NameAttribute, tsJSX},
	{"TypeScript", ">", fh.Operator, fh.Punctuation, tsJSX},
	{"TypeScript", "\\", fh.LiteralStringBacktick, fh.Error, tsTemplateEscape},
	{"Python", `"""Module docstring."""`, fh.LiteralStringDoc, fh.LiteralStringDouble, docstring},
	{"Python", `'''raw docstring'''`, fh.LiteralStringDoc, fh.LiteralStringSingle, docstring},
	{"markdown", "**Note**", fh.GenericStrong, fh.Text, mdEmphasis},
	{"markdown", "**a**", fh.GenericStrong, fh.Text, mdEmphasis},
	{"markdown", "#\nnot a heading\n", fh.Text, fh.GenericHeading, mdMarkerNewline},
	{"markdown", "-", fh.Text, fh.Keyword, mdMarkerNewline},
	{"markdown", ">\n", fh.Text, fh.Keyword, mdMarkerNewline},
	{"markdown", "not a quote\n", fh.Text, fh.GenericEmph, mdMarkerNewline},
	// react is JavaScript with JSX, so it has JavaScript's template-literal difference, and its template
	// literal has no rule for escapes such as \n (like TypeScript's).
	{"react", "}", fh.Punctuation, fh.LiteralStringInterpol, templateBraces},
	{"react", ")", fh.Punctuation, fh.LiteralStringBacktick, templateBraces},
	{"react", "}", fh.LiteralStringInterpol, fh.LiteralStringBacktick, templateBraces},
	{"react", "\\", fh.LiteralStringBacktick, fh.Error, reactTemplateEscape},
}

// reactTemplateEscape: Chroma's react template literal has no rule for escapes such as \n.
const reactTemplateEscape = "\\n and so on in a react (JSX) template literal are part of the string (Chroma makes \\ an Error)"

// diffRun is a contiguous part whose type differs.
type diffRun struct {
	at           int
	text         string
	ours, chroma fh.TokenType
}

// diffRuns returns the parts whose types differ between the token sequences a and b of the same input.
func diffRuns(code string, a, b []fh.Token) []diffRun {
	x, y := byteTypes(a), byteTypes(b)
	var runs []diffRun
	for k := 0; k < len(x); {
		if x[k] == y[k] {
			k++
			continue
		}
		end := k
		for end < len(x) && x[end] == x[k] && y[end] == y[k] {
			end++
		}
		runs = append(runs, diffRun{k, code[k:end], x[k], y[k]})
		k = end
	}
	return runs
}

// TestCompareWithChroma (check 2) checks that the token sequence is the same as Chroma's. Only the
// differences listed in intendedDiffs are accepted.
func TestCompareWithChroma(t *testing.T) {
	used := make([]bool, len(intendedDiffs))
	identical, total := 0, 0
	for n, c := range allBlocks(t) {
		if c.Code == "" {
			continue
		}
		total++
		l := lookup(t, c.Lang)
		ours := l.Tokenise(c.Code)
		theirs := chromaTokens(t, l, c.Code)
		code := joinTokens(ours)
		if code != joinTokens(theirs) {
			t.Errorf("case %d (%s): tokens do not cover the same text\n ours %q\nchroma %q", n, l.Name, code, joinTokens(theirs))
			continue
		}
		runs := diffRuns(code, ours, theirs)
		if len(runs) == 0 {
			identical++
			continue
		}
	runs:
		for _, r := range runs {
			for i, d := range intendedDiffs {
				if d.lang == l.Name && d.text == r.text && d.ours == r.ours && d.chroma == r.chroma {
					used[i] = true
					t.Logf("case %d (%s): intended difference at %d %q: %s", n, l.Name, r.at, r.text, d.reason)
					continue runs
				}
			}
			t.Errorf("case %d (%s): unexpected difference at %d %q: ours=%v chroma=%v",
				n, l.Name, r.at, r.text, r.ours, r.chroma)
		}
	}
	for i, d := range intendedDiffs {
		if !used[i] {
			t.Errorf("intended difference %q (%s) was not seen; remove it or add a case", d.text, d.lang)
		}
	}
	t.Logf("%d/%d cases identical to Chroma", identical, total)
}

// builtin returns true if name is a language supported by fasthighlight.
func builtin(name string) bool {
	for _, l := range fh.Builtins() {
		if l.Name == name {
			return true
		}
	}
	return false
}

func joinTokens(toks []fh.Token) string {
	var b strings.Builder
	for _, tk := range toks {
		b.WriteString(tk.Value)
	}
	return b.String()
}

// TestLanguagesMatchChroma (check 3) checks that, for every language in the main list (Builtins), the name,
// aliases, filename patterns and trailing-newline handling are the same as Chroma's. A lookup by language
// name must also resolve to the same language as Chroma's lexers.Get.
func TestLanguagesMatchChroma(t *testing.T) {
	langs := fh.Builtins()
	if len(langs) == 0 {
		t.Fatal("no built-in languages")
	}
	for _, l := range langs {
		cfg := chromaLexer(t, l).Config()
		if !slices.Equal(l.Aliases, cfg.Aliases) {
			t.Errorf("%s: aliases ours=%q chroma=%q", l.Name, l.Aliases, cfg.Aliases)
		}
		if !slices.Equal(l.Filenames, cfg.Filenames) {
			t.Errorf("%s: filenames ours=%q chroma=%q", l.Name, l.Filenames, cfg.Filenames)
		}
		if l.EnsureNewline != cfg.EnsureNL {
			t.Errorf("%s: EnsureNewline ours=%v chroma=%v", l.Name, l.EnsureNewline, cfg.EnsureNL)
		}
		keys := append([]string{l.Name, strings.ToLower(l.Name), strings.ToUpper(l.Name)}, l.Aliases...)
		for _, pat := range l.Filenames {
			for _, f := range expandClasses(pat) {
				if ext, ok := strings.CutPrefix(f, "*."); ok {
					keys = append(keys, ext, "file."+ext)
				} else {
					keys = append(keys, f) // a filename itself, such as "Pipfile"
				}
			}
		}
		if len(l.Filenames) > 0 { // HTTP has no filename patterns
			keys = append(keys, "dir/file."+strings.TrimPrefix(l.Filenames[0], "*."), "file."+strings.TrimPrefix(l.Filenames[0], "*.")+".bak")
		}
		for _, k := range keys {
			cl := lexers.Get(k)
			if cl == nil {
				t.Errorf("%s: Chroma does not resolve %q", l.Name, k)
				continue
			}
			want := cl.Config().Name // when several languages' patterns match, Chroma may choose another language (*.xslt is HTML)
			if !builtin(want) {
				// Chroma chooses a language that is not supported yet. When that language is added, the match is checked here.
				t.Logf("%s: Chroma resolves %q to %s, which fasthighlight does not support yet", l.Name, k, want)
				continue
			}
			if got := fh.LookupRegistered(k); got == nil || got.Name != want {
				t.Errorf("%s: LookupRegistered(%q) = %v, Chroma resolves it to %s", l.Name, k, got, want)
			}
		}
	}
}

// expandClasses expands a character set such as [bp] (without ranges) in a filename pattern into single
// characters ("*.x[bp]m" gives "*.xbm" and "*.xpm").
func expandClasses(pat string) []string {
	open := strings.IndexByte(pat, '[')
	if open < 0 {
		return []string{pat}
	}
	n := strings.IndexByte(pat[open:], ']')
	if n < 0 {
		return []string{pat}
	}
	var out []string
	for _, c := range pat[open+1 : open+n] {
		out = append(out, expandClasses(pat[:open]+string(c)+pat[open+n+1:])...)
	}
	return out
}

// TestHTMLMatchesChroma (check 4) checks that the same token sequence produces HTML that is byte-for-byte
// the same as Chroma's output.
func TestHTMLMatchesChroma(t *testing.T) {
	check := func(name string, toks []fh.Token) {
		t.Helper()
		if got, want := ourHTML(t, toks), chromaHTML(t, toks); got != want {
			t.Errorf("%s: %q\n got %q\nwant %q", name, toks, got, want)
		}
	}
	for n, c := range allBlocks(t) {
		check("block "+itoa(n), lookup(t, c.Lang).Tokenise(c.Code))
	}
	// An unsupported language (not highlighted).
	for _, code := range []string{"", "\n", "plain\n", "a\nb", "<a href=\"x\">'&'</a>\n", "\n\n\n", "日本語\n"} {
		check("text "+code, []fh.Token{{Type: fh.Text, Value: code}})
	}

	// Random token sequences. The types also include types that are not in the class name table and values
	// that Chroma does not have. EOFType (0) is not used, because it marks the end in Chroma's Iterator.
	var types []fh.TokenType
	for _, ct := range chroma.TokenTypeValues() {
		if ct != chroma.EOFType {
			types = append(types, fh.TokenType(ct))
		}
	}
	types = append(types, 2099, 3999, 9000, -50)
	values := []string{"", "", "\n", "\n\n", "a", "ab\n", "x\ny", "<&>'\"", "日本", " "}
	r := rand.New(rand.NewPCG(1, 2))
	for i := 0; i < 5000; i++ {
		toks := make([]fh.Token, r.IntN(8))
		for k := range toks {
			toks[k] = fh.Token{Type: types[r.IntN(len(types))], Value: values[r.IntN(len(values))]}
		}
		check("random "+itoa(i), toks)
	}
}

// TestConvertMatchesChroma (check 5) checks that the conversion result of the whole manuscript is the same
// as a conversion that uses Chroma's output.
func TestConvertMatchesChroma(t *testing.T) {
	src, err := os.ReadFile("../testdata/input.md")
	if err != nil {
		t.Fatal(err)
	}
	var want bytes.Buffer
	ref := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithRendererOptions(renderer.WithNodeRenderers(util.Prioritized(&chromaCodeRenderer{t: t}, 200))),
	)
	if err := ref.Convert(src, &want); err != nil {
		t.Fatal(err)
	}
	for _, parallel := range []markdown.Parallelism{markdown.ParallelNever, markdown.ParallelAlways} {
		c := markdown.NewConverter()
		c.Parallelism = parallel
		var got bytes.Buffer
		if err := c.Convert(src, &got); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got.Bytes(), want.Bytes()) {
			t.Errorf("parallelism %d: output differs from the Chroma-based conversion", parallel)
		}
	}
	// testdata/input.html (used by the tests of the main module) must also be the same as the one made with Chroma.
	golden, err := os.ReadFile("../testdata/input.html")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(golden, want.Bytes()) {
		t.Errorf("testdata/input.html differs from the Chroma-based conversion")
	}
}

// chromaCodeRenderer turns fasthighlight tokens into HTML with Chroma's output.
type chromaCodeRenderer struct{ t *testing.T }

func (r *chromaCodeRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindFencedCodeBlock, r.render)
}

func (r *chromaCodeRenderer) render(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	b := node.(*ast.FencedCodeBlock)
	var code bytes.Buffer
	for i := 0; i < b.Lines().Len(); i++ {
		line := b.Lines().At(i)
		code.Write(line.Value(src))
	}
	var toks []fh.Token
	if l := fh.LookupRegistered(string(b.Language(src))); l != nil {
		toks = l.Tokenise(code.String())
	} else {
		toks = []fh.Token{{Type: fh.Text, Value: code.String()}}
	}
	out := chromaHTML(r.t, toks)
	// The language name of the code block goes into data-lang and the language- class of <code> (Chroma's
	// output has neither).
	if name := html.EscapeString(string(b.Language(src))); name != "" {
		out = strings.Replace(out, `<pre class="highlight"><code>`,
			`<pre class="highlight" data-lang="`+name+`"><code class="language-`+name+`">`, 1)
	}
	_, err := io.WriteString(w, out)
	return ast.WalkContinue, err
}
