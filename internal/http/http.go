// Package http is the HTTP tokenizer. It is used by lang/http.
//
// The token types and boundaries reproduce the rules of Chroma 2.27.0's lexers/http.go without regular
// expressions, including its httpBodyContentTypeLexer: the message body is highlighted in the language
// that Chroma chooses for the Content-Type header (lexers.MatchMimeType, see mime.go). JSON, XML, HTML,
// CSS, JavaScript and INI are read by the tokenizers in internal/ (as with Chroma's using, their tokens
// are merged with the surrounding ones); the other languages are looked up among the registered
// languages. A body without a Content-Type stays one Generic token, and a body whose content type has no
// language is Text, as in Chroma.
//
// Chroma reads the body with the language's own Tokenise, so a language with ensure_nl (such as
// JavaScript) gets a newline appended to a body that does not end with one. That newline is not part of
// the input, so it is left out here (the tokens joined are always the input).
package http

import (
	"strings"

	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/css"
	"github.com/toshi-kuji/fasthighlight/internal/html"
	"github.com/toshi-kuji/fasthighlight/internal/ini"
	"github.com/toshi-kuji/fasthighlight/internal/js"
	"github.com/toshi-kuji/fasthighlight/internal/json"
	"github.com/toshi-kuji/fasthighlight/internal/lexkit"
	"github.com/toshi-kuji/fasthighlight/internal/xml"
)

// Options switches the differences from Chroma inside the body. The zero value is the production setting
// (the one lang/http registers).
type Options struct {
	// JS is passed to a JavaScript body and to the JavaScript inside an HTML body.
	JS js.Options
	// Lookup returns, from the name of a language that is not read by a tokenizer in internal/, the
	// function that tokenizes in that language (with the same contract as fasthighlight.Language.Tokenise,
	// so it may append a newline). It returns nil if there is no such language. If Lookup itself is nil,
	// Tokenise of the registered language (fasthighlight.LookupRegistered) is used. compat/ uses it to
	// compare the other languages in their Chroma-matching versions too.
	Lookup func(name string) func(code string) []fh.Token
}

// Prepare builds the tables of the languages read inside (CSS, also inside HTML).
func Prepare() { html.Prepare() }

// The states of Chroma's http.go.
type state int

const (
	root    state = iota
	headers       // after the request or status line
	content       // after the empty line that ends the headers
)

type lexer struct {
	o   Options
	x   *lexkit.Tokens
	src string
	// isContentType and contentType follow httpBodyContentTyper: a Name token "content-type" (in any case)
	// makes the next Literal token the content type.
	isContentType bool
	contentType   string
}

// Lex splits src into tokens (the same contract as Lex of fasthighlight.Language; HTTP does not append a
// newline at the end, so end is len(src)).
func Lex(src string, end int, o Options) []fh.Token {
	// A body in a language with ensure_nl is read with a newline appended (as in Chroma), so the tokens
	// are collected over text, and the appended newline is cut off at the end.
	text := src
	if !strings.HasSuffix(src, "\n") {
		text += "\n"
	}
	l := &lexer{o: o, x: lexkit.NewTokens(text), src: src}
	if b := l.head(end); b < end {
		l.body(b)
	}
	toks := l.x.Result()
	if len(text) > len(src) {
		// Cut off the appended newline if a body token included it.
		total := 0
		for _, t := range toks {
			total += len(t.Value)
		}
		if total > len(src) {
			last := &toks[len(toks)-1]
			if last.Value = last.Value[:len(last.Value)-1]; last.Value == "" {
				toks = toks[:len(toks)-1]
			}
		}
	}
	return toks
}

// head reads the start line and the headers (from 0 until the content state, but not at or after end)
// and returns where the body starts (end if there is none).
func (l *lexer) head(end int) int {
	src := l.src
	st := root
	for i := 0; i < end; {
		var n int
		switch st {
		case root:
			if n = l.requestLine(i); n == 0 {
				n = l.statusLine(i)
			}
			if n > 0 {
				st = headers
			}
		case headers:
			if n = l.header(i); n == 0 {
				n = l.continuation(i)
			}
			if n == 0 && src[i] == '\n' { // \r?\n
				l.x.Emit(fh.Text, i, i+1)
				n, st = 1, content
			}
		case content: // .+ (dot_all) matches the rest
			return i
		}
		if n == 0 {
			// No rule matches. The headers state always matches a newline, so going back to root on a
			// newline never happens.
			n = l.x.Error(i)
		}
		i += n
	}
	return end
}

