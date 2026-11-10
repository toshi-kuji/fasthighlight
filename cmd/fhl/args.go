package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"runtime"
	"strings"
)

// options is the result of reading the flags and inputs.
type options struct {
	out         string // value of -o; none if outSet is false
	outSet      bool
	lang        string
	single      bool
	staticSite  bool
	htmlLang    string
	htmlLangSet bool
	theme       string
	themeSet    bool
	printCSS    bool
	codeBlocks  bool
	themeSels   stringList
	noFrame     bool
	rawHTML     string
	force       bool
	listLangs   bool
	listThemes  bool
	check       bool
	strict      bool
	json        bool
	jobs        int
	quiet       bool
	version     bool
	help        bool
	inputs      []string
}

// errHelp reports that -h (not --help) was given.
var errHelp = errors.New("help requested")

// parseArgs reads the flags and inputs. Flags may appear before, after or between the inputs.
// Everything after -- is read as an input. If the same flag is given more than once, the last value is used.
func parseArgs(args []string) (*options, error) {
	o := &options{}
	fs := flag.NewFlagSet("fhl", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&o.out, "o", "", "")
	fs.StringVar(&o.lang, "lang", "", "")
	fs.BoolVar(&o.single, "single-html", false, "")
	fs.BoolVar(&o.staticSite, "static-site", false, "")
	fs.StringVar(&o.htmlLang, "html-lang", "en", "")
	fs.StringVar(&o.theme, "theme", "both:default", "")
	fs.BoolVar(&o.printCSS, "print-css", false, "")
	fs.BoolVar(&o.codeBlocks, "code-blocks", false, "")
	fs.Var(&o.themeSels, "theme-selector", "")
	fs.BoolVar(&o.noFrame, "no-frame", false, "")
	fs.StringVar(&o.rawHTML, "raw-html", "escape", "")
	fs.BoolVar(&o.force, "force", false, "")
	fs.BoolVar(&o.listLangs, "list-langs", false, "")
	fs.BoolVar(&o.listThemes, "list-themes", false, "")
	fs.BoolVar(&o.check, "check", false, "")
	fs.BoolVar(&o.strict, "strict", false, "")
	fs.BoolVar(&o.json, "json", false, "")
	fs.IntVar(&o.jobs, "j", runtime.GOMAXPROCS(0), "")
	fs.BoolVar(&o.quiet, "q", false, "")
	fs.BoolVar(&o.version, "version", false, "")
	fs.BoolVar(&o.help, "help", false, "")

	for {
		if err := fs.Parse(args); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return nil, errHelp
			}
			return nil, err
		}
		rest := fs.Args()
		if endedByDashes(fs, args[:len(args)-len(rest)]) {
			o.inputs = append(o.inputs, rest...)
			break
		}
		if len(rest) == 0 {
			break
		}
		o.inputs = append(o.inputs, rest[0])
		args = rest[1:]
	}
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "o":
			o.outSet = true
		case "theme":
			o.themeSet = true
		case "html-lang":
			o.htmlLangSet = true
		}
	})
	return o, nil
}

// endedByDashes reports whether the last of the arguments consumed by Parse was --, which marks the end of the flags.
// It returns false when -- was the value of a flag, as in -o --.
func endedByDashes(fs *flag.FlagSet, consumed []string) bool {
	for i := 0; i < len(consumed); i++ {
		a := consumed[i]
		if a == "--" {
			return i == len(consumed)-1
		}
		name := strings.TrimLeft(a, "-")
		if strings.Contains(name, "=") {
			continue
		}
		if f := fs.Lookup(name); f != nil {
			if b, ok := f.Value.(interface{ IsBoolFlag() bool }); ok && b.IsBoolFlag() {
				continue
			}
			i++ // the next argument is the value of this flag
		}
	}
	return false
}

