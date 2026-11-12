package mdparity_test

import (
	"bytes"
	"math/rand/v2"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/toshi-kuji/fasthighlight/internal/testcases"
	_ "github.com/toshi-kuji/fasthighlight/lang/all"
	md1 "github.com/toshi-kuji/fasthighlight/markdown"
	md2 "github.com/toshi-kuji/fasthighlight/markdown/v2"
	gm1 "github.com/yuin/goldmark"
	ast1 "github.com/yuin/goldmark/ast"
	text1 "github.com/yuin/goldmark/text"
	ast2 "github.com/yuin/goldmark/v2/ast"
	"github.com/yuin/goldmark/v2/parser"
)

func corpus(t *testing.T) map[string][]byte {
	t.Helper()
	docs := map[string][]byte{"edge": []byte(testcases.MarkdownEdge)}
	for _, f := range []string{"input.md", "multilang.md"} {
		b, err := os.ReadFile("../../testdata/" + f)
		if err != nil {
			t.Fatal(err)
		}
		docs[f] = b
	}
	return docs
}

// The v2 package writes the same HTML as the v1 package, for every Parallelism.
func TestSameHTMLAsV1(t *testing.T) {
	for name, src := range corpus(t) {
		for _, p := range []int{int(md1.ParallelAuto), int(md1.ParallelNever), int(md1.ParallelAlways)} {
			c1 := md1.NewConverter()
			c1.Parallelism = md1.Parallelism(p)
			c2 := md2.NewConverter()
			c2.Parallelism = md2.Parallelism(p)
			var b1, b2 bytes.Buffer
			if err := c1.Convert(src, &b1); err != nil {
				t.Fatal(err)
			}
			if err := c2.Convert(src, &b2); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(b1.Bytes(), b2.Bytes()) {
				t.Errorf("%s, parallelism %d: v2 output differs from v1\n%s", name, p, testcases.FirstDiff(b1.String(), b2.String()))
			}
		}
	}
}

// CodeBlocks and Scan give the same results as the v1 package. Only the line of an empty block differs: v1
// does not know it (0), v2 does.
func TestCodeBlocksAndScanSameAsV1(t *testing.T) {
	for name, src := range corpus(t) {
		c1, c2 := md1.NewConverter(), md2.NewConverter()
		d1, d2 := c1.Parse(name, src), c2.Parse(name, src)
		h1, h2 := c1.CodeBlocks(d1), c2.CodeBlocks(d2)
		if len(h1) != len(h2) {
			t.Fatalf("%s: %d blocks in v2, %d in v1", name, len(h2), len(h1))
		}
		for k := range h1 {
			a, b := h1[k], h2[k]
			if a.Line == 0 {
				a.Line = b.Line
			}
			if a.Document != b.Document || a.Line != b.Line || a.Lang != b.Lang || a.Open != b.Open ||
				a.Code != b.Code || a.Close != b.Close || a.Colored != b.Colored || a.Reason != b.Reason {
				t.Errorf("%s block %d: v2 %+v, v1 %+v", name, k, b, a)
			}
		}
		s1, s2 := md1.Scan(d1), md2.Scan(d2)
		if !reflect.DeepEqual(s1.Used, s2.Used) || s1.Uncolored() != s2.Uncolored() ||
			!sameBlocks(s1.NotBuiltIn, s2.NotBuiltIn) || !sameBlocks(s1.Unsupported, s2.Unsupported) ||
			!sameBlocks(s1.NoLanguage, s2.NoLanguage) {
			t.Errorf("%s: Scan differs\nv1 %+v\nv2 %+v", name, s1, s2)
		}
	}
}

func sameBlocks(a []md1.CodeBlock, b []md2.CodeBlock) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		x, y := a[k], b[k]
		if x.Line == 0 {
			x.Line = y.Line
		}
		if x.Document != y.Document || x.Line != y.Line || x.Lang != y.Lang || x.Package != y.Package {
			return false
		}
	}
	return true
}

// LanguageName reads the same name as the v1 package's, for info strings made of the characters that matter.
func TestLanguageNameSameAsV1(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	parts := []string{"a", "go", "c", "+", " ", "\t", "\\", "\\+", "\\a", "&", ";", "&amp;", "&#43;", "&#32;",
		"&#x2B;", "&lt;", "&nosuch;", "&#;", "&#x0000041;", "&#x000041;", "&#00000065;", "&#0000065;", "&#xFFFFFFF;", "&#X41;", "&#x;", "&#a;", "{", "}", ".", "é", "日本", "`", "~"}
	infos := []string{"", " go", "go ", "\tgo\t", "c\\+\\+", "c&#43;&#43;", "a&#32;b", "\\&amp;", "&amp;amp;", "&#x0000063;", "&#x00000041;", "&#xFFFFFFF;", "&#99999999;"}
	for range 20000 {
		var sb strings.Builder
		for range r.IntN(6) + 1 {
			sb.WriteString(parts[r.IntN(len(parts))])
		}
		infos = append(infos, sb.String())
	}
	for _, info := range infos {
		if strings.ContainsAny(info, "`") {
			continue // not an info string of a ``` fence
		}
		src := []byte("```" + info + "\nx\n```\n")
		if got, want := name2(src), name1(src); got != want {
			t.Errorf("info %q: v2 %q, v1 %q", info, got, want)
		}
	}
}

func name1(src []byte) string {
	root := gm1.New().Parser().Parse(text1.NewReader(src))
	var name string
	_ = ast1.Walk(root, func(n ast1.Node, entering bool) (ast1.WalkStatus, error) {
		if b, ok := n.(*ast1.FencedCodeBlock); ok && entering {
			name = md1.LanguageName(b, src)
		}
		return ast1.WalkContinue, nil
	})
	return name
}

func name2(src []byte) string {
	root := parser.New().Parse(src)
	var name string
	_ = ast2.Walk(root, func(n ast2.Node, entering bool) (ast2.WalkStatus, error) {
		if b, ok := n.(*ast2.CodeBlock); ok && entering {
			name = md2.LanguageName(b, src)
		}
		return ast2.WalkContinue, nil
	})
	return name
}
