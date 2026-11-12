package markdown

import (
	"bytes"
	"strconv"
	"unicode/utf8"

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
				if r, ok := resolveRef(info[i : i+j+1]); ok {
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

// resolveRef resolves one reference, &…; (whose inside isRefName accepted), as CommonMark does: a numeric
// reference is the character of that code point (U+FFFD for 0, a surrogate or one above U+10FFFF), and an
// entity reference is resolved only if the name is an HTML entity. goldmark v1's ResolveNumericReferences is
// not used for numbers: it reads "&#0000065;" (7 digits) as "5".
func resolveRef(ref []byte) ([]byte, bool) {
	inner := ref[1 : len(ref)-1]
	if inner[0] != '#' {
		r := util.ResolveEntityNames(ref)
		return r, !bytes.Equal(r, ref)
	}
	base, digits := 10, inner[1:]
	if digits[0] == 'x' || digits[0] == 'X' {
		base, digits = 16, digits[1:]
	}
	n, err := strconv.ParseUint(string(digits), base, 32)
	if err != nil {
		return nil, false
	}
	r := rune(n)
	if r == 0 || !utf8.ValidRune(r) {
		r = utf8.RuneError
	}
	return utf8.AppendRune(nil, r), true
}

// isRefName reports whether b can be the inside of an entity or numeric reference (between & and ;), as
// CommonMark defines them: a name of ASCII letters and digits, # and 1 to 7 decimal digits, or #x and 1 to 6
// hex digits. Only such a reference is resolved; without this check, "&\+&#43;" would be read as one reference
// up to the ; and the \+ in it would be left as is, and "&#x0000041;" would be resolved.
func isRefName(b []byte) bool {
	digits, max := b, 0
	switch {
	case len(b) > 1 && b[0] == '#' && (b[1] == 'x' || b[1] == 'X'):
		digits, max = b[2:], 6
	case len(b) > 0 && b[0] == '#':
		digits, max = b[1:], 7
	}
	if len(digits) == 0 || max > 0 && len(digits) > max {
		return false
	}
	for _, c := range digits {
		ok := '0' <= c && c <= '9' || max == 6 && ('a' <= c && c <= 'f' || 'A' <= c && c <= 'F') ||
			max == 0 && ('a' <= c && c <= 'z' || 'A' <= c && c <= 'Z')
		if !ok {
			return false
		}
	}
	return true
}
