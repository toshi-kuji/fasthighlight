// Package http registers HTTP. The message body is highlighted in the language of its Content-Type
// header. JSON, XML, HTML, CSS, JavaScript and INI bodies need no other import; bodies in the other
// languages fasthighlight supports are highlighted when those languages are registered (for example with
// lang/all), and are plain text otherwise.
//
//	import _ "github.com/toshi-kuji/fasthighlight/lang/http"
package http

import (
	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/http"
)

// Language is HTTP. Its name and aliases are listed in the catalog of the main package
// (fasthighlight.Builtin).
var Language = fh.Builtin("HTTP")

func init() {
	Language.Lex = func(text string, end int) []fh.Token { return http.Lex(text, end, http.Options{}) }
	Language.Prepare = http.Prepare
	fh.Register(Language)
}
