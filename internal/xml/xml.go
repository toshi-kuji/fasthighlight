// Package xml is the XML tokenizer. It is used by lang/xml (XML) and internal/http (message bodies).
//
// Token types and boundaries reproduce the rules of Chroma 2.27.0's xml.xml without regular expressions.
package xml

import (
	"strings"

	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/lexkit"
)

// The states of xml.xml in Chroma.
type state int

const (
	root    state = iota
	comment       // <!-- … -->
	tag           // <name … >
	attr          // attribute value
)

// Lex splits src into tokens (the same contract as Lex of fasthighlight.Language).
func Lex(src string, end int) []fh.Token {
	x := lexkit.NewTokens(src)
	LexInto(x, src, end)
	return x.Result()
}

// LexInto is the same as Lex but adds the tokens to x (for embedding XML in another language, such as the
// body of an HTTP message).
func LexInto(x *lexkit.Tokens, src string, end int) {
	stack := []state{root}
	for i := 0; i < end && len(stack) > 0; {
		st := stack[len(stack)-1]
		n, t, push, pop := match(src, i, st)
		if n == 0 {
			if src[i] == '\n' && st != root {
				stack = stack[:1]
				stack[0] = root
				continue
			}
			i += x.Error(i)
			continue
		}
		x.Emit(t, i, i+n)
		i += n
		if push >= 0 {
			stack = append(stack, push)
		}
		if pop {
			stack = stack[:len(stack)-1]
		}
	}
}

func isNameChar(r rune) bool { return lexkit.IsWord(r) || r == ':' || r == '.' || r == '-' }

// match tries the rules of state st in order. If none matches, n is 0.
func match(src string, i int, st state) (n int, t fh.TokenType, push state, pop bool) {
	push = -1
	c := src[i]
	rest := src[i:]
	switch st {
	case root:
		if n = lexkit.Span(src, i, notTextSpecial); n > 0 { // [^<&]+
			return n, fh.Text, push, false
		}
		if c == '&' { // &\S*?;
			for j := i + 1; j < len(src); {
				r, w := lexkit.Rune(src, j)
				if r == ';' {
					return j + 1 - i, fh.NameEntity, push, false
				}
				if lexkit.IsSpace(r) {
					break
				}
				j += w
			}
			return 0, 0, push, false
		}
		// From here on, c == '<'
		if strings.HasPrefix(rest, "<![CDATA[") { // \<\!\[CDATA\[.*?\]\]\>
			if k := strings.Index(src[i+9:], "]]>"); k >= 0 {
				return 9 + k + 3, fh.CommentPreproc, push, false
			}
		}
		if strings.HasPrefix(rest, "<!--") {
			return 4, fh.Comment, comment, false
		}
		if strings.HasPrefix(rest, "<?") { // <\?.*?\?>
			if k := strings.Index(src[i+2:], "?>"); k >= 0 {
				return 2 + k + 2, fh.CommentPreproc, push, false
			}
		}
		if strings.HasPrefix(rest, "<!") { // <![^>]*>
			if k := strings.IndexByte(src[i+2:], '>'); k >= 0 {
				return 2 + k + 1, fh.CommentPreproc, push, false
			}
		}
		// <\s*[\w:.-]+
		j := i + 1 + lexkit.Span(src, i+1, lexkit.IsSpace)
		if w := lexkit.Span(src, j, isNameChar); w > 0 {
			return j + w - i, fh.NameTag, tag, false
		}
		// <\s*/\s*[\w:.-]+\s*>
		if j < len(src) && src[j] == '/' {
			j++
			j += lexkit.Span(src, j, lexkit.IsSpace)
			if w := lexkit.Span(src, j, isNameChar); w > 0 {
				j += w
				j += lexkit.Span(src, j, lexkit.IsSpace)
				if j < len(src) && src[j] == '>' {
					return j + 1 - i, fh.NameTag, push, false
				}
			}
		}
	case comment:
		if n = strings.IndexByte(rest, '-'); n != 0 { // [^-]+
			if n < 0 {
				n = len(rest)
			}
			return n, fh.Comment, push, false
		}
		if strings.HasPrefix(rest, "-->") {
			return 3, fh.Comment, push, true
		}
		return 1, fh.Comment, push, false // -
	case tag:
		if n = lexkit.Span(src, i, lexkit.IsSpace); n > 0 {
			return n, fh.Text, push, false
		}
		// [\w.:-]+\s*=
		if w := lexkit.Span(src, i, isNameChar); w > 0 {
			j := i + w + lexkit.Span(src, i+w, lexkit.IsSpace)
			if j < len(src) && src[j] == '=' {
				return j + 1 - i, fh.NameAttribute, attr, false
			}
		}
		// /?\s*>
		j := i
		if c == '/' {
			j++
		}
		j += lexkit.Span(src, j, lexkit.IsSpace)
		if j < len(src) && src[j] == '>' {
			return j + 1 - i, fh.NameTag, push, true
		}
	case attr:
		if n = lexkit.Span(src, i, lexkit.IsSpace); n > 0 {
			return n, fh.Text, push, false
		}
		if c == '"' || c == '\'' { // ".*?" and '.*?' (dot_all)
			if k := strings.IndexByte(src[i+1:], c); k >= 0 {
				return k + 2, fh.LiteralString, push, true
			}
		}
		if n = lexkit.Span(src, i, notSpaceOrGT); n > 0 { // [^\s>]+
			return n, fh.LiteralString, push, true
		}
	}
	return 0, 0, push, false
}

func notTextSpecial(r rune) bool { return r != '<' && r != '&' }
func notSpaceOrGT(r rune) bool   { return !lexkit.IsSpace(r) && r != '>' }
