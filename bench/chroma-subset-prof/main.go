package main

import (
	"bytes"
	"fmt"
	"os"
	"runtime/pprof"
	"time"

	"github.com/alecthomas/chroma/v2"
	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"

	"github.com/toshi-kuji/fasthighlight/bench/internal/chromadata"
)

var data = chromadata.FS

type codeRenderer struct {
	lexers    map[string]chroma.Lexer
	style     *chroma.Style
	formatter *chromahtml.Formatter
}

func (r *codeRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindFencedCodeBlock, r.render)
}

func (r *codeRenderer) render(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*ast.FencedCodeBlock)
	var code bytes.Buffer
	for i := 0; i < n.Lines().Len(); i++ {
		line := n.Lines().At(i)
		code.Write(line.Value(source))
	}
	var it chroma.Iterator
	if lexer, ok := r.lexers[string(n.Language(source))]; ok {
		var err error
		if it, err = lexer.Tokenise(nil, code.String()); err != nil {
			return ast.WalkStop, err
		}
	} else {
		it = chroma.Literator(chroma.Token{Type: chroma.Text, Value: code.String()})
	}
	return ast.WalkContinue, r.formatter.Format(w, r.style, it)
}

func main() {
	ts := chroma.Coalesce(chroma.MustNewXMLLexer(data, "data/typescript.xml"))
	js := chroma.Coalesce(chroma.MustNewXMLLexer(data, "data/javascript.xml"))
	f, _ := data.Open("data/github.xml")
	cr := &codeRenderer{
		lexers:    map[string]chroma.Lexer{"ts": ts, "typescript": ts, "js": js, "javascript": js},
		style:     chroma.MustNewXMLStyle(f),
		formatter: chromahtml.New(chromahtml.WithClasses(true)),
	}
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithRendererOptions(renderer.WithNodeRenderers(util.Prioritized(cr, 200))),
	)
	src, _ := os.ReadFile(os.Args[1])
	var buf bytes.Buffer
	for i := 0; i < 50; i++ {
		buf.Reset()
		md.Convert(src, &buf)
	}
	pf, _ := os.Create("cpu.prof")
	pprof.StartCPUProfile(pf)
	n := 3000
	t := time.Now()
	for i := 0; i < n; i++ {
		buf.Reset()
		md.Convert(src, &buf)
	}
	pprof.StopCPUProfile()
	fmt.Printf("avg=%.3fms over %d\n", float64(time.Since(t).Microseconds())/1000/float64(n), n)
}
