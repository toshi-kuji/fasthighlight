// Package csharp registers C#.
//
//	import _ "github.com/toshi-kuji/fasthighlight/lang/csharp"
package csharp

import (
	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/csharp"
)

// Language is C#. Its name, aliases, and filename patterns are listed in the catalog of the main
// package (fasthighlight.Builtin).
var Language = fh.Builtin("C#")

func init() {
	Language.Lex = func(text string, end int) []fh.Token { return csharp.Lex(text, end, csharp.Options{}) }
	Language.Prepare = csharp.Prepare
	fh.Register(Language)
}
