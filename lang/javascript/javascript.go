// Package javascript registers JavaScript.
//
//	import _ "github.com/toshi-kuji/fasthighlight/lang/javascript"
package javascript

import (
	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/js"
)

// Language is JavaScript. Its name, aliases, and filename patterns are listed in the catalog of the main
// package (fasthighlight.Builtin).
// Unlike Chroma, { } inside ${ … } in a template literal is closed by counting depth (see
// the "What was changed" section of website/compatibility.md).
var Language = fh.Builtin("JavaScript")

func init() {
	Language.Lex = func(text string, end int) []fh.Token { return js.Lex(text, end, js.Options{}) }
	fh.Register(Language)
}
