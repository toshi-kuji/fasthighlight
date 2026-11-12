// Package markdown turns Markdown into HTML with highlighted code blocks, by adding fasthighlight to
// goldmark v2 (github.com/yuin/goldmark/v2). It is the goldmark v2 version of
// github.com/toshi-kuji/fasthighlight/markdown, which is for goldmark v1: the same API, with goldmark v2's
// types, and the same HTML for the same Markdown. Import only the one for the goldmark you use; the other
// goldmark is then neither downloaded nor built into your program.
//
// The highlighting itself is done by the fasthighlight package; programs that only highlight code can use
// that package alone, without goldmark.
//
// # Usage
//
// Register the languages to highlight by importing the language packages. Use lang/all for all languages.
//
//	import _ "github.com/toshi-kuji/fasthighlight/lang/all"
//
// To add it to your own goldmark configuration, use Extension. goldmark v2 has separate parser and
// renderer extensions; Extension is both, so pass the same value to both.
//
//	ext := &markdown.Extension{}
//	p := parser.New(parser.WithExtensions(extension.NewGFMParser(), ext))
//	r := html.New(html.WithExtensions(extension.NewGFMHTMLRenderer(), ext))
//	err := r.Render(w, src, p.Parse(src))
//
// If goldmark with GFM and Extension is enough, use Converter.
//
//	err := markdown.NewConverter().Convert(src, w)
//
// Converter can also check which languages documents use before converting them: parse all the
// documents with Parse, check them with Scan, and then write them out with Render. If you convert the
// Markdown yourself and need only the colored code, call CodeBlocks on a parsed document. If you parse with
// your own goldmark, wrap its result with NewDocument instead of parsing again with Parse.
//
// A code block of a language that is not registered is only escaped, without highlighting. For the shape
// of the HTML, see the fasthighlight package. When code blocks are highlighted in parallel, see the
// markdown package (for goldmark v1), which works the same way.
package markdown