// emit adds the token of one group. Like httpBodyContentTyper, it watches the Name and Literal tokens
// for the content type.
func (l *lexer) emit(t fh.TokenType, s, e int) {
	switch {
	case t == fh.Name && strings.ToLower(l.src[s:e]) == "content-type":
		l.isContentType = true
	case t == fh.Literal && l.isContentType:
		l.isContentType = false
		ct := strings.TrimSpace(l.src[s:e])
		if pos := strings.Index(ct, ";"); pos > 0 {
			ct = strings.TrimSpace(ct[:pos])
		}
		l.contentType = ct
	}
	l.x.Emit(t, s, e)
}

// newlineOrEnd returns the end of (\r?\n|\Z) at i, or -1 if it does not match.
func (l *lexer) newlineOrEnd(i int) int {
	switch {
	case i == len(l.src):
		return i
	case l.src[i] == '\n':
		return i + 1
	}
	return -1
}

func spaces(src string, i int) int {
	j := i
	for j < len(src) && src[j] == ' ' {
		j++
	}
	return j - i
}

// version returns the end of HTTP/[123](?:\.[01])? at i, or -1 if it does not match. The optional
// part is taken when present; dropping it never helps, because then a . follows the digit, and what
// follows must be a space or the end of the line.
func version(src string, i int) int {
	if !strings.HasPrefix(src[i:], "HTTP/") {
		return -1
	}
	j := i + 5
	if j >= len(src) || src[j] < '1' || src[j] > '3' {
		return -1
	}
	j++
	if j+1 < len(src) && src[j] == '.' && (src[j+1] == '0' || src[j+1] == '1') {
		j += 2
	}
	return j
}

var methods = [...]string{"GET", "POST", "PUT", "DELETE", "HEAD", "OPTIONS", "TRACE", "PATCH", "CONNECT"}

// requestLine tries
// (GET|POST|…|CONNECT)( +)([^ ]+)( +)(HTTP)(/)([123](?:\.[01])?)(\r?\n|\Z) at i and returns the length
// matched (0 if it does not match). With dot_all, [^ ]+ also takes newlines; it ends at the first space,
// because a space must follow it.
func (l *lexer) requestLine(i int) int {
	src := l.src
	m := ""
	for _, w := range methods {
		if strings.HasPrefix(src[i:], w) {
			m = w
			break
		}
	}
	if m == "" {
		return 0
	}
	a := i + len(m)
	b := a + spaces(src, a)
	if b == a {
		return 0
	}
	c := b + strings.IndexByte(src[b:], ' ')
	if c < b { // no space after the target (src[b] is not a space, so the target is not empty)
		return 0
	}
	d := c + spaces(src, c)
	v := version(src, d)
	if v < 0 {
		return 0
	}
	e := l.newlineOrEnd(v)
	if e < 0 {
		return 0
	}
	l.emit(fh.NameFunction, i, a)
	l.emit(fh.Text, a, b)
	l.emit(fh.NameNamespace, b, c)
	l.emit(fh.Text, c, d)
	l.emit(fh.KeywordReserved, d, d+4)
	l.emit(fh.Operator, d+4, d+5)
	l.emit(fh.LiteralNumber, d+5, v)
	l.emit(fh.Text, v, e)
	return e - i
}

// statusLine tries (HTTP)(/)([123](?:\.[01])?)( +)(\d{3})( *)([^\r\n]*)(\r?\n|\Z) at i and returns
// the length matched (0 if it does not match).
func (l *lexer) statusLine(i int) int {
	src := l.src
	v := version(src, i)
	if v < 0 {
		return 0
	}
	a := v + spaces(src, v)
	if a == v {
		return 0
	}
	b := a
	for range 3 {
		if b >= len(src) {
			return 0
		}
		r, w := lexkit.Rune(src, b)
		if !lexkit.IsDigit(r) {
			return 0
		}
		b += w
	}
	c := b + spaces(src, b)
	d := lexkit.LineEnd(src, c)
	e := l.newlineOrEnd(d)
	l.emit(fh.KeywordReserved, i, i+4)
	l.emit(fh.Operator, i+4, i+5)
	l.emit(fh.LiteralNumber, i+5, v)
	l.emit(fh.Text, v, a)
	l.emit(fh.LiteralNumber, a, b)
	l.emit(fh.Text, b, c)
	l.emit(fh.NameException, c, d)
	l.emit(fh.Text, d, e)
	return e - i
}

