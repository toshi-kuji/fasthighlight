// Package python registers Python.
//
//	import _ "github.com/toshi-kuji/fasthighlight/lang/python"
package python

import (
	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/python"
)

// Language is Python. Its name, aliases, and filename patterns are listed in the catalog of the main
// package (fasthighlight.Builtin).
var Language = fh.Builtin("Python")

func init() {
	Language.Lex = func(text string, end int) []fh.Token { return python.Lex(text, end, python.Options{}) }
	Language.Prepare = python.Prepare
	fh.Register(Language)
}
