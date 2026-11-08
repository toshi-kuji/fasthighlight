package markdown

import (
	"io"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"
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
const ParallelMinBytes = 32 * 1024

// Converter turns Markdown (GFM) into HTML. It is goldmark with Extension added. To use your own goldmark
// configuration, use Extension instead of Converter. Parallelism decides whether code blocks are
// highlighted in parallel. Do not use one Converter from several goroutines at the same time.
//
// For a single document, use Convert. To find out which languages are used before converting several
// documents, parse all the documents with Parse, check them with Scan, and then write them out with
// Render (parsing is done only once).
type Converter struct {
	Parallelism Parallelism
	md          goldmark.Markdown
	ext         *Extension
}

// NewConverter creates a Converter. Parallelism is ParallelAuto.
func NewConverter() *Converter {
	ext := &Extension{}
	md := goldmark.New(goldmark.WithExtensions(extension.GFM, ext))
	return &Converter{md: md, ext: ext}
}

// Document is parsed Markdown. It is created by Parse, or by NewDocument from a document parsed with your
// own goldmark.
type Document struct {
	// Name is the name of the document (a file name, for example). It appears as is in the result of Scan.
	Name   string
	src    []byte
	root   ast.Node
	blocks []*ast.FencedCodeBlock
	// parsed is true for a document from Parse, whose blocks were already highlighted in parallel when
	// Parallelism said so.
	parsed bool
}

// Parse parses src. name is used to identify the document in the result of Scan (it may be empty).
// When highlighting in parallel, the highlighting is also finished here.
func (c *Converter) Parse(name string, src []byte) *Document {
	c.ext.Parallelism = c.Parallelism
	d := newDocument(name, src, c.md.Parser().Parse(text.NewReader(src)))
	d.parsed = true
	return d
}

// NewDocument wraps a document you parsed with your own goldmark, so that CodeBlocks and Scan can be used
// on it without parsing it again. src is the Markdown that was parsed, and root is the node the parser
// returned. Code blocks are the *ast.FencedCodeBlock nodes under root, in order; a block that your AST
// transformers turned into another kind of node is not one.
//
// Render also works on it, with this package's settings (GFM and fasthighlight), not yours; to write the
// document with your own renderer, use your goldmark.
func NewDocument(name string, src []byte, root ast.Node) *Document {
	return newDocument(name, src, root)
}

func newDocument(name string, src []byte, root ast.Node) *Document {
	var blocks []*ast.FencedCodeBlock
	_ = ast.Walk(root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if b, ok := n.(*ast.FencedCodeBlock); ok && entering {
			blocks = append(blocks, b)
		}
		return ast.WalkContinue, nil
	})
	return &Document{Name: name, src: src, root: root, blocks: blocks}
}

// Convert turns src into HTML and writes it to w.
func (c *Converter) Convert(src []byte, w io.Writer) error {
	return c.Render(w, c.Parse("", src))
}

// Render turns a parsed document into HTML and writes it to w.
func (c *Converter) Render(w io.Writer, d *Document) error {
	return c.md.Renderer().Render(w, d.src, d.root)
}
