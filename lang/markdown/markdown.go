// Package markdown registers Markdown. It also registers YAML (lang/yaml) to read front matter.
//
//	import _ "github.com/toshi-kuji/fasthighlight/lang/markdown"
//
// Code blocks (```language-name) are highlighted with the registered languages. Code blocks in
// unregistered languages are not highlighted.
package markdown

import (
	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/markdown"
	_ "github.com/toshi-kuji/fasthighlight/lang/yaml" // front matter (Chroma always reads it as YAML)
)

// Language is Markdown. Its name, aliases, and filename patterns are listed in the catalog of the main
// package (fasthighlight.Builtin).
// As in Chroma, the name is the lowercase "markdown".
var Language = fh.Builtin("markdown")

func init() {
	Language.Lex = func(text string, end int) []fh.Token { return markdown.Lex(text, end, markdown.Options{}) }
	fh.Register(Language)
}
