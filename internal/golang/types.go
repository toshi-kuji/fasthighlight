package golang

import (
	"strings"

	fh "github.com/toshi-kuji/fasthighlight"
)

// Names in type positions get the type color: NameClass, and NameNamespace for the package before the dot
// (Client and storage in *storage.Client). Chroma's Go rules make every name other than the built-in types
// NameOther, so `data string` colors the type but `ctx context.Context` does not.
// Options.ChromaTypeNames turns this off for the comparison with Chroma.
//
// Only places where the grammar alone says "a type comes here" are colored:
//   - struct fields (name, then the type; a field with no name is an embedded type) and interface elements,
//   - var and const declarations (var c *http.Client), and type declarations (type D time.Duration),
//   - the parameters, results and receivers of functions, methods, function literals and function types,
//   - inside those types: *T, []T, [N]T, map[K]V, chan T, func types, and the arguments of generic types.
//
// Expressions are left as they are: http.Get(u) stays a function, and Foo(y) (a conversion or a call) is
// not colored. In a parameter list, a single name with nothing after it (the T of func(T), or the a of
// func(a, b T)) is a name or a type depending on the other parameters, so it is left as well; a qualified
// name (pkg.T) is always a type there and is colored.

// atom is one piece of the token sequence that the type reader looks at: a token, or one character of a
// punctuation token (the lexer joins adjacent punctuation, so `{}` and `]*` are one token).
type atom struct {
	tok  int  // index of the token
	c    byte // the character, for punctuation; 0 otherwise
	t    fh.TokenType
	word string // the token's text, for names, keywords and operators
	nl   bool   // whitespace containing a newline
}

// maxTypeDepth bounds the nesting of types read (( ( ( … in a type). Deeper input is not colored, which keeps
// the reader linear and its recursion short on unusual input.
const maxTypeDepth = 64

// typeReader colors the names in type positions of toks.
type typeReader struct {
	toks  []fh.Token
	atoms []atom
}

// markTypes changes the types of the names in type positions (see above). toks covers the source; raws are
// the [start, end) ranges of raw strings, whose insides are read as templates and are not looked at.
func markTypes(toks []fh.Token, raws [][2]int) {
	if !hasDeclaration(toks) {
		return
	}
	r := &typeReader{toks: toks, atoms: typeAtoms(toks, raws)}
	r.run()
}

// hasDeclaration reports whether a keyword that starts a type position appears.
func hasDeclaration(toks []fh.Token) bool {
	for _, t := range toks {
		if t.Type == fh.KeywordDeclaration {
			return true
		}
	}
	return false
}

// typeAtoms splits toks into atoms, leaving out the tokens inside raw strings (a raw string counts as one
// string atom, by the token that starts it).
func typeAtoms(toks []fh.Token, raws [][2]int) []atom {
	atoms := make([]atom, 0, len(toks))
	off, r := 0, 0
	for i, t := range toks {
		s := off
		off += len(t.Value)
		for r < len(raws) && raws[r][1] <= s {
			r++
		}
		if r < len(raws) && raws[r][0] < s && s < raws[r][1] {
			continue // inside a raw string
		}
		switch t.Type {
		case fh.Punctuation:
			for j := 0; j < len(t.Value); j++ {
				atoms = append(atoms, atom{tok: i, c: t.Value[j], t: t.Type})
			}
		case fh.TextWhitespace:
			atoms = append(atoms, atom{tok: i, t: t.Type, nl: strings.IndexByte(t.Value, '\n') >= 0})
		default:
			atoms = append(atoms, atom{tok: i, t: t.Type, word: t.Value})
		}
	}
	return atoms
}

func (r *typeReader) is(k int, c byte) bool { return k < len(r.atoms) && r.atoms[k].c == c }

func (r *typeReader) kind(k int, t fh.TokenType, word string) bool {
	return k < len(r.atoms) && r.atoms[k].t == t && r.atoms[k].word == word
}

// space skips whitespace without a newline and /* */ comments.
func (r *typeReader) space(k int) int {
	for k < len(r.atoms) {
		a := r.atoms[k]
		if a.t == fh.TextWhitespace && !a.nl || a.t == fh.CommentMultiline {
			k++
			continue
		}
		break
	}
	return k
}

