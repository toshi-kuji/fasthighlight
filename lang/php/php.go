// Package php registers PHP.
//
//	import _ "github.com/toshi-kuji/fasthighlight/lang/php"
package php

import (
	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/php"
)

// Language is PHP. Its name, aliases, and filename patterns are listed in the catalog of the main
// package (fasthighlight.Builtin).
var Language = fh.Builtin("PHP")

func init() {
	Language.Lex = func(text string, end int) []fh.Token { return php.Lex(text, end, php.Options{}) }
	Language.Prepare = php.Prepare
	fh.Register(Language)
}
