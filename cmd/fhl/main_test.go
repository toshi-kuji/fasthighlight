package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	fh "github.com/toshi-kuji/fasthighlight"
)

// fhlRun runs fhl with dir as the current directory.
func fhlRun(t *testing.T, dir string, stdin string, interactive bool, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	if dir != "" {
		t.Chdir(dir)
	}
	var out, errOut bytes.Buffer
	code = run(args, env{stdin: strings.NewReader(stdin), stdout: &out, stderr: &errOut, interactive: interactive})
	return code, out.String(), errOut.String()
}

// writeFiles writes files (path -> contents) to dir.
func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for p, s := range files {
		p = filepath.Join(dir, p)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func exists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

const readme = "# Usage\n\n```go\nfmt.Println(\"hi\")\n```\n"

const readmeHTML = `<h1>Usage</h1>
<pre class="highlight" data-lang="go"><code class="language-go"><span class="nx">fmt</span><span class="p">.</span><span class="nf">Println</span><span class="p">(</span><span class="s">&#34;hi&#34;</span><span class="p">)</span>
</code></pre>`

func TestDefaultWritesNextToInput(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"README.md": readme})
	code, stdout, stderr := fhlRun(t, dir, "", false, "README.md")
	if code != 0 || stdout != "" {
		t.Fatalf("code %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	if got := readFile(t, "README.html"); got != readmeHTML {
		t.Errorf("README.html:\n%s", got)
	}
	if got := readFile(t, "highlight.css"); got != lookupTheme("default").css(modeBoth, cssOptions{}) {
		t.Errorf("highlight.css differs")
	}
	if !strings.HasPrefix(stderr, "fhl: 1 files, 1 code blocks (0 uncolored) in ") {
		t.Errorf("summary: %q", stderr)
	}
}

func TestFolder(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"docs/a.md":           readme,
		"docs/sub/b.markdown": "text\n",
		"docs/sub/c.mkd":      "text\n",
		"docs/notes.txt":      "not markdown",
		"docs/.hidden/d.md":   "x",
		"docs/.e.md":          "x",
	})
	code, _, stderr := fhlRun(t, dir, "", false, "docs/")
	if code != 0 {
		t.Fatalf("code %d: %s", code, stderr)
	}
	for _, p := range []string{"docs/a.html", "docs/highlight.css", "docs/sub/b.html", "docs/sub/c.html", "docs/sub/highlight.css"} {
		if !exists(p) {
			t.Errorf("%s was not written", p)
		}
	}
	for _, p := range []string{"docs/notes.html", "docs/.hidden/d.html", "docs/.e.html"} {
		if exists(p) {
			t.Errorf("%s was written", p)
		}
	}
}

func TestOutputFolderKeepsLayout(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"docs/intro.md": readme, "docs/sub/b.md": "x\n"})
	code, stdout, stderr := fhlRun(t, dir, "", false, "-o", "dist/", "docs/", "--json")
	if code != 0 {
		t.Fatalf("code %d: %s", code, stderr)
	}
	if stderr != "" {
		t.Errorf("stderr with --json: %q", stderr)
	}
	for _, p := range []string{"dist/intro.html", "dist/highlight.css", "dist/sub/b.html", "dist/sub/highlight.css"} {
		if !exists(p) {
			t.Errorf("%s was not written", p)
		}
	}
	var r report
	if err := json.Unmarshal([]byte(stdout), &r); err != nil {
		t.Fatal(err)
	}
	want := []fileResult{
		{Input: filepath.Join("docs", "intro.md"), Output: filepath.Join("dist", "intro.html"), Blocks: 1},
		{Input: filepath.Join("docs", "sub", "b.md"), Output: filepath.Join("dist", "sub", "b.html")},
	}
	if fmt.Sprint(r.Files) != fmt.Sprint(want) {
		t.Errorf("files %v, want %v", r.Files, want)
	}
	if fmt.Sprint(r.Styles) != fmt.Sprint([]string{filepath.Join("dist", "highlight.css"), filepath.Join("dist", "sub", "highlight.css")}) {
		t.Errorf("styles %v", r.Styles)
	}
	if r.Summary.Files != 2 || r.Summary.Blocks != 1 {
		t.Errorf("summary %+v", r.Summary)
	}
}

func TestOutputFile(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"README.md": readme})
	code, _, stderr := fhlRun(t, dir, "", false, "-o", "out/page.html", "README.md")
	if code != 0 {
		t.Fatalf("code %d: %s", code, stderr)
	}
	if readFile(t, "out/page.html") != readmeHTML || !exists("out/highlight.css") {
		t.Errorf("out/page.html or out/highlight.css is wrong")
	}
	// -o file is allowed only when there is a single input.
	writeFiles(t, dir, map[string]string{"b.md": "x"})
	if code, _, stderr := fhlRun(t, "", "", false, "-o", "page.html", "README.md", "b.md"); code != 1 || !strings.Contains(stderr, "needs exactly one input file") {
		t.Errorf("two inputs with -o FILE: code %d, %q", code, stderr)
	}
}

func TestStdout(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"README.md": readme})
	code, stdout, stderr := fhlRun(t, dir, "", false, "-o", "-", "README.md")
	if code != 0 || stdout != readmeHTML {
		t.Fatalf("code %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	if exists("highlight.css") || exists("README.html") {
		t.Errorf("-o - wrote a file")
	}
	code, stdout, _ = fhlRun(t, "", "", false, "--single-html", "-o", "-", "README.md")
	want := "<!DOCTYPE html>\n<meta charset=\"utf-8\">\n<title>Usage</title>\n<style>\n" + lookupTheme("default").css(modeBoth, cssOptions{}) + "</style>\n" +
		strings.Replace(readmeHTML, "<h1>", `<h1 id="usage">`, 1) // a standalone page: headings get ids
	if code != 0 || stdout != want {
		t.Errorf("--single-html -o -: code %d\n%s", code, stdout)
	}
	if exists("highlight.css") {
		t.Errorf("--single-html wrote highlight.css")
	}
	code, _, stderr = fhlRun(t, "", "", false, "--theme=both:default", "-o", "-", "README.md")
	if code != 1 || !strings.Contains(stderr, "--theme has no effect with -o -") {
		t.Errorf("--theme with -o -: code %d, %q", code, stderr)
	}
	code, _, stderr = fhlRun(t, "", "", false, "--json", "-o", "-", "README.md")
	if code != 1 || !strings.Contains(stderr, "--json cannot be used with -o -") {
		t.Errorf("--json with -o -: code %d, %q", code, stderr)
	}
}

func TestSingleHTML(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"README.md": readme, "plain.md": "no heading <b>x</b>\n"})
	code, _, stderr := fhlRun(t, dir, "", false, "--single-html", "README.md", "plain.md")
	if code != 0 {
		t.Fatalf("code %d: %s", code, stderr)
	}
	if exists("highlight.css") {
		t.Errorf("--single-html wrote highlight.css")
	}
	// Headings get ids, so links within the page work (as with --static-site).
	if got := readFile(t, "README.html"); !strings.Contains(got, `<h1 id="usage">Usage</h1>`) {
		t.Errorf("README.html: no heading id:\n%s", got)
	}
	got := readFile(t, "plain.html")
	want := "<!DOCTYPE html>\n<meta charset=\"utf-8\">\n<title>plain.md</title>\n<style>\n" + lookupTheme("default").css(modeBoth, cssOptions{}) + "</style>\n<p>no heading &lt;b&gt;x&lt;/b&gt;</p>\n"
	if got != want {
		t.Errorf("plain.html:\n%s", got)
	}
	// Besides the four (DOCTYPE, character encoding, title, CSS), no <html>, <head>, <body> or <link> is added.
	for _, tag := range []string{"<html", "<head", "<body", "<link"} {
		if strings.Contains(got, tag) {
			t.Errorf("--single-html has %s", tag)
		}
	}
}

func TestStdin(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	code, _, stderr := fhlRun(t, "", readme, false, "-o", "note.html", "-")
	if code != 0 || readFile(t, "note.html") != readmeHTML {
		t.Fatalf("code %d: %s", code, stderr)
	}
	code, _, stderr = fhlRun(t, "", readme, false, "-")
	if code != 1 || !strings.Contains(stderr, "reading standard input needs -o FILE, -o DIR/ or -o -") {
		t.Errorf("stdin without -o: code %d, %q", code, stderr)
	}
	code, stdout, _ := fhlRun(t, "", "x := 1\n", false, "--lang", "go", "-o", "-", "-")
	if code != 0 || !strings.HasPrefix(stdout, `<pre class="highlight" data-lang="go">`) {
		t.Errorf("--lang go -: code %d, %q", code, stdout)
	}
	code, stdout, _ = fhlRun(t, "", "# T\n", false, "--single-html", "-o", "dir/", "-")
	if code != 0 || !strings.Contains(readFile(t, "dir/stdin.html"), "<title>T</title>") {
		t.Errorf("-o DIR/ -: code %d, %q", code, stdout)
	}
	code, _, stderr = fhlRun(t, "", "", false, "-o", "-", "-", "-")
	if code != 1 || !strings.Contains(stderr, "only once") {
		t.Errorf("two -: code %d, %q", code, stderr)
	}
}

