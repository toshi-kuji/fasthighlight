package swift

import (
	"unicode/utf8"

	"github.com/toshi-kuji/fasthighlight/internal/lexkit"
)

// maxType bounds every scan for a type (forward or backward), so that long runs stay linear.
const maxType = 512

// The type positions (only with Options.ChromaTypes false). Names before lexer.typeEnd become NameClass,
// the token type Chroma's rules give the name after class, struct, enum and so on; keywords and built-ins
// keep their own types. typeEnd is set
//   - after the ":" of a declaration (let x: T, var x: T) or of a parameter of func, init or subscript
//     (func f(_ a: T, b: U)); a ":" after case let x, in a call f(x: v), in a dictionary literal [k: v] or in
//     a ternary a ? b : c is not a type position,
//   - after the ":" of the inheritance list after class, struct, enum, protocol and extension names,
//   - after "->", and after as, as?, as! and is,
//   - for the type arguments of a "<" right after a name (Array<User>), and for that name itself.

// typeScan returns the end of the type that starts after spaces at i: names, ".", "?", "!", and balanced
// [ ] ( ) < >; inside brackets also ":", ",", spaces and "->". If list is true, ", " and "&" also continue at
// the top level (an inheritance list or a composition). It returns i when no type starts there.
func (l *lexer) typeScan(i int, list bool) int {
	src := l.src
	k := i
	for k < len(src) && (src[k] == ' ' || src[k] == '\t') {
		k++
	}
	end, depth := i, 0
	for k < len(src) && k < i+maxType {
		c := src[k]
		switch {
		case c == '.' || c == '?' || c == '!':
			k++
		case c == '[' || c == '(' || c == '<':
			depth++
			k++
		case c == ']' || c == ')' || c == '>':
			if depth == 0 {
				return end
			}
			depth--
			k++
		case c == '-' && l.at(k+1) == '>' && depth > 0:
			k += 2
		case c == ':' && depth > 0:
			k++
		case c == ',' || c == ' ' || c == '\t' || c == '&':
			if depth == 0 && !list {
				return end
			}
			k++
			continue // spaces and separators do not extend the end
		default:
			r, w := lexkit.Rune(src, k)
			if !(r == '_' || lexkit.IsWord(r)) {
				return end
			}
			k += w
		}
		if depth == 0 {
			end = k
		}
	}
	return end
}

// typeArgsEnd returns the end of the type arguments that start with the "<" at i (after a name), or 0: only
// names, ".", ",", ":", "?", "!", "[", "]", spaces and nested < > up to the closing ">". Anything else (&&,
// =, numbers, parentheses) means a comparison such as a < b && c > d.
func (l *lexer) typeArgsEnd(i int) int {
	src := l.src
	depth := 0
	for k := i; k < len(src) && k < i+maxType; {
		c := src[k]
		switch {
		case c == '<':
			depth++
		case c == '>':
			depth--
			if depth == 0 {
				return k + 1
			}
		case c == '.' || c == ',' || c == ':' || c == '?' || c == '!' || c == '[' || c == ']' || c == ' ' || c == '\t':
		default:
			r, w := lexkit.Rune(src, k)
			if !(r == '_' || r >= utf8.RuneSelf && lexkit.IsWord(r) || isNameStart(byte(r))) {
				return 0
			}
			k += w
			continue
		}
		k++
	}
	return 0
}

// skipBack returns the position before the spaces and tabs that end at j.
func (l *lexer) skipBack(j int) int {
	for j > 0 && (l.src[j-1] == ' ' || l.src[j-1] == '\t') {
		j--
	}
	return j
}

// nameBack returns the start of the name that ends at j (j if there is none).
func (l *lexer) nameBack(j int) int {
	for j > 0 {
		r, w := utf8.DecodeLastRuneInString(l.src[:j])
		if !(r == '_' || lexkit.IsWord(r)) {
			break
		}
		j -= w
	}
	return j
}

// typeColon reports whether the ":" at i starts a type: after let/var x (not case let x), or after a
// parameter name directly inside the "(" of func, init or subscript.
func (l *lexer) typeColon(i int) bool {
	src := l.src
	e := l.skipBack(i)
	s := l.nameBack(e)
	if s == e || s < i-maxType {
		return false
	}
	p := l.skipBack(s)
	if w := l.nameBack(p); w < p {
		switch src[w:p] {
		case "let", "var":
			c := l.skipBack(w)
			return src[l.nameBack(c):c] != "case"
		}
		// An external label before the parameter name (func f(to name: T)).
		p = l.skipBack(w)
	}
	if p == 0 || src[p-1] != ',' && src[p-1] != '(' {
		return false
	}
	// Find the "(" that encloses the parameter.
	depth, k := 0, p-1
	for ; k >= 0 && k > i-maxType; k-- {
		switch src[k] {
		case ')', ']', '>':
			depth++
		case '(', '[', '<':
			if depth == 0 {
				goto found
			}
			depth--
		}
	}
	return false
found:
	if src[k] != '(' {
		return false
	}
	k = l.skipBack(k)
	if k > 0 && src[k-1] == '>' { // generic parameters: func f<T: P>(
		d := 0
		for k > 0 && k > i-maxType {
			k--
			if src[k] == '>' {
				d++
			} else if src[k] == '<' {
				if d--; d == 0 {
					break
				}
			}
		}
		if d != 0 {
			return false
		}
		k = l.skipBack(k)
	}
	if k > 0 && (src[k-1] == '?' || src[k-1] == '!') { // init?(
		k--
	}
	n := l.nameBack(k)
	switch src[n:k] {
	case "init", "subscript":
		return true
	}
	if n == k {
		return false
	}
	f := l.skipBack(n)
	return f < n && src[l.nameBack(f):f] == "func"
}
