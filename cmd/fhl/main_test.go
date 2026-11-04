package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
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
<pre class="highlight" data-lang="go"><code><span class="line"><span class="cl"><span class="nx">fmt</span><span class="p">.</span><span class="nf">Println</span><span class="p">(</span><span class="s">&#34;hi&#34;</span><span class="p">)</span><span class="w">
</span></span></span></code></pre>`

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
	if got := readFile(t, "highlight.css"); got != lookupTheme("default").css(modeBoth) {
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
	want := "<!DOCTYPE html>\n<meta charset=\"utf-8\">\n<title>Usage</title>\n<style>\n" + lookupTheme("default").css(modeBoth) + "</style>\n" + readmeHTML
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
	got := readFile(t, "plain.html")
	want := "<!DOCTYPE html>\n<meta charset=\"utf-8\">\n<title>plain.md</title>\n<style>\n" + lookupTheme("default").css(modeBoth) + "</style>\n<p>no heading &lt;b&gt;x&lt;/b&gt;</p>\n"
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
	if code != 0 || !strings.HasPrefix(readFile(t, "main.html"), `<pre class="highlight" data-lang="go"><code>`) {
		t.Fatalf("code %d: %s", code, stderr)
	}
	code, stdout, _ := fhlRun(t, "", "", false, "--lang", "Go", "-o", "-", "main.txt")
	if code != 0 || !strings.HasPrefix(stdout, `<pre class="highlight" data-lang="Go"><code><span class="line"><span class="cl"><span class="nx">x`) {
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
	// The value of --theme must be joined with =, so that a bare --theme does not take the next input.
	for _, args := range [][]string{{"--theme"}, {"--theme", "docs/"}, {"x", "-theme", "dark:github"}} {
		if _, err := parseArgs(args); !errors.Is(err, errThemeValue) {
			t.Errorf("%v: err %v", args, err)
		}
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
	for _, s := range []string{`<pre class="highlight" data-lang="hs">`, `<pre class="highlight"><code>`, `data-lang="a&#34;b"`} {
		if !strings.Contains(out, s) {
			t.Errorf("no %s: %s", s, out)
		}
	}
	if strings.Contains(out, "language-") {
		t.Errorf("has a language- class: %s", out)
	}
}

func TestCheckAndStrict(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"docs/setup.md": "a\n\n```hs\nx\n```\n", "docs/intro.md": "```\nx\n```\n", "docs/ok.md": readme})
	code, stdout, stderr := fhlRun(t, dir, "", false, "--check", "docs/")
	if code != 0 || stdout != "" {
		t.Fatalf("code %d, %q", code, stdout)
	}
	for _, s := range []string{
		filepath.Join("docs", "setup.md") + `:3: code block language "hs" is not supported (see fhl --list-langs)`,
		filepath.Join("docs", "intro.md") + ":1: code block has no language name",
		"fhl: 3 files, 3 code blocks (2 uncolored) in ",
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
		"cat note.md | fhl -o note.html -", "fhl --lang go main.txt", "fhl --theme=dark:github README.md",
		"fhl --force docs/", "fhl --list-langs", "fhl --list-themes", "fhl --check docs/",
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
	if err := json.Unmarshal([]byte(out), &langs); err != nil || len(langs) != 24 {
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
			css := th.css(mode(m))
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
		"--theme=light": def.css(modeLight), "--theme=light:": def.css(modeLight), "--theme=light:default": def.css(modeLight),
		"--theme=dark": def.css(modeDark), "--theme=both": def.css(modeBoth),
	} {
		if code, out, stderr := fhlRun(t, "", "", false, v, "--single-html", "-o", "-", "a.md"); code != 0 || !strings.Contains(out, want) {
			t.Errorf("%s: code %d, %s", v, code, stderr)
		}
	}
	// Errors tell how to write the value.
	for _, args := range [][]string{{"--theme", "a.md"}, {"--theme", "dark:github", "a.md"}, {"--theme=github", "a.md"}, {"--theme=nope", "a.md"}} {
		if code, _, stderr := fhlRun(t, "", "", false, args...); code != 1 || !strings.Contains(stderr, "--theme=MODE:THEME") {
			t.Errorf("%v: code %d, %q", args, code, stderr)
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
		both, light, dark := th.css(modeBoth), th.css(modeLight), th.css(modeDark)
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
			css := th.css(mode(m))
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
			if css != th.css(mode(m)) {
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
		`<link rel="stylesheet" href="../highlight.css">`, `<h2 id="setup">Setup</h2>`, `<h2 id="heading">日本語</h2>`,
		`<a href="../index.html">home</a>`, `<a href="https://example.com/a.md">ext</a>`, `<a href="#intro">top</a>`,
	} {
		if !strings.Contains(intro, s) {
			t.Errorf("intro.html lacks %s:\n%s", s, intro)
		}
	}
	if strings.Contains(intro, "title: x") {
		t.Errorf("front matter was written")
	}
	if readFile(t, "dist/highlight.css") != lookupTheme("default").css(modeBoth) {
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
		!strings.Contains(readFile(t, "dist/index.html"), `<html lang="ja">`) || readFile(t, "dist/highlight.css") != lookupTheme("default").css(modeLight) {
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
	for in, want := range map[string]string{
		"guide.md": "guide.html", "a/b.markdown#x": "a/b.html#x", "c.MKD?v=1#y": "c.html?v=1#y", "../index.md": "../index.html",
		"https://x.org/a.md": "https://x.org/a.md", "mailto:a.md": "mailto:a.md", "/abs/a.md": "/abs/a.md", "#a.md": "#a.md",
		"img.png": "img.png", "": "", "dir/": "dir/",
	} {
		if got := rewriteMDLink(in); got != want {
			t.Errorf("rewriteMDLink(%q) = %q, want %q", in, got, want)
		}
	}
}
