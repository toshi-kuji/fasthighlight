package markdown

import (
	"bytes"

	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/mdcore"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// Extension is a goldmark extension. It turns code blocks (those fenced with ```) into HTML highlighted
// with the registered languages. A block of a language that is not registered is only escaped, without
// highlighting. For the shape of the HTML it writes, see the fasthighlight package documentation.
//
//	md := goldmark.New(goldmark.WithExtensions(extension.GFM, &markdown.Extension{}))
//
// Register the languages to highlight by importing the language packages (lang/<language>, or lang/all
// for all languages). One goldmark.Markdown may be used from several goroutines at the same time.
type Extension struct {
	// Parallelism is whether the code blocks of one document are highlighted in parallel. The zero value is ParallelAuto.
	Parallelism Parallelism
}

// Extend is goldmark.Extender's Extend.
func (e *Extension) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(parser.WithASTTransformers(util.Prioritized(&parallelTransformer{e}, 1000)))
	m.Renderer().AddOptions(renderer.WithNodeRenderers(util.Prioritized(codeRenderer{}, 200)))
}

// highlightedAttr is the name of the attribute with which a code block node carries HTML highlighted in
// advance in parallel. Code blocks are written by this extension's codeRenderer, so it never appears in the
// HTML as an attribute.
var highlightedAttr = []byte("fasthighlight-html")

// parallelTransformer, after the document is parsed and only when going parallel, highlights all the code
// blocks in advance. When sequential, it does nothing, and each block is highlighted as it is written.
type parallelTransformer struct{ e *Extension }

func (t *parallelTransformer) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) {
	if t.e.Parallelism == ParallelNever {
		return // nothing to do, so do not even walk the document
	}
	src := reader.Source()
	var blocks []*ast.FencedCodeBlock
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if b, ok := n.(*ast.FencedCodeBlock); ok && entering {
			blocks = append(blocks, b)
		}
		return ast.WalkContinue, nil
	})
	for k, out := range highlightAll(t.e.Parallelism, blocks, src) {
		if out != nil {
			blocks[k].SetAttribute(highlightedAttr, out)
		}
	}
}

// highlightAll highlights the code blocks in parallel when p says so, and returns the HTML of each block
// (nil for blocks of languages that are not registered, which are light and are written in place). It
// returns nil when it does not go parallel.
func highlightAll(p Parallelism, blocks []*ast.FencedCodeBlock, src []byte) [][]byte {
	return mdcore.HighlightAll(mdcore.Mode(p), len(blocks),
		func(k int) string { return blockCode(blocks[k], src) },
		func(k int) string { return LanguageName(blocks[k], src) })
}

// codeRenderer writes code blocks. It writes the HTML highlighted in advance if there is any, and
// otherwise highlights and writes.
type codeRenderer struct{}

func (codeRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindFencedCodeBlock, renderCode)
}

func renderCode(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	if v, ok := node.Attribute(highlightedAttr); ok {
		if out, ok := v.([]byte); ok {
			_, err := w.Write(out)
			return ast.WalkContinue, err
		}
	}
	b := node.(*ast.FencedCodeBlock)
	name := LanguageName(b, src)
	return ast.WalkContinue, fh.Highlight(w, name, blockCode(b, src))
}

// blockCode returns the contents of a code block.
func blockCode(b *ast.FencedCodeBlock, src []byte) string {
	lines := b.Lines()
	n := 0
	for i := 0; i < lines.Len(); i++ {
		seg := lines.At(i)
		n += seg.Len()
	}
	var code bytes.Buffer
	code.Grow(n)
	for i := 0; i < lines.Len(); i++ {
		line := lines.At(i)
		code.Write(line.Value(src))
	}
	return code.String()
}

// LanguageName returns the language name of a fenced code block, as the extension reads it: the first word of
// its info string, up to a space or tab, with backslash escapes and entity references resolved as CommonMark does (goldmark's
// FencedCodeBlock.Language splits only at a space and leaves both as they are).
func LanguageName(b *ast.FencedCodeBlock, src []byte) string {
	if b.Info == nil {
		return ""
	}
	info := bytes.TrimLeft(b.Info.Segment.Value(src), " \t")
	if i := bytes.IndexAny(info, " \t"); i >= 0 {
		info = info[:i]
	}
	if bytes.IndexAny(info, `\&`) < 0 {
		return string(info)
	}
	// One pass, so that what one rule produces is not read again by the other (\&amp; is "&amp;").
	var out []byte
	for i := 0; i < len(info); {
		switch c := info[i]; {
		case c == '\\' && i+1 < len(info) && util.IsPunct(info[i+1]):
			out = append(out, info[i+1])
			i += 2
			continue
		case c == '&':
			if j := bytes.IndexByte(info[i:], ';'); j > 0 && isRefName(info[i+1:i+j]) {
				ref := info[i : i+j+1]
				r := util.ResolveEntityNames(util.ResolveNumericReferences(ref))
				if !bytes.Equal(r, ref) {
					out = append(out, r...)
					i += j + 1
					continue
				}
			}
		}
		out = append(out, info[i])
		i++
	}
	return string(out)
}

// isRefName reports whether b can be the inside of an entity or numeric reference (between & and ;). Only such a
// reference is resolved; without this check, "&\+&#43;" would be read as one reference up to the ; and the \+
// in it would be left as is.
func isRefName(b []byte) bool {
	for _, c := range b {
		if !('a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9' || c == '#') {
			return false
		}
	}
	return true
}
