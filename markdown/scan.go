package markdown

import (
	"bytes"

	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/yuin/goldmark/ast"
)

// CodeBlock is the location and language name of a code block that is not highlighted.
type CodeBlock struct {
	// Document is the name of the document (the one passed to Parse).
	Document string
	// Line is the line number of the start of the code block (the ``` line), counting from 1.
	// For an empty block with neither a language name nor contents, the location is unknown, so it is 0.
	Line int
	// Lang is the code block's language name (the first word next to the ```). Empty if there is no language name.
	Lang string
	// Package is, for a language that is supported but not built in, the package to import to be able to highlight it.
	Package string
}

// ScanResult is the result of checking the languages used in a document's code blocks.
type ScanResult struct {
	// Used are the languages to highlight (registered and in use), in name order.
	Used []*fh.Language
	// NotBuiltIn are the blocks whose language fasthighlight supports but whose language package is not
	// imported. They are not highlighted. Importing Package makes highlighting possible.
	NotBuiltIn []CodeBlock
	// Unsupported are the blocks of languages fasthighlight does not support. They are not highlighted.
	Unsupported []CodeBlock
	// NoLanguage are the blocks without a language name. They are not highlighted.
	NoLanguage []CodeBlock
}

// Uncolored returns true if there are blocks that are not highlighted.
func (r *ScanResult) Uncolored() bool {
	return len(r.NotBuiltIn)+len(r.Unsupported)+len(r.NoLanguage) > 0
}

// Scan collects the language names of the documents' code blocks (those fenced with ``` or ~~~) and returns
// the languages to highlight and the list of blocks that are not highlighted. Several documents are
// checked together, once. Scan does not prepare the languages (build keyword tables and so on): each
// language is prepared when its first code block is highlighted.
// Calling it before Render lets you report the list before conversion.
func Scan(docs ...*Document) *ScanResult {
	r := &ScanResult{}
	used := map[*fh.Language]bool{}
	for _, d := range docs {
		for _, b := range d.blocks {
			name := string(b.Language(d.src))
			cb := CodeBlock{Document: d.Name, Line: fenceLine(d.src, b), Lang: name}
			switch l := fh.LookupRegistered(name); {
			case name == "":
				r.NoLanguage = append(r.NoLanguage, cb)
			case l != nil:
				used[l] = true
			default:
				if c := fh.LookupSupported(name); c != nil {
					cb.Package = c.Package
					r.NotBuiltIn = append(r.NotBuiltIn, cb)
				} else {
					r.Unsupported = append(r.Unsupported, cb)
				}
			}
		}
	}
	for _, l := range fh.Languages() {
		if used[l] {
			r.Used = append(r.Used, l)
		}
	}
	return r
}

// fenceLine returns the line number of the start of a code block (the ``` line), or 0 if unknown.
func fenceLine(src []byte, b *ast.FencedCodeBlock) int {
	switch {
	case b.Info != nil:
		return 1 + bytes.Count(src[:b.Info.Segment.Start], []byte("\n"))
	case b.Lines().Len() > 0:
		return bytes.Count(src[:b.Lines().At(0).Start], []byte("\n")) // the line before the first line of the contents
	}
	return 0
}
