package csharp

import (
	"strings"
	"unicode/utf8"

	"github.com/toshi-kuji/fasthighlight/internal/lexkit"
)

// Types in type positions (production only; see Options.ChromaTypes). The lexer sets typeEnd when it
// finds the start of a type, and the names before typeEnd become KeywordType (the type of C#'s
// built-in types in Chroma's rules). Keywords and built-in types keep their own token types. Each scan
// looks at most maxType bytes, so long runs stay linear.

// maxType is how far each scan looks. A type is short; the limit keeps a long run of "<" or names from
// being scanned again from each of them.
const maxType = 512

// isNameByte reports whether c can end a name (ASCII \w; a byte of a multibyte character counts too,
// since \w follows Unicode).
func isNameByte(c byte) bool { return c >= 0x80 || lexkit.IsWord(rune(c)) }

// isBlank reports whether c is a space or a tab.
func isBlank(c byte) bool { return c == ' ' || c == '\t' }

// blanks returns the end of the spaces and tabs from j.
func (l *lexer) blanks(j int) int {
	for j < len(l.src) && isBlank(l.src[j]) {
		j++
	}
	return j
}

// typeArgsEnd returns the end of the type arguments that start at the "<" at i, or 0 if what follows
// does not look like type arguments: names, ".", ",", "[", "]", "?", whitespace and nested "< >",
// closed by ">" within maxType bytes. Anything else (an operator, a number, a parenthesis, <<) means
// that "<" is a comparison or a shift.
func (l *lexer) typeArgsEnd(i int) int {
	if l.argsFrom < i && i < l.argsStop {
		return 0
	}
	src := l.src
	depth := 0
	j := i
	for j < len(src) && j < i+maxType {
		r, w := lexkit.Rune(src, j)
		switch {
		case r == '<':
			if j > i && src[j-1] == '<' { // <<
				return l.argsFailed(i, j)
			}
			depth++
		case r == '>':
			depth--
			if depth == 0 {
				return j + 1
			}
		case lexkit.IsDigit(r):
			if p, _ := utf8.DecodeLastRuneInString(src[:j]); !lexkit.IsWord(p) {
				return l.argsFailed(i, j) // a number, not a name
			}
		case r == '@' || lexkit.IsWord(r) || lexkit.IsSpace(r) || strings.ContainsRune(".,[]?", r):
		default:
			return l.argsFailed(i, j)
		}
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

// genericFollows reports whether what follows the type arguments that end at e lets them be type
// arguments in an expression (as C#'s grammar tells F(G<A, B>(7)) from a comparison): one of
// ( ) ] } : ; , . ? = ! | ^ & [ {, a newline or the end. A name or a number after ">" (a < b, c > d)
// means a comparison.
func (l *lexer) genericFollows(e int) bool {
	j := l.blanks(e)
	return j >= len(l.src) || strings.IndexByte("()]}:;,.?=!|^&[{\n", l.src[j]) >= 0
}

// typeExpr returns the end of the type that starts at the name at j: names joined by ".", type
// arguments, then any of "?" and "[,]"; 0 if no name starts at j or the "<" after it is not type
// arguments.
func (l *lexer) typeExpr(j int) int {
	limit := j + maxType
	n := l.name(j)
	if n == 0 {
		return 0
	}
	j += n
	for j < limit && l.at(j) == '.' {
		n := l.name(j + 1)
		if n == 0 {
			break
		}
		j += 1 + n
	}
	if l.at(j) == '<' {
		if j = l.typeArgsEnd(j); j == 0 {
			return 0
		}
	}
	for j < limit {
		if l.at(j) == '?' {
			j++
			continue
		}
		if l.at(j) == '[' {
			k := j + 1
			for k < limit && l.at(k) == ',' {
				k++
			}
			if l.at(k) == ']' {
				j = k + 1
				continue
			}
		}
		break
	}
	return j
}

// notDeclared holds contextual keywords that are not in Chroma's lists and can follow an expression
// (x with { }, case T t when c, x is A or B).
var notDeclared = [...]string{"with", "when", "and", "or", "not", "nameof"}

// declaredType sets typeEnd when the name at i starts the type of a declaration: a type (typeExpr),
// spaces, a declared name (with type arguments for a generic method), then =, ;, ,, ), (, { or "in"
// (foreach). That covers variables, fields, properties, parameters and return types. Two names in a
// row are not an expression in C#, so a comparison is not taken for a type.
func (l *lexer) declaredType(i int) {
	src := l.src
	// The start of a type, not a member after ".".
	p := i
	for p > 0 && p > i-maxType && isBlank(src[p-1]) {
		p--
	}
	if p > 0 && src[p-1] == '.' {
		return
	}
	e := l.typeExpr(i)
	if e == 0 {
		return
	}
	k := l.blanks(e)
	if k == e {
		return
	}
	n := l.name(k)
	if n == 0 {
		return
	}
	for _, list := range [...]int{lKeywords, lDeclarations, lTypes, lConstants} {
		if l.words.Match(src, k, list) > 0 {
			return
		}
	}
	for _, w := range notDeclared {
		if src[k:k+n] == w {
			return
		}
	}
	m := k + n
	if l.at(m) == '<' {
		if m = l.typeArgsEnd(m); m == 0 {
			return
		}
	}
	m = l.blanks(m)
	switch c := l.at(m); {
	case c == '=' && l.at(m+1) != '=':
	case c != 0 && strings.IndexByte(";,)({", c) >= 0:
	case strings.HasPrefix(src[m:], "in") && lexkit.WordBoundary(src, m+2):
	default:
		return
	}
	l.typeEnd = e
}

// typeAfterKeyword sets typeEnd when a type follows the keyword (new or as) that ends at j.
func (l *lexer) typeAfterKeyword(j int) {
	k := l.blanks(j)
	if k == j {
		return
	}
	if e := l.typeExpr(k); e > 0 {
		l.typeEnd = e
	}
}
