<div class="hero">

<p align="center"><img src="assets/logo.webp" alt="fasthighlight" width="480"></p>

# fasthighlight

Fast syntax highlighting written in Go, without regular expressions. Add it to goldmark to highlight code blocks in Markdown, or use the command `fhl` from a terminal, a script or a program in another language to turn Markdown into HTML, up to a basic static site.

```sh
go install github.com/toshi-kuji/fasthighlight/cmd/fhl@latest
```

<!-- Add after the first release: download a single binary from the releases page (TODO-releases) -->

[GitHub](https://github.com/toshi-kuji/fasthighlight) · [Contents](contents.html) · [Demo](themes.html#samples)

</div>

<div class="two-ways">

## Two ways to use it

| | Library `fasthighlight` | Command `fhl` |
| --- | --- | --- |
| For | Go developers building an app | Developers using other programming languages, AI agents, anyone with a terminal |
| How | Call it from your program | Run a command |
| Main job | Build HTML conversion into your app | Turn Markdown and code files into HTML |
| Good at | Web servers, blogs, documentation services | Static site builds, converting many files at once, automation |

The library is a part you build into Go apps. `fhl` is a document tool you run as a command: from a terminal, a script or a program in another language. Both run the same engine.

### The command

```sh
fhl README.md          # README.html and highlight.css next to README.md
fhl -o dist/ docs/     # every .md in docs/, into dist/
```

[More about `fhl`](cli.html). All flags and examples: `fhl --help`.

### The library

```go
import (
	"github.com/toshi-kuji/fasthighlight/markdown"

	_ "github.com/toshi-kuji/fasthighlight/lang/golang" // register the languages you use (here, Go)
	// _ "github.com/toshi-kuji/fasthighlight/lang/all"  // or all the languages at once
)

err := markdown.NewConverter().Convert(src, w)
```

[More about the library](library.html). API reference: [pkg.go.dev](https://pkg.go.dev/github.com/toshi-kuji/fasthighlight).

</div>

<div class="uses">

## What you can do with it

- **Build a tech blog or documentation site.** In Go, add the library to your app; in any other language, call `fhl`. [How](use-cases.html#static-site-builds)
- **Turn a folder into a basic static site.** `fhl --static-site -o dist/ docs/` makes pages that open as they are, with heading ids, links between pages and one style sheet to add your styles to. This site is built that way, in CI. [How](use-cases.html#a-documentation-site)
- **Convert a lot of Markdown at once.** `fhl -o dist/ docs/` converts a whole folder in one run. [How](use-cases.html#convert-a-folder)
- **Let an AI agent publish documents.** The AI writes Markdown, `fhl` turns it into HTML, and the AI styles it. `--json` gives the result in a form a program can read. [How](cli.html#ai-agents)
- **Check documents in CI.** `fhl --check --strict docs/` fails when a code block has no language name or a language `fhl` does not color. [How](use-cases.html#check-in-ci)
- **Turn source code into colored HTML.** `fhl main.go` [How](use-cases.html#code-files)
- **Make one HTML file that opens as it is.** `fhl --single-html README.md` [How](use-cases.html#single-html)

</div>

<div class="not-for">

## What it is not for

- **Replacing a full site generator.** `fhl --static-site` makes a basic site to build on: the page design, navigation and templates are up to you or your AI agent.
- **Running in the browser.** There is no browser version.
- **Coloring languages it does not support.** Code blocks in other languages are escaped but not colored.
- **Starting `fhl` for every web request.** Starting takes a few milliseconds; in a Go server, use the library.

</div>

<div class="faq">

## Questions

**Is it a library or a command?**
Both. The core is the Go library, and `fhl` is a command built on it (it converts with the library's `markdown.Extension`). They are one module in one repository, with the same version.

**Can I install only the command?**
Yes. `go install github.com/toshi-kuji/fasthighlight/cmd/fhl@latest` builds just `fhl`. Go downloads the whole module (with the library source) to build it.
<!-- After the first release, add: the binary on the releases page is a single file with the library built in; no Go needed. -->

**Does the library bring the command's code into my program?**
No. `go get github.com/toshi-kuji/fasthighlight` downloads the whole module as one zip, but Go compiles only the packages you import. `fhl` is a `main` package, so it cannot be imported.

**Do languages I don't use end up in my program?**
Only the languages you import. `lang/all` registers all of them; `lang/golang` and the others register one each. [Choosing languages](library.html#choose-the-languages)

**Does it work with goldmark?**
Yes. Add `markdown.Extension` to your own goldmark: `goldmark.New(goldmark.WithExtensions(extension.GFM, &markdown.Extension{}))`.

**Then why goldmark at all? Don't they do the same job?**
No. goldmark parses Markdown and writes headings, tables and the rest as HTML. fasthighlight colors only the contents of code blocks; it has no Markdown parser of its own.

**If I only highlight code (`Highlight`), is goldmark compiled in?**
No. The `fasthighlight` package and the language packages do not depend on goldmark; only the `markdown` package does.

**Does `fhl` use goldmark?**
Yes. `fhl` sets up goldmark with GFM and `markdown.Extension` itself, so that `--raw-html` can change how raw HTML is written.

**If I install `fhl`, can I use goldmark too?**
`fhl` has goldmark built in, so it needs nothing else to convert Markdown. It uses goldmark with GFM only; other goldmark extensions (such as footnotes) cannot be turned on from `fhl`. Installing `fhl` does not make goldmark available to your Go programs; for that, `go get` it in your module.

**What happens to code blocks in languages it does not support?**
They are escaped, without colors. `fhl --check` lists where they are, except blocks marked as plain text (`text` and the like), which ask for no colors.

</div>

<div class="speed">

## Speed

Each supported language has its own hand-written tokenizer, which reads the code character by character instead of trying a grammar of regular expressions at each position. This is a choice of speed over the reach of shared grammars; [how it works and what it costs](compatibility.html#why-it-is-fast).

The numbers below come from our own measurements. Where the difference shows, and where it does not: [When the speed matters](use-cases.html#when-the-speed-matters). To try your own documents, run `fhl`: it prints the number of files and the time at the end, for example `fhl: 128 files, 412 code blocks (3 uncolored) in 0.21s`.

### Converting a whole Markdown document

| Implementation | `input.md` (30 KB, 6 code blocks) | `multilang.md` (25 KB, 25 code blocks: one in each supported language, and one plain text) |
| --- | --- | --- |
| fasthighlight | 0.61–0.72 ms | 2.1–2.2 ms |
| Chroma (all languages) | 4.8–4.9 ms | 41.6–44.5 ms |
| Node: markdown-it + highlight.js | 2.8–2.9 ms | 9.8–10.0 ms |
| Node: markdown-it + Shiki | 8.7–9.7 ms | 68.4–71.9 ms |

- **What:** the time to turn a Markdown document into HTML with highlighted code blocks, warm (after initialization), median time per conversion.
- **Input:** [`input.md`](https://github.com/toshi-kuji/fasthighlight/blob/main/testdata/input.md) and [`multilang.md`](https://github.com/toshi-kuji/fasthighlight/blob/main/testdata/multilang.md).
- **Environment:** Nov 1, 2026, a 4-core cloud machine, Go 1.25.0, Node 22.23.4.
- **Note:** the implementations color code at different levels of detail, so this compares whole conversions, not identical output. How to measure it yourself: [docs/benchmark.md](https://github.com/toshi-kuji/fasthighlight/blob/main/docs/benchmark.md).

### Highlighting code only

| | fasthighlight | syntect | giallo |
| --- | --- | --- | --- |
| 21 snippets all three support (warm, total) | about 0.90 ms | about 21.4 ms | about 38.8 ms |
| 27 snippets fasthighlight and giallo support (warm, total) | about 1.19 ms | - | about 48.8 ms |
| Initialization | - | about 1 ms | about 58 ms |
| First use of each language (total) | about 3 ms | about 99 ms | about 764 ms |

- **What:** one code snippet to HTML with classes (highlighting and writing HTML), without Markdown parsing, on one thread. syntect and giallo are highlighting libraries written in Rust.
- **Input:** the 27 code blocks of `multilang.md` and `input.md` (except `text`), about 350 to 1,350 bytes each.
- **Output:** fasthighlight `Highlight`; syntect `ClassedHTMLGenerator` (`ClassStyle::Spaced`); giallo `HtmlRenderer` with `css_class_prefix`. All three write classes.
- **Environment:** Nov 3, 2026, a 4-core cloud machine, Go 1.25.0, Rust 1.97.0 (release build), syntect 5.3.0, giallo 0.5.2. Median of 9 rounds of 200 runs per snippet; the three were run in turn, three times each, in the same session.
- **Note:** the output differs in detail (the number of `<span>`s goes either way depending on the language, and the three color differently). syntect's default grammars have no TypeScript, TOML, Kotlin or Swift. giallo supports more than 220 languages, many more than fasthighlight. Only small snippets were measured. We wrote the measurement ourselves and have not checked that it uses each library the way its authors recommend.

</div>

<div class="langs">

## Supported languages

fasthighlight supports <!-- LANG-COUNT --> languages.

<!-- LANGS -->

Names, aliases and file name patterns are the same as [Chroma](https://github.com/alecthomas/chroma)'s. The highlighting follows Chroma 2.27.0, with [a few deliberate fixes](compatibility.html).

Code blocks marked as plain text (`text`, `plaintext`, `plain` or `no-highlight`, the names of Chroma's plain text lexer) are escaped without colors, and are not reported as uncolored, since that is what they ask for.

</div>

<div class="pages">

## Read more

- [The command `fhl`](cli.html): install, inputs and outputs, flags, AI agents. Reference: `fhl --help`.
- [The library](library.html): the two packages, converting, highlighting, languages. Reference: [pkg.go.dev](https://pkg.go.dev/github.com/toshi-kuji/fasthighlight).
- [Use cases](use-cases.html): what to run for common jobs.
- [Themes and CSS](themes.html): the color themes, CSS variables and token classes.
- [Chroma compatibility](compatibility.html): what matches Chroma, what was changed, and why it is fast.

This site is built from Markdown with `fhl` in CI: on every push to `main`, [a GitHub Actions workflow](https://github.com/toshi-kuji/fasthighlight/blob/main/.github/workflows/pages.yml) runs [the build script](https://github.com/toshi-kuji/fasthighlight/blob/main/website/build.sh) and publishes the result on GitHub Pages. Its code blocks are highlighted by fasthighlight. See [the source](https://github.com/toshi-kuji/fasthighlight/tree/main/website) on GitHub.

</div>
