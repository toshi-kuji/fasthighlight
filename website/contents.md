# Contents

## Pages

- [fasthighlight](index.html): what it is, how to start, speed, supported languages
- [The command `fhl`](cli.html): install, inputs and outputs, static sites, overwriting, flags, results, AI agents
- [The library](library.html): the two packages, converting Markdown, highlighting code, choosing languages, parallel highlighting
- [Use cases](use-cases.html): when the speed matters, and what to run for common jobs
- [Themes and CSS](themes.html): color themes, CSS variables, token classes
- [Chroma compatibility](compatibility.html): what matches Chroma, what was changed, why it is fast
- [Demo](demo/): try it in your browser (WebAssembly, built only for the demo)

## Reference

- `fhl --help`: every flag of `fhl`, with examples
- [pkg.go.dev: fasthighlight](https://pkg.go.dev/github.com/toshi-kuji/fasthighlight): the API for highlighting code
- [pkg.go.dev: fasthighlight/markdown](https://pkg.go.dev/github.com/toshi-kuji/fasthighlight/markdown): the API for Markdown with goldmark

## Source

- [GitHub](https://github.com/toshi-kuji/fasthighlight)
- [License (MIT)](https://github.com/toshi-kuji/fasthighlight/blob/main/LICENSE) and [third-party notices](https://github.com/toshi-kuji/fasthighlight/blob/main/THIRD_PARTY_NOTICES.md)

This site is built from Markdown with `fhl` in CI: when `website/`, the Go code or the workflow changes on `main`, [a GitHub Actions workflow](https://github.com/toshi-kuji/fasthighlight/blob/main/.github/workflows/pages.yml) runs [the build script](https://github.com/toshi-kuji/fasthighlight/blob/main/website/build.sh) and publishes the result on GitHub Pages. Its code blocks are highlighted by fasthighlight. See [the source](https://github.com/toshi-kuji/fasthighlight/tree/main/website) on GitHub.
