// Package markdown is the Markdown tokenizer. lang/markdown registers it. Options (the switch for
// the intended differences from Chroma) exists only for the comparison in compat/ and is not
// exposed.
//
// The token types and token boundaries reproduce the rules of Chroma 2.27.0's lexers/markdown.go
// (which are written in Go code) without using regular expressions. There are two states, root and
// inline, and states are never pushed (inline is used when reading the contents of a list item with
// UsingSelf, and by the last rule of root, include).
// The last rule .|\n matches at every position, so no Error token is produced.
//
// As in Chroma, the YAML front matter at the start of the document (the part enclosed by --- lines)
// is read as YAML, and the inside of a code block (```language) is read as that language. The
// language is looked up among the registered languages (Chroma looks it up among all of Chroma's
// languages. A language that is not registered becomes Text, just as when Chroma does not find the
// language).
package markdown

import (
	"strings"

	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/lexkit"
)

// Options is the switch for the intended differences from Chroma. The zero value is the production
// setting (the one lang/markdown registers).
type Options struct {
	// If Chroma is true, the following Chroma behaviors are reproduced. If false, they are fixed.
	//
	//  - Even when a newline immediately follows the # of a heading, the - or * of a list, or the > of
	//    a quote, the heading and so on extend to the next line.
	//  - Emphasis (such as **a**) at the start of the contents of a list item (a - or * followed by
	//    one space) is not treated as emphasis (because the contents are read without the preceding
	//    space; the emphasis rules require a preceding space).
	//  - Emphasis at the start of a line (*a*, **a**, ~~a~~) is not treated as emphasis (because a
	//    preceding space is required).
	Chroma bool
	// Lookup returns, from the language name of a code block (and "YAML" for front matter), the
	// function that tokenizes in that language. It returns nil if there is no such language. If
	// Lookup itself is nil, Tokenise of the registered language (fasthighlight.LookupRegistered) is
	// used. compat/ uses it to compare with the other languages also in their Chroma-matching
	// versions.
	Lookup func(name string) func(code string) []fh.Token
}

type lexer struct {
	o   Options
	x   *lexkit.Tokens
	src string
	sp  lexkit.SpanCache // the last run of whitespace at a line start
	// closeBracket and closeParen remember failed searches of link for ] and ), so that many [ without a ]
	// are not each searched to the end.
	closeBracket, closeParen lexkit.Finder
}

// Lex splits src into tokens (the same contract as Lex of fasthighlight.Language). Markdown does not
// append a newline at the end, so end is len(src). compat/ uses it to compare with Chroma while
// varying o.
func Lex(src string, end int, o Options) []fh.Token {
	l := &lexer{o: o, x: lexkit.NewTokens(src), src: src}
	i := 0
	if n := frontmatter(src); n > 0 {
		if lex := l.lookup("YAML"); lex != nil {
			l.emitTokens(0, lex(src[:n]))
			i = n
		}
	}
	for i < end {
		i = l.root(i)
	}
	return l.x.Result()
}

// frontmatter returns the length of the YAML front matter at the start of src (from the "---" line
// through the newline of the next "---" line). It returns 0 if there is none (the same as Chroma's
// splitFrontmatter; a newline is required after the closing line).
func frontmatter(src string) int {
	if !strings.HasPrefix(src, "---\n") {
		return 0
	}
	for pos := 4; pos < len(src); {
		next := strings.IndexByte(src[pos:], '\n')
		if next < 0 {
			break
		}
		if src[pos:pos+next] == "---" {
			return pos + next + 1
		}
		pos += next + 1
	}
	return 0
}

func (l *lexer) lookup(name string) func(string) []fh.Token {
	if l.o.Lookup != nil {
		return l.o.Lookup(name)
	}
	if lang := fh.LookupRegistered(name); lang != nil {
		return lang.Tokenise
	}
	return nil
}

// emitTokens adds the tokens toks of another language (concatenated, they form the beginning of
// src[at:]).
func (l *lexer) emitTokens(at int, toks []fh.Token) {
	for _, t := range toks {
		l.x.Emit(t.Type, at, at+len(t.Value))
		at += len(t.Value)
	}
}

