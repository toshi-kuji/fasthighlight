// fhl is a command that turns Markdown files into HTML with highlighted code blocks. fhl --help lists every flag
// with examples; website/cli.md describes the behavior in detail.
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"text/tabwriter"
	"time"

	fh "github.com/toshi-kuji/fasthighlight"
	_ "github.com/toshi-kuji/fasthighlight/lang/all"
	"github.com/toshi-kuji/fasthighlight/markdown"
	"github.com/yuin/goldmark"
)

// version is the version. In release binaries it is set at build time with -ldflags "-X main.version=vX.Y.Z".
var version string

func main() {
	e := env{
		stdin:       os.Stdin,
		stdout:      os.Stdout,
		stderr:      os.Stderr,
		interactive: isTerminal(os.Stdin.Fd()) && isTerminal(os.Stderr.Fd()),
	}
	os.Exit(run(os.Args[1:], e))
}

// env is the input and output used by run. Tests replace it.
type env struct {
	stdin          io.Reader
	stdout, stderr io.Writer
	interactive    bool // whether standard input and standard error are terminals
}

// fileResult is one entry of files in --json.
type fileResult struct {
	Input     string `json:"input"`
	Output    string `json:"output,omitempty"`
	Blocks    int    `json:"blocks"`
	Uncolored int    `json:"uncolored"`
}

// errorResult is one entry of errors in --json.
type errorResult struct {
	Message string   `json:"message"`
	Inputs  []string `json:"inputs,omitempty"`
}

// report is the result written by --json.
type report struct {
	Files     []fileResult  `json:"files"`
	Styles    []string      `json:"styles"`
	Skipped   []string      `json:"skipped"`
	Uncolored []uncolored   `json:"uncolored"`
	Copied    *[]string     `json:"copied,omitempty"` // files copied by --static-site; only with --static-site
	Errors    []errorResult `json:"errors"`
	Summary   summary       `json:"summary"`
}

type summary struct {
	Files     int     `json:"files"`
	Blocks    int     `json:"blocks"`
	Uncolored int     `json:"uncolored"`
	Skipped   int     `json:"skipped"`
	Errors    int     `json:"errors"`
	Seconds   float64 `json:"seconds"`
}