func TestCodeFile(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"main.go": "x := 1\n", "main.txt": "x := 1\n", "main.zig": "x"})
	code, _, stderr := fhlRun(t, dir, "", false, "main.go")
	if code != 0 || !strings.HasPrefix(readFile(t, "main.html"), `<pre class="highlight" data-lang="go"><code class="language-go">`) {
		t.Fatalf("code %d: %s", code, stderr)
	}
	code, stdout, _ := fhlRun(t, "", "", false, "--lang", "Go", "-o", "-", "main.txt")
	if code != 0 || !strings.HasPrefix(stdout, `<pre class="highlight" data-lang="Go"><code class="language-Go"><span class="nx">x`) {
		t.Errorf("--lang Go: %q", stdout)
	}
	code, _, stderr = fhlRun(t, "", "", false, "main.zig")
	if code != 1 || !strings.Contains(stderr, "fhl: no language for main.zig; use --lang NAME (see fhl --list-langs)") {
		t.Errorf("main.zig: code %d, %q", code, stderr)
	}
	code, _, stderr = fhlRun(t, "", "", false, "--lang", "zig", "main.zig")
	if code != 1 || !strings.Contains(stderr, `unknown language "zig"`) {
		t.Errorf("--lang zig: code %d, %q", code, stderr)
	}
}

func TestOverwrite(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"README.md": readme, "README.html": "old", "highlight.css": "mine"})
	code, _, stderr := fhlRun(t, dir, "", false, "README.md")
	// HTML could not be written, so highlight.css is not written either.
	if code != 1 || !strings.Contains(stderr, "fhl: README.html already exists; use --force to overwrite it") ||
		strings.Contains(stderr, "highlight.css") {
		t.Fatalf("not a terminal: code %d, %q", code, stderr)
	}
	if readFile(t, "README.html") != "old" || readFile(t, "highlight.css") != "mine" {
		t.Errorf("overwritten without --force")
	}
	code, _, stderr = fhlRun(t, "", "", false, "--force", "README.md")
	if code != 0 || readFile(t, "README.html") != readmeHTML || readFile(t, "highlight.css") == "mine" {
		t.Fatalf("--force: code %d, %q", code, stderr)
	}
	// If the contents are identical, nothing is written and no prompt is shown.
	code, _, stderr = fhlRun(t, "", "", false, "README.md")
	if code != 0 {
		t.Errorf("same content: code %d, %q", code, stderr)
	}
}

func TestOverwritePrompt(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"a.md": "a\n", "b.md": "b\n", "c.md": "c\n", "a.html": "old", "b.html": "old", "c.html": "old", "highlight.css": "old"})
	// a gets y and b gets N. Before c is asked, the rest is set to a (overwrite all).
	code, stdout, stderr := fhlRun(t, dir, "y\nn\na\n", true, "--json", "a.md", "b.md", "c.md")
	if code != 0 {
		t.Fatalf("code %d: %s", code, stderr)
	}
	if got := strings.Count(stderr, "[y/N/a]"); got != 3 {
		t.Errorf("asked %d times: %q", got, stderr)
	}
	if readFile(t, "a.html") != "<p>a</p>\n" || readFile(t, "b.html") != "old" || readFile(t, "c.html") != "<p>c</p>\n" || readFile(t, "highlight.css") == "old" {
		t.Errorf("answers were not followed")
	}
	var r report
	if err := json.Unmarshal([]byte(stdout), &r); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(r.Skipped) != "[b.html]" {
		t.Errorf("skipped %v", r.Skipped)
	}
	// With no answer (EOF), the answer is N.
	writeFiles(t, dir, map[string]string{"a.html": "old"})
	if code, _, _ := fhlRun(t, "", "", true, "a.md"); code != 0 || readFile(t, "a.html") != "old" {
		t.Errorf("EOF: code %d", code)
	}
	// When standard input is used as an input, answers cannot be read, so it behaves the same as when not run from a terminal.
	if code, _, stderr := fhlRun(t, "", "a\n", true, "-o", "a.html", "-"); code != 1 || !strings.Contains(stderr, "use --force") {
		t.Errorf("stdin input: code %d, %q", code, stderr)
	}
}

func TestIsTerminal(t *testing.T) {
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if isTerminal(f.Fd()) {
		t.Errorf("%s is a terminal", os.DevNull)
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	if isTerminal(r.Fd()) || isTerminal(w.Fd()) {
		t.Errorf("a pipe is a terminal")
	}
}

func TestParseArgs(t *testing.T) {
	same := [][]string{
		{"--json", "--strict", "docs/", "examples/"},
		{"docs/", "--json", "examples/", "--strict"},
		{"docs/", "examples/", "--strict", "--json"},
	}
	for _, args := range same {
		o, err := parseArgs(args)
		if err != nil {
			t.Fatal(err)
		}
		if !o.json || !o.strict || fmt.Sprint(o.inputs) != "[docs/ examples/]" {
			t.Errorf("%v: %+v", args, o)
		}
	}
	o, _ := parseArgs([]string{"docs/", "--", "--json", "-o"})
	if o.json || fmt.Sprint(o.inputs) != "[docs/ --json -o]" {
		t.Errorf("--: %+v", o)
	}
	o, _ = parseArgs([]string{"--lang", "go", "-"})
	if o.lang != "go" || fmt.Sprint(o.inputs) != "[-]" {
		t.Errorf("-: %+v", o)
	}
	o, _ = parseArgs([]string{"-o", "-", "a.md"})
	if o.out != "-" || !o.outSet || fmt.Sprint(o.inputs) != "[a.md]" {
		t.Errorf("-o -: %+v", o)
	}
	o, _ = parseArgs([]string{"-o", "--", "a.md", "--json"})
	if o.out != "--" || !o.json || fmt.Sprint(o.inputs) != "[a.md]" {
		t.Errorf("-o --: %+v", o)
	}
	o, _ = parseArgs([]string{"--theme=a", "x", "--theme=b"})
	if o.theme != "b" || !o.themeSet {
		t.Errorf("last value: %+v", o)
	}
	o, _ = parseArgs([]string{"x"})
	if o.theme != "both:default" || o.themeSet {
		t.Errorf("default theme: %+v", o)
	}
	// Values can be joined with = or given as the next argument, --theme included.
	for _, args := range [][]string{{"--theme", "dark:github", "x"}, {"x", "--theme=dark:github"}, {"x", "-theme", "dark:github"}} {
		if o, err := parseArgs(args); err != nil || o.theme != "dark:github" || fmt.Sprint(o.inputs) != "[x]" {
			t.Errorf("%v: %+v, %v", args, o, err)
		}
	}
	if o, err := parseArgs([]string{"--html-lang", "ja", "--html-lang=fr", "x"}); err != nil || o.htmlLang != "fr" {
		t.Errorf("--html-lang: %+v, %v", o, err)
	}
	// --theme as the value of another flag, or after --, is not the flag.
	if o, err := parseArgs([]string{"-o", "--theme", "x"}); err != nil || o.out != "--theme" {
		t.Errorf("-o --theme: %+v, %v", o, err)
	}
	if o, err := parseArgs([]string{"--", "--theme"}); err != nil || fmt.Sprint(o.inputs) != "[--theme]" {
		t.Errorf("-- --theme: %+v, %v", o, err)
	}
	if _, err := parseArgs([]string{"--nope"}); err == nil {
		t.Errorf("unknown flag was accepted")
	}
}

func TestCollisions(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"guide.md": "a", "guide.markdown": "b", "ok.md": "ok",
		"a/README.md": "a", "b/README.md": "b",
		"Guide.md": "c",
	})
	t.Chdir(dir)
	check := func(name string, wantMsg string, args ...string) {
		t.Helper()
		code, _, stderr := fhlRun(t, "", "", false, args...)
		if code != 1 || !strings.Contains(stderr, wantMsg) {
			t.Errorf("%s: code %d, stderr %q", name, code, stderr)
		}
	}
	check("same folder", "fhl: guide.md and guide.markdown would both write guide.html; rename one of them or convert them separately", "guide.md", "guide.markdown", "ok.md")
	if !exists("ok.html") || exists("guide.html") {
		t.Errorf("the other input was not converted, or a colliding one was")
	}
	check("-o dist/", "would both write "+filepath.Join("dist", "README.html"), "-o", "dist/", "a/README.md", "b/README.md")
	check("case", "would both write", "-o", "dist2/", "ok.md", "a/README.md", "--lang", "go", "Guide.md", "guide.md")
	check("-o highlight.css", "where fhl writes the style sheet", "-o", "highlight.css", "ok.md")
	check("own input", "fhl: ok.md would overwrite its own input; choose another -o", "--force", "-o", "ok.md", "ok.md")
	writeFiles(t, dir, map[string]string{"highlight.css": "x"})
	check("css input", "would overwrite the input highlight.css", "--force", "highlight.css")
	if readFile(t, "highlight.css") != "x" {
		t.Errorf("the input highlight.css was overwritten")
	}
	if runtime.GOOS != "windows" {
		if err := os.Symlink("ok.md", "link.html"); err != nil {
			t.Fatal(err)
		}
		check("symlink", "fhl: ok.md would overwrite its own input", "--force", "-o", "link.html", "ok.md")
		if readFile(t, "ok.md") != "ok" {
			t.Errorf("the input was overwritten through a symlink")
		}
	}
}

