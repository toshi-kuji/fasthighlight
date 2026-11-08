package fasthighlight

import (
	"bytes"
	"strings"
	"testing"
)

// The main rules of WriteHTML. Agreement of the tokens with Chroma is checked in compat/.
func TestWriteHTML(t *testing.T) {
	const (
		pre = `<pre class="highlight"><code>`
		end = `</code></pre>`
	)
	tests := []struct {
		name string
		toks []Token
		want string
	}{
		{"empty", []Token{{Text, ""}}, pre + end},
		{"no tokens", nil, pre + end},
		{"escape", []Token{{Text, "a<b>&'\"\n"}}, pre + "a&lt;b&gt;&amp;&#39;&#34;\n" + end},
		{"class and text", []Token{{KeywordReserved, "const"}, {Text, " x\n"}},
			pre + `<span class="kr">const</span> x` + "\n" + end},
		// Lines are not wrapped: a token across lines stays one <span>.
		{"token across lines", []Token{{CommentMultiline, "/* a\nb */"}, {Text, "\n"}},
			pre + `<span class="cm">/* a` + "\n" + `b */</span>` + "\n" + end},
		{"no trailing newline", []Token{{NameOther, "x"}}, pre + `<span class="nx">x</span>` + end},
		{"parent class", []Token{{NameKeyword, "x"}, {Text, "\n"}}, pre + `<span class="n">x</span>` + "\n" + end},
		// Whitespace is written without <span class="w">.
		{"whitespace", []Token{{Keyword, "if"}, {TextWhitespace, " \t"}, {NameOther, "x"}, {TextWhitespace, "\n"}},
			pre + `<span class="k">if</span>` + " \t" + `<span class="nx">x</span>` + "\n" + end},
		{"whitespace is escaped", []Token{{TextWhitespace, "<"}}, pre + "&lt;" + end},
		// Empty tokens write nothing.
		{"empty tokens", []Token{{Text, "a"}, {Keyword, ""}, {Text, "\n"}, {Keyword, ""}}, pre + "a\n" + end},
	}
	for _, tt := range tests {
		var b bytes.Buffer
		if err := WriteHTML(&b, tt.toks); err != nil {
			t.Fatal(err)
		}
		if got := b.String(); got != tt.want {
			t.Errorf("%s:\n got %q\nwant %q", tt.name, got, tt.want)
		}
	}
}

// The code block's language name goes into data-lang. The language name is escaped, and omitted if empty.
func TestHighlightDataLang(t *testing.T) {
	for _, c := range []struct{ lang, want string }{
		{"", `<pre class="highlight"><code>`},
		{"no-such-lang", `<pre class="highlight" data-lang="no-such-lang"><code class="language-no-such-lang">`},
		{`a"<&'>`, `<pre class="highlight" data-lang="a&#34;&lt;&amp;&#39;&gt;"><code class="language-a&#34;&lt;&amp;&#39;&gt;">`},
	} {
		var b strings.Builder
		if err := Highlight(&b, c.lang, "x\n"); err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(b.String(), c.want) {
			t.Errorf("lang %q: got %q, want prefix %q", c.lang, b.String(), c.want)
		}
	}
}