// lineRest returns the end (just after the newline) if the regular expression .+\n matches at the
// start of src[i:]. It returns -1 if it does not match.
func lineRest(src string, i int) int {
	if i >= len(src) || src[i] == '\n' {
		return -1
	}
	e := lexkit.LineEnd(src, i)
	if e >= len(src) {
		return -1
	}
	return e + 1
}

// closingFence returns the first position in src[i:] where the regular expression ^```$ matches (i
// is the start of a line). It returns -1 if there is none.
func closingFence(src string, i int) int {
	for {
		if strings.HasPrefix(src[i:], "```") && (i+3 == len(src) || src[i+3] == '\n') {
			return i
		}
		n := strings.IndexByte(src[i:], '\n')
		if n < 0 {
			return -1
		}
		i += n + 1
	}
}

// root adds the tokens of the rule that matches at the start of src[i:] in the root state, and
// returns the next position.
func (l *lexer) root(i int) int {
	src, x := l.src, l.x
	if n := comment(src, i); n > 0 {
		x.Emit(fh.CommentMultiline, i, i+n)
		return i + n
	}
	if i > 0 && src[i-1] != '\n' {
		return l.inline(src, i)
	}
	// ^(#[^#].+\n)
	if src[i] == '#' && i+1 < len(src) {
		r, n := lexkit.Rune(src, i+1)
		if r != '#' && (l.o.Chroma || r != '\n') {
			if e := lineRest(src, i+1+n); e > 0 {
				x.Emit(fh.GenericHeading, i, e)
				return e
			}
		}
	}
	// ^(#{2,6}.+\n). #{2,6} can shrink to two by backtracking, so one or more characters after ##
	// is enough.
	if strings.HasPrefix(src[i:], "##") {
		if e := lineRest(src, i+2); e > 0 {
			x.Emit(fh.GenericSubheading, i, e)
			return e
		}
	}
	q := i + l.sp.Span(src, i, lexkit.IsSpace) // ^(\s*)
	if q < len(src) && (src[q] == '*' || src[q] == '-') {
		// ^(\s*)([*-] )(\[[ xX]\])( .+\n)
		if q+6 <= len(src) && src[q+1] == ' ' && src[q+2] == '[' && strings.IndexByte(" xX", src[q+3]) >= 0 &&
			src[q+4] == ']' && src[q+5] == ' ' {
			if e := lineRest(src, q+6); e > 0 {
				x.Emit(fh.Text, i, q)
				x.Emit(fh.Keyword, q, q+2)
				x.Emit(fh.Keyword, q+2, q+5)
				l.inlineRange(q+5, e)
				return e
			}
		}
		// ^(\s*)([*-])(\s)(.+\n)
		if q+1 < len(src) {
			r, n := lexkit.Rune(src, q+1)
			if lexkit.IsSpace(r) && (l.o.Chroma || r != '\n') {
				if e := lineRest(src, q+1+n); e > 0 {
					x.Emit(fh.Text, i, q)
					x.Emit(fh.Keyword, q, q+1)
					if l.o.Chroma {
						x.Emit(fh.Text, q+1, q+1+n)
						l.inlineRange(q+1+n, e)
					} else {
						l.inlineRange(q+1, e)
					}
					return e
				}
			}
		}
	}
	// ^(\s*)([0-9]+\.)( .+\n)
	if d := digits(src, q); d > 0 && q+d+1 < len(src) && src[q+d] == '.' && src[q+d+1] == ' ' {
		if e := lineRest(src, q+d+2); e > 0 {
			x.Emit(fh.Text, i, q)
			x.Emit(fh.Keyword, q, q+d+1)
			l.inlineRange(q+d+1, e)
			return e
		}
	}
	// ^(\s*>\s)(.+\n)
	if q+1 < len(src) && src[q] == '>' {
		r, n := lexkit.Rune(src, q+1)
		if lexkit.IsSpace(r) && (l.o.Chroma || r != '\n') {
			if e := lineRest(src, q+1+n); e > 0 {
				x.Emit(fh.Keyword, i, q+1+n)
				x.Emit(fh.GenericEmph, q+1+n, e)
				return e
			}
		}
	}
	if strings.HasPrefix(src[i:], "```") {
		// ^(```\n)([\w\W]*?)(^```$)
		if i+3 < len(src) && src[i+3] == '\n' {
			if c := closingFence(src, i+4); c >= 0 {
				x.Emit(fh.LiteralString, i, i+4)
				x.Emit(fh.Text, i+4, c)
				x.Emit(fh.LiteralString, c, c+3)
				return c + 3
			}
		}
		// ^(```)(\w+)(\n)([\w\W]*?)(^```$). The contents are read as the language given by (\w+).
		w := i + 3 + lexkit.Span(src, i+3, lexkit.IsWord)
		if w > i+3 && w < len(src) && src[w] == '\n' {
			if c := closingFence(src, w+1); c >= 0 {
				x.Emit(fh.LiteralString, i, i+3)
				x.Emit(fh.LiteralString, i+3, w)
				x.Emit(fh.LiteralString, w, w+1)
				if lex := l.lookup(src[i+3 : w]); lex != nil {
					l.emitTokens(w+1, lex(src[w+1:c]))
				} else {
					x.Emit(fh.Text, w+1, c)
				}
				x.Emit(fh.LiteralString, c, c+3)
				return c + 3
			}
		}
	}
	return l.inline(src, i)
}