func TestFrontMatter(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"a.md": "---\ntitle: x\n---\n# A\n\n```\nx\n```\n",
		"b.md": "---\nnot closed\n\n```\nx\n```\n",
	})
	code, stdout, stderr := fhlRun(t, dir, "", false, "--check", "--json", "a.md", "b.md")
	if code != 0 {
		t.Fatalf("code %d: %s", code, stderr)
	}
	var r report
	if err := json.Unmarshal([]byte(stdout), &r); err != nil {
		t.Fatal(err)
	}
	want := []uncolored{{File: "a.md", Line: 6, Reason: "no-language"}, {File: "b.md", Line: 4, Reason: "no-language"}}
	if fmt.Sprint(r.Uncolored) != fmt.Sprint(want) {
		t.Errorf("uncolored %v, want %v", r.Uncolored, want)
	}
	_, out, _ := fhlRun(t, "", "", false, "-o", "-", "a.md")
	if strings.Contains(out, "title") || !strings.HasPrefix(out, "<h1>A</h1>") {
		t.Errorf("front matter was written: %q", out)
	}
	_, out, _ = fhlRun(t, "", "", false, "-o", "-", "b.md")
	if !strings.HasPrefix(out, "<hr>") {
		t.Errorf("unclosed front matter: %q", out)
	}
}

// TestFrontMatterOnlyYAML checks that a document starting with --- keeps its Markdown when what follows is not
// YAML: a thematic break, then a paragraph and a setext heading (whose underline is the second --- line).
func TestFrontMatterOnlyYAML(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{"---\n\nIntro paragraph that matters.\n\nSection\n---\n\nBody text.\n", "<hr>\n<p>Intro paragraph that matters.</p>\n<h2>Section</h2>\n<p>Body text.</p>\n"},
		{"---\nNote that this is a heading\n---\n", "<hr>\n<h2>Note that this is a heading</h2>\n"},
		{"---\n# Title\n---\n", "<hr>\n<h1>Title</h1>\n<hr>\n"},
		{"---\n- a\n- b\n---\n", "<hr>\n<ul>\n<li>a</li>\n<li>b</li>\n</ul>\n<hr>\n"},
		{"---\ntitle: Hello\ntags: [a, b]\n---\nBody\n", "<p>Body</p>\n"},
		{"---\ndraft:\n---\nBody\n", "<p>Body</p>\n"},
	} {
		dir := t.TempDir()
		writeFiles(t, dir, map[string]string{"p.md": c.src})
		_, out, stderr := fhlRun(t, dir, "", false, "-o", "-", "p.md")
		if out != c.want {
			t.Errorf("%q:\n got %q\nwant %q (%s)", c.src, out, c.want, stderr)
		}
	}
}

const rawInput = "<div class=\"note\">\n注意\n</div>\n\n文中の<b>太字</b>\n"

func TestRawHTML(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"raw.md": rawInput})
	tests := []struct{ mode, want string }{
		{"escape", "<pre>&lt;div class=&quot;note&quot;&gt;\n注意\n&lt;/div&gt;\n</pre>\n<p>文中の&lt;b&gt;太字&lt;/b&gt;</p>\n"},
		{"omit", "<!-- raw HTML omitted -->\n<p>文中の<!-- raw HTML omitted -->太字<!-- raw HTML omitted --></p>\n"},
		{"unsafe", rawInput[:len("<div class=\"note\">\n注意\n</div>\n")] + "<p>文中の<b>太字</b></p>\n"},
	}
	t.Chdir(dir)
	for _, tt := range tests {
		code, out, stderr := fhlRun(t, "", "", false, "--raw-html", tt.mode, "-o", "-", "raw.md")
		if code != 0 || out != tt.want {
			t.Errorf("%s: code %d, %q\nwant %q\n%s", tt.mode, code, out, tt.want, stderr)
		}
	}
	// Blocks that have a closing line (such as <script>) are also turned into text up to the closing line.
	writeFiles(t, dir, map[string]string{"s.md": "<script>\nalert(1)\n</script>\n"})
	_, out, _ := fhlRun(t, "", "", false, "-o", "-", "s.md")
	if out != "<pre>&lt;script&gt;\nalert(1)\n&lt;/script&gt;\n</pre>\n" {
		t.Errorf("script block: %q", out)
	}
	if code, _, stderr := fhlRun(t, "", "", false, "--raw-html", "safe", "raw.md"); code != 1 || !strings.Contains(stderr, "--raw-html must be") {
		t.Errorf("bad --raw-html: code %d, %q", code, stderr)
	}
}

func TestNewlines(t *testing.T) {
	dir := t.TempDir()
	lf := "---\na: 1\n---\n# T\n\n<div>\nx\n</div>\n\n```go\nx := 1\n```\n"
	writeFiles(t, dir, map[string]string{
		"lf.md":   lf,
		"crlf.md": strings.ReplaceAll(lf, "\n", "\r\n"),
		"cr.md":   strings.ReplaceAll(lf, "\n", "\r"),
	})
	t.Chdir(dir)
	_, want, _ := fhlRun(t, "", "", false, "-o", "-", "lf.md")
	for _, f := range []string{"crlf.md", "cr.md"} {
		_, got, _ := fhlRun(t, "", "", false, "-o", "-", f)
		if got != want || strings.Contains(got, "\r") {
			t.Errorf("%s: %q, want %q", f, got, want)
		}
	}
}

func TestTableStyle(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"t.md": "| a | b | c |\n| :-- | --: | :-: |\n| 1 | 2 | 3 |\n\n# h\n\n> q\n"})
	_, out, _ := fhlRun(t, dir, "", false, "-o", "-", "t.md")
	for _, a := range []string{"left", "right", "center"} {
		if !strings.Contains(out, `style="text-align:`+a+`"`) {
			t.Errorf("no text-align:%s: %s", a, out)
		}
	}
	if n := strings.Count(out, "style="); n != 6 {
		t.Errorf("%d style attributes, want 6 (only table cells): %s", n, out)
	}
}

func TestDataLang(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"d.md": "```hs\nx\n```\n\n```\ny\n```\n\n```a\"b\nz\n```\n"})
	_, out, _ := fhlRun(t, dir, "", false, "-o", "-", "d.md")
	// The name as written goes into data-lang and the language- class (escaped); a block without a name has neither.
	for _, s := range []string{
		`<pre class="highlight" data-lang="hs"><code class="language-hs">`,
		`<pre class="highlight"><code>y`,
		`data-lang="a&#34;b"><code class="language-a&#34;b">`,
	} {
		if !strings.Contains(out, s) {
			t.Errorf("no %s: %s", s, out)
		}
	}
	if n := strings.Count(out, "language-"); n != 2 {
		t.Errorf("%d language- classes, want 2: %s", n, out)
	}
}

func TestCheckAndStrict(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"docs/setup.md": "a\n\n```hs\nx\n```\n", "docs/intro.md": "```\nx\n```\n", "docs/ok.md": readme,
		"docs/plain.md": "```text\nx\n```\n\n```plain\ny\n```\n"}) // plain text is not reported
	code, stdout, stderr := fhlRun(t, dir, "", false, "--check", "docs/")
	if code != 0 || stdout != "" {
		t.Fatalf("code %d, %q", code, stdout)
	}
	for _, s := range []string{
		filepath.Join("docs", "setup.md") + `:3: code block language "hs" is not supported (see fhl --list-langs)`,
		filepath.Join("docs", "intro.md") + ":1: code block has no language name",
		"fhl: 4 files, 5 code blocks (2 uncolored) in ",
	} {
		if !strings.Contains(stderr, s) {
			t.Errorf("no %q in %q", s, stderr)
		}
	}
	if exists("docs/ok.html") || exists("docs/highlight.css") {
		t.Errorf("--check wrote a file")
	}
	if code, _, _ := fhlRun(t, "", "", false, "--check", "--strict", "docs/"); code != 2 {
		t.Errorf("--strict: code %d", code)
	}
	if code, _, stderr := fhlRun(t, "", "", false, "--check", "-q", "docs/"); code != 0 || stderr != "" {
		t.Errorf("-q: code %d, %q", code, stderr)
	}
	code, stdout, _ = fhlRun(t, "", "", false, "--check", "--json", "docs/")
	var r report
	if err := json.Unmarshal([]byte(stdout), &r); err != nil {
		t.Fatal(err)
	}
	want := []uncolored{
		{File: filepath.Join("docs", "intro.md"), Line: 1, Reason: "no-language"},
		{File: filepath.Join("docs", "setup.md"), Line: 3, Lang: "hs", Reason: "unsupported"},
	}
	if code != 0 || fmt.Sprint(r.Uncolored) != fmt.Sprint(want) || r.Summary.Uncolored != 2 || r.Files[0].Output != "" {
		t.Errorf("--check --json: code %d, %+v", code, r)
	}
}

