// Package swift registers Swift.
//
//	import _ "github.com/toshi-kuji/fasthighlight/lang/swift"
package swift

import (
	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/swift"
)

// Language is Swift. Its name, aliases, and filename patterns are listed in the catalog of the main
// package (fasthighlight.Builtin).
var Language = fh.Builtin("Swift")

func init() {
	Language.Lex = func(text string, end int) []fh.Token { return swift.Lex(text, end, swift.Options{}) }
	Language.Prepare = swift.Prepare
	fh.Register(Language)
}
