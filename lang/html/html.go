// Package html registers HTML. Inside <script> and <style>, it highlights JavaScript and CSS
// (there is no need to import lang/javascript or lang/css).
//
//	import _ "github.com/toshi-kuji/fasthighlight/lang/html"
package html

import (
	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/html"
	"github.com/toshi-kuji/fasthighlight/internal/js"
)

// Language is HTML. Its name, aliases, and filename patterns are listed in the catalog of the main
// package (fasthighlight.Builtin).
var Language = fh.Builtin("HTML")

func init() {
	Language.Lex = func(text string, end int) []fh.Token { return html.Lex(text, end, js.Options{}) }
	Language.Prepare = html.Prepare
	fh.Register(Language)
}
