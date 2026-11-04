// Package fasthighlight highlights code into HTML with language-specific tokenizers that do not use
// regular expressions. The highlighting follows the Chroma 2.27.0 language definitions as a reference,
// and fixes Chroma's mistakes.
//
// This package does the highlighting only and has no dependencies outside the standard library. To turn
// Markdown into HTML with highlighted code blocks, use the markdown package
// (github.com/toshi-kuji/fasthighlight/markdown), which adds this package to goldmark.
//
// The HTML it writes has the form <pre class="highlight"><code>…</code></pre>, and tokens are wrapped in
// <span>s with short class names such as k (keyword), s (string), and nf (function name) (the naming
// shared by Pygments, Chroma, and Rouge). Colors are applied with CSS. If a language name is given,
// <pre> gets data-lang="language name" (even for a language that is not registered).
//
// # Usage
//
// Register the languages to highlight by importing the language packages. Use lang/all for all languages;
// import only the ones you need to keep your program small.
//
//	import _ "github.com/toshi-kuji/fasthighlight/lang/all"
//
// Highlight one piece of code:
//
//	err := fasthighlight.Highlight(w, "go", code)
//
// Code of a language that is not registered is only escaped, without highlighting.
//
// # Languages
//
// LookupRegistered finds a language among the registered ones (those that can be used for highlighting
// now, including languages registered directly with Register). LookupSupported finds a language among the
// languages fasthighlight provides, whether their packages are imported or not, and tells which package
// to import.
//
// When you register a language yourself, set Language.ParallelHint by the same criterion as the
// languages fasthighlight provides: HintHeavy for languages whose highlighting takes much work (the
// programming languages, including CSS), and HintLight or nothing otherwise. The markdown package uses it
// to decide when to highlight a document's code blocks in parallel.
package fasthighlight