// usage is the usage text written for --help.
const usage = `fhl converts Markdown (GFM) to HTML with fast syntax highlighting of code blocks.
It writes body-only HTML and a highlight.css with the code colors.

Usage:
  fhl [flags] [input...]

An input is a file, a folder, or - (standard input). Flags can be written before,
after or between inputs. Everything after -- is an input. A value follows its flag
after a space or = (--theme light or --theme=light).

Examples:
  fhl README.md                           # README.html and highlight.css next to README.md
  fhl docs/                               # .html next to each .md in docs/, highlight.css per folder
  fhl -o dist/ docs/                      # write into dist/, keeping the folder layout
  fhl -o dist/ docs/ --json               # report the result as JSON (for AI agents and scripts)
  fhl --single-html README.md             # standalone README.html with the CSS in <head>
  fhl -o - README.md                      # body-only HTML to standard output (no CSS)
  fhl --single-html -o - README.md        # standalone HTML to standard output
  fhl --static-site -o dist/ docs/        # a static site: full pages, heading ids, other files copied
  cat note.md | fhl -o note.html -        # read standard input
  fhl --lang go main.txt                  # read a code file as code of that language
  fhl --theme dark:github README.md       # choose the colors: light, dark or both, and a theme
  fhl --print-css > highlight.css         # only the CSS, to standard output
  fhl --code-blocks page.md               # each code block, colored, as one line of JSON
  fhl --force docs/                       # overwrite without asking
  fhl --list-langs                        # supported languages
  fhl --list-themes                       # available color themes
  fhl --check docs/                       # only report code blocks that are not colored

Inputs:
  .md, .markdown, .mkd    converted as Markdown
  other files             converted as one code block (language from --lang or the file name)
  folders                 every Markdown file inside, including subfolders (names starting with . are skipped)
  -                       standard input as Markdown (or code with --lang); needs -o

Flags:
  -o PATH            where to write: a file, a folder (ends with / or exists), or - for standard output.
                     Default: next to each input.
  --lang NAME        read code files and standard input as code of this language
  --single-html      write a standalone HTML with the CSS embedded in <head>, and ids on headings
  --static-site      turn one folder into a static site: full pages linking one highlight.css at the
                     top, ids on headings, links to .md rewritten to .html, other files copied
  --html-lang LANG   the lang of <html> with --static-site (default "en")
  --theme MODE[:THEME]
                     code colors. MODE is light, dark or both (both switches with the
                     viewer's light or dark preference); THEME is one of --list-themes.
                     Default: both:default
  --theme-selector MODE:SELECTOR
                     with --theme both, also use the MODE colors (light or dark) under the
                     CSS selector, over the device setting; can be given more than once
                     (e.g. dark:':root[data-theme="dark"]')
  --no-frame         leave out the rule that sets the background and text color of
                     pre.highlight, so that your own styles for code blocks apply
  --code-blocks      write each fenced code block of the Markdown inputs, colored, as one
                     line of JSON on standard output (file, line, lang, open, code, close,
                     colored, reason), for tools that convert the Markdown themselves
  --print-css        write only the CSS (as for highlight.css, with --theme) to standard
                     output and exit; give no inputs
  --raw-html MODE    raw HTML in Markdown: escape (default; shown as text), omit (dropped),
                     unsafe (written as is; do not use for untrusted Markdown)
  --force            overwrite existing files without asking
  --list-langs       print the supported languages and exit (JSON with --json)
  --list-themes      print the available themes and exit (JSON with --json)
  --check            write nothing; only report code blocks that are not colored
  --strict           exit with 2 if a code block is not colored
  --json             print the result as JSON on standard output
  -j N               files converted at the same time (default: the number of CPUs)
  -q                 do not print the summary (errors only)
  --version          print the version (and note a newer release) and exit
  --help             print this help and exit

Existing files are overwritten only after asking [y/N/a] on a terminal; otherwise
fhl stops with an error unless --force is given. Files with identical content are
left alone.

Exit status: 0 success, 1 error, 2 uncolored code blocks with --strict.
`

// flagError writes a flag error together with what to do next.
func flagError(err error) string {
	msg := err.Error()
	if name, ok := strings.CutPrefix(msg, "flag needs an argument: -"); ok {
		if hint, ok := valueHints[name]; ok {
			return "fhl: " + flagName(name) + " needs a value: " + hint
		}
		return "fhl: " + flagName(name) + " needs a value; see fhl --help"
	}
	if name, ok := strings.CutPrefix(msg, "flag provided but not defined: -"); ok {
		return "fhl: unknown flag " + flagName(name) + "; see fhl --help"
	}
	if rest, ok := strings.CutPrefix(msg, "invalid value "); ok {
		// invalid value "x" for flag -j: parse error
		if v, name, ok := strings.Cut(rest, " for flag -"); ok {
			name, _, _ = strings.Cut(name, ":")
			if hint, ok := valueHints[name]; ok {
				return "fhl: " + flagName(name) + " must be " + hint + ", not " + v
			}
		}
	}
	return fmt.Sprintf("fhl: %v; see fhl --help", err)
}

// stringList is a flag that can be given more than once; it keeps every value in order.
type stringList []string

func (l *stringList) String() string     { return strings.Join(*l, " ") }
func (l *stringList) Set(v string) error { *l = append(*l, v); return nil }

// valueHints tells what each flag with a value takes. It completes the messages about a missing or wrong value.
var valueHints = map[string]string{
	"o":              "a file, a folder (DIR/) or - for standard output",
	"lang":           "a language name such as go (see fhl --list-langs)",
	"html-lang":      "a language tag such as en or ja",
	"theme":          themeUsage,
	"raw-html":       "escape, omit or unsafe",
	"j":              "a number of 1 or more",
	"theme-selector": "dark:SELECTOR or light:SELECTOR, such as dark:':root[data-theme=\"dark\"]'",
}

// flagName writes a flag as the help does: -o and -j with one dash, the others with two.
func flagName(name string) string {
	name = strings.TrimLeft(name, "-")
	if len(name) == 1 {
		return "-" + name
	}
	return "--" + name
}
