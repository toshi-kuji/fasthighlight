// Package markdown turns Markdown into HTML with highlighted code blocks, by adding fasthighlight to
// goldmark (github.com/yuin/goldmark). The highlighting itself is done by the fasthighlight package;
// programs that only highlight code can use that package alone, without goldmark.
//
// # Usage
//
// Register the languages to highlight by importing the language packages. Use lang/all for all languages.
//
//	import _ "github.com/toshi-kuji/fasthighlight/lang/all"
//
// To add it to your own goldmark configuration, use Extension.
//
//	md := goldmark.New(goldmark.WithExtensions(extension.GFM, &markdown.Extension{}))
//
// If goldmark with GFM and Extension is enough, use Converter.
//
//	err := markdown.NewConverter().Convert(src, w)
//
// Converter can also check which languages documents use before converting them: parse all the
// documents with Parse, check them with Scan, and then write them out with Render. Scan reports the code
// blocks that will not be highlighted, including those of languages fasthighlight supports but whose
// packages are not imported, with the package to import.
//
// If you convert the Markdown yourself and need only the colored code, call CodeBlocks on a parsed
// document: it returns each code block, highlighted, without the rest of the HTML.
//
// A code block of a language that is not registered is only escaped, without highlighting. For the shape
// of the HTML, see the fasthighlight package.
//
// # Parallel highlighting
//
// By default (ParallelAuto), the code blocks of one document are highlighted in parallel only when both
// of the following hold. For small documents, and for few blocks of languages that are light to
// highlight, going parallel would be slower.
//
//   - The total size of the contents of the code blocks to highlight exceeds ParallelMinBytes (32KB).
//   - The number of code blocks to highlight is at least the largest number those languages call for:
//     3 for a language whose fasthighlight.Language.ParallelHint is fasthighlight.HintHeavy, 4 otherwise
//     (HintLight or HintDefault).
//
// The heavy languages are the programming languages (including CSS); the data and document languages
// (JSON, YAML, HTML, Markdown, SQL, and so on) are not specified. These numbers were decided by measuring
// a document in which about 32KB of code in total was split into 3 blocks of the same language: going
// parallel took on average about 0.82 times as long for programming languages (0.61 to 0.99 times), and
// 1.00 to 1.28 times for data and document languages (the same or slower). You can also always run
// sequentially (ParallelNever) or always in parallel (ParallelAlways).
package markdown
