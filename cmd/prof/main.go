package main

import (
	"bytes"
	"fmt"
	"os"
	"runtime/pprof"
	"time"

	_ "github.com/toshi-kuji/fasthighlight/lang/all"
	"github.com/toshi-kuji/fasthighlight/markdown"
)

func main() {
	path := "testdata/input.md"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}
	src, _ := os.ReadFile(path)
	c := markdown.NewConverter()
	var buf bytes.Buffer
	for i := 0; i < 50; i++ {
		buf.Reset()
		c.Convert(src, &buf)
	}
	f, _ := os.Create("cpu.prof")
	pprof.StartCPUProfile(f)
	n := 10000
	if len(os.Args) > 2 {
		fmt.Sscan(os.Args[2], &n)
	}
	t := time.Now()
	for i := 0; i < n; i++ {
		buf.Reset()
		c.Convert(src, &buf)
	}
	pprof.StopCPUProfile()
	fmt.Printf("avg=%.3fms over %d\n", float64(time.Since(t).Microseconds())/1000/float64(n), n)
}
