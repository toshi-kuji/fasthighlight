[fasthighlight](index.html) · [Contents](contents.html)

# The command `fhl`

`fhl` turns Markdown (GFM) and code files into HTML with highlighted code blocks. It writes body-only HTML and a `highlight.css` with the code colors, so that you or an AI agent can put the HTML into your own page and style it.

This page covers the basics. All flags and examples: run `fhl --help`.

## Install

```sh
go install github.com/toshi-kuji/fasthighlight/cmd/fhl@latest
```

<!-- Add after the first release: a single binary for Linux, macOS and Windows (amd64, arm64) on the releases page (TODO-releases). No dependencies. -->

All the supported languages are built in. `fhl --version` prints the version; `fhl` and the library share one version.

## Use

```sh
fhl README.md                      # README.html and highlight.css next to README.md
fhl docs/                          # an .html next to each .md in docs/, and a highlight.css per folder
fhl -o dist/ docs/                 # write into dist/, keeping the folder layout
fhl --single-html README.md        # one standalone HTML file with the CSS in <head>
fhl -o - README.md                 # body-only HTML to standard output
cat note.md | fhl -o note.html -   # read standard input
fhl main.go                        # a code file becomes one highlighted code block
```

Flags can go before, after or between the inputs (`fhl docs/ --json` works). Everything after `--` is an input.

## Inputs

| Input | Handling |
| --- | --- |
| `.md`, `.markdown`, `.mkd` | converted as Markdown |
| other files | converted as one code block. The language comes from `--lang`, or from the file name (`*.go` is Go); otherwise it is an error |
| folders | every Markdown file inside, including subfolders. Names starting with `.` (such as `.git`) are skipped |
| `-` | standard input, as Markdown (or as code with `--lang`). Needs `-o` |

## Outputs

The HTML file is named after the input, with `.html` (`README.md` becomes `README.html`).

| `-o` | Where |
| --- | --- |
| none | next to each input |
| `-o FILE` | that file (one input only) |
| `-o DIR/` | inside that folder, keeping the folder layout of the inputs. A value ending with `/`, or an existing folder, is a folder |
| `-o -` | standard output (one input only). Body-only HTML, no `highlight.css`; with `--single-html`, a standalone HTML |

