package golang

import (
	"strings"

	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/lexkit"
)

// Inside a Go raw string (`…`), Chroma reads the text as a Go template (go_template.xml) and turns Other
// tokens into LiteralString. This file reproduces those rules.

// States of the template.
type tstate uint8

const (
	troot tstate = iota
	ttemplate
	tsubexpr
)

type tlexer struct {
	o     Options
	x     *lexkit.Tokens
	src   string
	stack []tstate
	words lexkit.WordCache
}

// lexTemplate reads src as a Go template and adds it to x (as with Chroma's using, it does not append
// a newline at the end).
func lexTemplate(x *lexkit.Tokens, src string, o Options) {
	l := &tlexer{o: o, x: x, src: src, stack: []tstate{troot}, words: lexkit.WordCache{Lists: lists}}
	for i := 0; i < len(src); {
		st := l.stack[len(l.stack)-1]
		n, ok := l.step(st, i)
		if !ok {
			if src[i] == '\n' && st != troot {
				l.stack = append(l.stack[:0], troot)
				continue
			}
			n = l.x.Error(i)
		}
		i += n
	}
}

func (l *tlexer) emit(t fh.TokenType, s, e int) (int, bool) { l.x.Emit(t, s, e); return e - s, true }

func (l *tlexer) at(i int) byte {
	if i < len(l.src) {
		return l.src[i]
	}
	return 0
}

func (l *tlexer) step(st tstate, i int) (int, bool) {
	src := l.src
	switch st {
	case troot:
		if strings.HasPrefix(src[i:], "{{") {
			// {{(- )?/\*(.|\n)*?\*/( -)?}}
			j := i + 2
			if strings.HasPrefix(src[j:], "- /*") {
				j += 2
			}
			if strings.HasPrefix(src[j:], "/*") {
				for m := j + 2; ; {
					k := strings.Index(src[m:], "*/")
					if k < 0 {
						break
					}
					e := m + k + 2
					if strings.HasPrefix(src[e:], " -}}") {
						return l.emit(fh.CommentMultiline, i, e+4)
					}
					if strings.HasPrefix(src[e:], "}}") {
						return l.emit(fh.CommentMultiline, i, e+2)
					}
					m += k + 1
				}
			}
			// {{[-]?
			e := i + 2
			if l.at(e) == '-' {
				e++
			}
			l.stack = append(l.stack, ttemplate)
			return l.emit(fh.CommentPreproc, i, e)
		}
		// [^{]+ and { (turns Other into LiteralString)
		if src[i] == '{' {
			return l.emit(fh.LiteralString, i, i+1)
		}
		j := i + 1
		for j < len(src) && src[j] != '{' {
			j++
		}
		return l.emit(fh.LiteralString, i, j)
	case ttemplate:
		// [-]?}}
		if strings.HasPrefix(src[i:], "}}") {
			l.stack = l.stack[:len(l.stack)-1]
			return l.emit(fh.CommentPreproc, i, i+2)
		}
		if strings.HasPrefix(src[i:], "-}}") {
			l.stack = l.stack[:len(l.stack)-1]
			return l.emit(fh.CommentPreproc, i, i+3)
		}
		if src[i] == '(' {
			l.stack = append(l.stack, tsubexpr)
			return l.emit(fh.Operator, i, i+1)
		}
		if src[i] == '"' {
			if n := quotedLen(src, i); n > 0 {
				return l.emit(fh.LiteralString, i, i+n)
			}
		}
		return l.expression(i)
	case tsubexpr:
		if src[i] == ')' {
			l.stack = l.stack[:len(l.stack)-1]
			return l.emit(fh.Operator, i, i+1)
		}
		return l.expression(i)
	}
	return 0, false
}

// expression tries the rules of the expression state.
func (l *tlexer) expression(i int) (int, bool) {
	src := l.src
	c := src[i]
	if n := lexkit.Span(src, i, lexkit.IsSpace); n > 0 {
		return l.emit(fh.TextWhitespace, i, i+n)
	}
	if c == '(' {
		l.stack = append(l.stack, tsubexpr)
		return l.emit(fh.Operator, i, i+1)
	}
	if n := l.words.Match(src, i, lTemplateKeywords); n > 0 {
		return l.emit(fh.Keyword, i, i+n)
	}
	// \||:?=|,
	switch {
	case c == '|', c == ',', c == '=':
		return l.emit(fh.Operator, i, i+1)
	case c == ':' && l.at(i+1) == '=':
		return l.emit(fh.Operator, i, i+2)
	}
	// [$]?[^\W\d]\w* (in regexp2, [^\W\d] is the same as \w. See Options.ChromaIdent)
	j := i
	if c == '$' {
		j++
	}
	if n, _ := l.words.Word(src, j); n > 0 && (l.o.ChromaIdent || !lexkit.IsDigit(runeAt(src, j))) {
		return l.emit(fh.NameOther, i, j+n)
	}
	// \$|[$]?\.(?:[^\W\d]\w*)? (\$ matches first, so $. becomes just $)
	if c == '$' {
		return l.emit(fh.NameAttribute, i, i+1)
	}
	if c == '.' {
		n, _ := l.words.Word(src, i+1)
		if n > 0 && !l.o.ChromaIdent && lexkit.IsDigit(runeAt(src, i+1)) {
			n = 0 // [^\W\d] does not match a digit
		}
		return l.emit(fh.NameAttribute, i, i+1+n)
	}
	if c == '"' {
		if n := quotedLen(src, i); n > 0 {
			return l.emit(fh.LiteralString, i, i+n)
		}
	}
	if e, t := l.number(i); e > 0 {
		return l.emit(t, i, e)
	}
	if c == '\'' {
		if n := charLen(src, i); n > 0 {
			return l.emit(fh.LiteralStringChar, i, i+n)
		}
	}
	if c == '`' { // `[^`]*`
		if k := strings.IndexByte(src[i+1:], '`'); k >= 0 {
			return l.emit(fh.LiteralString, i, i+k+2)
		}
	}
	return 0, false
}