func TestJSONShape(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"a.md": readme})
	_, stdout, _ := fhlRun(t, dir, "", false, "--json", "a.md", "missing.md")
	var m map[string]any
	if err := json.Unmarshal([]byte(stdout), &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"files", "styles", "skipped", "uncolored", "errors", "summary"} {
		if _, ok := m[k]; !ok {
			t.Errorf("no %q", k)
		}
	}
	s := m["summary"].(map[string]any)
	for _, k := range []string{"files", "blocks", "uncolored", "skipped", "errors", "seconds"} {
		if _, ok := s[k]; !ok {
			t.Errorf("no summary.%q", k)
		}
	}
	errs := m["errors"].([]any)
	if len(errs) != 1 || !strings.Contains(errs[0].(map[string]any)["message"].(string), "cannot read missing.md") {
		t.Errorf("errors %v", errs)
	}
}

func TestExitCodes(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"a.md": "```hs\nx\n```\n"})
	t.Chdir(dir)
	for _, tt := range []struct {
		args []string
		want int
	}{
		{[]string{"--check", "a.md"}, 0},
		{[]string{"--check", "--strict", "a.md"}, 2},
		{[]string{"--check", "--strict", "a.md", "missing.md"}, 1},
		{[]string{"--nope"}, 1},
		{[]string{}, 1},
		{[]string{"--theme=nope", "a.md"}, 1},
		{[]string{"--theme", "a.md"}, 1},
		{[]string{"--theme=github", "a.md"}, 1},
		{[]string{"--theme=light:nope", "a.md"}, 1},
		{[]string{"-j", "0", "a.md"}, 1},
	} {
		if code, _, _ := fhlRun(t, "", "", false, tt.args...); code != tt.want {
			t.Errorf("%v: code %d, want %d", tt.args, code, tt.want)
		}
	}
}

func TestDeterministic(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{}
	for i := range 20 {
		files[fmt.Sprintf("docs/%02d.md", i)] = fmt.Sprintf("# %d\n\n```go\nx := %d\n```\n\n```hs\ny\n```\n", i, i)
	}
	writeFiles(t, dir, files)
	t.Chdir(dir)
	outs := map[string]string{}
	for _, j := range []string{"1", "4"} {
		for range 2 {
			out := "out" + j + "/"
			if code, _, stderr := fhlRun(t, "", "", false, "--force", "-j", j, "-o", out, "docs/"); code != 0 {
				t.Fatalf("code %d: %s", code, stderr)
			}
			for f := range files {
				name := strings.TrimSuffix(filepath.Base(f), ".md") + ".html"
				got := readFile(t, filepath.Join(out, name))
				if prev, ok := outs[name]; ok && prev != got {
					t.Errorf("%s differs between runs", name)
				}
				outs[name] = got
			}
		}
	}
	// files in --json are also in input order.
	_, stdout, _ := fhlRun(t, "", "", false, "--check", "--json", "-j", "4", "docs/")
	var r report
	_ = json.Unmarshal([]byte(stdout), &r)
	for i, f := range r.Files {
		if f.Input != filepath.Join("docs", fmt.Sprintf("%02d.md", i)) {
			t.Errorf("files[%d] is %s", i, f.Input)
		}
	}
}

