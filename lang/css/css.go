// Package css registers CSS.
//
//	import _ "github.com/toshi-kuji/fasthighlight/lang/css"
package css

import (
	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/css"
)

// Language is CSS. Its name, aliases, and filename patterns are listed in the catalog of the main
// package (fasthighlight.Builtin).
var Language = fh.Builtin("CSS")

func init() {
	Language.Lex = css.Lex
	Language.Prepare = css.Prepare
	fh.Register(Language)
}