// blank skips whitespace, newlines and comments.
func (r *typeReader) blank(k int) int {
	for k < len(r.atoms) && (r.atoms[k].t == fh.TextWhitespace || r.atoms[k].t.Category() == fh.Comment) {
		k++
	}
	return k
}

// lineEnd reports whether the atom at k ends a line or a statement: a newline, a // comment, ; or the end.
func (r *typeReader) lineEnd(k int) bool {
	if k >= len(r.atoms) {
		return true
	}
	a := r.atoms[k]
	return a.nl || a.c == ';' || a.t == fh.CommentSingle || a.t == fh.CommentPreproc
}

// color gives the name at k the type color, or the namespace color when a dot follows.
func (r *typeReader) color(k int, t fh.TokenType) { r.toks[r.atoms[k].tok].Type = t }

// skipTo returns the index after the bracket that closes the one opened just before k, or len(atoms). A
// closing bracket of another kind ends the skip there (it is not consumed), so a list left open in broken code
// does not run past the struct or block around it. With line true, it stops (returning -1) at a newline outside
// nested brackets.
func (r *typeReader) skipTo(k int, line bool) int {
	open := []byte{closer(r.atoms[k-1].c)}
	for ; k < len(r.atoms); k++ {
		a := r.atoms[k]
		switch {
		case a.c == '(' || a.c == '[' || a.c == '{':
			open = append(open, closer(a.c))
		case a.c == ')' || a.c == ']' || a.c == '}':
			if a.c != open[len(open)-1] {
				return k
			}
			if open = open[:len(open)-1]; len(open) == 0 {
				return k + 1
			}
		case line && a.nl:
			return -1
		}
	}
	return len(r.atoms)
}

// closer returns the bracket that closes c.
func closer(c byte) byte {
	switch c {
	case '(':
		return ')'
	case '[':
		return ']'
	}
	return '}'
}

// run walks the whole source and reads the types where a declaration or a function puts them.
func (r *typeReader) run() {
	type block struct {
		word  string // var, const or type
		depth int    // the bracket depth inside its parentheses
	}
	var blocks []block
	depth := 0
	spec := "" // the next name starts a spec of this declaration
	for k := 0; k < len(r.atoms); {
		a := r.atoms[k]
		if a.t == fh.TextWhitespace || a.t.Category() == fh.Comment {
			if r.lineEnd(k) && len(blocks) > 0 && blocks[len(blocks)-1].depth == depth {
				spec = blocks[len(blocks)-1].word
			}
			k++
			continue
		}
		if spec != "" && a.t == fh.NameOther {
			k = max(r.spec(spec, k), k+1)
			spec = ""
			continue
		}
		spec = ""
		switch {
		case a.t == fh.KeywordDeclaration && (a.word == "var" || a.word == "const" || a.word == "type"):
			j := r.space(k + 1)
			if r.is(j, '(') {
				depth++
				blocks = append(blocks, block{a.word, depth})
				k = j + 1
			} else {
				k++
			}
			spec = a.word
			continue
		case a.t == fh.KeywordDeclaration && a.word == "func":
			k = max(r.function(k+1), k+1)
			continue
		case a.t == fh.KeywordDeclaration && (a.word == "struct" || a.word == "interface"):
			if e := r.typ(k, 0); e > k {
				k = e
				continue
			}
		case a.c == '(' || a.c == '[' || a.c == '{':
			depth++
		case a.c == ')' || a.c == ']' || a.c == '}':
			depth--
			for len(blocks) > 0 && blocks[len(blocks)-1].depth > depth {
				blocks = blocks[:len(blocks)-1]
			}
		case a.c == ';':
			if len(blocks) > 0 && blocks[len(blocks)-1].depth == depth {
				spec = blocks[len(blocks)-1].word
			}
		}
		k++
	}
}

// spec reads one var, const or type spec starting at the name at k and returns where it ends (the values
// after = are left to run, which may find function literals in them).
func (r *typeReader) spec(word string, k int) int {
	if word == "type" {
		k++
		if r.is(k, '[') { // type parameters
			if k = r.skipTo(k+1, false); k < 0 {
				return -1
			}
		}
		k = r.space(k)
		if r.is(k, '=') {
			k = r.space(k + 1)
		}
		return r.typ(k, 0)
	}
	// var and const: names, then the type unless = (or the end of the line) comes first.
	k++
	for {
		j := r.space(k)
		if !r.is(j, ',') {
			break
		}
		j = r.space(j + 1)
		if j >= len(r.atoms) || r.atoms[j].t != fh.NameOther {
			return j
		}
		k = j + 1
	}
	j := r.space(k)
	if j == k || r.lineEnd(j) || r.is(j, '=') || r.is(j, ')') {
		return j
	}
	return r.typ(j, 0)
}

