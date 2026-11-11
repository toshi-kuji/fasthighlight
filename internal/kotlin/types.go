package kotlin

import (
	"strings"
	"unicode/utf8"

	"github.com/toshi-kuji/fasthighlight/internal/lexkit"
)

// maxType is how far the type scans look. A type is short; the limit keeps a long run of "<" or ":"
// from being scanned again from each of them, so the time stays linear.
const maxType = 512

// isNameChar reports whether r can be in a name ([\p{L}\p{N}] and _ ).
func isNameChar(r rune) bool { return r == '_' || isCont(r) }

// markType extends typeEnd when the ":" or "<" at i starts a type (production only):
//   - ":" (not part of ::, ?: or an annotation's use-site target such as @file:) is followed by a
//     type in Kotlin: a declaration, a parameter, a return type after ")", or a supertype. The names
//     of the dotted name after it are types.
//   - "<" right after a name (no space between) starts type arguments when typeArgsEnd accepts them.
func (l *lexer) markType(i int) {
	e := 0
	switch l.src[i] {
	case ':':
		if l.annotationTarget(i) {
			return
		}
		e = l.colonTypeEnd(i + 1)
	case '<':
		r, _ := utf8.DecodeLastRuneInString(l.src[:i])
		if i == 0 || !isNameChar(r) && r != '`' {
			return
		}
		e = l.typeArgsEnd(i)
	}
	if e > l.typeEnd {
		l.typeEnd = e
	}
}

// annotationTarget reports whether the ":" at i follows @name (as in @file:JvmName or @get:Rule).
func (l *lexer) annotationTarget(i int) bool {
	src := l.src
	for k := i; k > 0 && k > i-maxType; {
		r, w := utf8.DecodeLastRuneInString(src[:k])
		if r == '@' {
			return true
		}
		if !isNameChar(r) {
			return false
		}
		k -= w
	}
	return false
}

// colonTypeEnd returns the end of the dotted name (a.b.C) that starts after spaces at i, or 0.
// Type arguments after it are found when the lexer reaches their "<".
func (l *lexer) colonTypeEnd(i int) int {
	j := i
	for j < len(l.src) && j < i+maxType && (l.src[j] == ' ' || l.src[j] == '\t') {
		j++
	}
	e := 0
	for j < len(l.src) && j < i+maxType {
		n := l.ident(j)
		if n == 0 {
			break
		}
		e = j + n
		if l.at(e) != '.' {
			break
		}
		j = e + 1
	}
	return e
}

// typeArgsEnd returns the end of the type arguments that start at the "<" at i, or 0 if what follows
// does not look like type arguments: names, ".", ",", ":", "?", "*", "&" (not &&), whitespace and nested
// "< >", closed by ">" within maxType bytes. Anything else (an operator such as && or ||, a number, a
// parenthesis, a string) means that "<" is a comparison, as in a<b && c>d. After the closing ">", a
// name or number (other than by and where) also means a comparison, as in f(a<b, c>d).
func (l *lexer) typeArgsEnd(i int) int {
	if l.argsFrom < i && i < l.argsStop {
		return 0
	}
	src := l.src
	depth := 0
	inName := false
	j := i
	for j < len(src) && j < i+maxType {
		r, w := lexkit.Rune(src, j)
		word := false
		switch {
		case r == '<':
			depth++
		case r == '>':
			depth--
			if depth == 0 {
				if l.followsType(j + 1) {
					return j + 1
				}
				return l.argsFailed(i, j)
			}
		case r == '&' && l.at(j+1) == '&':
			return l.argsFailed(i, j)
		case isNameChar(r):
			if !inName && lexkit.IsDigit(r) {
				return l.argsFailed(i, j)
			}
			word = true
		case lexkit.IsSpace(r), r == '.', r == ',', r == ':', r == '?', r == '*', r == '&':
		default:
			return l.argsFailed(i, j)
		}
		inName = word
		j += w
	}
	return l.argsFailed(i, j)
}

// argsFailed remembers that the type-argument scan from i failed at stop and returns 0. A later "<" before
// stop scans the same characters to the same place and fails too (or, inside an unclosed run, would at
// most find shorter type arguments, which are left as names), so it is not scanned again: without this,
// each "<" of a long run such as a<a<a<… would scan maxType bytes.
func (l *lexer) argsFailed(i, stop int) int {
	if stop > l.argsStop {
		l.argsFrom, l.argsStop = i, stop
	}
	return 0
}

// followsType reports whether what comes at i can follow type arguments: not a name or a number,
// except the keywords by and where after a space.
func (l *lexer) followsType(i int) bool {
	j := i
	for j < len(l.src) && j < i+maxType && (l.src[j] == ' ' || l.src[j] == '\t') {
		j++
	}
	r, _ := l.runeAt(j)
	if r == '"' || r == '\'' {
		return false
	}
	if r < 0 || !isNameChar(r) {
		return true
	}
	if j == i {
		return false
	}
	for _, w := range [...]string{"by", "where"} {
		if strings.HasPrefix(l.src[j:], w) {
			if r, _ := l.runeAt(j + len(w)); !isNameChar(r) {
				return true
			}
		}
	}
	return false
}