func isNameChar(r rune) bool { return !lexkit.IsSpace(r) && r != ':' }

// header tries ([^\s:]+)( *)(:)( *)([^\r\n]+)(\r?\n|\Z) at i and returns the length matched (0 if it
// does not match). When nothing but spaces follows the colon, ( *) gives one space back to ([^\r\n]+).
func (l *lexer) header(i int) int {
	src := l.src
	a := i + lexkit.Span(src, i, isNameChar)
	if a == i {
		return 0
	}
	b := a + spaces(src, a)
	if b >= len(src) || src[b] != ':' {
		return 0
	}
	c := b + 1
	d := c + spaces(src, c)
	e := lexkit.LineEnd(src, d)
	if e == d {
		if d == c {
			return 0
		}
		d--
	}
	f := l.newlineOrEnd(e)
	l.emit(fh.Name, i, a)
	l.emit(fh.Text, a, b)
	l.emit(fh.Operator, b, c)
	l.emit(fh.Text, c, d)
	l.emit(fh.Literal, d, e)
	l.emit(fh.Text, e, f)
	return f - i
}

// continuation tries ([\t ]+)([^\r\n]+)(\r?\n|\Z) at i and returns the length matched (0 if it does not
// match). When nothing but blanks is on the line, ([\t ]+) gives one character back to ([^\r\n]+).
func (l *lexer) continuation(i int) int {
	src := l.src
	a := i
	for a < len(src) && (src[a] == ' ' || src[a] == '\t') {
		a++
	}
	if a == i {
		return 0
	}
	e := lexkit.LineEnd(src, a)
	if e == a {
		if a-i < 2 {
			return 0
		}
		a--
	}
	f := l.newlineOrEnd(e)
	l.emit(fh.Text, i, a)
	l.emit(fh.Literal, a, e)
	l.emit(fh.Text, e, f)
	return f - i
}

// body reads src[i:] (the Generic token of Chroma's content state).
func (l *lexer) body(i int) {
	src := l.src
	if l.contentType == "" {
		l.x.Emit(fh.Generic, i, len(src))
		return
	}
	name := MimeLanguage(l.contentType)
	// plain is the body; a language with ensure_nl reads it with a newline appended.
	plain, end := src[i:], len(src)-i
	switch name {
	case "":
		l.x.Emit(fh.Text, i, len(src))
	case "JSON":
		json.LexInto(l.x.Sub(plain, i), plain, end)
	case "XML":
		xml.LexInto(l.x.Sub(plain, i), plain, end)
	case "HTML":
		html.LexInto(l.x.Sub(plain, i), plain, end, l.o.JS)
	case "CSS":
		css.LexInto(l.x.Sub(plain, i), plain, end)
	case "INI":
		ini.LexInto(l.x.Sub(plain, i), plain, end)
	case "JavaScript": // ensure_nl
		sub := plain
		if !strings.HasSuffix(sub, "\n") {
			sub += "\n"
		}
		js.LexInto(l.x.Sub(sub, i), sub, end, l.o.JS)
	default:
		lex := l.lookup(name)
		if lex == nil {
			l.x.Emit(fh.Text, i, len(src))
			return
		}
		at := i
		for _, t := range lex(plain) {
			l.x.Emit(t.Type, at, at+len(t.Value))
			at += len(t.Value)
		}
	}
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

// ContentType returns the content type that chooses the language of the body of src, the same as
// Chroma's httpBodyContentTyper finds it: the value of the last Content-Type header, without spaces
// around it and without anything from the first ; on. It returns "" if there is none. src must have its
// newlines normalized. compat/ uses it to skip bodies in languages fasthighlight does not support.
func ContentType(src string) string {
	l := &lexer{x: lexkit.NewTokens(src), src: src}
	l.head(len(src))
	return l.contentType
}
