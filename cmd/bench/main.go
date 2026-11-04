package main

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"syscall"
	"time"

	_ "github.com/toshi-kuji/fasthighlight/lang/all"
	"github.com/toshi-kuji/fasthighlight/markdown"
)

func cpuTime() time.Duration {
	var ru syscall.Rusage
	syscall.Getrusage(syscall.RUSAGE_SELF, &ru)
	return time.Duration(ru.Utime.Nano() + ru.Stime.Nano())
}

func main() {
	src, _ := os.ReadFile(os.Args[2])

	t0 := time.Now()
	c := markdown.NewConverter()
	switch os.Args[1] {
	case "seq":
		c.Parallelism = markdown.ParallelNever
	case "par":
		c.Parallelism = markdown.ParallelAlways
	case "auto":
	default:
		panic("usage: bench seq|par|auto input.md out.html")
	}
	tSetup := time.Since(t0)

	var buf bytes.Buffer
	t1 := time.Now()
	if err := c.Convert(src, &buf); err != nil {
		panic(err)
	}
	tFirst := time.Since(t1)
	os.WriteFile(os.Args[3], buf.Bytes(), 0o644)
	if os.Getenv("ONCE") != "" {
		return
	}

	for i := 0; i < 20; i++ {
		buf.Reset()
		c.Convert(src, &buf)
	}
	n := 300
	d := make([]float64, n)
	cpu0 := cpuTime()
	for i := 0; i < n; i++ {
		buf.Reset()
		t := time.Now()
		c.Convert(src, &buf)
		d[i] = float64(time.Since(t).Microseconds()) / 1000
	}
	cpuPer := float64((cpuTime() - cpu0).Microseconds()) / 1000 / float64(n)
	sort.Float64s(d)
	fmt.Printf("%s setup=%.2fms first=%.2fms warm_median=%.3fms warm_p95=%.3fms cpu_per_op=%.3fms bytes=%d\n",
		os.Args[1], float64(tSetup.Microseconds())/1000, float64(tFirst.Microseconds())/1000, d[n/2], d[n*95/100], cpuPer, buf.Len())
}
