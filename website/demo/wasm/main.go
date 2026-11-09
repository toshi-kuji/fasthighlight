//go:build js && wasm

// Command wasm is the WebAssembly side of the demo page (website/demo). It registers
// fhlHighlight(lang, code), which returns the HTML of one highlighted code block. website/build.sh
// builds it with GOOS=js GOARCH=wasm; fasthighlight does not ship it.
package main

import (
	"bytes"
	"syscall/js"

	"github.com/toshi-kuji/fasthighlight"
	_ "github.com/toshi-kuji/fasthighlight/lang/all"
)

func main() {
	js.Global().Set("fhlHighlight", js.FuncOf(func(this js.Value, a []js.Value) any {
		var b bytes.Buffer
		_ = fasthighlight.Highlight(&b, a[0].String(), a[1].String()) // writing to a bytes.Buffer does not fail
		return b.String()
	}))
	select {}
}
