package markdown

import (
	"bytes"
	"strings"

	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/yuin/goldmark/ast"
)

// HighlightedBlock is one code block of a document, highlighted. Open + Code + Close is the same HTML that
// Render writes for the block; Code alone is what goes inside <code>.
type HighlightedBlock struct {
	// Document is the name of the document (the one passed to Parse).
	Document string
	// Line is the line number of the start of the code block (the ``` line), counting from 1. For an empty
	// block with neither a language name nor contents, the location is unknown, so it is 0.
	Line int
	// Lang is the code block's language name (the first word next to the ```). Empty if there is no language name.
	Lang string
	// Open is the opening tags: <pre class="highlight" …><code …>.
	Open string
	// Code is the contents of <code>: the code, highlighted (or only escaped when Colored is false).
	Code string
	// Close is the closing tags: </code></pre>.
	Close string
	// Colored is true if the code is highlighted.
	Colored bool
	// Reason says why the code is not highlighted, when Colored is false: "no-language" (no language name),
	// "plain" (marked as plain text, see fasthighlight.IsPlainText), "not-built-in" (a supported language whose
	// package is not imported) or "unsupported".
	Reason string
}

// CodeBlocks returns the code blocks of a parsed document (those fenced with ``` or ~~~), highlighted, in
// order. It writes no other HTML. Use it when you convert the Markdown yourself and need only the colored
// code. The blocks are highlighted in parallel as Parallelism decides (see Parse); calling Render afterwards
// still works. For a document from NewDocument, the blocks are highlighted here, also as Parallelism
// decides (unless your goldmark has Extension, which already did it while parsing).
func (c *Converter) CodeBlocks(d *Document) []HighlightedBlock {
	var pre [][]byte // for a document from NewDocument: the HTML highlighted here in parallel
	if !d.parsed && !carriesHTML(d.blocks) {
		pre = highlightAll(c.Parallelism, d.blocks, d.src)
	}
	out := make([]HighlightedBlock, 0, len(d.blocks))
	for k, b := range d.blocks {
		name := string(b.Language(d.src))
		var html []byte
		if v, ok := b.Attribute(highlightedAttr); ok {
			html, _ = v.([]byte)
		} else if pre != nil {
			html = pre[k]
		}
		if html == nil {
			var buf bytes.Buffer
			_ = fh.Highlight(&buf, name, blockCode(b, d.src)) // writing to a bytes.Buffer does not fail
			html = buf.Bytes()
		}
		h := string(html)
		// The HTML is <pre …><code …>, the code, then </code></pre>; the code itself has no <code.
		i := strings.Index(h, "<code")
		i += strings.IndexByte(h[i:], '>') + 1
		j := len(h) - len("</code></pre>")
		hb := HighlightedBlock{Document: d.Name, Line: fenceLine(d.src, b), Lang: name,
			Open: h[:i], Code: h[i:j], Close: h[j:], Colored: true}
		switch {
		case name == "":
			hb.Colored, hb.Reason = false, "no-language"
		case fh.LookupRegistered(name) != nil:
		case fh.IsPlainText(name):
			hb.Colored, hb.Reason = false, "plain"
		case fh.LookupSupported(name) != nil:
			hb.Colored, hb.Reason = false, "not-built-in"
		default:
			hb.Colored, hb.Reason = false, "unsupported"
		}
		out = append(out, hb)
	}
	return out
}

// carriesHTML reports whether any of the blocks carries HTML highlighted in advance (by Extension in the
// goldmark that parsed them).
func carriesHTML(blocks []*ast.FencedCodeBlock) bool {
	for _, b := range blocks {
		if _, ok := b.Attribute(highlightedAttr); ok {
			return true
		}
	}
	return false
}
