# Benchmarks

How the speed numbers in the README and on the documentation site were measured, and how to measure them yourself.

## What you need

- Go 1.25 or later (`go 1.25` in `bench/go.mod` and `compat/go.mod`, because Chroma 2.27.0 needs it; if your Go is older, the `go` command fetches a newer toolchain). fasthighlight itself needs Go 1.24 or later.
- To measure the comparison implementations too: Node 22 or later, and npm.
- An input Markdown document. There are three:
  - `testdata/input.md` (the default): about 30 KB, 350 lines, 6 code blocks (2 TypeScript, 1 JavaScript, 3 plain text).
  - `testdata/multilang.md`: about 25 KB, 25 code blocks (one in each of the 24 supported languages, and one plain text).
  - A large document: `scripts/make-large.sh > bench-bin/large.md` concatenates the two above, alternately, 5 times (about 270 KB, 155 code blocks).

## Measure everything at once

```sh
scripts/bench.sh                    # fasthighlight only (automatic, sequential and parallel highlighting)
BASELINES=1 scripts/bench.sh        # the comparison implementations too
scripts/bench.sh path/to/other.md   # another document
```

`bench/chroma-subset` and `bench/gotreesitter` only have TypeScript and JavaScript, so they are measured on the default document only. Shiki loads only the languages used in the document's code blocks.

The binaries and the HTML they write go to `bench-bin/` (not tracked by Git).

- **Cold:** from process start, through one conversion, to exit (`ONCE=1`), 7 times. Reported in seconds.
- **Warm:** after initialization, 20 conversions to warm up, then 300 conversions; the median and p95 per conversion are reported. This is repeated 3 times.
- The outputs of the automatic, sequential and parallel modes are checked to be byte-for-byte identical (`cmp`).
- The number of `<span` elements is reported as a rough measure of how finely each implementation colors code.

## The comparison implementations

`bench/` is a separate Go module, so that goldmark-highlighting and gotreesitter do not become dependencies of fasthighlight.

| Directory | What it is |
| --- | --- |
| `cmd/bench` | fasthighlight (`auto` uses the default `markdown.ParallelAuto`, `seq` highlights sequentially, `par` in parallel) |
| `bench/chroma-subset` | Chroma, loading only the TypeScript and JavaScript lexer definitions (XML) |
| `bench/chroma-all` | goldmark-highlighting with all of Chroma's languages |
| `bench/gotreesitter` | gotreesitter, with only the TypeScript and JavaScript grammars (selected with build tags) |
| `bench/node` | markdown-it with highlight.js (`hljs`), or with Shiki (`shiki`) |
| `bench/chroma-init` | the initialization cost of all of Chroma's languages |
| `bench/chroma-subset-prof` | a `pprof` version of `bench/chroma-subset` |

The versions of the Node dependencies are pinned in `bench/node/package.json` (markdown-it 15.0.3, highlight.js 11.12.0, shiki 4.5.0).

To see Chroma's initialization cost:

```sh
cd bench && go build -o ../bench-bin/chroma-init ./chroma-init
GODEBUG=inittrace=1 ../bench-bin/chroma-init 2>&1 | grep chroma
```

## Go benchmarks

The speed of `markdown.Converter.Convert`, and of `Parse` + `Scan` + `Render` (checking the languages before converting):

```sh
go test -run='^$' -bench=. -benchmem ./markdown
```

The speed of tokenizing only, compared with Chroma on the same code (the code blocks of `testdata/multilang.md`), without Markdown parsing or HTML output:

```sh
cd compat && go test -run='^$' -bench=BenchmarkTokenise -benchtime=300ms -count=3 .
```

## Where the time goes (pprof)

```sh
go build -o bench-bin/prof ./cmd/prof
./bench-bin/prof                                # converts testdata/input.md 10000 times and writes cpu.prof
./bench-bin/prof testdata/multilang.md 3000     # another document and count
go tool pprof -top -cum -nodecount=500 bench-bin/prof cpu.prof
go tool pprof -top -cum -focus='fasthighlight\.Highlight$' bench-bin/prof cpu.prof     # highlighting code
go tool pprof -top -cum -focus='parser\.\(\*parser\)\.Parse$' bench-bin/prof cpu.prof  # parsing Markdown
```

## Notes

- Measure the implementations you compare one after another, in the same session: on a cloud machine, the times can drift by a few milliseconds over time.
- Write down the environment with the results (the number of cores, the Go and Node versions, and so on).
