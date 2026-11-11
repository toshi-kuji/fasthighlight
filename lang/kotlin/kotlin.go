// Package kotlin registers Kotlin.
//
//	import _ "github.com/toshi-kuji/fasthighlight/lang/kotlin"
package kotlin

import (
	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/kotlin"
)

// Language is Kotlin. Its name, aliases, and filename patterns are listed in the catalog of the main
// package (fasthighlight.Builtin).
var Language = fh.Builtin("Kotlin")

func init() {
	Language.Lex = func(text string, end int) []fh.Token { return kotlin.Lex(text, end, kotlin.Options{}) }
	Language.Prepare = kotlin.Prepare
	fh.Register(Language)
}
