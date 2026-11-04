[fasthighlight](index.html) · [Contents](contents.html)

# Chroma compatibility

fasthighlight follows [Chroma](https://github.com/alecthomas/chroma), the syntax highlighter widely used in Go, as its reference.

> Chroma 2.27.0 is the reference, and the intended changes to its results are listed. For the 24 supported languages, the tokens are checked by fuzz tests to match Chroma everywhere except at those changes.

Fuzz tests compare the two on large numbers of generated inputs. They are a check, not a proof that every input matches.

## What matches

- **Tokens:** the kinds of tokens and where they split follow the rules of Chroma 2.27.0's language definitions. Long word lists (keywords and the like) are generated from Chroma's rules, not copied by hand.
- **Languages:** the names, aliases and file name patterns are the same as Chroma's, and names are looked up the same way.
- **HTML:** for a given token sequence, the HTML is the same as Chroma's HTML formatter with `WithClasses(true)`, byte for byte, except for two things:
  - the outer class is `highlight` (Chroma uses `chroma`), so that no product name appears in your HTML;
  - code blocks from Markdown carry the language name in `data-lang`.

  The token class names are the same short names (`k`, `s`, `nf`, …), so CSS written for Chroma's classes works.

## What was changed

Chroma is the reference, not something to copy exactly. Where Chroma has a bug, or behaves in a way that gets in the way of readers, fasthighlight differs. Changes are limited to places where the colors would clearly go wrong (an error color, or a string or tag color running on past where it should end).

<div class="changes">

| Where | Chroma | fasthighlight |
| --- | --- | --- |
| `{}` inside `${ … }` of a template literal (TypeScript, JavaScript, `<script>` in HTML) | the first `}` ends the `${ … }`, so what follows looks like a string | counts the braces |
| TypeScript JSX (`<div …>`) | `<` followed by a letter is a tag, so `Map<string, number>` and `a<b` become tags, sometimes followed by errors | not treated as tags (`<` is an operator) |
| Escapes such as `\n` in TypeScript template literals | `\` is an error | string color, as in JavaScript |
| Names starting with a non-ASCII letter in TypeScript, Swift and Ruby | error color | name color |
| Ruby strings of the form `% abc ` | the delimiter is output twice | output once |
| Ruby `defined?` | not a keyword in `defined? x` or `defined?(x)` | a keyword |
| The start of an R name | `[`, `\`, `]`, `^` and `` ` `` can start a name (a typo for `[a-zA-Z]` in the rule), so `x[[1]]` and `n^2` are names | letters only |
| The start of a Go name | digits other than ASCII, and numbers in templates (`{{ 1 }}`), can start a name | digits cannot start a name |
| Java numbers (`42`, `1.5`, `0x1F`) | name color | number color |
| C++ raw strings (`R"(…)"`) | the rule almost never matches, so `R` is a name and the rest a normal string | raw strings |
| Python docstrings (`"""…"""` at the start of a line) | the rule almost never matches, so they get the normal string color | docstring color |
| PHP `<?php` and `?>` | `<?` is an operator and `php` a name; after `?>`, everything is an error | `<?php` and `?>` get the preprocessor color, and the text between `?>` and the next `<?` is read as HTML |
| A newline right after `#`, `-`, `*` or `>` in Markdown | the next line becomes part of the heading, list item or quote | a newline there does not count |
| Emphasis at the start of a line or a list item in Markdown (`**Note**: …`, `- **a** b`) | not emphasis | emphasis |
| Code blocks inside Markdown | colored with any of Chroma's languages | colored with the registered languages only |
| Front matter in Markdown with `\r` alone as newlines | found before newlines are normalized | found after, like everything else |
| Very long tokens of the same kind in Markdown code blocks and front matter (8,192 bytes or more) | split at one place | split at another place (looks the same) |
| Rules whose regular expression takes more than 250 ms on an input | treated as not matching, so the result depends on how fast the machine is | always the same result |

</div>

Each change has a switch, used only in tests, that turns it back to Chroma's behavior; the fuzz tests compare that version with Chroma, so the changes are known to be the only differences.

## Why it is fast

### How others do it

Many highlighters, Chroma among them, use grammars made of regular expressions (such as the grammars of VS Code or Sublime Text). One shared mechanism handles many languages, existing grammars can be reused, and adding a language means adding a grammar file. This is a sound design for supporting many languages.

### How fasthighlight does it

fasthighlight supports 24 languages, and each has its own hand-written tokenizer. From its current state and the next character, it goes straight to what needs doing: for a letter, it reads the whole name and looks it up in the keyword table. There is no step that tries many grammar rules at each position.

### The trade-off

| | Grammars of regular expressions | Hand-written tokenizers (fasthighlight) |
| --- | --- | --- |
| Adding a language | add a grammar file | write code for the language |
| Maintaining a grammar | edit the grammar file | edit the code |
| Existing grammars | reusable | not usable as they are |
| Speed | depends on a general regular expression engine | each language can be tuned |
| Number of languages | many (giallo: more than 220) | 24 |

To keep the maintenance down, fasthighlight follows Chroma's rules instead of designing its own, generates long word lists from Chroma's rules, and checks the result against Chroma with fuzz tests.

### What this does not mean

- It is not a new algorithm. Compilers have long tokenized this way; what is different is choosing per-language speed over reusing shared grammars.
- Regular expressions are not slow in themselves. The measured differences also come from how many rules are tried, how scopes are tracked, and how detailed the output is. (This is our reading; we have not profiled the other libraries.)
- Only the highlighting engine works without regular expressions. Markdown is parsed by goldmark.
- Code blocks in other languages are escaped, without colors.

The measurements are on the [top page](index.html#speed).
