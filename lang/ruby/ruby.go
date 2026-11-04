// Package ruby registers Ruby.
//
//	import _ "github.com/toshi-kuji/fasthighlight/lang/ruby"
package ruby

import (
	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/ruby"
)

// Language is Ruby. Its name, aliases, and filename patterns are listed in the catalog of the main
// package (fasthighlight.Builtin).
var Language = fh.Builtin("Ruby")

func init() {
	Language.Lex = func(text string, end int) []fh.Token { return ruby.Lex(text, end, ruby.Options{}) }
	Language.Prepare = ruby.Prepare
	fh.Register(Language)
}
