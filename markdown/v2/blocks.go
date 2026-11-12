package markdown

import (
	"github.com/toshi-kuji/fasthighlight/internal/mdcore"
	"github.com/yuin/goldmark/v2/ast"
)

// HighlightedBlock is one code block of a document, highlighted. Open + Code + Close is the same HTML that
// Render writes for the block; Code alone is what goes inside <code>.
type HighlightedBlock struct {
	// Node is the code block's node in the document's AST. With your own goldmark (NewDocument), your
	// renderer can look the block up by its node instead of by its position.
	Node *ast.CodeBlock
	// Document is the name of the document (the one passed to Parse).
	Document string
	// Line is the line number of the start of the code block (the ``` line), counting from 1.
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
// decides (unless your parser has Extension, which already did it while parsing).
func (c *Converter) CodeBlocks(d *Document) []HighlightedBlock {
	var pre [][]byte // for a document from NewDocument: the HTML highlighted here in parallel
	if !d.parsed && !carriesHTML(d.blocks) {
		pre = highlightAll(c.Parallelism, d.blocks, d.src)
	}
	out := make([]HighlightedBlock, 0, len(d.blocks))
	for k, b := range d.blocks {
		name := LanguageName(b, d.src)
		html, ok := precomputed(b, d.src)
		if !ok && pre != nil {
			html = pre[k]
		}
		if html == nil {
			html = mdcore.Highlight(name, blockCode(b, d.src))
		}
		open, code, close := mdcore.Split(string(html))
		reason, _ := mdcore.Reason(name)
		out = append(out, HighlightedBlock{Node: b, Document: d.Name, Line: fenceLine(d.src, b), Lang: name,
			Open: open, Code: code, Close: close, Colored: reason == "", Reason: reason})
	}
	return out
}

// carriesHTML reports whether any of the blocks carries HTML highlighted in advance (by Extension in the
// parser that parsed them).
func carriesHTML(blocks []*ast.CodeBlock) bool {
	for _, b := range blocks {
		if _, ok := b.Attribute(highlightedAttr); ok {
			return true
		}
	}
	return false
}
