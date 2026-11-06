# AGENTS.md

fasthighlight is a Go library that makes syntax highlighting in Markdown conversion fast, with a hand-written tokenizer per language that uses no regular expressions. HTML class names are generic, not product names (the outer frame is `highlight`; tokens use short names such as `k`, `s` and `nf`). The highlighting follows Chroma, and fixes Chroma's mistakes.

- What the command and the library do is described on the documentation site (`website/`): `cli.md`, `library.md`, `themes.md`, and `compatibility.md` for compatibility with Chroma and the intended differences.
- How to measure speed is in [docs/benchmark.md](docs/benchmark.md).

## Commands

```sh
go test -race -count=1 ./...   # tests
go vet ./...                   # checks
gofmt -l .                     # should print nothing

# Comparison with Chroma (compat/ is a separate module)
(cd compat && go vet ./... && go test -race -count=1 ./...)

# Fuzz tests (one at a time; inputs they find go to testdata/fuzz/ and run with the normal go test too)
go test -run='^$' -fuzz=FuzzTokenise -fuzztime=1m .
go test -run='^$' -fuzz=FuzzConvert -fuzztime=1m ./markdown
(cd compat && go test -run='^$' -fuzz=FuzzHTMLMatchesChroma -fuzztime=1m .)
(cd compat && go test -run='^$' -fuzz=FuzzTokensMatchChroma -fuzztime=1m .)   # same tokens as Chroma for any input

# Comparison implementations (bench/ is a separate module)
cd bench
go vet ./...
go build -o /dev/null ./chroma-all ./chroma-subset ./chroma-subset-prof ./chroma-init
go build -o /dev/null -tags 'grammar_subset grammar_subset_typescript grammar_subset_javascript' ./gotreesitter

# Compare tokenizing only with Chroma
(cd compat && go test -run='^$' -bench=BenchmarkTokenise .)

# Distribution archives of fhl (.github/workflows/release.yml builds them and attaches them to the release, when a tag vX.Y.Z
# is pushed, or when it is run by hand on main with a version vX.Y.Z, which also makes the tag)
scripts/release.sh dev /tmp/dist

# The documentation site (converts the Markdown in website/ with fhl --static-site; the output defaults to _site/)
website/build.sh

# Benchmarks (procedure in docs/benchmark.md)
scripts/bench.sh
BASELINES=1 scripts/bench.sh   # also measure the comparison implementations
```

CI (`.github/workflows/ci.yml`) runs the tests, the checks, the comparison with Chroma, short fuzz tests (20 seconds each), the build of the comparison implementations, and a check that the distribution archives of fhl can be built. It runs only when Go files, `go.mod` and `go.sum`, the `testdata/` of each module, the XML embedded in `bench/`, `scripts/release.sh`, or the workflows themselves change (an allowlist); documentation-only changes do not run it. When CI starts reading a new file or folder, add it to the `paths` of `ci.yml` (both `push` and `pull_request`). `.github/workflows/pages.yml` builds the documentation site with `website/build.sh` and publishes it on GitHub Pages whenever `main` changes.

## Layout

| Path | Role |
| --- | --- |
| root | The core package `fasthighlight`. It only highlights, and depends on nothing outside the standard library. `language.go` (registering and looking up languages, `ParallelHint`), `catalog.go` (names and so on of all supported languages), `token.go` (token types and class names), `html.go` (HTML output and `Highlight`) |
| `markdown` | The package that works with goldmark: `extension.go` (the goldmark extension), `convert.go` (`Converter`, with GFM and the extension), `scan.go` (collecting language names before converting). Deciding when to highlight in parallel is here too |
| `lang/<language>` | One package per language. Importing it registers the language. `lang/all` registers all of them |
| `internal/c` | The tokenizer for C and C++ (`Options.CPP` selects which) |
| `internal/python`, `internal/java`, `internal/php`, `internal/swift`, `internal/r`, `internal/golang`, `internal/ruby`, `internal/markdown`, `internal/typescript` | Tokenizers of languages that have switches (`Options`) for the intended differences from Chroma. `lang/<language>` only registers them |
| `internal/js`, `internal/css`, `internal/html` | Tokenizers of languages embedded in other languages (and HTML). `LexInto` appends their tokens after those of the outer language (`lexkit.Tokens.Sub`) |
| `internal/lexkit` | Shared parts of the tokenizers: the same character classes as the regular expressions Chroma uses (regexp2: `\s`, `\d`, `\w`, `\b`), and collecting tokens |
| `compat/` | Tests that compare with Chroma (tokens, HTML, whole documents). A separate Go module, so that Chroma is not a dependency of the core |
| `internal/testcases` | Inputs shared by the core tests and `compat/` (code blocks of the documents, edge cases) |
| `internal/scantest` | Tests of `markdown.Scan`. It imports only some languages, so "supported but not imported" can be checked with real languages |
| `cmd/fhl` | The command `fhl` (Markdown to HTML; `fhl --help` and `website/cli.md`) |
| `cmd/bench`, `cmd/prof` | Benchmarks and `pprof` |
| `bench/` | Comparison implementations (Chroma, gotreesitter, Node). A separate Go module, so that they are not dependencies of the core |
| `testdata/` | Documents used by the benchmarks and tests (`input.md`, `multilang.md`), and the expected HTML of `input.md` (`input.html`), made with Chroma's formatter with the outer class name changed to `highlight` |
| `website/` | The documentation site: the pages in Markdown, `style.css`, `build.sh`, the icon (`favicon.png`, cut from the logo), and the Google Search Console verification file (`google*.html`, copied as it is; do not edit or remove it) |
| `docs/` | `benchmark.md`: how to run the benchmarks |

