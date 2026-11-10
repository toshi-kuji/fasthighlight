// Package html is the HTML tokenizer. It is used by lang/html.
//
// The token types and boundaries reproduce the rules of Chroma 2.27.0's html.xml without regular
// expressions. The inside of <script> is read by internal/js and the inside of <style> by internal/css (as
// with Chroma's using, the embedded language does not get a newline appended). html.xml is
// case-insensitive (case_insensitive).
package html

import (
	"strings"
	"unicode/utf8"

	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/css"
	"github.com/toshi-kuji/fasthighlight/internal/js"
	"github.com/toshi-kuji/fasthighlight/internal/lexkit"
)

// The states of Chroma's html.xml.
type state int

const (
	root          state = iota
	comment             // <!-- … -->
	tag                 // <name … >
	attr                // an attribute value
	scriptContent       // inside <script> … </script>
	styleContent        // inside <style> … </style>
)

type lexer struct {
	x     *lexkit.Tokens
	src   string
	jsOpt js.Options
	stack []state
	// entity remembers the last scan for &\S*?;.
	entity lexkit.EntityCache
}

// Prepare builds the tables of the language used inside (CSS).
func Prepare() { css.Prepare() }

// Lex splits src into tokens (same contract as Lex in fasthighlight.Language). o is passed to the
// JavaScript inside <script>.
func Lex(src string, end int, o js.Options) []fh.Token {
	x := lexkit.NewTokens(src)
	LexInto(x, src, end, o)
	return x.Result()
}

// LexInto is the same as Lex but adds the tokens to x (for embedding HTML in another language, such as
// after the ?> of PHP).
func LexInto(x *lexkit.Tokens, src string, end int, o js.Options) {
	l := &lexer{x: x, src: src, jsOpt: o, stack: []state{root}}
	for i := 0; i < end && len(l.stack) > 0; {
		st := l.stack[len(l.stack)-1]
		n, ok := l.step(st, i)
		if !ok {
			if src[i] == '\n' && st != root {
				l.stack = append(l.stack[:0], root)
				continue
			}
			n = l.x.Error(i)
		}
		i += n
	}
}

func (l *lexer) emit(t fh.TokenType, s, e int) (int, bool) { l.x.Emit(t, s, e); return e - s, true }
func (l *lexer) push(s ...state)                           { l.stack = append(l.stack, s...) }
func (l *lexer) pop()                                      { l.stack = l.stack[:len(l.stack)-1] }

// prefixFold returns the number of bytes if the start of src[i:] matches word (0 if it does not). As with
// regexp2's case-insensitive comparison, it compares with lexkit.FoldEq (word is lowercase).
func prefixFold(src string, i int, word string) int {
	j := i
	for _, w := range word {
		if j >= len(src) {
			return 0
		}
		r, n := utf8.DecodeRuneInString(src[j:])
		if !lexkit.FoldEq(r, w) {
			return 0
		}
		j += n
	}
	return j - i
}

func isNameChar(r rune) bool { return lexkit.IsWord(r) || r == ':' || r == '.' || r == '-' }
func isAttrChar(r rune) bool { return lexkit.IsWord(r) || r == ':' || r == '-' }

// step tries the rules of state st in order. If one matches, it returns the number of bytes advanced and
// true.
func (l *lexer) step(st state, i int) (int, bool) {
	src := l.src
	c := src[i]
	switch st {
	case root:
		return l.root(i)
	case comment:
		if k := strings.IndexByte(src[i:], '-'); k != 0 { // [^-]+
			if k < 0 {
				k = len(src) - i
			}
			return l.emit(fh.Comment, i, i+k)
		}
		if strings.HasPrefix(src[i:], "-->") {
			l.pop()
			return l.emit(fh.Comment, i, i+3)
		}
		return l.emit(fh.Comment, i, i+1)
	case tag:
		if n := lexkit.Span(src, i, lexkit.IsSpace); n > 0 {
			return l.emit(fh.Text, i, i+n)
		}
		if w := lexkit.Span(src, i, isAttrChar); w > 0 {
			// ([\w:-]+\s*)(=)(\s*) or [\w:-]+
			s := i + w + lexkit.Span(src, i+w, lexkit.IsSpace)
			if s < len(src) && src[s] == '=' {
				e := s + 1 + lexkit.Span(src, s+1, lexkit.IsSpace)
				l.x.Emit(fh.NameAttribute, i, s)
				l.x.Emit(fh.Operator, s, s+1)
				l.x.Emit(fh.Text, s+1, e)
				l.push(attr)
				return e - i, true
			}
			return l.emit(fh.NameAttribute, i, i+w)
		}
		// (/?)(\s*)(>)
		j := i
		if c == '/' {
			j++
		}
		k := j + lexkit.Span(src, j, lexkit.IsSpace)
		if k < len(src) && src[k] == '>' {
			l.x.Emit(fh.Punctuation, i, j)
			l.x.Emit(fh.Text, j, k)
			l.x.Emit(fh.Punctuation, k, k+1)
			l.pop()
			return k + 1 - i, true
		}
	case attr:
		if c == '"' || c == '\'' { // ".*?" and '.*?' (dot_all)
			if k := strings.IndexByte(src[i+1:], c); k >= 0 {
				l.pop()
				return l.emit(fh.LiteralString, i, i+1+k+1)
			}
		}
		if n := lexkit.Span(src, i, notSpaceOrGT); n > 0 { // [^\s>]+
			l.pop()
			return l.emit(fh.LiteralString, i, i+n)
		}
	case scriptContent, styleContent:
		word := "script"
		if st == styleContent {
			word = "style"
		}
		if e := closeTagEnd(src, i, word); e > 0 {
			l.closeTag(i, word)
			l.pop()
			return e - i, true
		}
		// .+?(?=<\s*/\s*script\s*>)
		p := i + 1
		for {
			k := strings.IndexByte(src[p:], '<')
			if k < 0 {
				return 0, false
			}
			p += k
			if closeTagEnd(src, p, word) > 0 {
				break
			}
			p++
		}
		if st == scriptContent {
			js.LexInto(l.x.Sub(src[i:p], i), src[i:p], p-i, l.jsOpt)
		} else {
			css.LexInto(l.x.Sub(src[i:p], i), src[i:p], p-i)
		}
		return p - i, true
	}
	return 0, false
}

