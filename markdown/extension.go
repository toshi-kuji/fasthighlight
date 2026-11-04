package markdown

import (
	"bytes"
	"runtime"
	"sync"
	"sync/atomic"

	fh "github.com/toshi-kuji/fasthighlight"
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
		return
	}
	src := reader.Source()
	var blocks []*ast.FencedCodeBlock
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if b, ok := n.(*ast.FencedCodeBlock); ok && entering {
			blocks = append(blocks, b)
		}
		return ast.WalkContinue, nil
	})
	if len(blocks) < 2 {
		return
	}
	codes := make([]string, len(blocks))
	names := make([]string, len(blocks))
	langs := make([]*fh.Language, len(blocks))
	colored, need, size := 0, 3, 0
	for k, b := range blocks {
		codes[k] = blockCode(b, src)
		names[k] = string(b.Language(src))
		if langs[k] = fh.LookupRegistered(names[k]); langs[k] != nil {
			colored++
			need = max(need, needBlocks(langs[k]))
			size += len(codes[k])
		}
	}
	if !parallel(t.e.Parallelism, colored, need, size) {
		return
	}
	// The blocks to highlight are shared among as many goroutines as there are CPUs (including the calling
	// goroutine). Blocks that are not highlighted are light, so they are written in place at render time.
	outs := make([][]byte, len(blocks))
	var next atomic.Int32
	work := func() {
		for {
			k := int(next.Add(1)) - 1
			if k >= len(blocks) {
				return
			}
			if langs[k] == nil {
				continue
			}
			var buf bytes.Buffer
			buf.Grow(len(codes[k]) * 4)
			_ = fh.Highlight(&buf, names[k], codes[k]) // writing to a bytes.Buffer does not fail
			outs[k] = buf.Bytes()
		}
	}
	var wg sync.WaitGroup
	for range min(runtime.GOMAXPROCS(0), colored) - 1 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			work()
		}()
	}
	work()
	wg.Wait()
	for k, b := range blocks {
		if outs[k] != nil {
			b.SetAttribute(highlightedAttr, outs[k])
		}
	}
}

// parallel returns true if highlighting should be parallel, when there are n code blocks to highlight, the
// number of blocks needed to go parallel is need (the largest the languages' ParallelHint calls for), and their
// contents total size bytes.
func parallel(p Parallelism, n, need, size int) bool {
	switch p {
	case ParallelAlways:
		return true
	case ParallelNever:
		return false
	}
	return size > ParallelMinBytes && n >= need
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
	name := string(b.Language(src))
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

// needBlocks returns the number of code blocks needed to highlight language l in parallel with
// ParallelAuto: 3 if its ParallelHint is HintHeavy, 4 otherwise (HintLight or HintDefault).
func needBlocks(l *fh.Language) int {
	if l.ParallelHint == fh.HintHeavy {
		return 3
	}
	return 4
}