func TestPendingLimit(t *testing.T) {
	dir := t.TempDir()
	for i := range 10000 {
		p := filepath.Join(dir, "docs", fmt.Sprintf("%05d.md", i))
		if i%1000 == 0 {
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(p, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	maxPending.Store(0)
	code, _, stderr := fhlRun(t, dir, "", false, "-j", "4", "-o", "out/", "docs/")
	if code != 0 {
		t.Fatalf("code %d: %s", code, stderr)
	}
	if got := maxPending.Load(); got > 16 || got < 1 {
		t.Errorf("max pending %d, want 1..16", got)
	}
}

func TestHelp(t *testing.T) {
	code, out, _ := fhlRun(t, "", "", false, "--help")
	if code != 0 {
		t.Fatalf("code %d", code)
	}
	// The help must show an example of each common use.
	for _, cmd := range []string{
		"fhl README.md", "fhl docs/", "fhl -o dist/ docs/", "fhl -o dist/ docs/ --json", "fhl --single-html README.md",
		"fhl -o - README.md", "fhl --single-html -o - README.md", "fhl --static-site -o dist/ docs/",
		"cat note.md | fhl -o note.html -", "fhl --lang go main.txt", "fhl --theme dark:github README.md",
		"fhl --print-css > highlight.css", "fhl --force docs/", "fhl --list-langs", "fhl --list-themes", "fhl --check docs/",
	} {
		if !strings.Contains(out, cmd) {
			t.Errorf("--help lacks %q", cmd)
		}
	}
	if code, out2, _ := fhlRun(t, "", "", false, "-h"); code != 0 || out2 != out {
		t.Errorf("-h: code %d", code)
	}
}

func TestListsAndVersion(t *testing.T) {
	code, out, _ := fhlRun(t, "", "", false, "--list-langs")
	if code != 0 || !strings.Contains(out, "TypeScript") || !strings.Contains(out, "*.go") {
		t.Errorf("--list-langs: %d %q", code, out)
	}
	_, out, _ = fhlRun(t, "", "", false, "--list-langs", "--json")
	var langs []map[string]any
	if err := json.Unmarshal([]byte(out), &langs); err != nil || len(langs) != len(fh.Builtins()) {
		t.Errorf("--list-langs --json: %d languages, %v", len(langs), err)
	}
	_, out, _ = fhlRun(t, "", "", false, "--list-themes", "--json")
	var list []map[string]string
	if err := json.Unmarshal([]byte(out), &list); err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, th := range list {
		names = append(names, th["name"])
	}
	// Same as the theme tables in README.md and website/themes.md.
	want := "[default github vscode solarized one catppuccin tokyo-night]"
	if fmt.Sprint(names) != want {
		t.Errorf("--list-themes: %v", names)
	}
	_, out, _ = fhlRun(t, "", "", false, "--version")
	if !strings.HasPrefix(out, "fhl ") {
		t.Errorf("--version: %q", out)
	}
}

func TestThemeFlag(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"a.md": readme})
	t.Chdir(dir)
	for _, th := range themes {
		for m, mn := range modeNames {
			v := "--theme=" + mn + ":" + th.name
			css := th.css(mode(m), cssOptions{})
			code, out, stderr := fhlRun(t, "", "", false, v, "--single-html", "-o", "-", "a.md")
			if code != 0 || !strings.Contains(out, css) {
				t.Errorf("%s: code %d, %s", v, code, stderr)
			}
			if code, _, stderr := fhlRun(t, "", "", false, "--force", v, "a.md"); code != 0 || readFile(t, "highlight.css") != css {
				t.Errorf("%s: highlight.css differs (code %d, %s)", v, code, stderr)
			}
		}
	}
	// The theme can be left out (default), and default can be written.
	def := lookupTheme("default")
	for v, want := range map[string]string{
		"--theme=light": def.css(modeLight, cssOptions{}), "--theme=light:": def.css(modeLight, cssOptions{}), "--theme=light:default": def.css(modeLight, cssOptions{}),
		"--theme=dark": def.css(modeDark, cssOptions{}), "--theme=both": def.css(modeBoth, cssOptions{}),
	} {
		if code, out, stderr := fhlRun(t, "", "", false, v, "--single-html", "-o", "-", "a.md"); code != 0 || !strings.Contains(out, want) {
			t.Errorf("%s: code %d, %s", v, code, stderr)
		}
	}
	// The value can also be the next argument.
	if code, out, stderr := fhlRun(t, "", "", false, "--theme", "dark:github", "--single-html", "-o", "-", "a.md"); code != 0 ||
		!strings.Contains(out, "--theme dark:github") {
		t.Errorf("--theme dark:github: code %d, %s", code, stderr)
	}
	// Errors tell how to write the value.
	for _, tt := range []struct {
		args []string
		want string
	}{
		{[]string{"--theme", "a.md"}, `--theme must be MODE or MODE:THEME, not "a.md" (MODE is light, dark or both; THEME is one of fhl --list-themes, default if omitted), e.g. --theme light:github`},
		{[]string{"a.md", "--theme"}, "--theme needs a value: MODE or MODE:THEME"},
		{[]string{"--theme=nope", "a.md"}, `--theme must be MODE or MODE:THEME`},
		{[]string{"--theme", "github", "a.md"}, "--theme github needs the mode first, e.g. --theme light:github"},
	} {
		if code, _, stderr := fhlRun(t, "", "", false, tt.args...); code != 1 || !strings.Contains(stderr, tt.want) {
			t.Errorf("%v: code %d, %q", tt.args, code, stderr)
		}
	}
	if code, _, stderr := fhlRun(t, "", "", false, "--theme=light:nope", "a.md"); code != 1 || !strings.Contains(stderr, "see fhl --list-themes") {
		t.Errorf("unknown theme: code %d, %q", code, stderr)
	}
}

// TestThemeModes checks which colors each mode writes: both has the light colors and the dark ones under
// prefers-color-scheme; light and dark have only their own colors, with no switching.
func TestThemeModes(t *testing.T) {
	for _, th := range themes {
		both, light, dark := th.css(modeBoth, cssOptions{}), th.css(modeLight, cssOptions{}), th.css(modeDark, cssOptions{})
		bg := func(p palette) string { return "--highlight-background: " + p["background"] + ";" }
		if !strings.Contains(both, bg(th.light)) || !strings.Contains(both, "@media (prefers-color-scheme: dark)") || !strings.Contains(both, bg(th.dark)) {
			t.Errorf("%s: both lacks light or dark colors", th.name)
		}
		for name, css := range map[string]string{"light": light, "dark": dark} {
			if strings.Contains(css, "prefers-color-scheme") || strings.Contains(css, "color-scheme") {
				t.Errorf("%s %s: has switching", th.name, name)
			}
		}
		if !strings.Contains(light, bg(th.light)) || strings.Contains(light, bg(th.dark)) {
			t.Errorf("%s: light has wrong colors", th.name)
		}
		if !strings.Contains(dark, bg(th.dark)) || strings.Contains(dark, bg(th.light)) {
			t.Errorf("%s: dark has wrong colors", th.name)
		}
	}
}

func TestCSS(t *testing.T) {
	for _, th := range themes {
		for _, p := range []palette{th.light, th.dark} {
			for _, r := range roles {
				if !regexp.MustCompile(`^#[0-9a-f]{6}$`).MatchString(p[r.name]) {
					t.Errorf("%s: bad color for %s: %q", th.name, r.name, p[r.name])
				}
			}
		}
		for m := range modeNames {
			css := th.css(mode(m), cssOptions{})
			for _, c := range classRules {
				sel := "pre.highlight ." + c.class + " {"
				if !strings.Contains(css, "/* "+c.class+": ") || !strings.Contains(css, sel) {
					t.Errorf("%s: no rule or comment for .%s", th.name, c.class)
				}
			}
			for _, line := range strings.Split(css, "\n") {
				if strings.HasSuffix(line, "}") && strings.Contains(line, "{") && !strings.HasPrefix(line, "pre.highlight") {
					t.Errorf("%s: rule not under pre.highlight: %s", th.name, line)
				}
				if strings.Contains(line, "var(--highlight-") {
					name := line[strings.Index(line, "var(--highlight-")+len("var(--highlight-"):]
					name = name[:strings.Index(name, ")")]
					if th.light[name] == "" || th.dark[name] == "" {
						t.Errorf("%s: no color for --highlight-%s", th.name, name)
					}
				}
			}
			if css != th.css(mode(m), cssOptions{}) {
				t.Errorf("%s: CSS is not deterministic", th.name)
			}
		}
	}
}

func TestStaticSite(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"site/index.md":       "# Home\n\nSee [the guide](guide/intro.md#setup) and [mail](mailto:a@b.c).\n\n![logo](img/logo.png)\n",
		"site/guide/intro.md": "---\ntitle: x\n---\n# Intro\n\n## Setup\n\n## 日本語\n\n[home](../index.md) [ext](https://example.com/a.md) [top](#intro)\n",
		"site/img/logo.png":   "PNG",
		"site/.secret":        "x",
		"site/.git/config":    "x",
	})
	t.Chdir(dir)
	code, out, stderr := fhlRun(t, "", "", false, "--static-site", "--json", "-o", "dist/", "site")
	if code != 0 {
		t.Fatalf("code %d, %s", code, stderr)
	}
	index := readFile(t, "dist/index.html")
	wantHead := "<!DOCTYPE html>\n<html lang=\"en\">\n<meta charset=\"utf-8\">\n<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n<title>Home</title>\n<link rel=\"stylesheet\" href=\"highlight.css\">\n<h1 id=\"home\">Home</h1>\n"
	if !strings.HasPrefix(index, wantHead) {
		t.Errorf("index.html head:\n%s", index)
	}
	if !strings.Contains(index, `<a href="guide/intro.html#setup">`) || !strings.Contains(index, `<a href="mailto:a@b.c">`) || !strings.Contains(index, `<img src="img/logo.png"`) {
		t.Errorf("index.html links:\n%s", index)
	}
	intro := readFile(t, "dist/guide/intro.html")
	for _, s := range []string{
		`<link rel="stylesheet" href="../highlight.css">`, `<h2 id="setup">Setup</h2>`, `<h2 id="日本語">日本語</h2>`,
		`<a href="../index.html">home</a>`, `<a href="https://example.com/a.md">ext</a>`, `<a href="#intro">top</a>`,
	} {
		if !strings.Contains(intro, s) {
			t.Errorf("intro.html lacks %s:\n%s", s, intro)
		}
	}
	if strings.Contains(intro, "title: x") {
		t.Errorf("front matter was written")
	}
	if readFile(t, "dist/highlight.css") != lookupTheme("default").css(modeBoth, cssOptions{}) {
		t.Errorf("highlight.css differs")
	}
	if _, err := os.Stat("dist/guide/highlight.css"); err == nil {
		t.Errorf("highlight.css written in a subfolder")
	}
	if readFile(t, "dist/img/logo.png") != "PNG" {
		t.Errorf("logo.png not copied")
	}
	for _, p := range []string{"dist/.secret", "dist/.git/config"} {
		if _, err := os.Stat(p); err == nil {
			t.Errorf("%s copied", p)
		}
	}
	var rep map[string]any
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(rep["copied"]) != "["+filepath.Join("dist", "img", "logo.png")+"]" || fmt.Sprint(rep["styles"]) != "["+filepath.Join("dist", "highlight.css")+"]" {
		t.Errorf("--json: copied %v, styles %v", rep["copied"], rep["styles"])
	}

	// --html-lang and --theme.
	if code, _, stderr := fhlRun(t, "", "", false, "--static-site", "--force", "--html-lang", "ja", "--theme=light", "-o", "dist/", "site"); code != 0 ||
		!strings.Contains(readFile(t, "dist/index.html"), `<html lang="ja">`) || readFile(t, "dist/highlight.css") != lookupTheme("default").css(modeLight, cssOptions{}) {
		t.Errorf("--html-lang ja --theme=light: code %d, %s", code, stderr)
	}

	// Without -o: pages next to the Markdown, one highlight.css at the top, nothing copied.
	if code, _, stderr := fhlRun(t, "", "", false, "--static-site", "site"); code != 0 {
		t.Fatalf("no -o: code %d, %s", code, stderr)
	}
	if !strings.Contains(readFile(t, "site/guide/intro.html"), `href="../highlight.css"`) || readFile(t, "site/highlight.css") == "" {
		t.Errorf("no -o: pages or highlight.css missing")
	}
	if _, err := os.Stat("site/guide/highlight.css"); err == nil {
		t.Errorf("no -o: highlight.css written in a subfolder")
	}

	// --check writes and copies nothing.
	if code, _, _ := fhlRun(t, "", "", false, "--static-site", "--check", "-o", "check/", "site"); code != 0 {
		t.Errorf("--check: code %d", code)
	}
	if _, err := os.Stat("check"); err == nil {
		t.Errorf("--check wrote check/")
	}
}

func TestStaticSiteCopyCollision(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"site/guide.md":      "# Guide\n",
		"site/guide.html":    "old",
		"site/highlight.css": "mine",
		"site/a.txt":         "A",
	})
	t.Chdir(dir)
	code, _, stderr := fhlRun(t, "", "", false, "--static-site", "-o", "dist/", "site")
	if code != 1 || !strings.Contains(stderr, "guide.html would be copied") || !strings.Contains(stderr, "highlight.css would be copied") {
		t.Errorf("code %d, %s", code, stderr)
	}
	if strings.Contains(readFile(t, "dist/guide.html"), "old") || readFile(t, "dist/highlight.css") == "mine" || readFile(t, "dist/a.txt") != "A" {
		t.Errorf("pages, style sheet or copies are wrong")
	}
}

