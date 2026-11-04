package main

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"time"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
)

var start = time.Now() // package init order: measured from main package var init

func main() {
	src, _ := os.ReadFile(os.Args[1])
	t0 := time.Now()
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM,
			highlighting.NewHighlighting(
				highlighting.WithStyle("github"),
				highlighting.WithFormatOptions(chromahtml.WithClasses(true)),
			)),
	)
	tInit := time.Since(t0)

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
	fmt.Printf("go init=%.2fms first=%.2fms warm_median=%.3fms warm_p95=%.3fms since_main_var=%.2fms bytes=%d\n",
		float64(tInit.Microseconds())/1000, float64(tFirst.Microseconds())/1000, d[n/2], d[n*95/100],
		float64(time.Since(start).Microseconds())/1000, buf.Len())
}
