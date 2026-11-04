package fasthighlight

import (
	"bufio"
	"io"
	"strings"
)

// Writing HTML. The output is byte-for-byte the same as Chroma 2.27.0's formatters/html used with
// WithClasses(true), except for the outer wrapper's class name (Chroma uses chroma, here it is highlight)
// and the data-lang below (Copyright (C) 2017 Alec Thomas, MIT License; see THIRD_PARTY_NOTICES.md).
//
// The shape of the output:
//
//	<pre class="highlight"><code><span class="line"><span class="cl">…</span></span>…</code></pre>
//
// When the code block's language name is known (Highlight, and the markdown package's Extension and
// Converter), <pre> gets data-lang="language name" (Chroma's output does not have it). WriteHTML does not
// know the language name, so it does not add it.
//
// Each line is wrapped in <span class="line"><span class="cl">, and tokens are wrapped in a <span> with the
// type's class name. Types whose class name is empty (such as Text) are not wrapped. Lines are split as
// in Chroma's SplitTokensIntoLines, and the newline is included at the end of the line. If the last line
// is just one empty token, that line is not written.

const (
	preStart  = `<pre class="highlight"><code>`
	preLang   = `<pre class="highlight" data-lang="`
	preEnd    = `</code></pre>`
	lineStart = `<span class="line"><span class="cl">`
	lineEnd   = `</span></span>`
	spanEnd   = `</span>`
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

// writeHTML is the same as WriteHTML. If lang is not empty, <pre> gets data-lang="lang".
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
		hw.str(`"><code>`)
	}
	for _, t := range toks {
		hw.token(t)
	}
	hw.finish()
	hw.str(preEnd)
	if bw != nil && hw.err == nil {
		hw.err = bw.Flush()
	}
	return hw.err
}

type htmlWriter struct {
	w      io.StringWriter
	err    error
	inLine bool
	// pending are the empty tokens met at the start of a line. Chroma does not write the last line if it is
	// just one empty token, so writing waits until more comes.
	pending []TokenType
}

func (h *htmlWriter) str(s string) {
	if h.err == nil {
		_, h.err = h.w.WriteString(s)
	}
}

func (h *htmlWriter) token(t Token) {
	v := t.Value
	if v == "" {
		if h.inLine {
			h.piece(t.Type, "")
		} else {
			h.pending = append(h.pending, t.Type)
		}
		return
	}
	for v != "" {
		n := strings.IndexByte(v, '\n') + 1
		if n == 0 {
			n = len(v)
		}
		h.piece(t.Type, v[:n])
		if v[n-1] == '\n' {
			h.str(lineEnd)
			h.inLine = false
		}
		v = v[n:]
	}
}

// piece writes part of the tokens within one line. It starts the line if necessary.
func (h *htmlWriter) piece(t TokenType, s string) {
	if !h.inLine {
		h.str(lineStart)
		h.inLine = true
		for _, p := range h.pending {
			h.span(p, "")
		}
		h.pending = h.pending[:0]
	}
	h.span(t, s)
}

func (h *htmlWriter) span(t TokenType, s string) {
	start := spanStart(t)
	if start == "" {
		h.escape(s)
		return
	}
	h.str(start)
	h.escape(s)
	h.str(spanEnd)
}

func (h *htmlWriter) finish() {
	switch {
	case h.inLine:
		h.str(lineEnd)
	case len(h.pending) > 1:
		// Chroma also writes a last line that consists only of two or more empty tokens.
		h.str(lineStart)
		for _, p := range h.pending {
			h.span(p, "")
		}
		h.str(lineEnd)
	}
	h.inLine = false
	h.pending = h.pending[:0]
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
// it is put in the <pre> data-lang. A language that is not registered is written as Text without
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