## Rules

- **Dependencies:** The core (root) and the language packages (`lang/...` and the tokenizers in `internal/`) depend on nothing outside the standard library. Only the `markdown` package and `cmd/fhl` may use goldmark, and goldmark is the only external dependency of the module. `TestNoGoldmarkInHighlighting` (`deps_test.go`) checks that the core and `lang/...` do not depend on goldmark. Tests that compare with Chroma go in `compat/`, and comparison implementations in `bench/`.
- **Compatibility with Chroma:** Chroma is a reference, not something to reproduce exactly. Token types and boundaries follow the rules of Chroma 2.27.0's language definitions (XML), but Chroma's mistakes and behavior that gets in the way of readers are fixed. When the result differs from Chroma on purpose, add it to the "What was changed" table in `website/compatibility.md`. The HTML output is byte-for-byte the same as Chroma's `formatters/html` (`WithClasses(true)`), except for the outer class name (`highlight` here, `chroma` in Chroma) and `data-lang` with the code block's language name (which Chroma does not have) (`chromaHTML` in `compat/` replaces the class name before comparing, and adds `data-lang` when comparing whole documents). Class names never use product names.
- **The tokenizer contract:** Joining all tokens gives the input with newlines normalized (`\r\n` and `\r` turned into `\n`). For languages that append a newline at the end, the appended newline may be part of the last token (see `Lex` and `Tokenise` in `language.go`). Never produce empty tokens, or adjacent tokens of the same type (except when the previous token is `fh.CoalesceLimit` bytes or longer). `FuzzTokenise` checks this.
- **Parallel highlighting is automatic:** By default (`markdown.ParallelAuto`), the code blocks of a document are highlighted in parallel only when the code to highlight is more than 32 KB in total and there are at least as many blocks as the languages call for: 3 when the language's `Language.ParallelHint` is `HintHeavy`, 4 otherwise (`HintLight`, or `HintDefault` when unspecified) (`needBlocks` in `markdown`). Languages whose highlighting takes much work (now the programming languages, including CSS) are `HintHeavy`; data and document languages are left unspecified. When adding a language, set its `ParallelHint` in `catalog.go` by this rule. For small documents, starting goroutines costs more than it saves.
- **Measuring:** Measure the implementations you compare one after another, in the same session. Write down the environment (number of cores, Go version and so on) with the results.
- **Licenses:** When adding files copied or generated from Chroma (its XML lexer rules, word lists and so on), or anything else taken from another project (such as the colors of a theme), update `THIRD_PARTY_NOTICES.md`.
- **Inputs found by fuzzing:** Commit the files placed in `testdata/fuzz/` (and `compat/testdata/fuzz/`) and never delete them; they are regression tests.
- **Chroma's regular expression timeout:** Chroma gives each regular expression match 250 ms; on timeout, it treats the rule as not matching and goes on to the next rule. When the CPU is busy, Chroma's result can change. Do not run comparisons such as `FuzzTokensMatchChroma` together with other heavy work. If a difference does not reproduce when the input is run alone, suspect the timeout. Long inputs with much backtracking (for example, a long word against bash's heredoc rule) time out even on an idle CPU, so `FuzzTokensMatchChroma` and `TestRandomPiecesMatchChroma` skip differences, instead of failing, when Chroma took 250 ms or more (`chromaTokensTimed`).
- **Adding a language:** Add it to `catalog.go` with the name, aliases, file name patterns and `EnsureNewline` of Chroma's language definition. In `lang/<language>`, get it with `fh.Builtin("<name>")`, attach `Lex` and `Prepare` in `init`, and `fh.Register` it. Build keyword tables and the like in `Prepare`, not at import time. Add it to `lang/all` too. `TestLanguagesMatchChroma` in `compat/` checks that the list and the lookups are the same as Chroma's, and `TestBuiltinsAreRegistered` in the core checks that the list matches `lang/all`. Add examples of the language to `EdgeCases` in `internal/testcases`. Add it to `exactLanguages` in `compat/fuzz_test.go`, and fix it until `FuzzTokensMatchChroma` finds it the same as Chroma for any input. Add pieces that split the language's rules finely (where backtracking or state changes happen) to `pieces` in `compat/random_test.go` (`FASTHIGHLIGHT_RANDOM_N=100000` runs more of them; `FASTHIGHLIGHT_RANDOM_LANG=Python` limits them to one language). When Chroma's rules have long word lists (keywords and so on), do not copy them by hand; generate them from Chroma's rules (for example, `compat/cmd/gencsslists` makes `internal/css/lists.go`, `compat/cmd/genpythonlists` makes `internal/python/lists.go`, `compat/cmd/genclists` makes `internal/c/lists.go`, and `compat/cmd/genlists` makes the `lists.go` of Java, C#, Kotlin, Swift, Go, Rust, SQL, R, PHP, Ruby and TypeScript; look words up with `lexkit.WordLists`, or `lexkit.FoldWordLists` when case does not matter). Also add a test that compares every word of the lists with Chroma (for example, `TestCSSWordListsMatchChroma`, `TestPythonWordListsMatchChroma`, `TestCWordListsMatchChroma`, `TestWordListsMatchChroma`).
- **Reproducing Chroma's rules:** Chroma uses regexp2 for regular expressions, so `\s`, `\d`, `\w` and `\b` follow Unicode (use `internal/lexkit`). A character that matches no rule becomes an `Error` token, one character at a time. In a state other than root, if no rule matches a newline, go back to root. `Lex` does not start a new match at or after `end` (the handling of the appended newline; see `Lex` in `language.go`). For rules whose result depends on backtracking (quoted strings and so on), reproduce the backtracking too (`lexkit.Run`). regexp2 reads some things differently from Go's `regexp`: `{,2}` is not a repetition but the literal characters `{,2}`; `[^\W\d]` is the same as `\w` (the `\d` has no effect); and case-insensitive matching follows the characters linked by `unicode.SimpleFold`, not `unicode.ToLower` (`lexkit.FoldEq`). When in doubt, check with regexp2 directly. `Emit` a token for each match of a Chroma rule (one per group for bygroups). Tokens of the same type are not merged once the merged token would be 8192 bytes (`fh.CoalesceLimit`) or more, so emitting in finer pieces changes where the tokens split compared with Chroma.
- **Differences from Chroma:** `TestCompareWithChroma` in `compat/` fails when a difference that is not in `intendedDiffs` appears. To accept a difference, add it to both the "What was changed" table in `website/compatibility.md` and `intendedDiffs`. Implement intended differences with a "behave like Chroma" switch in the tokenizer (for example `js.Options.ChromaBraces`, `typescript.Options.Chroma`, `python.Options.ChromaDocstring`, `c.Options.ChromaRawString`, `java.Options.ChromaIdent`), and compare the switched version with Chroma through `chromaMode` in `compat/fuzz_test.go`. This lets the fuzz tests confirm that the switch is the only difference. The switches exist only for the tests and are not exported. Tokenizers of languages with switches live in `internal/<language>` (keeping `Options` and `Lex` unexported), and `lang/<language>` only registers them.
- **Languages that embed other languages:** The tokenizers of embedded languages live in `internal/<language>`, and append their tokens after the outer ones with `LexInto(x.Sub(…), …)` (as with Chroma's `using`, the merge limit counts on from the outer tokens). Embedded languages do not append a newline at the end.
- **Writing:** Everything in this repository is in English: the README, the documentation site, `docs/`, code comments (Go documentation comments, and comments in shell scripts and workflows), commit messages, and pull request titles, descriptions and comments. Go documentation comments start with the name of the identifier (`// Language is ...`).