func TestStaticSiteErrors(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"site/a.md": "# A\n", "other/b.md": "# B\n", "c.md": "# C\n"})
	t.Chdir(dir)
	for _, tt := range []struct {
		args []string
		want string
	}{
		{[]string{"--static-site", "c.md"}, "needs a folder"},
		{[]string{"--static-site", "site", "other"}, "exactly one folder"},
		{[]string{"--static-site", "-"}, "needs a folder"},
		{[]string{"--static-site", "--single-html", "site"}, "cannot be used together"},
		{[]string{"--static-site", "-o", "-", "site"}, "use -o DIR/"},
		{[]string{"--static-site", "-o", "out.html", "site"}, "use -o DIR/"},
		{[]string{"--static-site", "--lang", "go", "site"}, "--lang has no effect"},
		{[]string{"--static-site", "--html-lang", "", "site"}, "language tag"},
		{[]string{"--html-lang", "ja", "c.md"}, "only for --static-site"},
	} {
		if code, _, stderr := fhlRun(t, "", "", false, tt.args...); code != 1 || !strings.Contains(stderr, tt.want) {
			t.Errorf("%v: code %d, %q", tt.args, code, stderr)
		}
	}
	// Without --static-site, nothing changes: no ids and no rewritten links.
	writeFiles(t, dir, map[string]string{"d.md": "# D\n\n[a](a.md)\n"})
	if code, out, _ := fhlRun(t, "", "", false, "-o", "-", "d.md"); code != 0 || out != "<h1>D</h1>\n<p><a href=\"a.md\">a</a></p>\n" {
		t.Errorf("without --static-site: %q", out)
	}
}

func TestRewriteMDLink(t *testing.T) {
	pages := map[string]bool{"guide.md": true, "a/b.markdown": true, "c.MKD": true, "../index.md": true, "my page.md": true}
	isPage := func(rel string) bool { return pages[rel] }
	for in, want := range map[string]string{
		"guide.md": "guide.html", "a/b.markdown#x": "a/b.html#x", "c.MKD?v=1#y": "c.html?v=1#y", "../index.md": "../index.html",
		"my%20page.md":       "my%20page.html",
		"https://x.org/a.md": "https://x.org/a.md", "mailto:a.md": "mailto:a.md", "/abs/a.md": "/abs/a.md", "#a.md": "#a.md",
		"img.png": "img.png", "": "", "dir/": "dir/",
		"other.md": "other.md", "bad%zz.md": "bad%zz.md", // not a page of the site
	} {
		if got := rewriteMDLink(in, isPage); got != want {
			t.Errorf("rewriteMDLink(%q) = %q, want %q", in, got, want)
		}
	}
}

// --static-site rewrites only the links to its own pages: a link to a Markdown file outside the folder, in a
// dot folder (which the site skips) or that does not exist keeps pointing at the file.
func TestStaticSiteLinks(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"README.md":         "# R\n",
		"site/index.md":     "[g](guide.md) [s](sub/s.md#x) [r](../README.md) [h](.hidden/x.md) [n](none.md)\n",
		"site/guide.md":     "# G\n",
		"site/sub/s.md":     "[up](../index.md) [self](s.md)\n",
		"site/.hidden/x.md": "# X\n",
	})
	t.Chdir(dir)
	if code, _, stderr := fhlRun(t, "", "", false, "--static-site", "-o", "dist/", "site"); code != 0 {
		t.Fatalf("code %d, %s", code, stderr)
	}
	for _, want := range []string{`href="guide.html"`, `href="sub/s.html#x"`, `href="../README.md"`, `href=".hidden/x.md"`, `href="none.md"`} {
		if got := readFile(t, "dist/index.html"); !strings.Contains(got, want) {
			t.Errorf("index.html has no %s:\n%s", want, got)
		}
	}
	for _, want := range []string{`href="../index.html"`, `href="s.html"`} {
		if got := readFile(t, "dist/sub/s.html"); !strings.Contains(got, want) {
			t.Errorf("sub/s.html has no %s:\n%s", want, got)
		}
	}
}

func TestOverwriteUnreadable(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs a file that is writable but not readable (not on Windows, and not as root)")
	}
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"README.md": readme, "README.html": "old"})
	p := filepath.Join(dir, "README.html")
	if err := os.Chmod(p, 0o200); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(p, 0o644) })
	// fhl cannot see what is in it, so it is an error, on a terminal or not, and even with --force.
	for _, c := range []struct {
		stdin       string
		interactive bool
		args        []string
	}{
		{"", false, []string{"README.md"}},
		{"y\n", true, []string{"README.md"}},
		{"", false, []string{"--force", "README.md"}},
	} {
		code, _, stderr := fhlRun(t, dir, c.stdin, c.interactive, c.args...)
		if code != 1 || !strings.Contains(stderr, "fhl: README.html already exists but cannot be read") ||
			strings.Contains(stderr, "overwrite README.html?") {
			t.Errorf("%v (terminal %v): code %d, %q", c.args, c.interactive, code, stderr)
		}
	}
	os.Chmod(p, 0o644)
	if readFile(t, p) != "old" {
		t.Errorf("an unreadable file was overwritten")
	}
}

func TestOutputInsideInput(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"docs/index.md": readme, "docs/img/logo.png": "PNG"})
	for _, args := range [][]string{
		{"--static-site", "-o", "docs/dist/", "docs/"},
		{"-o", "docs/out/", "docs/"},
	} {
		for i := range 2 { // a second run must not convert or copy the first run's output
			code, _, stderr := fhlRun(t, dir, "", false, append([]string{"--force"}, args...)...)
			if code != 0 {
				t.Fatalf("%v, run %d: code %d, %q", args, i+1, code, stderr)
			}
			dir = ""
		}
		out := filepath.Clean(args[len(args)-2])
		var nested []string
		filepath.WalkDir(out, func(p string, d os.DirEntry, err error) error {
			if err == nil && d.IsDir() && p != out && strings.HasPrefix(d.Name(), filepath.Base(out)) {
				nested = append(nested, p)
			}
			return nil
		})
		if len(nested) > 0 {
			t.Errorf("%v: the output was copied into itself: %v", args, nested)
		}
	}
	if !exists("docs/dist/index.html") || !exists("docs/dist/img/logo.png") || !exists("docs/out/index.html") {
		t.Errorf("pages or copies missing")
	}
}

// failWriter fails every write, like a full disk or a closed pipe.
type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errors.New("no space left on device") }

func TestJSONWriteError(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"a.md": readme})
	t.Chdir(dir)
	var errOut bytes.Buffer
	code := run([]string{"--json", "-o", "out/", "a.md"}, env{stdin: strings.NewReader(""), stdout: failWriter{}, stderr: &errOut})
	if code != 1 || !strings.Contains(errOut.String(), "fhl: cannot write the --json result") {
		t.Errorf("code %d, %q", code, errOut.String())
	}
	// The lists, too, fail when standard output cannot be written.
	for _, args := range [][]string{{"--list-langs"}, {"--list-langs", "--json"}, {"--list-themes"}, {"--list-themes", "--json"}} {
		errOut.Reset()
		code := run(args, env{stdin: strings.NewReader(""), stdout: failWriter{}, stderr: &errOut})
		if code != 1 || !strings.Contains(errOut.String(), "fhl: cannot write to standard output") {
			t.Errorf("%v: code %d, %q", args, code, errOut.String())
		}
	}
}

// In a conversion, at most three blocks per reason are listed, then a count; --check lists them all.
func TestUncoloredSummary(t *testing.T) {
	dir := t.TempDir()
	var b strings.Builder
	for i := range 5 {
		fmt.Fprintf(&b, "```\n%d\n```\n\n", i) // no language name: lines 1, 5, 9, 13, 17
	}
	for _, l := range []string{"hs", "cobol", "fortran", "ada"} {
		fmt.Fprintf(&b, "```%s\nx\n```\n\n", l) // lines 21, 25, 29, 33
	}
	writeFiles(t, dir, map[string]string{"a.md": b.String()})
	code, _, stderr := fhlRun(t, dir, "", false, "a.md")
	if code != 0 {
		t.Fatalf("code %d, %q", code, stderr)
	}
	want := "a.md:1: code block has no language name\n" +
		"a.md:5: code block has no language name\n" +
		"a.md:9: code block has no language name\n" +
		"fhl: 2 more code blocks have no language name (fhl --check lists all)\n" +
		"a.md:21: code block language \"hs\" is not supported (see fhl --list-langs)\n" +
		"a.md:25: code block language \"cobol\" is not supported (see fhl --list-langs)\n" +
		"a.md:29: code block language \"fortran\" is not supported (see fhl --list-langs)\n" +
		"fhl: 1 more code block in a language that is not supported (fhl --check lists all)\n" +
		"fhl: 1 files, 9 code blocks (9 uncolored) in "
	if !strings.HasPrefix(stderr, want) {
		t.Errorf("conversion:\n%s\nwant:\n%s", stderr, want)
	}
	_, _, stderr = fhlRun(t, "", "", false, "--check", "a.md")
	if strings.Contains(stderr, "more code block") || strings.Count(stderr, "a.md:") != 9 {
		t.Errorf("--check does not list all:\n%s", stderr)
	}
}

