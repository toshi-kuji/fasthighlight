// Package ini registers INI.
//
//	import _ "github.com/toshi-kuji/fasthighlight/lang/ini"
package ini

import (
	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/ini"
)

// Language is INI. Its name, aliases, and filename patterns are listed in the catalog of the main
// package (fasthighlight.Builtin).
var Language = fh.Builtin("INI")

func init() {
	Language.Lex = ini.Lex
	fh.Register(Language)
}
