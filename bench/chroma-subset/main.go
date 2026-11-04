package main

import (
	"bytes"
	"fmt"
	"os"
	"sort"
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
	name := "chroma-subset"
	t0 := time.Now()
	ts := chroma.Coalesce(chroma.MustNewXMLLexer(data, "data/typescript.xml"))
	js := chroma.Coalesce(chroma.MustNewXMLLexer(data, "data/javascript.xml"))
	f, _ := data.Open("data/github.xml")
	style := chroma.MustNewXMLStyle(f)
	cr := &codeRenderer{
		lexers:    map[string]chroma.Lexer{"ts": ts, "typescript": ts, "js": js, "javascript": js},
		style:     style,
		formatter: chromahtml.New(chromahtml.WithClasses(true)),
	}
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithRendererOptions(renderer.WithNodeRenderers(util.Prioritized(cr, 200))),
	)
	tSetup := time.Since(t0)
	src, _ := os.ReadFile(os.Args[1])
	var buf bytes.Buffer
	t1 := time.Now()
	if err := md.Convert(src, &buf); err != nil {
		panic(err)
	}
	tFirst := time.Since(t1)
	os.WriteFile(os.Args[2], buf.Bytes(), 0o644)
	if os.Getenv("ONCE") != "" {
		return
	}
	for i := 0; i < 20; i++ {
		buf.Reset()
		md.Convert(src, &buf)
	}
	n := 300
	d := make([]float64, n)
	for i := 0; i < n; i++ {
		buf.Reset()
		t := time.Now()
		md.Convert(src, &buf)
		d[i] = float64(time.Since(t).Microseconds()) / 1000
	}
	sort.Float64s(d)
	fmt.Printf("%s setup=%.2fms first=%.2fms warm_median=%.3fms warm_p95=%.3fms bytes=%d\n",
		name, float64(tSetup.Microseconds())/1000, float64(tFirst.Microseconds())/1000, d[n/2], d[n*95/100], buf.Len())
}
