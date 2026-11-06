// Package react registers react (JavaScript with JSX).
//
//	import _ "github.com/toshi-kuji/fasthighlight/lang/react"
package react

import (
	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/js"
)

// Language is react (JavaScript with JSX). Its name, aliases, and filename patterns are listed in the
// catalog of the main package (fasthighlight.Builtin).
// Unlike Chroma, { } inside ${ … } in a template literal is closed by counting depth, and an escape such
// as \n in a template literal is part of the string (see the "What was changed" section of
// website/compatibility.md).
var Language = fh.Builtin("react")

func init() {
	Language.Lex = func(text string, end int) []fh.Token { return js.LexReact(text, end, false) }
	fh.Register(Language)
}