func notSpaceOrGT(r rune) bool { return !lexkit.IsSpace(r) && r != '>' }

// closeTagEnd returns the end if src[i:] matches <\s*/\s*word\s*>, or 0 if it does not.
func closeTagEnd(src string, i int, word string) int {
	if src[i] != '<' {
		return 0
	}
	j := i + 1 + lexkit.Span(src, i+1, lexkit.IsSpace)
	if j >= len(src) || src[j] != '/' {
		return 0
	}
	j++
	j += lexkit.Span(src, j, lexkit.IsSpace)
	n := prefixFold(src, j, word)
	if n == 0 {
		return 0
	}
	j += n
	j += lexkit.Span(src, j, lexkit.IsSpace)
	if j >= len(src) || src[j] != '>' {
		return 0
	}
	return j + 1
}

// closeTag turns the </word> starting at i into the 7 tokens (<)(\s*)(/)(\s*)(word)(\s*)(>).
func (l *lexer) closeTag(i int, word string) {
	src := l.src
	a := i + 1
	b := a + lexkit.Span(src, a, lexkit.IsSpace)
	c := b + 1
	d := c + lexkit.Span(src, c, lexkit.IsSpace)
	e := d + prefixFold(src, d, word)
	f := e + lexkit.Span(src, e, lexkit.IsSpace)
	l.x.Emit(fh.Punctuation, i, a)
	l.x.Emit(fh.Text, a, b)
	l.x.Emit(fh.Punctuation, b, c)
	l.x.Emit(fh.Text, c, d)
	l.x.Emit(fh.NameTag, d, e)
	l.x.Emit(fh.Text, e, f)
	l.x.Emit(fh.Punctuation, f, f+1)
}

// root tries the rules of the root state.
func (l *lexer) root(i int) (int, bool) {
	src := l.src
	c := src[i]
	if c != '<' && c != '&' { // [^<&]+
		j := i
		for j < len(src) && src[j] != '<' && src[j] != '&' {
			j++
		}
		return l.emit(fh.Text, i, j)
	}
	if c == '&' { // &\S*?;
		if e := l.entity.End(src, i); e > 0 {
			return l.emit(fh.NameEntity, i, e)
		}
		return 0, false
	}
	// From here on, c == '<'
	if n := prefixFold(src, i, "<![cdata["); n > 0 { // \<\!\[CDATA\[.*?\]\]\>
		if k := strings.Index(src[i+n:], "]]>"); k >= 0 {
			return l.emit(fh.CommentPreproc, i, i+n+k+3)
		}
	}
	rest := src[i:]
	if strings.HasPrefix(rest, "<!--") {
		l.push(comment)
		return l.emit(fh.Comment, i, i+4)
	}
	if strings.HasPrefix(rest, "<?") { // <\?.*?\?>
		if k := strings.Index(src[i+2:], "?>"); k >= 0 {
			return l.emit(fh.CommentPreproc, i, i+2+k+2)
		}
	}
	if strings.HasPrefix(rest, "<!") { // <![^>]*>
		if k := strings.IndexByte(src[i+2:], '>'); k >= 0 {
			return l.emit(fh.CommentPreproc, i, i+2+k+1)
		}
	}
	a := i + 1
	b := a + lexkit.Span(src, a, lexkit.IsSpace)
	// (<)(\s*)(script)(\s*) and (<)(\s*)(style)(\s*)
	for _, w := range [...]struct {
		word    string
		content state
	}{{"script", scriptContent}, {"style", styleContent}} {
		if n := prefixFold(src, b, w.word); n > 0 {
			e := b + n
			f := e + lexkit.Span(src, e, lexkit.IsSpace)
			l.x.Emit(fh.Punctuation, i, a)
			l.x.Emit(fh.Text, a, b)
			l.x.Emit(fh.NameTag, b, e)
			l.x.Emit(fh.Text, e, f)
			l.push(w.content, tag)
			return f - i, true
		}
	}
	// (<)(\s*)([\w:.-]+)
	if n := lexkit.Span(src, b, isNameChar); n > 0 {
		l.x.Emit(fh.Punctuation, i, a)
		l.x.Emit(fh.Text, a, b)
		l.x.Emit(fh.NameTag, b, b+n)
		l.push(tag)
		return b + n - i, true
	}
	// (<)(\s*)(/)(\s*)([\w:.-]+)(\s*)(>)
	if b < len(src) && src[b] == '/' {
		c2 := b + 1
		d := c2 + lexkit.Span(src, c2, lexkit.IsSpace)
		if n := lexkit.Span(src, d, isNameChar); n > 0 {
			e := d + n
			f := e + lexkit.Span(src, e, lexkit.IsSpace)
			if f < len(src) && src[f] == '>' {
				l.x.Emit(fh.Punctuation, i, a)
				l.x.Emit(fh.Text, a, b)
				l.x.Emit(fh.Punctuation, b, c2)
				l.x.Emit(fh.Text, c2, d)
				l.x.Emit(fh.NameTag, d, e)
				l.x.Emit(fh.Text, e, f)
				l.x.Emit(fh.Punctuation, f, f+1)
				return f + 1 - i, true
			}
		}
	}
	return 0, false
}