// run runs fhl and returns the exit code.
func run(args []string, e env) int {
	start := time.Now()
	o, err := parseArgs(args)
	if errors.Is(err, errHelp) {
		fmt.Fprint(e.stdout, usage)
		return 0
	}
	if errors.Is(err, errThemeValue) {
		fmt.Fprintln(e.stderr, err)
		return 1
	}
	if err != nil {
		fmt.Fprintln(e.stderr, flagError(err))
		return 1
	}
	switch {
	case o.help:
		fmt.Fprint(e.stdout, usage)
		return 0
	case o.version:
		fmt.Fprintln(e.stdout, "fhl "+versionString())
		return 0
	case o.listLangs:
		return listLangs(e.stdout, o.json)
	case o.listThemes:
		return listThemes(e.stdout, o.json)
	}
	if msg := checkOptions(o); msg != "" {
		fmt.Fprintln(e.stderr, msg)
		return 1
	}
	th, thMode, _ := parseTheme(o.theme)

	jobs, perrs, err := plan(o)
	if err != nil {
		fmt.Fprintln(e.stderr, err)
		return 1
	}
	r := &report{Files: []fileResult{}, Styles: []string{}, Skipped: []string{}, Uncolored: []uncolored{}, Errors: []errorResult{}}
	// siteRoot is the top of the output of --static-site, where its one highlight.css is written.
	siteRoot := ""
	if o.staticSite {
		siteRoot = o.inputs[0]
		if o.outSet {
			siteRoot = o.out
		}
		r.Copied = &[]string{}
	}
	fail := func(msg string, inputs ...string) {
		r.Errors = append(r.Errors, errorResult{Message: msg, Inputs: inputs})
		fmt.Fprintln(e.stderr, msg)
	}
	for _, pe := range perrs {
		fail(pe.msg, pe.inputs...)
	}

	var stdinData []byte
	usesStdin := false
	for _, j := range jobs {
		if j.stdin {
			usesStdin = true
			if stdinData, err = io.ReadAll(e.stdin); err != nil {
				fail(fmt.Sprintf("fhl: cannot read standard input: %v", err), "-")
				jobs = nil
			}
		}
	}

	w := &writer{
		force:       o.force,
		interactive: e.interactive && !usesStdin,
		answers:     bufio.NewReader(e.stdin),
		prompt:      e.stderr,
	}
	parallel := markdown.ParallelNever
	if len(jobs) == 1 {
		parallel = markdown.ParallelAuto
	}
	css := th.css(thMode)
	type result struct {
		c   *converted
		err error
	}
	mds := make(chan goldmark.Markdown, o.jobs) // reused by the converting goroutines
	for range o.jobs {
		mds <- newMarkdown(o.rawHTML, parallel, o.staticSite)
	}
	convert := func(j *job) result {
		src := stdinData
		if !j.stdin {
			b, err := os.ReadFile(j.path)
			if err != nil {
				return result{err: fmt.Errorf("fhl: cannot read %s: %v", j.input, cause(err))}
			}
			src = b
		}
		s := fh.NormalizeNewlines(string(src))
		if j.lang != nil {
			c, err := convertCode(j.langID, s)
			return result{c, err}
		}
		md := <-mds
		defer func() { mds <- md }()
		c, err := convertMarkdown(md, j.input, []byte(s))
		return result{c, err}
	}

	styleDirs := map[string]bool{} // directories where HTML was written (or was already identical)
	var styleOrder []string
	handle := func(j *job, res result) {
		if res.err != nil {
			fail(res.err.Error(), j.input)
			return
		}
		c := res.c
		fr := fileResult{Input: j.input, Blocks: c.blocks, Uncolored: len(c.uncolored)}
		done := func() {
			r.Files = append(r.Files, fr)
			r.Summary.Blocks += c.blocks
			r.Uncolored = append(r.Uncolored, c.uncolored...)
		}
		if j.output == "" {
			done()
			return
		}
		out := c.body
		title := c.title
		if title == "" {
			title = j.title
		}
		switch {
		case o.single:
			out = singleHTML(title, css, c.body)
		case o.staticSite:
			href, err := filepath.Rel(filepath.Dir(j.output), filepath.Join(siteRoot, styleName))
			if err != nil {
				href = styleName
			}
			out = sitePage(o.htmlLang, title, filepath.ToSlash(href), c.body)
		}
		if j.output == "-" {
			if _, err := e.stdout.Write(out); err != nil {
				fail(fmt.Sprintf("fhl: cannot write standard output: %v", err), j.input)
				return
			}
			fr.Output = "-"
			done()
			return
		}
		st, err := w.write(j.output, out)
		if err != nil {
			fail(err.Error(), j.input)
			return
		}
		fr.Output = j.output
		done()
		if st == skipped {
			r.Skipped = append(r.Skipped, j.output)
			return
		}
		if o.staticSite {
			d := filepath.Clean(siteRoot)
			if !styleDirs[d] {
				styleDirs[d] = true
				styleOrder = append(styleOrder, d)
			}
		} else if d := filepath.Dir(j.output); !o.single && !styleDirs[d] {
			styleDirs[d] = true
			styleOrder = append(styleOrder, d)
		}
	}
	pipeline(jobs, o.jobs, convert, handle)

	// highlight.css is written once per directory where HTML was written (once for directories that are the same underlying directory).
	styleKeys := map[string]bool{}
	for _, d := range styleOrder {
		p := filepath.Join(d, styleName)
		k := fileKey(p)
		if styleKeys[k] {
			continue
		}
		styleKeys[k] = true
		st, err := w.write(p, []byte(css))
		switch {
		case err != nil:
			fail(err.Error())
		case st == skipped:
			r.Skipped = append(r.Skipped, p)
		default:
			r.Styles = append(r.Styles, p)
		}
	}

	// --static-site copies the other files of the folder (images and so on) into the output folder.
	if o.staticSite && o.outSet && !o.check {
		var outs []string
		for _, j := range jobs {
			outs = append(outs, j.output)
		}
		copies, cerrs := planCopies(o.inputs[0], o.out, append(outs, filepath.Join(siteRoot, styleName)))
		for _, ce := range cerrs {
			fail(ce.msg, ce.inputs...)
		}
		for _, cp := range copies {
			data, err := os.ReadFile(cp.src)
			if err != nil {
				fail(fmt.Sprintf("fhl: cannot read %s: %v", cp.src, cause(err)), cp.src)
				continue
			}
			st, err := w.write(cp.dst, data)
			switch {
			case err != nil:
				fail(err.Error(), cp.src)
			case st == skipped:
				r.Skipped = append(r.Skipped, cp.dst)
			default:
				*r.Copied = append(*r.Copied, cp.dst)
			}
		}
	}

	r.Summary.Files = len(r.Files)
	r.Summary.Uncolored = len(r.Uncolored)
	r.Summary.Skipped = len(r.Skipped)
	r.Summary.Errors = len(r.Errors)
	r.Summary.Seconds = float64(time.Since(start).Round(time.Millisecond)) / float64(time.Second)
	jsonFailed := false
	if o.json {
		if err := encodeJSON(e.stdout, r); err != nil {
			fmt.Fprintf(e.stderr, "fhl: cannot write the --json result: %v\n", err)
			jsonFailed = true
		}
	} else if !o.quiet {
		for _, u := range r.Uncolored {
			at := u.File
			if u.Line > 0 {
				at = fmt.Sprintf("%s:%d", u.File, u.Line)
			}
			if u.Reason == "no-language" {
				fmt.Fprintf(e.stderr, "%s: code block has no language name\n", at)
			} else {
				fmt.Fprintf(e.stderr, "%s: code block language %q is not supported (see fhl --list-langs)\n", at, u.Lang)
			}
		}
		fmt.Fprintf(e.stderr, "fhl: %d files, %d code blocks (%d uncolored) in %.2fs\n",
			r.Summary.Files, r.Summary.Blocks, r.Summary.Uncolored, r.Summary.Seconds)
	}
	switch {
	case len(r.Errors) > 0, jsonFailed:
		return 1
	case o.strict && len(r.Uncolored) > 0:
		return 2
	}
	return 0
}

