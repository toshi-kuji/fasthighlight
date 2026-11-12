// Package mdcore is the part of the goldmark extensions that does not depend on goldmark: deciding when to
// highlight the code blocks of a document in parallel, highlighting them, and classifying language names. It is
// shared by the markdown package (goldmark v1) and the markdown/v2 package (goldmark v2), so that each imports
// only its own goldmark.
package mdcore

import (
	"bytes"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	fh "github.com/toshi-kuji/fasthighlight"
)

// Mode is the value of the packages' Parallelism, with the same numbering.
type Mode int

// The modes, in the order of the packages' Parallelism constants.
const (
	Auto Mode = iota
	Never
	Always
)

// MinBytes is the packages' ParallelMinBytes.
const MinBytes = 32 * 1024

// HighlightAll highlights the code blocks with the given contents and language names in parallel when p says
// so, and returns the HTML of each block (nil for blocks of languages that are not registered, which are light
// and are written in place). It returns nil when it does not go parallel. code(k) and name(k) give the
// contents and language name of block k of n; when p is not Never and n is 2 or more, both are called for every
// block, before deciding whether to go parallel.
func HighlightAll(p Mode, n int, code, name func(k int) string) [][]byte {
	if p == Never || n < 2 {
		return nil
	}
	codes := make([]string, n)
	names := make([]string, n)
	langs := make([]*fh.Language, n)
	colored, need, size := 0, 3, 0
	for k := range n {
		codes[k] = code(k)
		names[k] = name(k)
		if langs[k] = fh.LookupRegistered(names[k]); langs[k] != nil {
			colored++
			need = max(need, NeedBlocks(langs[k]))
			size += len(codes[k])
		}
	}
	if !Parallel(p, colored, need, size) {
		return nil
	}
	// The blocks to highlight are shared among as many goroutines as there are CPUs (including the calling
	// goroutine).
	outs := make([][]byte, n)
	var next atomic.Int32
	work := func() {
		for {
			k := int(next.Add(1)) - 1
			if k >= n {
				return
			}
			if langs[k] == nil {
				continue
			}
			outs[k] = Highlight(names[k], codes[k])
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
	return outs
}

// Parallel returns true if highlighting should be parallel, when there are n code blocks to highlight, the
// number of blocks needed to go parallel is need (the largest the languages' ParallelHint calls for), and their
// contents total size bytes.
func Parallel(p Mode, n, need, size int) bool {
	switch p {
	case Always:
		return true
	case Never:
		return false
	}
	return size > MinBytes && n >= need
}

// NeedBlocks returns the number of code blocks needed to highlight language l in parallel with Auto: 3 if its
// ParallelHint is HintHeavy, 4 otherwise (HintLight or HintDefault).
func NeedBlocks(l *fh.Language) int {
	if l.ParallelHint == fh.HintHeavy {
		return 3
	}
	return 4
}

// Reason says why a code block with language name name is not highlighted: "" when it is highlighted (then
// l is its language), otherwise "no-language", "plain", "not-built-in" or "unsupported". For "not-built-in",
// pkg is the package to import.
func Reason(name string) (reason, pkg string, l *fh.Language) {
	if name == "" {
		return "no-language", "", nil
	}
	if l := fh.LookupRegistered(name); l != nil {
		return "", "", l
	}
	if fh.IsPlainText(name) {
		return "plain", "", nil
	}
	if c := fh.LookupSupported(name); c != nil {
		return "not-built-in", c.Package, nil
	}
	return "unsupported", "", nil
}

// Split splits the HTML of a highlighted code block, <pre …><code …>, the code, then </code></pre>, into its
// three parts. The code itself has no <code.
func Split(h string) (open, code, close string) {
	i := strings.Index(h, "<code")
	i += strings.IndexByte(h[i:], '>') + 1
	j := len(h) - len("</code></pre>")
	return h[:i], h[i:j], h[j:]
}

// Highlight highlights one code block and returns its HTML.
func Highlight(name, code string) []byte {
	var buf bytes.Buffer
	buf.Grow(len(code) * 4)
	_ = fh.Highlight(&buf, name, code) // writing to a bytes.Buffer does not fail
	return buf.Bytes()
}