// number tries the number rules of expression (a leading - is allowed).
func (l *tlexer) number(i int) (int, fh.TokenType) {
	src := l.src
	digits := func(j int) int { return lexkit.Span(src, j, lexkit.IsDigit) }
	signedExp := func(j int) int {
		if c := l.at(j); (c == 'e' || c == 'E') && (l.at(j+1) == '-' || l.at(j+1) == '+') {
			if d := digits(j + 2); d > 0 {
				return 2 + d
			}
		}
		return 0
	}
	optExp := func(j int) int {
		if c := l.at(j); c != 'e' && c != 'E' {
			return 0
		}
		k := j + 1
		if c := l.at(k); (c == '-' || c == '+') && digits(k+1) > 0 {
			k++
		}
		if d := digits(k); d > 0 {
			return k + d - j
		}
		return 0
	}
	m := i // after the -
	if l.at(i) == '-' {
		m++
	}
	d := digits(m)
	if d > 0 {
		if l.at(m+d) == 'i' { // -?\d+i
			return m + d + 1, fh.LiteralNumber
		}
		if l.at(m+d) == '.' { // -?\d+\.\d*([Ee][-+]\d+)?i
			j := m + d + 1
			j += digits(j)
			if x := signedExp(j); x > 0 && l.at(j+x) == 'i' {
				return j + x + 1, fh.LiteralNumber
			}
			if l.at(j) == 'i' {
				return j + 1, fh.LiteralNumber
			}
		}
	}
	if src[i] == '.' { // \.\d+([Ee][-+]\d+)?i
		if d2 := digits(i + 1); d2 > 0 {
			j := i + 1 + d2
			if x := signedExp(j); x > 0 && l.at(j+x) == 'i' {
				return j + x + 1, fh.LiteralNumber
			}
			if l.at(j) == 'i' {
				return j + 1, fh.LiteralNumber
			}
		}
	}
	if d > 0 {
		if x := signedExp(m + d); x > 0 && l.at(m+d+x) == 'i' { // -?\d+[Ee][-+]\d+i
			return m + d + x + 1, fh.LiteralNumber
		}
		j := m + d // -?\d+(\.\d+[eE][+\-]?\d+|\.\d*|[eE][+\-]?\d+)
		if l.at(j) == '.' {
			if d2 := digits(j + 1); d2 > 0 {
				if x := optExp(j + 1 + d2); x > 0 {
					return j + 1 + d2 + x, fh.LiteralNumberFloat
				}
			}
			return j + 1 + digits(j+1), fh.LiteralNumberFloat
		}
		if x := optExp(j); x > 0 {
			return j + x, fh.LiteralNumberFloat
		}
	}
	if l.at(m) == '.' { // -?\.\d+([eE][+\-]?\d+)?
		if d2 := digits(m + 1); d2 > 0 {
			return m + 1 + d2 + optExp(m+1+d2), fh.LiteralNumberFloat
		}
	}
	if l.at(m) == '0' {
		isOct := func(c byte) bool { return '0' <= c && c <= '7' }
		if isOct(l.at(m + 1)) { // -?0[0-7]+
			k := m + 1
			for isOct(l.at(k)) {
				k++
			}
			return k, fh.LiteralNumberOct
		}
		if c := l.at(m + 1); (c == 'x' || c == 'X') && isHex(l.at(m+2)) { // -?0[xX][0-9a-fA-F]+
			k := m + 2
			for isHex(l.at(k)) {
				k++
			}
			return k, fh.LiteralNumberHex
		}
		if l.at(m+1) == 'b' { // -?0b[01_]+
			isBin := func(c byte) bool { return c == '0' || c == '1' || c == '_' }
			if isBin(l.at(m + 2)) {
				k := m + 2
				for isBin(l.at(k)) {
					k++
				}
				return k, fh.LiteralNumberBin
			}
		}
		return m + 1, fh.LiteralNumberInteger
	}
	if c := l.at(m); '1' <= c && c <= '9' { // -?[1-9][0-9]*
		k := m + 1
		for '0' <= l.at(k) && l.at(k) <= '9' {
			k++
		}
		return k, fh.LiteralNumberInteger
	}
	return 0, 0
}
