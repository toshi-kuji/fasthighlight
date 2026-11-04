package fasthighlight

import (
	"bytes"
	"strings"
	"testing"
)

// The main rules of WriteHTML. Agreement with Chroma's output is checked in compat/.
func TestWriteHTML(t *testing.T) {
	const (
		pre = `<pre class="highlight"><code>`
		end = `</code></pre>`
		ls  = `<span class="line"><span class="cl">`
		le  = `</span></span>`
	)
	tests := []struct {
		name string
		toks []Token
		want string
	}{
		{"empty", []Token{{Text, ""}}, pre + end},
		{"no tokens", nil, pre + end},
		{"escape", []Token{{Text, "a<b>&'\"\n"}},
			pre + ls + "a&lt;b&gt;&amp;&#39;&#34;\n" + le + end},
		{"class and text", []Token{{KeywordReserved, "const"}, {Text, " x\n"}},
			pre + ls + `<span class="kr">const</span> x` + "\n" + le + end},
		{"token across lines", []Token{{CommentMultiline, "/* a\nb */"}, {Text, "\n"}},
			pre + ls + `<span class="cm">/* a` + "\n</span>" + le +
				ls + `<span class="cm">b */</span>` + "\n" + le + end},
		{"no trailing newline", []Token{{NameOther, "x"}},
			pre + ls + `<span class="nx">x</span>` + le + end},
		{"parent class", []Token{{NameKeyword, "x"}, {Text, "\n"}},
			pre + ls + `<span class="n">x</span>` + "\n" + le + end},
		{"trailing lone empty token is dropped", []Token{{Text, "a\n"}, {Keyword, ""}},
			pre + ls + "a\n" + le + end},
		{"empty token inside a line", []Token{{Text, "a"}, {Keyword, ""}, {Text, "\n"}},
			pre + ls + `a<span class="k"></span>` + "\n" + le + end},
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
		{"no-such-lang", `<pre class="highlight" data-lang="no-such-lang"><code>`},
		{`a"<&'>`, `<pre class="highlight" data-lang="a&#34;&lt;&amp;&#39;&gt;"><code>`},
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
