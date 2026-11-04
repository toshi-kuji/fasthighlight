package main

import (
	"bytes"
	"fmt"
	"html"
	"os"
	"sort"
	"time"

	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

type codeRenderer struct {
	hl map[string]*gotreesitter.Highlighter
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
	src := code.Bytes()
	w.WriteString(`<pre class="ts-hl"><code>`)
	pos := uint32(0)
	if h, ok := r.hl[string(n.Language(source))]; ok {
		for _, rg := range h.Highlight(src) {
			if rg.StartByte < pos || rg.EndByte > uint32(len(src)) {
				continue
			}
			w.WriteString(html.EscapeString(string(src[pos:rg.StartByte])))
			w.WriteString(`<span class="hl-` + rg.Capture + `">`)
			w.WriteString(html.EscapeString(string(src[rg.StartByte:rg.EndByte])))
			w.WriteString(`</span>`)
			pos = rg.EndByte
		}
	}
	w.WriteString(html.EscapeString(string(src[pos:])))
	w.WriteString("</code></pre>\n")
	return ast.WalkContinue, nil
}

func newHL(name string) *gotreesitter.Highlighter {
	e := grammars.DetectLanguageByName(name)
	if e == nil {
		panic("no grammar: " + name)
	}
	h, err := gotreesitter.NewHighlighter(e.Language(), e.HighlightQuery)
	if err != nil {
		panic(err)
	}
	return h
}

func main() {
	name := "gotreesitter-subset"
	t0 := time.Now()
	ts, js := newHL("typescript"), newHL("javascript")
	cr := &codeRenderer{hl: map[string]*gotreesitter.Highlighter{"ts": ts, "typescript": ts, "js": js, "javascript": js}}
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
