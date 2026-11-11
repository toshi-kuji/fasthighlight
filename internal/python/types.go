package python

import "unicode/utf8"

// maxType bounds each scan for a type hint (forward over the annotation, backward over the target of
// a variable annotation), so that the time stays linear. An annotation longer than this is left as
// it is.
const maxType = 512

// typeState tracks where type hints are (see Options.ChromaTypes). Only expr's rules in the root
// state update it.
type typeState struct {
	end        int  // plain names that start before end are in a type hint
	depth      int  // bracket depth ( [ {
	defPending bool // after "def name", before its "("
	param      int  // the depth inside a def's parameter list (0 if not in one)
	inDefault  bool // in a parameter's default value (after "=")
}

func (t *typeState) newline() {
	if t.depth == 0 {
		t.defPending, t.param, t.inDefault = false, 0, false
	}
}

// trackType updates the type state for the character at i (called by expr before its rules) and,
// where a type hint starts, sets types.end to the end of the hint.
func (l *lexer) trackType(i int) {
	t := &l.types
	switch l.src[i] {
	case '(', '[', '{':
		if l.src[i] == '(' && t.defPending && t.depth == 0 {
			t.defPending, t.param, t.inDefault = false, 1, false
		}
		t.depth++
	case ')', ']', '}':
		if t.depth > 0 {
			t.depth--
		}
		if t.depth < t.param {
			t.param, t.inDefault = 0, false
		}
	case ',':
		if t.param > 0 && t.depth == t.param {
			t.inDefault = false
		}
	case '=':
		// "=" alone (not "==", and the "=" of "!=", "<=", ">=" and ":=" is a different position)
		if t.param > 0 && t.depth == t.param && l.at(i+1) != '=' {
			t.inDefault = true
		}
	case '-':
		if l.at(i+1) == '>' { // "->" appears only before a return annotation
			l.markType(i + 2)
		}
	case ':':
		if l.at(i+1) == '=' {
			return
		}
		if t.param > 0 && t.depth == t.param && !t.inDefault {
			l.markType(i + 1)
		} else if t.depth == 0 && t.param == 0 && !t.defPending && l.annotationTarget(i) {
			l.markType(i + 1)
		}
	}
}

// markType sets types.end to the end of the type expression that starts at i: names, numbers, ".",
// "|", spaces, and [ ] with "," inside them (newlines too inside them). It stops at anything else
// (such as "=", ")" or "," outside brackets, or a newline); if a bracket is still open there, or the
// scan reaches maxType, nothing is marked.
func (l *lexer) markType(i int) {
	src := l.src
	depth := 0
	j := i
	for j < len(src) {
		if j-i >= maxType {
			return
		}
		c := src[j]
		switch {
		case c == ' ' || c == '\t' || c == '.' || c == '|' || '0' <= c && c <= '9':
			j++
			continue
		case c == '[':
			depth++
			j++
			continue
		case c == ']' && depth > 0:
			depth--
			j++
			continue
		case (c == ',' || c == '\n') && depth > 0:
			j++
			continue
		}
		if n := nameLen(src, j); n > 0 {
			j += n
			continue
		}
		break
	}
	if depth == 0 && j > l.types.end {
		l.types.end = j
	}
}

// annotationTarget reports whether the ":" at i follows only a target at the start of a line:
// indentation, then a name or a dotted name (a.b.c) that is not a keyword, then spaces.
func (l *lexer) annotationTarget(i int) bool {
	src := l.src
	lo := i - maxType
	j := backSpace(src, i, lo)
	start := j
	for {
		k := j
		for k > lo && k > 0 {
			r, n := utf8.DecodeLastRuneInString(src[:k])
			if !isNameChar(r) {
				break
			}
			k -= n
		}
		if k == j || nameLen(src, k) != j-k {
			return false
		}
		start = k // in the end, the first word
		k = backSpace(src, k, lo)
		if k > 0 && k > lo && src[k-1] == '.' {
			j = backSpace(src, k-1, lo)
			continue
		}
		j = k
		break
	}
	if !(j == 0 || src[j-1] == '\n') {
		return false
	}
	// the first word must not be a keyword (else:, lambda x:, and so on)
	w := src[start : start+nameLen(src, start)]
	if words[w]&(1<<lKeywords|1<<lExprKeywords|1<<lOperatorWords|1<<lConstants) != 0 {
		return false
	}
	switch w {
	case "def", "class", "from", "import", "match", "case", "type":
		return false
	}
	return true
}

// backSpace returns the position before the spaces and tabs that end at j (not below lo).
func backSpace(src string, j, lo int) int {
	for j > 0 && j > lo && (src[j-1] == ' ' || src[j-1] == '\t') {
		j--
	}
	return j
}
