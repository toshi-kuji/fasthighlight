package rust

import (
	"strings"

	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/internal/lexkit"
)

// Types in production (Options.ChromaTypes false). Chroma's rules color only the one name right after
// ":" or "->" (NameClass), so in `let xs: Vec<User>` and `m: HashMap<String, User>` the User is Name.
// Here the lexer reads the whole type after ":" and "->", and the type arguments of a turbofish
// (collect::<Vec<User>>()), and sets typeEnd to its end; the names before typeEnd become NameClass
// (NameNamespace when "::" follows them). When what follows does not look like a type (a call, a field
// access, a comparison, a shift, a number outside [ ]), nothing is marked and Chroma's coloring stays.

// maxType is how far typeExprEnd looks. A type is short; the limit keeps a long run of ":" or "::<" from
// being scanned again from each of them, so the time stays linear.
const maxType = 512

// markType sets typeEnd at the ":" or "->" at i (n bytes) or at the "<" of a turbofish.
func (l *lexer) markType(i, n int) {
	if l.o.ChromaTypes || i < l.typeEnd {
		return
	}
	src := l.src
	switch {
	case src[i] == '<':
		if i >= 2 && src[i-2:i] == "::" {
			if e := typeExprEnd(src, i, true); e > 0 {
				l.typeEnd = e
			}
		}
	case src[i] == ':' && (l.at(i+1) == ':' || i > 0 && src[i-1] == ':'):
		// a path (a::b) or a turbofish (::<), not a type annotation
	default:
		if e := typeExprEnd(src, i+n, false); e > 0 {
			l.typeEnd = e
		}
	}
}

// typeName returns the type of the name of n bytes at i inside a type: NameNamespace when "::" follows,
// NameClass otherwise.
func (l *lexer) typeName(i, n int) fh.TokenType {
	if strings.HasPrefix(l.src[i+n:], "::") {
		return fh.NameNamespace
	}
	return fh.NameClass
}

func isFnWord(w string) bool { return w == "fn" || w == "Fn" || w == "FnMut" || w == "FnOnce" }

// prefixWord reports whether the keyword w can come right before a type (dyn Trait, &mut T, impl Trait,
// *const T, unsafe fn, extern "C" fn is not read).
func prefixWord(w string) bool {
	switch w {
	case "dyn", "mut", "impl", "const", "unsafe", "for":
		return true
	}
	return false
}

// typeExprEnd returns the end of the type that starts after spaces at i, or 0 if what follows does not
// look like a type. With turbofish set, src[i] is the "<" of ::< and the type arguments end at their ">".
//
// A type is made of names, "::", "&" (not &&), lifetimes, "*" (raw pointers), nested < >, ( ) after
// fn/Fn/FnMut/FnOnce or as a tuple, [ ] with ";" and numbers inside, "," and "=" (Item = T) inside
// brackets, "->" after ")", "!" as a type, and "+" before an upper-case name, a lifetime or "?" (bounds).
// At bracket depth 0 it ends at "," ")" "]" "=" ";" "{" "|" ">" "?", a newline-separated name, or the end
// of input; anything else (".", a "(" after a name, "!" after a name, &&, a number, a single ":") means
// it is not a type, as in S { x: a.b } or x: a < b.
func typeExprEnd(src string, i int, turbofish bool) int {
	const (
		start  = iota // at the start, or after a separator: a type can start here
		name          // after a name or a closed type
		closed        // after ")" (-> may follow)
	)
	angle, paren, brack := 0, 0, 0
	prev := start
	prevWord := ""
	sawName := false
	j := i
	if turbofish {
		angle = 1
		j++
	}
	nested := func() bool { return angle > 0 || paren > 0 || brack > 0 }
	stop := func() int {
		if nested() || !sawName {
			return 0
		}
		return j
	}
	for j < len(src) && j < i+maxType {
		c := src[j]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			j++
			continue
		case isIdentStart(c):
			n := 1 + lexkit.Span(src, j+1, lexkit.IsWord)
			w := src[j : j+n]
			if prev == name && !prefixWord(prevWord) {
				return stop() // -> T where …
			}
			prev, prevWord, sawName = name, w, true
			j += n
			continue
		case '0' <= c && c <= '9':
			if brack == 0 || prev != start {
				return 0
			}
			for j < len(src) && isDecU(src[j]) {
				j++
			}
			prev, prevWord = name, ""
			continue
		case c == ':':
			if j+1 < len(src) && src[j+1] == ':' && prev != closed {
				j += 2
				prev, prevWord = start, ""
				continue
			}
			return stop()
		case c == '<':
			if prev != name || prefixWord(prevWord) {
				return 0
			}
			angle++
		case c == '>':
			if angle == 0 {
				return stop()
			}
			angle--
			if turbofish && angle == 0 {
				if paren > 0 || brack > 0 {
					return 0
				}
				return j + 1
			}
			prev, prevWord = name, ""
			j++
			continue
		case c == '-' && j+1 < len(src) && src[j+1] == '>':
			if prev != closed {
				return stop()
			}
			j += 2
			prev, prevWord = start, ""
			continue
		case c == '&':
			if j+1 < len(src) && src[j+1] == '&' {
				return 0
			}
			if prev != start {
				return stop()
			}
		case c == '*', c == '?':
			if prev != start {
				return stop()
			}
		case c == '!':
			if prev != start {
				return 0 // a macro
			}
			prev, prevWord = name, ""
			j++
			continue
		case c == '\'':
			n := 0
			if j+1 < len(src) && isIdentStart(src[j+1]) {
				n = 1 + lexkit.Span(src, j+2, lexkit.IsWord)
			}
			if n == 0 || j+1+n < len(src) && src[j+1+n] == '\'' || prev != start {
				return 0 // a character
			}
			j += 1 + n
			continue
		case c == '(':
			if !(prev == start || prev == name && isFnWord(prevWord)) {
				return 0 // a call
			}
			paren++
		case c == ')':
			if paren == 0 {
				return stop()
			}
			paren--
			prev, prevWord = closed, ""
			j++
			continue
		case c == '[':
			if prev != start {
				return 0 // an index
			}
			brack++
		case c == ']':
			if brack == 0 {
				return stop()
			}
			brack--
			prev, prevWord = name, ""
			j++
			continue
		case c == ';':
			if brack == 0 {
				return stop()
			}
		case c == ',':
			if !nested() {
				return stop()
			}
		case c == '=':
			if !nested() {
				return stop()
			}
			if angle == 0 || j+1 < len(src) && (src[j+1] == '=' || src[j+1] == '>') {
				return 0
			}
		case c == '+':
			k := j + 1
			for k < len(src) && (src[k] == ' ' || src[k] == '\t') {
				k++
			}
			if prev == start || k >= len(src) || !('A' <= src[k] && src[k] <= 'Z' || src[k] == '\'' || src[k] == '?') {
				return stop()
			}
		case c == '.':
			return 0 // a field or method: S { x: a.b }
		case c == '{' || c == '|':
			return stop()
		default:
			if nested() {
				return 0
			}
			return stop()
		}
		prev, prevWord = start, ""
		j++
	}
	if j >= len(src) {
		return stop()
	}
	return 0
}
