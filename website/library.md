[fasthighlight](index.html) · [Contents](contents.html)

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

Do not use one `Converter` from several goroutines at the same time.

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

A language that is not registered is escaped without colors. To work with the tokens yourself, use [`Language.Tokenise`](https://pkg.go.dev/github.com/toshi-kuji/fasthighlight#Language.Tokenise) and [`WriteHTML`](https://pkg.go.dev/github.com/toshi-kuji/fasthighlight#WriteHTML).

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

In a small program on Linux amd64 built with Go 1.25.0 and `-ldflags "-s -w"`, calling only `Highlight` took about 1.63 MB; importing the Go language added about 0.07 MB, and importing all the languages (`lang/all`) about 0.77 MB. Using `markdown.Converter` (with goldmark) added about 1.04 MB more.

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

Language names are matched as in Chroma: case does not matter, and a name that is not a language name or alias is tried as a file extension (`mts`) or file name (`app.ts`). Only the first word after the ``` is used (`ts title="app.ts"` is `ts`).

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
<pre class="highlight" data-lang="go"><code class="language-go"><span class="line"><span class="cl"><span class="nx">fmt</span><span class="p">.</span><span class="nf">Println</span><span class="p">(</span><span class="s">&#34;hi&#34;</span><span class="p">)</span><span class="w">
</span></span></span></code></pre>
```

- `data-lang` and the `language-` class of `<code>` hold the language name as written in the Markdown (the class is the one CommonMark recommends). Both are missing when there is no language name.
- The output is the same for the same input, byte for byte.
- The library writes no CSS. Use the `highlight.css` that `fhl` writes, or your own; the classes are listed in [Themes and CSS](themes.html).
- Apart from the outer class name (`highlight`, where Chroma uses `chroma`), `data-lang` and the `language-` class, the HTML for a given token sequence is the same as Chroma's HTML formatter with `WithClasses(true)`. See [Chroma compatibility](compatibility.html).
