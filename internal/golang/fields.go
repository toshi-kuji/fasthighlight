package golang

import (
	"strings"

	fh "github.com/toshi-kuji/fasthighlight"
)

// The type of a field in a struct body (Client in `Storage *storage.Client`) gets the type color
// (NameClass, and NameNamespace for the package before the dot). Chroma's Go rules make every name other
// than the built-in types NameOther, so `data string` colors string but not `ctx context.Context`.
// Options.ChromaFieldTypes turns this off for the comparison with Chroma.
//
// The rule is the one a reader uses: in a struct body, a field starts at the start of a line (or after ;),
// its names are followed by a space, and what follows the space is the type, up to the tag, a comment or
// the end of the line. A field with no name (an embedded field, `io.Reader` or `*Base`) is a type
// itself. Inside the type, the parameters of a func type and the length of an array ([N]T) are left as
// they are; the arguments of a generic type and the key of a map are types.

// atom is one piece of the token sequence that the field rule looks at: a token, or one character of a
// punctuation token (the lexer joins adjacent punctuation, so `{}` and `]*` are one token).
type atom struct {
	tok  int  // index of the token
	c    byte // the character, for punctuation; 0 otherwise
	t    fh.TokenType
	word string // the token's text, for names and keywords
	nl   bool   // whitespace containing a newline
}

// Kinds of open brackets.
const (
	frameBrace  = iota // { of a block, a composite literal or an interface
	frameStruct        // { of a struct body
	frameParen         // ( inside a field type: func parameters and results
	frameTypes         // [ holding types: generic arguments, a map key, []
	frameLength        // [ holding an array length (an expression)
)

// Field states in a struct body.
const (
	fieldStart = iota // at the start of a field: names or an embedded type come next
	fieldType         // reading the type
	fieldRest         // after the type (a tag): nothing more to color
)

type frame struct {
	kind  int
	state int // for frameStruct: the field state inside the body
}

