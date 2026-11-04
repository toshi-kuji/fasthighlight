[fasthighlight](index.html) · [Contents](contents.html)

# Use cases

When the speed matters, and what to run for common jobs. What each flag and function does is on [The command `fhl`](cli.html) and [The library](library.html).

## When the speed matters

fasthighlight speeds up the highlighting of code; the rest of the Markdown is converted by goldmark. So the difference shows in two cases:

- **Many code blocks, or long ones.** API references, tutorials and pages of source code, where most of the document is code. In a document that is mostly text, most of the time is goldmark's, and the difference is small.
- **Converting on each request (server-side rendering).** When pages are made when someone opens them, because users write the Markdown, what a page shows depends on who reads it, or there are too many documents to build in advance. The time of each conversion becomes the server's load and the wait for every reader. See [A web server](#a-web-server).

For anything else, such as converting a README now and then, any highlighter is fast enough. The numbers are under [Speed](index.html#speed) on the top page.

## With the command `fhl`

### Convert a folder

Turn a folder of Markdown into HTML in one run, for example before publishing.

```sh
fhl -o dist/ docs/
```

`dist/` gets the same folder layout as `docs/`, with a `highlight.css` in each folder. In CI, add `--force`: without a terminal, `fhl` stops instead of overwriting.

More: [Outputs](cli.html#outputs), [Overwriting](cli.html#overwriting), `fhl --help`

### Check in CI

Fail a CI job when a code block has no language name, or a language `fhl` does not color.

```sh
fhl --check --strict docs/
```

`--check` writes nothing. With `--strict`, the exit status is 2 if any block is not colored, and each one is listed with its file and line.

More: [Results and exit status](cli.html#results-and-exit-status), `fhl --help`

### A documentation site

Turn a folder of Markdown into a static site, and publish the output folder as it is (for example, on GitHub Pages).

```sh
fhl --static-site --raw-html=unsafe --theme=light -o dist/ docs/
```

Pages get a minimal `<head>`, headings get ids, links to `.md` files point to the pages, and images are copied. Write an `index.md` and your own links between pages; `fhl` adds no navigation and no styles beyond the code colors. To style the site, add your CSS to `dist/highlight.css` after converting. Drop `--raw-html=unsafe` unless you write HTML in the Markdown yourself.

More: [A static site](cli.html#a-static-site) (what to prepare and what to watch out for), `fhl --help`

### Light or dark code colors

Pick the code colors for a site that is light only, dark only, or follows the viewer's device.

```sh
fhl --theme=light docs/              # light colors only
fhl --theme=dark:tokyo-night docs/   # Tokyo Night, dark colors only
fhl --theme=both:github docs/        # GitHub, light or dark by the viewer's device
```

`both` follows the viewer's device setting only. If your site has its own light and dark switch, connect the colors to it yourself.

More: [Themes and CSS](themes.html), `fhl --list-themes`

### Static site builds

Use `fhl` from a static site build in Node, Python, Ruby or any other language, as an external command.

```sh
fhl -o build/html/ content/
```

Run it once for the whole folder rather than once per file: each start takes a few milliseconds. The output is body-only HTML, so put it into your own templates, and load `highlight.css` from them.

More: [The HTML](cli.html#the-html), `fhl --help`

### Single HTML

Make one Markdown file into one HTML file that opens as it is, to send to someone.

```sh
fhl --single-html README.md
```

The CSS is in `<head>`, and nothing else is needed.

More: [The HTML](cli.html#the-html), `fhl --help`

### Code files

Turn a source file into colored HTML.

```sh
fhl main.go
fhl --lang go main.txt   # when the file name does not tell the language
```

The language comes from the file name (`fhl --list-langs` shows the patterns), or from `--lang`.

More: [Inputs](cli.html#inputs), `fhl --help`

### Pipes

Use `fhl` between other commands.

```sh
cat note.md | fhl -o - -
```

Body-only HTML goes to standard output; no CSS is written.

More: [Outputs](cli.html#outputs), `fhl --help`

### AI agents

Have an AI agent convert documents and read the result.

```sh
fhl -o dist/ docs/ --json
```

The result, including code blocks that were not colored and any errors, is JSON on standard output.

More: [AI agents](cli.html#ai-agents), `fhl --help`

## With the library

### Your own goldmark

Add highlighting to your goldmark configuration.

```go
md := goldmark.New(goldmark.WithExtensions(extension.GFM, &markdown.Extension{}))
```

Import the languages too (`lang/all`, or each language).

More: [Add to your own goldmark](library.html#add-to-your-own-goldmark), [pkg.go.dev](https://pkg.go.dev/github.com/toshi-kuji/fasthighlight/markdown#Extension)

### A web server

Convert Markdown on each request.

```go
c := markdown.NewConverter()
err := c.Convert(src, w)
```

Large documents get their code blocks highlighted in parallel automatically. A `Converter` must not be used from several goroutines at the same time; use one per goroutine, or a `goldmark.Markdown` with `markdown.Extension`, which can be shared.

More: [Parallel highlighting](library.html#parallel-highlighting), [pkg.go.dev](https://pkg.go.dev/github.com/toshi-kuji/fasthighlight/markdown#Converter)

### One piece of code

Color a code snippet without Markdown.

```go
err := fasthighlight.Highlight(w, "go", code)
```

Without the `markdown` package, goldmark is not compiled into your program.

More: [Highlight one piece of code](library.html#highlight-one-piece-of-code), [pkg.go.dev](https://pkg.go.dev/github.com/toshi-kuji/fasthighlight#Highlight)

### Warn before converting

Find the code blocks that will not be colored, before converting.

```go
c := markdown.NewConverter()
doc := c.Parse("guide.md", src)
r := markdown.Scan(doc)
// report r.NotBuiltIn, r.Unsupported and r.NoLanguage
err := c.Render(w, doc)
```

For a single language name, `fasthighlight.LookupSupported` tells whether it is supported and which package to import.

More: [Find code blocks that will not be colored](library.html#find-code-blocks-that-will-not-be-colored), [pkg.go.dev](https://pkg.go.dev/github.com/toshi-kuji/fasthighlight/markdown#Scan)

### A smaller program

Keep the program small by importing only the languages you use.

```go
import _ "github.com/toshi-kuji/fasthighlight/lang/golang"
```

`lang/all` imports all the languages.

More: [Choose the languages](library.html#choose-the-languages)
