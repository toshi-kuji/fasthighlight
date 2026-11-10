# The library

This page covers the basics. The full API is on pkg.go.dev:

- [fasthighlight](https://pkg.go.dev/github.com/toshi-kuji/fasthighlight): highlighting code
- [fasthighlight/markdown](https://pkg.go.dev/github.com/toshi-kuji/fasthighlight/markdown): Markdown with goldmark

## Install

```sh
go get github.com/toshi-kuji/fasthighlight
```

It needs Go 1.24 or later. The module has two kinds of packages. One `go get` covers both.

| Package | What it does | Depends on |
| --- | --- | --- |
| `github.com/toshi-kuji/fasthighlight` | highlights code | the standard library only |
| `github.com/toshi-kuji/fasthighlight/markdown` | turns Markdown into HTML with highlighted code blocks | [goldmark](https://github.com/yuin/goldmark) |

Import `markdown` to convert Markdown. To color code only, import `fasthighlight` alone; then goldmark is not compiled into your program.

Either way, also import the languages to color (below).

## Convert Markdown

[`markdown.Converter`](https://pkg.go.dev/github.com/toshi-kuji/fasthighlight/markdown#Converter) is goldmark with GFM and fasthighlight added.

```go
import (
	"github.com/toshi-kuji/fasthighlight/markdown"

	_ "github.com/toshi-kuji/fasthighlight/lang/golang" // register the languages you use (here, Go)
	// _ "github.com/toshi-kuji/fasthighlight/lang/all"  // or all the languages at once
)

err := markdown.NewConverter().Convert(src, w)
```

Do not call `Parse` or `Convert` of one `Converter` from several goroutines at the same time. `CodeBlocks` and `Render` may be called from several goroutines at the same time, on documents already parsed.

## Add to your own goldmark

[`markdown.Extension`](https://pkg.go.dev/github.com/toshi-kuji/fasthighlight/markdown#Extension) is a goldmark extension. Add it to your own configuration:

```go
md := goldmark.New(goldmark.WithExtensions(extension.GFM, &markdown.Extension{}))
err := md.Convert(src, w)
```

A `goldmark.Markdown` with the extension can be used from several goroutines at the same time.

## Highlight one piece of code

[`fasthighlight.Highlight`](https://pkg.go.dev/github.com/toshi-kuji/fasthighlight#Highlight) turns one piece of code into the HTML of one code block:

```go
import (
	"github.com/toshi-kuji/fasthighlight"
	_ "github.com/toshi-kuji/fasthighlight/lang/golang"
)

err := fasthighlight.Highlight(w, "go", code)
```

A language that is not registered is escaped without colors. `Highlight` may be called from several goroutines at the same time, so you can highlight many pieces of code in parallel yourself.

### Work with the tokens

To build your own HTML or look at the code's structure, get the tokens with [`Language.Tokenise`](https://pkg.go.dev/github.com/toshi-kuji/fasthighlight#Language.Tokenise). Each token has a type (`Keyword`, `NameFunction`, `LiteralString`, …) and its text; joined together, the tokens are the code (with newlines normalized to `\n`). [`WriteHTML`](https://pkg.go.dev/github.com/toshi-kuji/fasthighlight#WriteHTML) turns tokens into the same HTML as `Highlight`.

```go
l := fasthighlight.LookupRegistered("go") // nil if the language is not registered
for _, t := range l.Tokenise(code) {
	fmt.Println(t.Type, t.Value)
}
```

## Choose the languages

Languages are registered by importing their packages, as with `database/sql` drivers.

```go
import _ "github.com/toshi-kuji/fasthighlight/lang/all" // all the languages
```

```go
import (
	_ "github.com/toshi-kuji/fasthighlight/lang/golang"
	_ "github.com/toshi-kuji/fasthighlight/lang/python"
)
```

Only the languages you import are compiled into your program, and each language you import is compiled in even if no document uses it. Keyword tables and the like are built when a language is first used, not at import.

As of v0.2.0, in a small program on Linux amd64 built with Go 1.25.0 and `-ldflags "-s -w"`, calling only `Highlight` took about 1.63 MB; importing the Go language added about 0.07 MB, and importing all the languages (`lang/all`) about 0.77 MB. Using `markdown.Converter` (with goldmark) added about 1.04 MB more.

## Find code blocks that will not be colored

To list them before converting (for example, to warn about a language you did not import), parse the documents, scan them, and then write them out. Each document is parsed once.

```go
c := markdown.NewConverter()
doc := c.Parse("guide.md", src)
r := markdown.Scan(doc) // several documents: markdown.Scan(doc1, doc2, ...)
for _, b := range r.NotBuiltIn {
	fmt.Printf("%s:%d: %s needs %s\n", b.Document, b.Line, b.Lang, b.Package)
}
err := c.Render(w, doc)
```

[`markdown.Scan`](https://pkg.go.dev/github.com/toshi-kuji/fasthighlight/markdown#Scan) sorts the blocks that will not be colored into three groups:

| Field | Meaning |
| --- | --- |
| `NotBuiltIn` | fasthighlight supports the language, but its package is not imported. `Package` is the package to import |
| `Unsupported` | fasthighlight does not support the language |
| `NoLanguage` | the block has no language name |

Blocks marked as plain text (`text`, `plaintext`, `plain` or `no-highlight`) are not colored either, but are in none of the groups. [`fasthighlight.IsPlainText`](https://pkg.go.dev/github.com/toshi-kuji/fasthighlight#IsPlainText) tells such names.

To check a single language name, use [`fasthighlight.LookupSupported`](https://pkg.go.dev/github.com/toshi-kuji/fasthighlight#LookupSupported). It searches all the supported languages, imported or not, and the result's `Package` is the package to import. [`LookupRegistered`](https://pkg.go.dev/github.com/toshi-kuji/fasthighlight#LookupRegistered) searches only the languages that can color now.

Language names are matched as in Chroma: case does not matter, and a name that is not a language name or alias is tried as a file extension (`mts`) or file name (`app.ts`). Only the first word after the ``` is used, up to a space or tab (`ts title="app.ts"` is `ts`), with backslash escapes and entity references resolved as in CommonMark (`c\+\+` and `c&#43;&#43;` are `c++`). `markdown.LanguageName` returns it for a block.

## Only the code blocks

If you convert the Markdown yourself and need only the colored code, parse the document and call [`CodeBlocks`](https://pkg.go.dev/github.com/toshi-kuji/fasthighlight/markdown#Converter.CodeBlocks). It returns each fenced code block, highlighted, in order, and writes no other HTML. It is the library side of `fhl --code-blocks`.

```go
c := markdown.NewConverter()
doc := c.Parse("page.md", src)
for _, b := range c.CodeBlocks(doc) {
	// b.Line, b.Lang: where the block is and its language name
	// b.Open + b.Code + b.Close: the same HTML Render writes; b.Code alone goes inside <code>
	// b.Colored, b.Reason: whether it got colors, and if not why (no-language, plain, not-built-in, unsupported)
}
```

The blocks are highlighted in parallel as `Parallelism` decides, as in `Render`.

### With your own goldmark

If you parse with your own goldmark (your own extensions, AST transformers or renderers), wrap its result with [`NewDocument`](https://pkg.go.dev/github.com/toshi-kuji/fasthighlight/markdown#NewDocument) instead of parsing the document again with `Parse`. `CodeBlocks` and `Scan` work on it as on a document from `Parse`, and each document is parsed once.

```go
root := md.Parser().Parse(text.NewReader(src)) // md: your goldmark
c := markdown.NewConverter()
for _, b := range c.CodeBlocks(markdown.NewDocument("page.md", src, root)) {
	// the same blocks as above
}
```

Each block's `Node` is its node in your AST, so your renderer can look a block up by its node (for example, in a `map[*ast.FencedCodeBlock]markdown.HighlightedBlock`) instead of by its position. `CodeBlocks` may be called for several pages in parallel.

The code blocks are the fenced code block nodes (`*ast.FencedCodeBlock`) under the root, in order. A block that your AST transformers turned into another kind of node is not one.

## Parallel highlighting

By default ([`markdown.ParallelAuto`](https://pkg.go.dev/github.com/toshi-kuji/fasthighlight/markdown#Parallelism)), the code blocks of one document are highlighted in parallel only when it pays off:

- the code to color is more than 32 KB in total, and
- there are at least as many blocks as the languages call for: 3 for a language whose `ParallelHint` is `HintHeavy` (the programming languages, including CSS), 4 otherwise (such as JSON, YAML, HTML and Markdown).

Below that, starting goroutines costs more than it saves. Set `Parallelism` on `Converter` or `Extension` to `ParallelNever` or `ParallelAlways` to override this.

If you register a language of your own, set [`ParallelHint`](https://pkg.go.dev/github.com/toshi-kuji/fasthighlight#ParallelHint) by the same rule: `HintHeavy` if highlighting it takes much work (a programming language), `HintLight` otherwise. When unsure, leave it unspecified (`HintDefault`, treated like `HintLight`).

## The HTML

````markdown
```go
fmt.Println("hi")
```
````

becomes

```html
<pre class="highlight" data-lang="go"><code class="language-go"><span class="nx">fmt</span><span class="p">.</span><span class="nf">Println</span><span class="p">(</span><span class="s">&#34;hi&#34;</span><span class="p">)</span>
</code></pre>
```

- `data-lang` and the `language-` class of `<code>` hold the language name as written in the Markdown (the class is the one CommonMark recommends). Both are missing when there is no language name.
- The output is the same for the same input, byte for byte.
- The library writes no CSS. Use the `highlight.css` that `fhl` writes, or your own; the classes are listed in [Themes and CSS](themes.html).
- The token classes are the same as those of Chroma's HTML formatter with `WithClasses(true)`, but the HTML is smaller: lines are not wrapped in `<span class="line"><span class="cl">` and whitespace is not wrapped in `<span class="w">`. See [Chroma compatibility](compatibility.html).
