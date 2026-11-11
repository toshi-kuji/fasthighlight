package php

import "strings"

// Coloring user-defined types in type declarations (production only; Options.ChromaTypes turns it off).
//
//   - A name followed by a variable is a type: the name and the names joined to it by | or & (not || or &&),
//     followed by "$name", "&$name" or "...$name" ("&" touching the variable). This covers parameters (User $u, ?User $u, A|B $u,
//     A&B &$u, User ...$us), typed properties (private User $u;) and catch (Exception $e). A ternary
//     ($a ? B : $c), a case label (case X:), a named argument (f(x: 1)) and a constant are not followed by
//     a variable, so they stay NameOther. The name must also be in a declaration (declContext): directly
//     after the "(" or "," of a parameter list or catch, or after a property modifier, so that constants in
//     expressions such as FLAG &$x or X . $y stay NameOther.
//   - A return type: the name (with ?, | and &) after a ":" that follows the ")" of a parameter list whose
//     "(" comes after function, fn, function name, or use.
//
// Built-in types (int, string, ...) stay NameOther and keywords keep their own types. Every scan looks at
// most maxType bytes, so long runs stay linear.

// maxType is how far a type scan looks.
const maxType = 512

// builtinTypes are the built-in type names that are not in Chroma's keyword lists (they stay NameOther).
var builtinTypes = []string{"int", "float", "string", "bool", "iterable", "callable", "object", "mixed", "void", "never", "self", "integer", "boolean", "double"}

func isBuiltinType(name string) bool {
	for _, b := range builtinTypes {
		if strings.EqualFold(name, b) {
			return true
		}
	}
	return false
}

// isType reports whether the name src[s:e] is a user-defined type.
func (l *lexer) isType(s, e int) bool {
	if isBuiltinType(l.src[s:e]) {
		return false
	}
	if s < l.typeEnd {
		return true
	}
	if s < l.noTypeEnd {
		return false
	}
	if !l.declContext(s) {
		return false
	}
	end, ok := l.typeChain(e, s+maxType)
	if ok {
		l.typeEnd = end
	} else {
		l.noTypeEnd = end
	}
	return ok
}

// typeChain reads, from the end of a name at i, (spaces [|&] spaces name)* spaces, then a variable
// ("$x", "&$x" or "...$x"). It returns the end of the last name and true when the variable follows, or
// the position where it stopped and false. It looks no further than limit.
func (l *lexer) typeChain(i, limit int) (int, bool) {
	last := i
	j := i
	for j < limit {
		j += l.spaces(j)
		switch c := l.at(j); {
		case c == '$':
			return last, l.ident(l.dollars(j), true) > 0
		case c == '.' && strings.HasPrefix(l.src[j:], "..."):
			k := j + 3
			k += l.spaces(k)
			return last, l.at(k) == '$' && l.ident(l.dollars(k), true) > 0
		case (c == '|' || c == '&') && l.at(j+1) != c:
			k := j + 1
			if c == '&' && (l.at(k) == '$' || strings.HasPrefix(l.src[k:], "...")) {
				// A &$x: a by-reference parameter. "&" must touch the variable, so that a constant in
				// FLAG & $x stays a value.
				j = k
				continue
			}
			k += l.spaces(k)
			n := l.ident(k, true)
			if n == 0 {
				return j, false
			}
			last, j = k+n, k+n
		default:
			return j, false
		}
	}
	return j, false
}

// declContext reports whether a type starting at s is in a declaration: walking back over the names
// joined by | or & and a leading ?, it finds the "(" or "," of a parameter list (of function, fn, a
// function name or catch), or a property modifier (public, private, protected, static, readonly, var).
// It looks back at most maxType bytes.
func (l *lexer) declContext(s int) bool {
	src := l.src
	stop := max(s-maxType, 0)
	k := l.skipBack(s, stop)
	for k > stop {
		c := src[k-1]
		if c == '?' {
			k = l.skipBack(k-1, stop)
			continue
		}
		if (c == '|' || c == '&') && (k < 2 || src[k-2] != c) {
			k = l.skipBack(k-1, stop)
			w := k
			for w > stop && (isWordByte(src[w-1]) || src[w-1] == '\\') {
				w--
			}
			if w == k {
				return false
			}
			k = l.skipBack(w, stop)
			continue
		}
		break
	}
	if k <= stop {
		return false
	}
	switch src[k-1] {
	case '(':
		return l.paramsOpen(k-1, true)
	case ',':
		depth := 0
		for j := k - 2; j >= stop; j-- {
			switch src[j] {
			case ')':
				depth++
			case '(':
				if depth == 0 {
					return l.paramsOpen(j, false)
				}
				depth--
			case ';', '{', '}':
				return false
			}
		}
		return false
	}
	w := l.wordBefore(k)
	switch strings.ToLower(strings.TrimSpace(src[w:k])) {
	case "public", "private", "protected", "static", "readonly", "var":
		return true
	}
	return false
}

