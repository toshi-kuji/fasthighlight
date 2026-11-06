// Package bash registers Bash (sh, zsh, and other shells).
//
//	import _ "github.com/toshi-kuji/fasthighlight/lang/bash"
//
// Token types and boundaries reproduce the rules of Chroma 2.27.0's bash.xml without regular expressions.
package bash

import (
	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/bash"
)

// Language is Bash. Its name, aliases, and filename patterns are listed in the catalog of the main
// package (fasthighlight.Builtin).
var Language = fh.Builtin("Bash")

func init() {
	Language.Lex = bash.Lex
	fh.Register(Language)
}