// function reads what follows the keyword func: a receiver, a name and type parameters (in a declaration),
// the parameters and the results. The body is left to run.
func (r *typeReader) function(k int) int {
	k = r.space(k)
	if r.is(k, '(') {
		e := r.params(k, 0)
		if j := r.space(e); j < len(r.atoms) && (r.atoms[j].t == fh.NameFunction || r.atoms[j].t == fh.NameOther) {
			k = j // e closed the receiver; a method name follows
		} else {
			return r.results(e, 0) // a function literal or a function type
		}
	}
	if k < len(r.atoms) && (r.atoms[k].t == fh.NameFunction || r.atoms[k].t == fh.NameOther) {
		k++
		if r.is(k, '[') { // type parameters
			if k = r.skipTo(k+1, false); k < 0 {
				return -1
			}
		}
		k = r.space(k)
	}
	if !r.is(k, '(') {
		return k
	}
	return r.results(r.params(k, 0), 0)
}

// signature reads the parameters and the results of a function type or an interface method; k is at (.
func (r *typeReader) signature(k, d int) int {
	if !r.is(k, '(') {
		return -1
	}
	return r.results(r.params(k, d), d)
}

// results reads the results after the parameters (ending at k), if any.
func (r *typeReader) results(k, d int) int {
	j := r.space(k)
	switch {
	case r.is(j, '('):
		return r.params(j, d)
	case r.lineEnd(j) || r.is(j, '{') || r.is(j, ')') || r.is(j, ']') || r.is(j, '}') || r.is(j, ',') || r.is(j, '=') ||
		r.atoms[j].t.SubCategory() == fh.LiteralString:
		return k
	}
	if e := r.typ(j, d); e > j {
		return e
	}
	return k
}

// params reads a parameter list starting at ( and returns the index after ). A name followed by a space and a
// type is a name; a qualified name, or anything else that reads as a type, is a type; a single name alone is
// left as it is.
func (r *typeReader) params(k, d int) int {
	k++
	for {
		k = r.blank(k)
		if k >= len(r.atoms) {
			return k
		}
		if r.is(k, ')') {
			return k + 1
		}
		e := -1
		if a := r.atoms[k]; a.t == fh.NameOther && !r.is(k+1, '.') && !r.is(k+1, '[') {
			j := r.space(k + 1)
			switch {
			case r.is(j, ',') || r.is(j, ')') || r.lineEnd(j):
				e = k + 1 // a single name: a name or a type, depending on the others
			case j > k+1:
				e = r.typ(j, d) // name type
			}
		} else {
			e = r.typ(k, d)
		}
		if e < 0 {
			return r.skipTo(k, false)
		}
		k = r.blank(e)
		switch {
		case r.is(k, ','):
			k++
		case r.is(k, ')'):
			return k + 1
		default:
			return r.skipTo(k, false)
		}
	}
}