// skipBack returns k moved back over spaces, not before stop.
func (l *lexer) skipBack(k, stop int) int {
	for k > stop && isSpaceByte(l.src[k-1]) {
		k--
	}
	return k
}

// paramsOpen reports whether the "(" at k opens a parameter list: after function, fn, function and a
// name (with an optional &), or, when catch is set, catch.
func (l *lexer) paramsOpen(k int, catch bool) bool {
	src := l.src
	w := l.wordBefore(k)
	switch strings.ToLower(strings.TrimSpace(src[w:k])) {
	case "function", "fn":
		return true
	case "catch":
		return catch
	case "":
		return false
	}
	k = l.skipBack(w, 0)
	if k > 0 && src[k-1] == '&' {
		k--
	}
	w = l.wordBefore(k)
	return strings.EqualFold(strings.TrimSpace(src[w:k]), "function")
}

// returnType sets typeEnd when the ":" at i starts a return type.
func (l *lexer) returnType(i int) {
	if !l.afterParams(i) {
		return
	}
	j := i + 1
	j += l.spaces(j)
	if l.at(j) == '?' {
		j++
		j += l.spaces(j)
	}
	n := l.ident(j, true)
	if n == 0 {
		return
	}
	end, limit := j+n, i+maxType
	for k := end; k < limit; {
		k += l.spaces(k)
		c := l.at(k)
		if !(c == '|' || c == '&') || l.at(k+1) == c {
			break
		}
		k++
		k += l.spaces(k)
		m := l.ident(k, true)
		if m == 0 {
			break
		}
		k += m
		end = k
	}
	l.typeEnd = max(l.typeEnd, end)
}

// afterParams reports whether the ":" at i follows the ")" of a parameter list: before its "(" comes
// function, fn, use, or function and a name (with an optional &).
func (l *lexer) afterParams(i int) bool {
	src := l.src
	j := i
	for j > 0 && isSpaceByte(src[j-1]) {
		j--
	}
	if j == 0 || src[j-1] != ')' {
		return false
	}
	depth := 0
	k := j - 1
	for ; k >= 0 && k > j-maxType; k-- {
		if src[k] == ')' {
			depth++
		} else if src[k] == '(' {
			depth--
			if depth == 0 {
				break
			}
		}
	}
	if k < 0 || k <= j-maxType {
		return false
	}
	w := l.wordBefore(k)
	switch strings.ToLower(strings.TrimSpace(src[w:k])) {
	case "function", "fn", "use":
		return true
	case "":
		return false
	}
	k = w
	for k > 0 && isSpaceByte(src[k-1]) {
		k--
	}
	if k > 0 && src[k-1] == '&' {
		k--
	}
	w = l.wordBefore(k)
	return strings.EqualFold(strings.TrimSpace(src[w:k]), "function")
}

// wordBefore skips spaces back from k and returns the start of the ASCII word that ends there (with
// the spaces after it, so src[start:k] trimmed is the word; for a plain word without spaces it is exact).
func (l *lexer) wordBefore(k int) int {
	src := l.src
	e := k
	for e > 0 && isSpaceByte(src[e-1]) {
		e--
	}
	s := e
	for s > 0 && s > k-maxType && isWordByte(src[s-1]) {
		s--
	}
	if s == e {
		return k
	}
	return s
}

func isSpaceByte(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }

func isWordByte(c byte) bool {
	return c == '_' || '0' <= c && c <= '9' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || c >= 0x80
}