- `highlight.css` is written once per folder that gets HTML, and shared by the HTML in it. It is not written with `--single-html` or `-o -`. With `--static-site`, it is written once, at the top of the output ([A static site](#a-static-site)).
- The same input always gives the same output, byte for byte.
- The output folder can be inside an input folder (`-o docs/dist/ docs/`): it is skipped when the input folder is read, so earlier output is not converted or copied again.

### The HTML

By default, `fhl` writes only what goes inside `<body>`: no `<!DOCTYPE>`, `<head>` or link to the CSS. Load `highlight.css` from your own page or template.

With `--single-html`, the file opens as it is. It has `<!DOCTYPE html>`, `<meta charset="utf-8">`, a `<title>` (the first heading, or the file name) and the CSS in a `<style>`, and nothing else.

Code blocks look like this:

```html
<pre class="highlight" data-lang="go"><code><span class="line"><span class="cl"><span class="nx">fmt</span>…
```

`data-lang` holds the language name written in the Markdown. It is there for unsupported languages too, and missing when there is no language name. Elements get no `style` attributes, except the column alignment of GFM tables. Token classes are explained in [Themes and CSS](themes.html).

### Raw HTML in Markdown

`--raw-html` chooses what happens to HTML tags written in the Markdown.

| Value | Result | `<b>bold</b>` shows as |
| --- | --- | --- |
| `escape` (default) | written as visible text. HTML blocks are wrapped in a `<pre>` without a class, to keep their lines | the text `<b>bold</b>` |
| `omit` | dropped (`<!-- raw HTML omitted -->`). Text between inline tags stays | bold (not bold) |
| `unsafe` | written as is; `<script>` runs. Only for Markdown you trust | **bold** |

## A static site

`--static-site` turns one folder of Markdown into a basic static site: pages that open in a browser as they are, in the same folder layout, ready to put on any web server or GitHub Pages. It is a starting point: the styles and the page layout are yours to add.

```sh
fhl --static-site -o dist/ docs/
fhl --static-site --theme=light --html-lang ja -o dist/ docs/
```

### What it does

- **Pages.** Each Markdown file becomes a page that starts with `<!DOCTYPE html>`, `<html lang="en">` (change it with `--html-lang`), the charset, the viewport, a `<title>` (the first heading, or the file name) and a link to `highlight.css`.
- **One style sheet.** `highlight.css` is written once, at the top of the output, and every page links to it with a relative path, so the site works in any folder (such as `https://user.github.io/project/`).
- **Heading ids.** Headings get ids, so you can link to sections: `## Results and exit status` becomes `results-and-exit-status`. A second heading with the same text gets `-1` added. A heading with no ASCII letters or digits (for example, only Japanese) becomes `heading`, `heading-1` and so on.
- **Links between pages.** Relative links to Markdown files are rewritten to the pages: `[setup](guide.md#setup)` becomes `guide.html#setup`. Links that start with a scheme (`https:`), `/` or `#` are left alone.
- **Other files.** Images and any other files in the folder are copied to the same place in the output. Files and folders whose names start with `.` are skipped.

### What it does not do

- **No styles beyond the code colors.** Headings, paragraphs and tables look as the browser shows them by default.
- **No navigation, no template.** Pages do not link to each other unless your Markdown does.
- **No options for your own CSS or `<head>` content.** Add your styles to the `highlight.css` at the top of the output after converting: every page already links to it. Converting again writes it anew (after asking, or with `--force`), so add them again each time (this site's [build script](https://github.com/toshi-kuji/fasthighlight/blob/main/website/build.sh) appends its style sheet this way). For other `<head>` content, edit the pages after converting, for example with a short script.

### Before you run it

- **Put everything for the site in one folder.** `--static-site` takes exactly one folder. Files outside it are not copied; a link or image that points outside it will not work on the server.
- **Write an index or contents page.** With no navigation, readers find pages through your links. Name the top page `index.md`, so that it becomes `index.html`.
- **Link to `.md` files, or to `.html` files.** Both work: links to `.md` files are rewritten.
- **Write raw HTML only if you need it, and turn it on.** By default, HTML tags in Markdown are shown as text. For your own trusted pages, add `--raw-html=unsafe`.
- **Choose the code colors for your site.** Without `--theme`, code blocks switch to dark colors when the viewer's device prefers dark (`both:default`). For a light-only site, add `--theme=light`. See [Themes and CSS](themes.html).
- **Keep names apart.** A file that would be copied onto a page or onto `highlight.css` is not copied, and `fhl` reports an error: for example `guide.html` next to `guide.md`, or your own `highlight.css` at the top of the folder.
- **Link to sections by the English heading text.** For headings in other scripts, the ids are `heading`, `heading-1` and so on, which change when you add headings.
- **Use `-o DIR/`.** Without `-o`, pages are written next to the Markdown files, `highlight.css` at the top of the folder, and nothing is copied. Converting again overwrites only after asking (or with `--force`); files whose content did not change are left alone.
- **Check first, if you like.** `fhl --static-site --check docs/` writes nothing and only lists code blocks that will not be colored.

## Overwriting

If an output file already exists:

- On a terminal, `fhl` asks `[y/N/a]` for each file: `y` overwrites, `N` (default) skips it, `a` overwrites the rest too.
- Without a terminal (CI, scripts, AI agents), nobody can answer, so `fhl` stops with an error that mentions `--force`.
- `--force` overwrites without asking.
- A file whose content would not change is left alone, without asking.
- A file that `fhl` cannot read is an error, even with `--force`: `fhl` reads an existing file before overwriting it, and does not overwrite what it cannot see. Make it readable, or remove it.

## Collisions

Before converting, `fhl` compares every input and every file it would write.

- If two inputs would write the same file (`guide.md` and `guide.markdown` both make `guide.html`), none of them is written, and each gets an error. The other inputs are converted.
- If an output would overwrite an input (`fhl -o README.md README.md`), it is an error, even with `--force`.
- Names that point to the same file (links, or names differing only in case) count as the same file.

## Other behavior

- **Front matter:** a block between `---` lines at the top of a document is skipped. Without a closing `---`, it is converted as text.
- **Newlines:** `\n`, `\r\n` and `\r` are all read; the output always uses `\n`.

## Flags you will use most

| Flag | Meaning |
| --- | --- |
| `-o PATH` | where to write (above) |
| `--single-html` | a standalone HTML with the CSS in `<head>` |
| `--static-site` | turn one folder into a static site ([A static site](#a-static-site)) |
| `--html-lang LANG` | the `lang` of `<html>` with `--static-site` (default `en`) |
| `--theme=MODE:THEME` | code colors: `MODE` is `light`, `dark` or `both`, and `THEME` is one of `fhl --list-themes` (default `both:default`; see [Themes and CSS](themes.html)) |
| `--raw-html MODE` | `escape` (default), `omit` or `unsafe` |
| `--lang NAME` | read code files and standard input as this language |
| `--force` | overwrite without asking |
| `--check` | write nothing; only report code blocks that are not colored |
| `--strict` | exit with 2 if a code block is not colored |
| `--json` | print the result as JSON on standard output |
| `--list-langs`, `--list-themes` | print the languages or themes (JSON with `--json`) |

All flags and examples: `fhl --help`.

## Results and exit status

At the end, `fhl` prints one summary line to standard error, plus one line per code block that is not colored:

```text
docs/setup.md:42: code block language "hs" is not supported (see fhl --list-langs)
docs/intro.md:10: code block has no language name
fhl: 128 files, 412 code blocks (3 uncolored) in 0.21s
```

`-q` prints errors only. Errors say what happened and what to do next, in one line:

```text
fhl: README.html already exists; use --force to overwrite it
```

| Exit status | Meaning |
| --- | --- |
| 0 | success (uncolored code blocks and files skipped with `N` are not errors) |
| 1 | an error (a bad flag, an unreadable input, an unwritable output, an overwrite without a terminal, a collision). Other files are still converted |
| 2 | with `--strict`, a code block is not colored |

## AI agents

`fhl` is made to be run by AI agents. To use it in an agent's work:

- Tell the agent to run `fhl --help` first. The help has every flag and example.
- Have it add `--json`, and read the result from standard output instead of the messages on standard error:

  ```sh
  fhl -o dist/ docs/ --json
  ```

  ```json
  {
    "files": [{"input": "docs/intro.md", "output": "dist/intro.html", "blocks": 5, "uncolored": 1}],
    "styles": ["dist/highlight.css"],
    "skipped": [],
    "uncolored": [{"file": "docs/intro.md", "line": 10, "lang": "", "reason": "no-language"}],
    "errors": [],
    "summary": {"files": 1, "blocks": 5, "uncolored": 1, "skipped": 0, "errors": 0, "seconds": 0.01}
  }
  ```

  `reason` is `unsupported` or `no-language`.
- Without a terminal, `fhl` never waits for an answer: it stops with an error instead of overwriting. Add `--force` when overwriting is intended.
- The output is body-only HTML and a `highlight.css` whose colors are CSS variables, each class with a comment that says what it is. An agent can read the CSS and style the page around it.
- Use the exit status: 0, 1 or 2 (above).