// digits returns the number of bytes of [0-9] at the start of src[i:].
func digits(src string, i int) int {
	n := 0
	for i+n < len(src) && '0' <= src[i+n] && src[i+n] <= '9' {
		n++
	}
	return n
}

// comment returns the length that the regular expression <!--[\w\W]*?--> matches at the start of
// src[i:]. It returns 0 if it does not match.
func comment(src string, i int) int {
	if !strings.HasPrefix(src[i:], "<!--") {
		return 0
	}
	n := strings.Index(src[i+4:], "-->")
	if n < 0 {
		return 0
	}
	return 4 + n + 3
}

// inlineRange reads src[s:e] as new input in the inline state (Chroma's UsingSelf("inline")).
// The inline rules do not look ahead, so it is enough to read src[:e] from s.
func (l *lexer) inlineRange(s, e int) {
	src := l.src[:e]
	for s < e {
		s = l.inline(src, s)
	}
}

// notWordAt returns true if the lookahead (?=\W|\n) matches at position i of src (the character at
// i is not a word character).
func notWordAt(src string, i int) bool {
	if i >= len(src) {
		return false
	}
	r, _ := lexkit.Rune(src, i)
	return !lexkit.IsWord(r)
}

// inline adds the tokens of the rule that matches at the start of src[i:] in the inline state, and
// returns the next position.
func (l *lexer) inline(src string, i int) int {
	x := l.x
	if n := comment(src, i); n > 0 {
		x.Emit(fh.CommentMultiline, i, i+n)
		return i + n
	}
	c := src[i]
	// \\.
	if c == '\\' && i+1 < len(src) && src[i+1] != '\n' {
		_, n := lexkit.Rune(src, i+1)
		x.Emit(fh.Text, i, i+1+n)
		return i + 1 + n
	}
	// The emphasis rules start with (\s). In production, the start of a line (^) is also accepted.
	if r, n := lexkit.Rune(src, i); lexkit.IsSpace(r) {
		if e := l.emphasis(src, i, i+n); e > 0 {
			return e
		}
	} else if !l.o.Chroma && (i == 0 || src[i-1] == '\n') && (c == '*' || c == '_' || c == '~') {
		if e := l.emphasis(src, i, i); e > 0 {
			return e
		}
	}
	switch c {
	case '`': // `[^`]+`
		if n := strings.IndexByte(src[i+1:], '`'); n > 0 {
			x.Emit(fh.LiteralStringBacktick, i, i+1+n+1)
			return i + 1 + n + 1
		}
	case '@', '#': // [@#][\w/:]+
		if n := lexkit.Span(src, i+1, entityRune); n > 0 {
			x.Emit(fh.NameEntity, i, i+1+n)
			return i + 1 + n
		}
	case '!', '[': // (!?\[)([^]]+)(\])(\()([^)]+)(\))
		if e := l.link(src, i); e > 0 {
			return e
		}
	}
	_, n := lexkit.Rune(src, i) // .|\n
	x.Emit(fh.Text, i, i+n)
	return i + n
}

