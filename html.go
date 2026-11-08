package fasthighlight

import (
	"bufio"
	"io"
)

// Writing HTML. The class names of the tokens are Chroma's (WithClasses(true) in formatters/html; Copyright (C)
// 2017 Alec Thomas, MIT License; see THIRD_PARTY_NOTICES.md), but the shape is smaller than Chroma's: lines are
// not wrapped (Chroma wraps each one in <span class="line"><span class="cl"> for line numbers and highlighted
// lines, which fhl does not have), and whitespace is written as it is, without <span class="w">.
//
// The shape of the output:
//
//	<pre class="highlight"><code><span class="kd">func</span> <span class="nf">main</span>…</code></pre>
//
// When the code block's language name is known (Highlight, and the markdown package's Extension and
// Converter), <pre> gets data-lang="language name" and <code> gets class="language-language name", the
// class CommonMark recommends and other tools read (Chroma's output has neither). Both hold the name as
// written in the code block. WriteHTML does not know the language name, so it adds neither.
//
// Each token is wrapped in a <span> with its type's class name. Tokens whose class name is empty (such as
// Text) and whitespace (TextWhitespace) are written without one. Empty tokens are not written.

const (
	preStart = `<pre class="highlight"><code>`
	preLang  = `<pre class="highlight" data-lang="`
	preEnd   = `</code></pre>`
	spanEnd  = `</span>`
)

// spanStarts are the opening tags for each type. For a type whose class name is empty, it is "".
var spanStarts = func() map[TokenType]string {
	m := make(map[TokenType]string, len(typeNames))
	for t := range typeNames {
		m[t] = spanStartOf(t)
	}
	return m
}()

// classOf is the same as Chroma's Formatter.class. A type not in the table uses the class name of its parent type that is in the table.
func classOf(t TokenType) string {
	for t != 0 {
		if c, ok := classNames[t]; ok {
			return c
		}
		t = t.Parent()
	}
	return classNames[t]
}

func spanStartOf(t TokenType) string {
	c := classOf(t)
	if c == "" {
		return ""
	}
	return `<span class="` + c + `">`
}

func spanStart(t TokenType) string {
	if s, ok := spanStarts[t]; ok {
		return s
	}
	return spanStartOf(t)
}

// WriteHTML turns tokens into HTML and writes it to w.
func WriteHTML(w io.Writer, toks []Token) error {
	return writeHTML(w, toks, "")
}

// writeHTML is the same as WriteHTML. If lang is not empty, <pre> gets data-lang="lang" and <code> gets
// class="language-lang".
func writeHTML(w io.Writer, toks []Token, lang string) error {
	sw, ok := w.(io.StringWriter)
	var bw *bufio.Writer
	if !ok {
		bw = bufio.NewWriter(w)
		sw = bw
	}
	hw := htmlWriter{w: sw}
	if lang == "" {
		hw.str(preStart)
	} else {
		hw.str(preLang)
		hw.escape(lang)
		hw.str(`"><code class="language-`)
		hw.escape(lang)
		hw.str(`">`)
	}
	for _, t := range toks {
		hw.token(t)
	}
	hw.str(preEnd)
	if bw != nil && hw.err == nil {
		hw.err = bw.Flush()
	}
	return hw.err
}

type htmlWriter struct {
	w   io.StringWriter
	err error
}

func (h *htmlWriter) str(s string) {
	if h.err == nil {
		_, h.err = h.w.WriteString(s)
	}
}

// token writes one token: its text in a <span> with the class name of its type, or the text alone for
// whitespace and types without a class name.
func (h *htmlWriter) token(t Token) {
	if t.Value == "" {
		return
	}
	start := ""
	if t.Type != TextWhitespace {
		start = spanStart(t.Type)
	}
	if start == "" {
		h.escape(t.Value)
		return
	}
	h.str(start)
	h.escape(t.Value)
	h.str(spanEnd)
}

// escape writes s with the same replacements as html.EscapeString.
func (h *htmlWriter) escape(s string) {
	last := 0
	for i := 0; i < len(s); i++ {
		var r string
		switch s[i] {
		case '&':
			r = "&amp;"
		case '<':
			r = "&lt;"
		case '>':
			r = "&gt;"
		case '\'':
			r = "&#39;"
		case '"':
			r = "&#34;"
		default:
			continue
		}
		h.str(s[last:i])
		h.str(r)
		last = i + 1
	}
	h.str(s[last:])
}

// Highlight turns one code block into HTML. lang is the code block's language name; if it is not empty,
// it is put in the <pre> data-lang and the <code> class (language-lang). A language that is not registered is written as Text without
// highlighting (only escaped).
func Highlight(w io.Writer, lang, code string) error {
	return writeBlock(w, LookupRegistered(lang), lang, code)
}

// writeBlock turns the code block contents code into HTML for language l (not highlighted if nil) and
// writes it to w. name is the language name written in the code block; if it is not empty, it is put in
// data-lang.
func writeBlock(w io.Writer, l *Language, name, code string) error {
	if l == nil {
		return writeHTML(w, []Token{{Type: Text, Value: code}}, name)
	}
	return writeHTML(w, l.Tokenise(code), name)
}
