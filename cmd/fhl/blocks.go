package main

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/markdown"
)

// codeBlock is one line of the output of --code-blocks: a fenced code block of a Markdown input, colored.
// open + code + close is the same HTML that a conversion writes for the block.
type codeBlock struct {
	File    string `json:"file"`             // the input ("-" for standard input)
	Line    int    `json:"line"`             // the line of the opening fence (1-based), 0 if unknown
	Lang    string `json:"lang"`             // the language name written after the fence, "" if none
	Open    string `json:"open"`             // <pre class="highlight" …><code …>
	Code    string `json:"code"`             // the contents of <code>: the colored code
	Close   string `json:"close"`            // </code></pre>
	Colored bool   `json:"colored"`          // whether the code got colors
	Reason  string `json:"reason,omitempty"` // why not: no-language, unsupported, not-built-in or plain
}

// runCodeBlocks runs --code-blocks: it reads the Markdown inputs and writes each fenced code block, colored,
// as one line of JSON (JSON Lines) on standard output, in the order of the inputs and of the blocks in them.
// It returns the exit code.
func runCodeBlocks(o *options, e env) int {
	if msg := checkCodeBlocks(o); msg != "" {
		fmt.Fprintln(e.stderr, msg)
		return 1
	}
	out := bufio.NewWriter(e.stdout)
	var enc jsonEncoder
	failed := false
	files, blocks, uncolored := 0, 0, 0
	conv := markdown.NewConverter()
	for _, in := range o.inputs {
		paths, errs := codeBlockInputs(in)
		for _, msg := range errs {
			fmt.Fprintln(e.stderr, msg)
			failed = true
		}
		for _, p := range paths {
			var src []byte
			var err error
			if p == "-" {
				src, err = io.ReadAll(e.stdin)
			} else {
				src, err = os.ReadFile(p)
			}
			if err != nil {
				fmt.Fprintf(e.stderr, "fhl: cannot read %s: %v\n", p, cause(err))
				failed = true
				continue
			}
			files++
			for _, b := range codeBlocksOf(conv, p, []byte(fh.NormalizeNewlines(string(src)))) {
				blocks++
				if !b.Colored && b.Reason != "plain" {
					uncolored++
				}
				if err := enc.encodeCodeBlock(out, b); err != nil {
					fmt.Fprintf(e.stderr, "fhl: cannot write the output: %v\n", err)
					return 1
				}
			}
		}
	}
	if err := out.Flush(); err != nil {
		fmt.Fprintf(e.stderr, "fhl: cannot write the output: %v\n", err)
		return 1
	}
	if !o.quiet {
		fmt.Fprintf(e.stderr, "fhl: %d files, %d code blocks (%d uncolored)\n", files, blocks, uncolored)
	}
	switch {
	case failed:
		return 1
	case o.strict && uncolored > 0:
		return 2
	}
	return 0
}

// checkCodeBlocks validates the flags of --code-blocks. It returns the message if there is an error.
func checkCodeBlocks(o *options) string {
	other := ""
	switch {
	case o.printCSS:
		other = "--print-css"
	case o.outSet:
		other = "-o"
	case o.json:
		other = "--json"
	case o.single:
		other = "--single-html"
	case o.staticSite:
		other = "--static-site"
	case o.check:
		other = "--check"
	case o.lang != "":
		other = "--lang"
	case o.force:
		other = "--force"
	case cssFlag(o) != "":
		other = cssFlag(o)
	}
	switch {
	case other == "--json":
		return "fhl: --code-blocks already writes JSON (one line per code block); leave out --json"
	case other != "":
		return "fhl: --code-blocks writes only the code blocks, as JSON Lines on standard output; " + other + " cannot be used with it"
	case len(o.inputs) == 0:
		return "fhl: no input; give Markdown files, folders or - (see fhl --help)"
	case count(o.inputs, "-") > 1:
		return "fhl: - (standard input) can be given only once" // as in a conversion: a second - would read nothing
	}
	return ""
}

// count returns how many times s is in list.
func count(list []string, s string) int {
	n := 0
	for _, x := range list {
		if x == s {
			n++
		}
	}
	return n
}

// codeBlockInputs returns the Markdown files of one input of --code-blocks: the input itself (or "-"), or the
// Markdown files in a folder, as a conversion reads them (names starting with . are skipped).
func codeBlockInputs(in string) ([]string, []string) {
	if in == "-" {
		return []string{"-"}, nil
	}
	fi, err := os.Stat(in)
	if err != nil {
		return nil, []string{fmt.Sprintf("fhl: cannot read %s: %v", in, cause(err))}
	}
	if !fi.IsDir() {
		if !isMarkdown(in) {
			return nil, []string{fmt.Sprintf("fhl: --code-blocks reads Markdown (.md, .markdown, .mkd), not %s", in)}
		}
		return []string{in}, nil
	}
	var paths, errs []string
	_ = filepath.WalkDir(in, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			errs = append(errs, fmt.Sprintf("fhl: cannot read %s: %v", p, cause(err)))
			return nil
		}
		if p != in && strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() || !isMarkdown(p) {
			return nil
		}
		if !d.Type().IsRegular() {
			if fi, err := os.Stat(p); err != nil || !fi.Mode().IsRegular() {
				return nil
			}
		}
		paths = append(paths, p)
		return nil
	})
	return paths, errs
}

// codeBlocksOf returns the fenced code blocks of the Markdown src (newlines normalized), colored by the
// markdown package's CodeBlocks. Front matter is skipped, and lines are counted in the original src.
func codeBlocksOf(c *markdown.Converter, name string, src []byte) []codeBlock {
	body, offset := stripFrontMatter(src)
	var blocks []codeBlock
	for _, b := range c.CodeBlocks(c.Parse(name, body)) {
		line := b.Line
		if line > 0 {
			line += offset
		}
		blocks = append(blocks, codeBlock{File: name, Line: line, Lang: b.Lang, Open: b.Open, Code: b.Code,
			Close: b.Close, Colored: b.Colored, Reason: b.Reason})
	}
	return blocks
}
