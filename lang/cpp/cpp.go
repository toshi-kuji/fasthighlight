// Package cpp registers C++.
//
//	import _ "github.com/toshi-kuji/fasthighlight/lang/cpp"
package cpp

import (
	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/c"
)

// Language is C++. Its name, aliases, and filename patterns are listed in the catalog of the main
// package (fasthighlight.Builtin).
var Language = fh.Builtin("C++")

func init() {
	Language.Lex = func(text string, end int) []fh.Token { return c.Lex(text, end, c.Options{CPP: true}) }
	Language.Prepare = c.Prepare
	fh.Register(Language)
}
