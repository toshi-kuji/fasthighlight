// Package json registers JSON.
//
//	import _ "github.com/toshi-kuji/fasthighlight/lang/json"
//
// Token types and boundaries reproduce the rules of Chroma 2.27.0's json.xml without regular expressions.
package json

import (
	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/json"
)

// Language is JSON. Its name, aliases, and filename patterns are listed in the catalog of the main
// package (fasthighlight.Builtin).
var Language = fh.Builtin("JSON")

func init() {
	Language.Lex = json.Lex
	fh.Register(Language)
}