// TestValueErrors checks the messages when a flag's value is missing or wrong. When the value is left out, the
// flag takes the next input as its value, and the message must be about that value, not "no input".
func TestValueErrors(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"in/a.md": "# a\n"})
	t.Chdir(dir)
	for _, tt := range []struct {
		args []string
		want string
	}{
		// The value took the input.
		{[]string{"--theme", "in/"}, `fhl: --theme must be MODE or MODE:THEME, not "in/" (MODE is light`},
		{[]string{"--html-lang", "in/"}, `fhl: --html-lang must be a language tag such as en or ja, not "in/"`},
		{[]string{"--static-site", "--html-lang", "in/"}, `fhl: --html-lang must be a language tag such as en or ja, not "in/"`},
		{[]string{"--lang", "in/"}, `fhl: unknown language "in/" for --lang; see fhl --list-langs`},
		{[]string{"--raw-html", "in/"}, `fhl: --raw-html must be escape, omit or unsafe, not "in/"`},
		{[]string{"-j", "in/"}, `fhl: -j must be a number of 1 or more, not "in/"`},
		{[]string{"-o", "in/"}, `fhl: no input; give files, folders or - (-o took "in/" as the output; if it is an input, give -o a value first)`},
		// No value at all.
		{[]string{"in/", "--theme"}, "fhl: --theme needs a value: MODE or MODE:THEME"},
		{[]string{"in/", "--html-lang"}, "fhl: --html-lang needs a value: a language tag such as en or ja"},
		{[]string{"in/", "--lang"}, "fhl: --lang needs a value: a language name such as go (see fhl --list-langs)"},
		{[]string{"in/", "--raw-html"}, "fhl: --raw-html needs a value: escape, omit or unsafe"},
		{[]string{"in/", "-j"}, "fhl: -j needs a value: a number of 1 or more"},
		{[]string{"in/", "-o"}, "fhl: -o needs a value: a file, a folder (DIR/) or - for standard output"},
		// Other flag errors name the flag as the help does.
		{[]string{"--nope", "in/"}, "fhl: unknown flag --nope; see fhl --help"},
		{[]string{}, "fhl: no input; give files, folders or - (see fhl --help)"},
		{[]string{"-o", "-"}, "fhl: no input; give files, folders or - (see fhl --help)"},
	} {
		code, _, stderr := fhlRun(t, "", "", false, tt.args...)
		if code != 1 || strings.TrimSpace(stderr) != tt.want && !strings.HasPrefix(stderr, tt.want) {
			t.Errorf("%v: code %d\n got %q\nwant %q", tt.args, code, stderr, tt.want)
		}
	}
	// Language tags that are accepted.
	for _, v := range []string{"en", "ja", "zh-Hant-TW", "en-US", "es-419"} {
		if !isLangTag(v) {
			t.Errorf("isLangTag(%q) = false", v)
		}
	}
	for _, v := range []string{"", "in/", "e n", "en-", "-en", "1en", `en"`, "toolongpart"} {
		if isLangTag(v) {
			t.Errorf("isLangTag(%q) = true", v)
		}
	}
}

// TestPrintCSS checks --print-css: the CSS of --theme on standard output, the same as highlight.css.
func TestPrintCSS(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"a.md": "# a\n"})
	t.Chdir(dir)
	for _, theme := range []string{"", "dark:github", "light"} {
		args := []string{"--print-css"}
		conv := []string{"-q", "--force", "a.md"}
		if theme != "" {
			args = append(args, "--theme", theme)
			conv = append(conv, "--theme", theme)
		}
		code, out, stderr := fhlRun(t, "", "", false, args...)
		if code != 0 || stderr != "" {
			t.Fatalf("%v: code %d, %s", args, code, stderr)
		}
		if code, _, stderr := fhlRun(t, "", "", false, conv...); code != 0 {
			t.Fatalf("%v: code %d, %s", conv, code, stderr)
		}
		want, err := os.ReadFile("highlight.css")
		if err != nil {
			t.Fatal(err)
		}
		if out != string(want) {
			t.Errorf("%v: the CSS differs from highlight.css", args)
		}
	}
	for _, tt := range []struct {
		args []string
		want string
	}{
		{[]string{"--print-css", "a.md"}, "fhl: --print-css writes only the CSS to standard output; give no inputs"},
		{[]string{"--print-css", "-o", "x.css"}, "fhl: --print-css writes to standard output; redirect it instead of -o"},
		{[]string{"--print-css", "--json"}, "fhl: --json cannot be used with --print-css"},
		{[]string{"--print-css", "--theme", "nope"}, "fhl: --theme must be MODE or MODE:THEME"},
		{[]string{"-o", "-", "--theme", "dark", "a.md"}, "for only the CSS, use --print-css"},
	} {
		if code, _, stderr := fhlRun(t, "", "", false, tt.args...); code != 1 || !strings.Contains(stderr, tt.want) {
			t.Errorf("%v: code %d, %q", tt.args, code, stderr)
		}
	}
}

// TestThemeSelector checks --theme-selector and --no-frame: the shape of the CSS, where they apply, and the
// errors.
func TestThemeSelector(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"a.md": "# a\n"})
	t.Chdir(dir)
	def := lookupTheme("default")
	plain := def.css(modeBoth, cssOptions{})
	// block returns the variables of a CSS of one mode as a block under sel.
	block := func(m mode, sel string) string {
		css := def.css(m, cssOptions{})
		i := strings.Index(css, ":root {\n")
		return sel + " {\n" + css[i+len(":root {\n"):i+strings.Index(css[i:], "}\n")] + "}\n"
	}
	media := "@media (prefers-color-scheme: dark) {"
	for _, tt := range []struct {
		args   []string
		blocks string // the blocks after the @media switch, in order
	}{
		// The site's switch on top of the device setting.
		{[]string{"dark::root[data-theme=\"dark\"]", "light::root[data-theme=\"light\"]"},
			block(modeDark, `:root[data-theme="dark"]`) + block(modeLight, `:root[data-theme="light"]`)},
		// Only the site's switch: light whenever the attribute is not dark.
		{[]string{"dark::root[data-theme=\"dark\"]", "light::root:not([data-theme=\"dark\"])"},
			block(modeDark, `:root[data-theme="dark"]`) + block(modeLight, `:root:not([data-theme="dark"])`)},
		{[]string{"dark:.dark"}, block(modeDark, ".dark")},
	} {
		args := []string{"--print-css"}
		for _, v := range tt.args {
			args = append(args, "--theme-selector", v)
		}
		code, out, stderr := fhlRun(t, "", "", false, args...)
		if code != 0 {
			t.Fatalf("%v: code %d, %s", tt.args, code, stderr)
		}
		// The CSS without selectors, with the blocks right after the @media switch (which stays).
		end := strings.Index(plain, "  }\n}\n") + len("  }\n}\n")
		head := plain[:strings.Index(plain, "(prefers-color-scheme).\n")+len("(prefers-color-scheme).\n")]
		if !strings.Contains(out, media) || !strings.HasSuffix(out, tt.blocks+plain[end:]) ||
			!strings.HasPrefix(out, head) || !strings.Contains(out, " * Under ") {
			t.Errorf("%v:\n%s", tt.args, out)
		}
	}
	_, out, _ := fhlRun(t, "", "", false, "--print-css", "--no-frame")
	if strings.Contains(out, "pre.highlight {") || !strings.Contains(out, media) || !strings.Contains(out, "pre.highlight .kd {") {
		t.Errorf("--no-frame CSS:\n%s", out)
	}
	// The default CSS does not change.
	if _, out, _ = fhlRun(t, "", "", false, "--print-css"); out != plain || !strings.Contains(out, media) {
		t.Errorf("default CSS changed")
	}
	// highlight.css and --single-html get the same CSS.
	if code, _, stderr := fhlRun(t, "", "", false, "-q", "--force", "--theme-selector", "dark:.dark", "--no-frame", "a.md"); code != 0 {
		t.Fatalf("code %d, %s", code, stderr)
	}
	want := def.css(modeBoth, cssOptions{[]themeSelector{{modeDark, ".dark"}}, true})
	if b, _ := os.ReadFile("highlight.css"); string(b) != want {
		t.Errorf("highlight.css:\n%s", b)
	}
	if _, out, _ := fhlRun(t, "", "", false, "--single-html", "-o", "-", "--theme-selector", "dark:.dark", "a.md"); !strings.Contains(out, "\n.dark {\n") {
		t.Errorf("--single-html:\n%s", out)
	}
	for _, tt := range []struct {
		args []string
		want string
	}{
		{[]string{"--print-css", "--theme-selector", ".dark"}, `fhl: --theme-selector must start with dark: or light:, then a CSS selector`},
		{[]string{"--print-css", "--theme-selector", "dark:.d", "--theme-selector", "both:.x"}, `fhl: --theme-selector must start with dark: or light:, then a CSS selector, such as dark:':root[data-theme="dark"]', not "both:.x"`},
		{[]string{"--print-css", "--theme-selector", "dark:"}, `fhl: --theme-selector needs a CSS selector after the mode`},
		{[]string{"--print-css", "--theme-selector", "dark:a{b"}, `fhl: --theme-selector needs a CSS selector after the mode, such as dark:':root[data-theme="dark"]', not "dark:a{b"`},
		{[]string{"--print-css", "--theme-selector", "dark:x</style>"}, "fhl: --theme-selector needs a CSS selector"},
		{[]string{"--theme", "light", "--theme-selector", "dark:.dark", "a.md"}, "fhl: --theme-selector is only for --theme both (the default)"},
		{[]string{"a.md", "--theme-selector"}, "fhl: --theme-selector needs a value: dark:SELECTOR or light:SELECTOR"},
		{[]string{"-o", "-", "--no-frame", "a.md"}, "fhl: --no-frame has no effect with -o -"},
		{[]string{"-o", "-", "--theme-selector", "dark:.dark", "a.md"}, "fhl: --theme-selector has no effect with -o -"},
	} {
		if code, _, stderr := fhlRun(t, "", "", false, tt.args...); code != 1 || !strings.Contains(stderr, tt.want) {
			t.Errorf("%v: code %d, %q", tt.args, code, stderr)
		}
	}
}