// markFieldTypes changes the types of the names in the field types of struct bodies (see above). toks
// covers src; raws are the [start, end) ranges of raw strings, whose insides are read as templates and
// are not looked at.
func markFieldTypes(toks []fh.Token, raws [][2]int) {
	if !hasStruct(toks) {
		return
	}
	atoms := fieldAtoms(toks, raws)
	var stack []frame
	var braces []int       // indexes in stack of the open braces, so the innermost one is found without a scan
	pendingStruct := false // the last keyword was struct, so the next { opens a struct body
	// cur returns the innermost struct body and the number of brackets opened inside its current field,
	// or nil if the innermost brace is not a struct body.
	cur := func() (*frame, int) {
		if len(braces) == 0 {
			return nil, 0
		}
		b := braces[len(braces)-1]
		if stack[b].kind != frameStruct {
			return nil, 0
		}
		return &stack[b], len(stack) - 1 - b
	}
	// next returns the index of the atom after k that is not whitespace without a newline or a /* */ comment,
	// or len(atoms).
	next := func(k int) int {
		for k++; k < len(atoms) && (atoms[k].t == fh.TextWhitespace && !atoms[k].nl || atoms[k].t == fh.CommentMultiline); k++ {
		}
		return k
	}
	isPunct := func(k int, c byte) bool { return k < len(atoms) && atoms[k].c == c }
	// endsField: the atom at k ends a field when it follows a name (the name is then an embedded type).
	endsField := func(k int) bool {
		if k >= len(atoms) {
			return true
		}
		a := atoms[k]
		return a.nl || a.c == '}' || a.c == ';' || a.t.SubCategory() == fh.LiteralString || a.t.Category() == fh.Comment
	}
	color := func(k int) {
		if isPunct(k+1, '.') {
			toks[atoms[k].tok].Type = fh.NameNamespace
		} else {
			toks[atoms[k].tok].Type = fh.NameClass
		}
	}
	for k, a := range atoms {
		f, depth := cur()
		switch {
		case a.c == '{':
			kind := frameBrace
			if pendingStruct {
				kind = frameStruct
			}
			braces = append(braces, len(stack))
			stack = append(stack, frame{kind: kind, state: fieldStart})
			pendingStruct = false
			continue
		case a.c == '}':
			// Close up to and including the innermost brace (brackets left open by a mistake go with it).
			if len(braces) > 0 {
				stack = stack[:braces[len(braces)-1]]
				braces = braces[:len(braces)-1]
			} else {
				stack = stack[:0]
			}
			pendingStruct = false
			continue
		case a.t == fh.KeywordDeclaration && (a.word == "struct" || a.word == "interface"):
			pendingStruct = a.word == "struct"
			continue
		case a.t == fh.TextWhitespace || a.t.Category() == fh.Comment:
		default:
			pendingStruct = false
		}
		if f == nil {
			continue
		}
		if a.nl || a.c == ';' {
			// A field ends at a newline or ;, even if a bracket in it was left open.
			stack = stack[:len(stack)-depth]
			f.state = fieldStart
			continue
		}
		if depth > 0 {
			// Inside a bracket opened in the field's type.
			top := &stack[len(stack)-1]
			switch {
			case a.c == '(' || a.c == '[':
				kind := frameParen
				if a.c == '[' {
					kind = frameLength
					if top.kind == frameTypes {
						kind = bracketKind(atoms, k, true)
					}
				}
				stack = append(stack, frame{kind: kind})
			case a.c == ')' || a.c == ']':
				stack = stack[:len(stack)-1]
			case a.t == fh.NameOther && top.kind == frameTypes:
				color(k)
			}
			continue
		}
		switch f.state {
		case fieldStart:
			switch {
			case a.t == fh.NameOther:
				n := next(k)
				switch {
				case isPunct(k+1, '.'): // pkg.Type
					color(k)
					f.state = fieldType
				case isPunct(k+1, '['): // Type[T]
					color(k)
					f.state = fieldType
				case isPunct(k+1, ','): // a, b T
				case n > k+1 && !endsField(n): // a T
					f.state = fieldType
				case endsField(n): // Type
					color(k)
					f.state = fieldRest
				}
			case a.t == fh.Operator && strings.Trim(a.word, "*") == "": // *Type
				f.state = fieldType
			}
		case fieldType:
			switch {
			case a.t == fh.NameOther:
				color(k)
			case a.c == '(':
				stack = append(stack, frame{kind: frameParen})
			case a.c == '[':
				stack = append(stack, frame{kind: bracketKind(atoms, k, false)})
			case a.t.SubCategory() == fh.LiteralString: // the tag
				f.state = fieldRest
			}
		}
	}
}

// bracketKind tells what the [ at atoms[k] holds: types after a name (generic arguments) or map, inside
// brackets that hold types, or when it is []; otherwise an array length.
func bracketKind(atoms []atom, k int, inTypes bool) int {
	if k+1 < len(atoms) && atoms[k+1].c == ']' {
		return frameTypes
	}
	if k > 0 {
		p := atoms[k-1]
		if p.t == fh.NameOther || p.t == fh.NameClass || p.t == fh.KeywordDeclaration && p.word == "map" {
			return frameTypes
		}
	}
	if inTypes {
		return frameTypes
	}
	return frameLength
}

// hasStruct reports whether the struct keyword appears.
func hasStruct(toks []fh.Token) bool {
	for _, t := range toks {
		if t.Type == fh.KeywordDeclaration && t.Value == "struct" {
			return true
		}
	}
	return false
}

// fieldAtoms splits toks into atoms, leaving out the tokens inside raw strings (a raw string counts as one
// string atom, by the token that starts it).
func fieldAtoms(toks []fh.Token, raws [][2]int) []atom {
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
		switch {
		case t.Type == fh.Punctuation:
			for j := 0; j < len(t.Value); j++ {
				atoms = append(atoms, atom{tok: i, c: t.Value[j], t: t.Type})
			}
		case t.Type == fh.TextWhitespace:
			atoms = append(atoms, atom{tok: i, t: t.Type, nl: strings.IndexByte(t.Value, '\n') >= 0})
		case r < len(raws) && raws[r][0] == s:
			atoms = append(atoms, atom{tok: i, t: fh.LiteralString})
		default:
			atoms = append(atoms, atom{tok: i, t: t.Type, word: t.Value})
		}
	}
	return atoms
}