func entityRune(r rune) bool { return r == '/' || r == ':' || lexkit.IsWord(r) }

// emphasis tries the three emphasis rules, taking src[i:q] as the preceding space ((\s), or, in
// production, the empty string at the start of a line). If one matches, it adds the tokens and
// returns the end position; otherwise it returns 0.
func (l *lexer) emphasis(src string, i, q int) int {
	x := l.x
	if q >= len(src) {
		return 0
	}
	m := src[q]
	if m != '*' && m != '_' && m != '~' {
		return 0
	}
	if m != '~' {
		// (\s)(\*|_)((?:(?!\2).)*)(\2)((?=\W|\n)). The contents are a run of characters other than the
		// marker and a newline, and backtracking does not change that.
		e := q + 1
		for e < len(src) && src[e] != m && src[e] != '\n' {
			e++
		}
		if e < len(src) && src[e] == m && notWordAt(src, e+1) {
			x.Emit(fh.Text, i, q)
			x.Emit(fh.GenericEmph, q, q+1)
			x.Emit(fh.GenericEmph, q+1, e)
			x.Emit(fh.GenericEmph, e, e+1)
			return e + 1
		}
		// (\s)((\*\*|__).*?)\3((?=\W|\n)). Chroma adds the nested third group (the opening marker) after
		// the second group, so the opening marker appears in place of the closing marker (it is the same
		// string, so it is added at the position of the closing marker).
		if q+1 < len(src) && src[q+1] == m {
			for e := q + 2; ; {
				if e+1 < len(src) && src[e] == m && src[e+1] == m && notWordAt(src, e+2) {
					x.Emit(fh.Text, i, q)
					x.Emit(fh.GenericStrong, q, e)
					x.Emit(fh.GenericStrong, e, e+2)
					return e + 2
				}
				if e >= len(src) || src[e] == '\n' {
					return 0
				}
				_, n := lexkit.Rune(src, e)
				e += n
			}
		}
		return 0
	}
	// (\s)(~~[^~]+~~)((?=\W|\n))
	if !strings.HasPrefix(src[q:], "~~") {
		return 0
	}
	n := strings.IndexByte(src[q+2:], '~')
	if n <= 0 {
		return 0
	}
	e := q + 2 + n
	if !strings.HasPrefix(src[e:], "~~") || !notWordAt(src, e+2) {
		return 0
	}
	x.Emit(fh.Text, i, q)
	x.Emit(fh.GenericDeleted, q, e+2)
	return e + 2
}

// link tries (!?\[)([^]]+)(\])(\()([^)]+)(\)) at the start of src[i:]. It returns the end position if
// it matches, or 0 if it does not.
func (l *lexer) link(src string, i int) int {
	o := i
	if src[o] == '!' {
		o++
	}
	if o >= len(src) || src[o] != '[' {
		return 0
	}
	n := l.closeBracket.Index(src, o+1, "]")
	if n <= 0 {
		return 0
	}
	b := o + 1 + n // ]
	if b+1 >= len(src) || src[b+1] != '(' {
		return 0
	}
	n = l.closeParen.Index(src, b+2, ")")
	if n <= 0 {
		return 0
	}
	p := b + 2 + n // )
	x := l.x
	x.Emit(fh.Text, i, o+1)
	x.Emit(fh.NameTag, o+1, b)
	x.Emit(fh.Text, b, b+1)
	x.Emit(fh.Text, b+1, b+2)
	x.Emit(fh.NameAttribute, b+2, p)
	x.Emit(fh.Text, p, p+1)
	return p + 1
}
