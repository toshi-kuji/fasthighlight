// Package xml registers XML.
//
//	import _ "github.com/toshi-kuji/fasthighlight/lang/xml"
package xml

import (
	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/xml"
)

// Language is XML. Its name, aliases, and filename patterns are listed in the catalog of the main
// package (fasthighlight.Builtin).
var Language = fh.Builtin("XML")

func init() {
	Language.Lex = xml.Lex
	fh.Register(Language)
}