// TestCodeBlocks checks --code-blocks: one line of JSON per fenced code block, with the same HTML as a
// conversion writes, and its inputs, summary, exit status and errors.
func TestCodeBlocks(t *testing.T) {
	dir := t.TempDir()
	doc := "---\ntitle: x\n---\n# Doc\n\n```go\nx := \"<a>\"\n```\n\n- item\n\n  ```rust\n  fn main() {}\n  ```\n\n" +
		"```\nno language\n```\n\n```text\nlog\n```\n\n````cobol\nDISPLAY \"```\".\n````\n\n```go\n```\n\n    indented, not fenced\n"
	writeFiles(t, dir, map[string]string{
		"doc.md": doc, "docs/b.md": "```py\nx = 1\n```\n", "docs/a.md": "```js\nlet x\n```\n",
		"docs/.hidden/c.md": "```go\nx\n```\n", "docs/note.txt": "```go\nx\n```\n", "main.go": "package main\n",
	})
	t.Chdir(dir)
	read := func(out string) []codeBlock {
		var bs []codeBlock
		for _, line := range strings.Split(strings.TrimSuffix(out, "\n"), "\n") {
			var b codeBlock
			if err := json.Unmarshal([]byte(line), &b); err != nil {
				t.Fatalf("not JSON: %q", line)
			}
			bs = append(bs, b)
		}
		return bs
	}
	code, out, stderr := fhlRun(t, "", "", false, "--code-blocks", "doc.md")
	if code != 0 || stderr != "fhl: 1 files, 6 code blocks (2 uncolored)\n" {
		t.Fatalf("code %d, %q", code, stderr)
	}
	bs := read(out)
	type want struct {
		line    int
		lang    string
		colored bool
		reason  string
	}
	wants := []want{{6, "go", true, ""}, {12, "rust", true, ""}, {16, "", false, "no-language"}, {20, "text", false, "plain"},
		{24, "cobol", false, "unsupported"}, {28, "go", true, ""}}
	if len(bs) != len(wants) {
		t.Fatalf("%d blocks, want %d:\n%s", len(bs), len(wants), out)
	}
	for i, w := range wants {
		b := bs[i]
		if b.File != "doc.md" || b.Line != w.line || b.Lang != w.lang || b.Colored != w.colored || b.Reason != w.reason ||
			b.Close != "</code></pre>" || !strings.HasPrefix(b.Open, `<pre class="highlight"`) || strings.Contains(b.Code, "<code") {
			t.Errorf("block %d: %+v", i, b)
		}
	}
	// open + code + close is the HTML that a conversion writes for each block, in the same order.
	_, conv, _ := fhlRun(t, "", "", false, "-q", "-o", "-", "doc.md")
	var pres []string
	for rest := conv; ; {
		i := strings.Index(rest, `<pre class="highlight"`)
		if i < 0 {
			break
		}
		j := strings.Index(rest[i:], "</code></pre>") + len("</code></pre>")
		pres = append(pres, rest[i:i+j])
		rest = rest[i+j:]
	}
	if len(pres) != len(bs) {
		t.Fatalf("the conversion has %d highlighted blocks, --code-blocks %d", len(pres), len(bs))
	}
	for i, b := range bs {
		if b.Open+b.Code+b.Close != pres[i] {
			t.Errorf("block %d differs from the conversion:\n%s\n%s", i, b.Open+b.Code+b.Close, pres[i])
		}
	}
	// Standard input, folders (in name order, names starting with . skipped), -q and --strict.
	code, out, stderr = fhlRun(t, "", doc, false, "-q", "--code-blocks", "-", "docs/")
	var files []string
	for _, b := range read(out) {
		files = append(files, b.File+":"+b.Lang)
	}
	if code != 0 || stderr != "" || fmt.Sprint(files) != fmt.Sprint([]string{"-:go", "-:rust", "-:", "-:text", "-:cobol", "-:go",
		filepath.Join("docs", "a.md") + ":js", filepath.Join("docs", "b.md") + ":py"}) {
		t.Errorf("code %d, %q, %v", code, stderr, files)
	}
	if code, _, _ := fhlRun(t, "", "", false, "-q", "--strict", "--code-blocks", "doc.md"); code != 2 {
		t.Errorf("--strict: code %d", code)
	}
	if code, _, _ := fhlRun(t, "", "", false, "-q", "--strict", "--code-blocks", "docs/"); code != 0 {
		t.Errorf("--strict, all colored: code %d", code)
	}
	// A file that cannot be read is reported, and the others are still written.
	code, out, stderr = fhlRun(t, "", "", false, "-q", "--code-blocks", "missing.md", "docs/a.md")
	if code != 1 || len(read(out)) != 1 || !strings.Contains(stderr, "fhl: cannot read missing.md") {
		t.Errorf("missing: code %d, %q, %q", code, out, stderr)
	}
	for _, tt := range []struct {
		args []string
		want string
	}{
		{[]string{"--code-blocks"}, "fhl: no input; give Markdown files, folders or -"},
		{[]string{"--code-blocks", "main.go"}, "fhl: --code-blocks reads Markdown (.md, .markdown, .mkd), not main.go"},
		{[]string{"--code-blocks", "--json", "doc.md"}, "fhl: --code-blocks already writes JSON (one line per code block); leave out --json"},
		{[]string{"--code-blocks", "-o", "x.jsonl", "doc.md"}, "fhl: --code-blocks writes only the code blocks, as JSON Lines on standard output; -o cannot be used with it"},
		{[]string{"--code-blocks", "--print-css"}, "--print-css cannot be used with it"},
		{[]string{"--code-blocks", "--single-html", "doc.md"}, "--single-html cannot be used with it"},
		{[]string{"--code-blocks", "--theme", "dark", "doc.md"}, "--theme cannot be used with it"},
	} {
		if code, _, stderr := fhlRun(t, "", "", false, tt.args...); code != 1 || !strings.Contains(stderr, tt.want) {
			t.Errorf("%v: code %d, %q", tt.args, code, stderr)
		}
	}
}

// TestDefaultContrast checks that every color of the default theme has a contrast ratio of at least 6 against its
// background (WCAG), in light and dark, and that only the default theme makes keywords bold.
func TestDefaultContrast(t *testing.T) {
	lum := func(hex string) float64 {
		var rgb [3]float64
		for i := range rgb {
			var v int
			if _, err := fmt.Sscanf(hex[1+2*i:3+2*i], "%02x", &v); err != nil {
				t.Fatalf("bad color %q", hex)
			}
			c := float64(v) / 255
			if c <= 0.03928 {
				rgb[i] = c / 12.92
			} else {
				rgb[i] = math.Pow((c+0.055)/1.055, 2.4)
			}
		}
		return 0.2126*rgb[0] + 0.7152*rgb[1] + 0.0722*rgb[2]
	}
	def := lookupTheme("default")
	for name, p := range map[string]palette{"light": def.light, "dark": def.dark} {
		bg := lum(p["background"])
		for _, r := range roles {
			if r.name == "background" {
				continue
			}
			l := lum(p[r.name])
			ratio := (max(l, bg) + 0.05) / (min(l, bg) + 0.05)
			if ratio < 6 {
				t.Errorf("%s %s %s: contrast %.2f, want 6 or more", name, r.name, p[r.name], ratio)
			}
		}
	}
	for _, th := range themes {
		css := th.css(modeBoth, cssOptions{})
		if bold := strings.Contains(css, "pre.highlight .kd { color: var(--highlight-keyword); font-weight: bold; }"); bold != (th.name == "default") {
			t.Errorf("%s: bold keywords %v", th.name, bold)
		}
	}
}
