// Package typescript registers TypeScript.
//
//	import _ "github.com/toshi-kuji/fasthighlight/lang/typescript"
package typescript

import (
	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/typescript"
)

// Language is TypeScript. Its name, aliases, and filename patterns are listed in the catalog of the main
// package (fasthighlight.Builtin).
// Unlike Chroma, it does not handle JSX (<tag …>) (see the "What was changed" section of website/compatibility.md).
var Language = fh.Builtin("TypeScript")

func init() {
	Language.Lex = func(text string, end int) []fh.Token { return typescript.Lex(text, end, typescript.Options{}) }
	Language.Prepare = typescript.Prepare
	fh.Register(Language)
}
