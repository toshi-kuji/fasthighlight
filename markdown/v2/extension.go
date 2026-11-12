package markdown

import (
	"bytes"
	"io"

	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/mdcore"
	"github.com/yuin/goldmark/v2/ast"
	"github.com/yuin/goldmark/v2/parser"
	"github.com/yuin/goldmark/v2/renderer"
	"github.com/yuin/goldmark/v2/renderer/html"
	"github.com/yuin/goldmark/v2/text"
	"github.com/yuin/goldmark/v2/util"
)

// Extension is a goldmark v2 extension. It turns code blocks (those fenced with ```) into HTML highlighted
// with the registered languages. A block of a language that is not registered is only escaped, without
// highlighting. Indented code blocks are left to the renderer that would write them without it. For the shape
// of the HTML it writes, see the fasthighlight package documentation.
//
// goldmark v2 has separate parser and renderer extensions; Extension is both, so pass the same value to both:
//
//	ext := &markdown.Extension{}
//	p := parser.New(parser.WithExtensions(extension.NewGFMParser(), ext))
//	r := html.New(html.WithExtensions(extension.NewGFMHTMLRenderer(), ext))
//
// Register the languages to highlight by importing the language packages (lang/<language>, or lang/all
// for all languages). The parser and the renderer may be used from several goroutines at the same time.
type Extension struct {
	// Parallelism is whether the code blocks of one document are highlighted in parallel. The zero value is ParallelAuto.
	Parallelism Parallelism
}

var (
	_ parser.Extension = (*Extension)(nil)
	_ html.Extension   = (*Extension)(nil)
)

// ParserOptions is parser.Extension's ParserOptions. It adds the AST transformer that highlights the code
// blocks in parallel, when Parallelism says so.
func (e *Extension) ParserOptions(_ *parser.Config) []parser.Option {
	return []parser.Option{
		parser.WithASTTransformers(util.Prioritized[parser.ASTTransformer](&parallelTransformer{e}, 1000)),
	}
}

// RendererOptions is html.Extension's RendererOptions. It decorates the code block renderer, so that fenced
// code blocks are highlighted and indented ones are written by the renderer it decorates.
func (e *Extension) RendererOptions(_ *html.Config) []html.Option {
	return []html.Option{html.WithNodeRendererDecorator(ast.KindCodeBlock, decorateCode)}
}

// highlightedAttr is the name of the attribute with which a code block node carries HTML highlighted in
// advance in parallel. Code blocks are written by this extension's renderer, so it never appears in the HTML
// as an attribute.
const highlightedAttr = "fasthighlight-html"

// parallelTransformer, after the document is parsed and only when going parallel, highlights all the code
// blocks in advance. When sequential, it does nothing, and each block is highlighted as it is written.
type parallelTransformer struct{ e *Extension }

func (t *parallelTransformer) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) {
	if t.e.Parallelism == ParallelNever {
		return // nothing to do, so do not even walk the document
	}
	src := reader.Source()
	blocks := fencedBlocks(doc)
	for k, out := range highlightAll(t.e.Parallelism, blocks, src) {
		if out != nil {
			// IdentityDecoder: the value is written as is, never decoded.
			blocks[k].SetAttribute(highlightedAttr, text.NewMultiLineValue(out, text.IdentityDecoder))
		}
	}
}

// fencedBlocks returns the fenced code blocks under root, in order.
func fencedBlocks(root ast.Node) []*ast.CodeBlock {
	var blocks []*ast.CodeBlock
	_ = ast.Walk(root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if b, ok := n.(*ast.CodeBlock); ok && entering && b.CodeBlockKind == ast.CodeBlockKindFenced {
			blocks = append(blocks, b)
		}
		return ast.WalkContinue, nil
	})
	return blocks
}

// highlightAll highlights the code blocks in parallel when p says so, and returns the HTML of each block
// (nil for blocks of languages that are not registered, which are light and are written in place). It
// returns nil when it does not go parallel.
func highlightAll(p Parallelism, blocks []*ast.CodeBlock, src []byte) [][]byte {
	return mdcore.HighlightAll(mdcore.Mode(p), len(blocks),
		func(k int) string { return blockCode(blocks[k], src) },
		func(k int) string { return LanguageName(blocks[k], src) })
}

// precomputed returns the HTML highlighted in advance for b, if there is any.
func precomputed(b *ast.CodeBlock, src []byte) ([]byte, bool) {
	v, ok := b.Attribute(highlightedAttr)
	if !ok {
		return nil, false
	}
	return v.Bytes(src), true
}

// decorateCode returns the code block renderer: it writes fenced code blocks, with the HTML highlighted in
// advance if there is any and otherwise highlighting them, and leaves the other code blocks to next.
func decorateCode(next html.NodeRenderer) html.NodeRenderer {
	return html.NodeRendererFunc(func(w io.Writer, src []byte, node ast.Node, entering bool,
		rc renderer.Context) (ast.WalkStatus, error) {
		b, ok := node.(*ast.CodeBlock)
		if !ok || b.CodeBlockKind != ast.CodeBlockKindFenced {
			return next.Render(w, src, node, entering, rc)
		}
		if !entering {
			return ast.WalkContinue, nil
		}
		if out, ok := precomputed(b, src); ok {
			_, err := w.Write(out)
			return ast.WalkContinue, err
		}
		return ast.WalkContinue, fh.Highlight(w, LanguageName(b, src), blockCode(b, src))
	})
}

// blockCode returns the contents of a code block.
func blockCode(b *ast.CodeBlock, src []byte) string {
	return b.Value.Str(src)
}

// infoDecoder resolves backslash escapes and entity references in a language name, as CommonMark does.
var infoDecoder = text.NewDecoder()

// LanguageName returns the language name of a fenced code block, as the extension reads it: the first word of
// its info string, up to a space or tab, with backslash escapes and entity references resolved as CommonMark
// does. It is "" for an indented code block. It is the same name the markdown package (for goldmark v1) reads.
func LanguageName(b *ast.CodeBlock, src []byte) string {
	if b.CodeBlockKind != ast.CodeBlockKindFenced || b.Info.IsEmpty() {
		return ""
	}
	// The word is cut out of the info string as written, and only then decoded, so that a space written as
	// a reference (&#32;) does not end it.
	info := bytes.TrimLeft(b.Info.Bytes(src), " \t")
	if i := bytes.IndexAny(info, " \t"); i >= 0 {
		info = info[:i]
	}
	return string(infoDecoder.Decode(info))
}