// checkOptions validates the flag values and their combinations. It returns the message if there is an error.
func checkOptions(o *options) string {
	switch {
	case len(o.inputs) == 0:
		return "fhl: no input; give files, folders or - (see fhl --help)"
	case o.rawHTML != "escape" && o.rawHTML != "omit" && o.rawHTML != "unsafe":
		return fmt.Sprintf("fhl: --raw-html must be escape, omit or unsafe, not %q", o.rawHTML)
	case themeError(o.theme) != "":
		return themeError(o.theme)
	case o.lang != "" && fh.LookupRegistered(o.lang) == nil:
		return fmt.Sprintf("fhl: unknown language %q for --lang; see fhl --list-langs", o.lang)
	case o.jobs < 1:
		return fmt.Sprintf("fhl: -j must be 1 or more, not %d", o.jobs)
	case o.outSet && o.out == "":
		return "fhl: -o needs a path; use -o FILE, -o DIR/ or -o -"
	case o.htmlLangSet && !o.staticSite:
		return "fhl: --html-lang is only for --static-site, which writes <html>"
	case o.staticSite:
		if msg := checkStaticSite(o); msg != "" {
			return msg
		}
	case o.check:
		return ""
	case o.json && o.outSet && o.out == "-":
		return "fhl: --json cannot be used with -o -"
	case o.themeSet && o.outSet && o.out == "-" && !o.single:
		return "fhl: --theme has no effect with -o - unless --single-html is also given, because no CSS is written"
	}
	return ""
}

// checkStaticSite validates the inputs and flags of --static-site. It returns the message if there is an error.
func checkStaticSite(o *options) string {
	switch {
	case o.single:
		return "fhl: --static-site and --single-html cannot be used together"
	case o.lang != "":
		return "fhl: --lang has no effect with --static-site, which reads only Markdown"
	case o.outSet && (o.out == "-" || !isDirPath(o.out)):
		return "fhl: --static-site writes a folder; use -o DIR/ (or no -o to write next to the Markdown files)"
	case len(o.inputs) != 1:
		return "fhl: --static-site needs exactly one folder"
	case strings.TrimSpace(o.htmlLang) == "" || strings.ContainsAny(o.htmlLang, "\"<>& "):
		return fmt.Sprintf("fhl: --html-lang must be a language tag such as en or ja, not %q", o.htmlLang)
	}
	if fi, err := os.Stat(o.inputs[0]); err != nil || !fi.IsDir() {
		return fmt.Sprintf("fhl: --static-site needs a folder, not %s", o.inputs[0])
	}
	return ""
}

// versionString returns the version.
func versionString() string {
	if version != "" {
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return "devel"
}

// listLangs writes the list of supported languages.
func listLangs(w io.Writer, asJSON bool) int {
	type lang struct {
		Name      string   `json:"name"`
		Aliases   []string `json:"aliases"`
		Filenames []string `json:"filenames"`
	}
	var langs []lang
	for _, l := range fh.Languages() {
		langs = append(langs, lang{l.Name, append([]string{}, l.Aliases...), append([]string{}, l.Filenames...)})
	}
	if asJSON {
		return writeJSON(w, langs)
	}
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tALIASES\tFILES")
	for _, l := range langs {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", l.Name, strings.Join(l.Aliases, ", "), strings.Join(l.Filenames, ", "))
	}
	tw.Flush()
	return 0
}

// themeError returns the error message for the value of --theme, or "" if it is valid.
func themeError(v string) string {
	_, _, msg := parseTheme(v)
	return msg
}

// listThemes writes the list of available color themes.
func listThemes(w io.Writer, asJSON bool) int {
	type entry struct {
		Name  string `json:"name"`
		Label string `json:"label"`
		Light string `json:"light"`
		Dark  string `json:"dark"`
	}
	var list []entry
	for _, t := range themes {
		list = append(list, entry{t.name, t.label, t.lightSource, t.darkSource})
	}
	if asJSON {
		return writeJSON(w, list)
	}
	fmt.Fprintln(w, "Use --theme=MODE:THEME, where MODE is light, dark or both (e.g. --theme=light:github).")
	fmt.Fprintln(w)
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "THEME\tNAME\tLIGHT\tDARK")
	for _, t := range list {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", t.Name, t.Label, t.Light, t.Dark)
	}
	tw.Flush()
	return 0
}

func writeJSON(w io.Writer, v any) int {
	if encodeJSON(w, v) != nil {
		return 1
	}
	return 0
}

// encodeJSON writes v to w as indented JSON.
func encodeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}
