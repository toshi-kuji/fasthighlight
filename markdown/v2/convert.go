package markdown

import (
	"io"

	"github.com/toshi-kuji/fasthighlight/internal/mdcore"
	"github.com/yuin/goldmark/v2/ast"
	"github.com/yuin/goldmark/v2/extension"
	"github.com/yuin/goldmark/v2/parser"
	"github.com/yuin/goldmark/v2/renderer/html"
)

// Parallelism is whether code blocks are highlighted in parallel.
type Parallelism int

const (
	// ParallelAuto is the default. It highlights in parallel when the total size of the contents of the code
	// blocks to be highlighted (those of registered languages) exceeds ParallelMinBytes bytes, and the
	// number of blocks is at least the largest number those languages call for: 3 for a language whose
	// fasthighlight.Language.ParallelHint is fasthighlight.HintHeavy, 4 otherwise. For small documents,
	// and for few blocks of languages that are light to highlight (JSON, YAML, and so on), the goroutine
	// cost outweighs the gain and it would be slower.
	ParallelAuto Parallelism = iota
	// ParallelNever always highlights sequentially.
	ParallelNever
	// ParallelAlways always highlights in parallel when there are 2 or more code blocks.
	ParallelAlways
)

// ParallelMinBytes is the total size of the contents of the code blocks to be highlighted above which
// ParallelAuto goes parallel (it must be exceeded).
const ParallelMinBytes = mdcore.MinBytes

// Converter turns Markdown (GFM) into HTML. It is a goldmark v2 parser and HTML renderer with GFM and
// Extension. To use your own goldmark configuration, use Extension instead of Converter. Parallelism decides
// whether code blocks are highlighted in parallel. Parse and Convert must not be called from several
// goroutines at the same time. CodeBlocks and Render may be: one Converter can serve the pages of a site in
// parallel once they are parsed (with Parse one at a time, or with your own goldmark and NewDocument). Do not
// change Parallelism while it is in use.
//
// For a single document, use Convert. To find out which languages are used before converting several
// documents, parse all the documents with Parse, check them with Scan, and then write them out with
// Render (parsing is done only once).
type Converter struct {
	Parallelism Parallelism
	p           parser.Parser
	r           html.Renderer
	ext         *Extension
}

// NewConverter creates a Converter. Parallelism is ParallelAuto.
func NewConverter() *Converter {
	ext := &Extension{}
	return &Converter{
		p:   parser.New(parser.WithExtensions(extension.NewGFMParser(), ext)),
		r:   html.New(html.WithExtensions(extension.NewGFMHTMLRenderer(), ext)),
		ext: ext,
	}
}

// Document is parsed Markdown. It is created by Parse, or by NewDocument from a document parsed with your
// own goldmark.
type Document struct {
	// Name is the name of the document (a file name, for example). It appears as is in the result of Scan.
	Name   string
	src    []byte
	root   ast.Node
	blocks []*ast.CodeBlock
	// parsed is true for a document from Parse, whose blocks were already highlighted in parallel when
	// Parallelism said so.
	parsed bool
}

// Parse parses src. name is used to identify the document in the result of Scan (it may be empty).
// When highlighting in parallel, the highlighting is also finished here.
func (c *Converter) Parse(name string, src []byte) *Document {
	c.ext.Parallelism = c.Parallelism
	d := NewDocument(name, src, c.p.Parse(src))
	d.parsed = true
	return d
}

// NewDocument wraps a document you parsed with your own goldmark, so that CodeBlocks and Scan can be used
// on it without parsing it again. src is the Markdown that was parsed, and root is the node the parser
// returned. Code blocks are the fenced *ast.CodeBlock nodes under root, in order; a block that your AST
// transformers turned into another kind of node is not one.
//
// Render also works on it, with this package's settings (GFM and fasthighlight), not yours; to write the
// document with your own renderer, use your renderer.
func NewDocument(name string, src []byte, root ast.Node) *Document {
	return &Document{Name: name, src: src, root: root, blocks: fencedBlocks(root)}
}

// Convert turns src into HTML and writes it to w.
func (c *Converter) Convert(src []byte, w io.Writer) error {
	return c.Render(w, c.Parse("", src))
}

// Render turns a parsed document into HTML and writes it to w.
func (c *Converter) Render(w io.Writer, d *Document) error {
	return c.r.Render(w, d.src, d.root)
}
