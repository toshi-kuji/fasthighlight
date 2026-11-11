// Package rust registers Rust.
//
//	import _ "github.com/toshi-kuji/fasthighlight/lang/rust"
package rust

import (
	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/rust"
)

// Language is Rust. Its name, aliases, and filename patterns are listed in the catalog of the main
// package (fasthighlight.Builtin).
var Language = fh.Builtin("Rust")

func init() {
	Language.Lex = func(text string, end int) []fh.Token { return rust.Lex(text, end, rust.Options{}) }
	Language.Prepare = rust.Prepare
	fh.Register(Language)
}
