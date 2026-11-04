// Package r registers R.
//
//	import _ "github.com/toshi-kuji/fasthighlight/lang/r"
package r

import (
	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/r"
)

// Language is R. Its name, aliases, and filename patterns are listed in the catalog of the main
// package (fasthighlight.Builtin).
var Language = fh.Builtin("R")

func init() {
	Language.Lex = func(text string, end int) []fh.Token { return r.Lex(text, end, r.Options{}) }
	Language.Prepare = r.Prepare
	fh.Register(Language)
}
