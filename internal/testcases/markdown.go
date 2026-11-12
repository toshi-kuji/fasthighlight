package testcases

import "fmt"

// MarkdownEdge holds code blocks in the places and forms that are easy to get wrong: indented code (left to goldmark),
// ~~~ fences, nesting, info strings with escapes, references and extra words, and languages that are not
// highlighted.
const MarkdownEdge = "# T\n\n" +
	"    indented <code> & stuff\n\n" +
	"```go\nfunc f() {}\n```\n\n" +
	"~~~ python extra words\nprint(\"x\")\n~~~\n\n" +
	"- item\n\n  ```js\n  let a = 1 < 2;\n  ```\n\n" +
	"> ```rust\n> fn main() {}\n> ```\n\n" +
	"```c\\+\\+\nint x;\n```\n\n" +
	"```c&#43;&#43;\nint y;\n```\n\n" +
	"```\nno language <b>\n```\n\n" +
	"```text\nplain\n```\n\n" +
	"```nosuchlang\nx\n```\n\n" +
	"```\n```\n\n" +
	"```go\n```\n"

// FirstDiff shows where got first differs from want, for test failures.
func FirstDiff(a, b string) string {
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	lo := max(0, i-80)
	return fmt.Sprintf("at byte %d:\nwant …%q\ngot  …%q", i, a[lo:min(len(a), i+80)], b[lo:min(len(b), i+80)])
}
