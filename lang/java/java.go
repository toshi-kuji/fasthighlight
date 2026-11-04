// Package java registers Java.
//
//	import _ "github.com/toshi-kuji/fasthighlight/lang/java"
package java

import (
	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/java"
)

// Language is Java. Its name, aliases, and filename patterns are listed in the catalog of the main
// package (fasthighlight.Builtin).
var Language = fh.Builtin("Java")

func init() {
	Language.Lex = func(text string, end int) []fh.Token { return java.Lex(text, end, java.Options{}) }
	Language.Prepare = java.Prepare
	fh.Register(Language)
}
