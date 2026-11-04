package php_test

import (
	"strings"
	"testing"

	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/lang/php"
)

// <?php and ?> become CommentPreproc, and the text from ?> to the next <? is read as HTML (an
// intended difference from Chroma).
func TestTags(t *testing.T) {
	code := "<?php echo 1; ?>\n<b class=\"x\">hi</b>\n<?= $y ?>\n<p>end</p>\n"
	toks := php.Language.Tokenise(code)
	var b strings.Builder
	for _, tk := range toks {
		b.WriteString(tk.Value)
		if tk.Type == fh.Error {
			t.Errorf("unexpected Error token %q", tk.Value)
		}
	}
	if b.String() != code {
		t.Fatalf("tokens do not cover the input: %q", b.String())
	}
	want := map[string]fh.TokenType{"<?php": fh.CommentPreproc, "?>": fh.CommentPreproc, "<?": fh.CommentPreproc,
		"b": fh.NameTag, "class": fh.NameAttribute, "\"x\"": fh.LiteralString, "echo": fh.Keyword, "$y": fh.NameVariable, "p": fh.NameTag}
	seen := map[string]bool{}
	for _, tk := range toks {
		if w, ok := want[tk.Value]; ok {
			seen[tk.Value] = true
			if tk.Type != w {
				t.Errorf("%q: type %s, want %s", tk.Value, tk.Type, w)
			}
		}
	}
	for v := range want {
		if !seen[v] {
			t.Errorf("token %q not found in %v", v, toks)
		}
	}
}