// typ reads a type starting at k (after spaces) and colors the names in it. It returns the index after the
// type, or -1 if no type starts there.
func (r *typeReader) typ(k, d int) int {
	if d > maxTypeDepth {
		return -1
	}
	d++
	k = r.space(k)
	if k >= len(r.atoms) {
		return -1
	}
	a := r.atoms[k]
	switch {
	case a.t == fh.Operator && a.word != "" && strings.Trim(a.word, "*") == "": // *T (** is one token)
		return r.typ(k+1, d)
	case a.t == fh.Operator && a.word == "...": // a variadic parameter
		return r.typ(k+1, d)
	case a.t == fh.Operator && a.word == "<-": // <-chan T
		if j := r.space(k + 1); r.kind(j, fh.KeywordDeclaration, "chan") {
			return r.typ(j+1, d)
		}
		return -1
	case a.c == '[':
		if r.is(k+1, ']') { // []T
			return r.typ(k+2, d)
		}
		e := r.skipTo(k+1, true) // [N]T: the length is an expression
		if e < 0 {
			return -1
		}
		return r.typ(e, d)
	case a.c == '(': // (T)
		e := r.typ(k+1, d)
		if e < 0 {
			return -1
		}
		if e = r.space(e); !r.is(e, ')') {
			return -1
		}
		return e + 1
	case a.t == fh.KeywordDeclaration:
		switch a.word {
		case "map":
			j := r.space(k + 1)
			if !r.is(j, '[') {
				return -1
			}
			e := r.typ(j+1, d)
			if e < 0 {
				return -1
			}
			if e = r.space(e); !r.is(e, ']') {
				return -1
			}
			return r.typ(e+1, d)
		case "chan":
			j := r.space(k + 1)
			if r.kind(j, fh.Operator, "<-") {
				j++
			}
			return r.typ(j, d)
		case "func":
			return r.signature(r.space(k+1), d)
		case "struct", "interface":
			j := r.space(k + 1)
			if !r.is(j, '{') {
				return -1
			}
			return r.body(j, a.word == "struct", d)
		}
		return -1
	case a.t == fh.KeywordType:
		return k + 1
	case a.t == fh.NameOther:
		e := k + 1
		if r.is(e, '.') && e+1 < len(r.atoms) && r.atoms[e+1].t == fh.NameOther {
			r.color(k, fh.NameNamespace)
			k, e = e+1, e+2
		}
		r.color(k, fh.NameClass)
		if r.is(e, '[') { // generic arguments
			return r.typeArgs(e, d)
		}
		return e
	}
	return -1
}

// typeArgs reads the arguments of a generic type, starting at [.
func (r *typeReader) typeArgs(k, d int) int {
	k++
	for {
		k = r.blank(k)
		e := r.typ(k, d)
		if e < 0 {
			return r.skipTo(k, false)
		}
		k = r.blank(e)
		switch {
		case r.is(k, ','):
			k++
		case r.is(k, ']'):
			return k + 1
		default:
			return r.skipTo(k, false)
		}
	}
}

// body reads the body of a struct (fields) or an interface (methods and embedded types), starting at {.
func (r *typeReader) body(k int, isStruct bool, d int) int {
	k++
	for {
		k = r.blank(k)
		for r.is(k, ';') {
			k = r.blank(k + 1)
		}
		if k >= len(r.atoms) {
			return k
		}
		if r.is(k, '}') {
			return k + 1
		}
		var e int
		if isStruct {
			e = r.field(k, d)
		} else {
			e = r.element(k, d)
		}
		if e < 0 {
			e = k
		}
		// The rest of the element (a tag, a constraint union) is not read; skip to the end of the line.
		for e = max(e, k+1); e < len(r.atoms) && !r.lineEnd(e); e++ {
			if r.is(e, '}') {
				return e + 1
			}
			if r.is(e, '{') || r.is(e, '(') || r.is(e, '[') {
				if j := r.skipTo(e+1, true); j > e+1 {
					e = j - 1
				}
			}
		}
		k = e
	}
}

// field reads one struct field: names and a type, or an embedded type.
func (r *typeReader) field(k, d int) int {
	a := r.atoms[k]
	if a.t != fh.NameOther || r.is(k+1, '.') || r.is(k+1, '[') {
		return r.typ(k, d) // *T, pkg.T, T[A]
	}
	j := k + 1
	for {
		n := r.space(j)
		if !r.is(n, ',') {
			break
		}
		n = r.space(n + 1)
		if n >= len(r.atoms) || r.atoms[n].t != fh.NameOther {
			return n
		}
		j = n + 1
	}
	n := r.space(j)
	if j == k+1 && (r.lineEnd(n) || r.is(n, '}') || n < len(r.atoms) && r.atoms[n].t.SubCategory() == fh.LiteralString) {
		r.color(k, fh.NameClass) // an embedded type
		return k + 1
	}
	if n == j {
		return -1
	}
	return r.typ(n, d)
}

// element reads one interface element: a method (name and signature) or an embedded type.
func (r *typeReader) element(k, d int) int {
	if r.atoms[k].t == fh.NameFunction {
		return r.signature(r.space(k+1), d)
	}
	return r.typ(k, d)
}
