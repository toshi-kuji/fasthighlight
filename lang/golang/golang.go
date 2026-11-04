// Package golang registers Go.
//
//	import _ "github.com/toshi-kuji/fasthighlight/lang/golang"
package golang

import (
	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/golang"
)

// Language is Go. Its name, aliases, and filename patterns are listed in the catalog of the main
// package (fasthighlight.Builtin).
var Language = fh.Builtin("Go")

func init() {
	Language.Lex = func(text string, end int) []fh.Token { return golang.Lex(text, end, golang.Options{}) }
	Language.Prepare = golang.Prepare
	fh.Register(Language)
}
