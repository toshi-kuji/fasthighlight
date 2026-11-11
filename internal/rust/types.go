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

// typeCtx tells whether a single ":" starts a type (let x: T, fn f(a: T), |x: T|, struct S { a: T },
// T: Bound) or a value, above all the value of a field in a struct literal (P { a: x }), which keeps
// Chroma's coloring. It follows the brackets in one pass. A "{" opens a declaration body when struct,
// enum, union, trait, impl, mod or extern came since the last ";", "{" or "}", or when it is a variant
// inside an enum body; in a declaration body (and at the top level) every ":" starts a type. Any other
// "{" (a struct literal, a block, a match) annotates only after let, or after static or const at the start
// of a statement. Directly inside ( ) a ":" is a parameter, inside [ ] it is never a type, and inside the
// |…| of closure parameters it is a type. When unsure it does not mark: a missed type keeps Chroma's
// coloring, a value colored as a type is wrong.
type typeCtx struct {
	frames      []frame // open brackets; frames[0] is the top level
	declPending bool    // struct/enum/... since the last boundary
	enumPending bool
	stmtStart   bool // nothing but pub since the last boundary
	closure     bool // inside the |…| of closure parameters
	prev        byte // the last byte of the previous non-space token ('a' for a word)
}

// frame is an open bracket.
type frame struct {
	open      byte // '{', '(' or '['
	decl      bool // a declaration body
	enum      bool // an enum body: a "{" in it is a variant
	letActive bool // let (or a statement static/const) since the last boundary in this frame
}

func (c *typeCtx) top() *frame {
	if len(c.frames) == 0 {
		c.frames = append(c.frames, frame{open: '{', decl: true})
		c.stmtStart = true
	}
	return &c.frames[len(c.frames)-1]
}

// annotation reports whether the single ":" being read starts a type.
func (c *typeCtx) annotation() bool {
	if c.closure {
		return true
	}
	switch f := c.top(); f.open {
	case '(':
		return true
	case '[':
		return false
	default:
		return f.decl || f.letActive
	}
}

func (c *typeCtx) boundary() {
	c.declPending, c.enumPending, c.stmtStart, c.closure = false, false, true, false
}

// track updates the context with the token(s) of n bytes that the base state read at i.
func (l *lexer) track(i, n int) {
	c := &l.ctx
	src := l.src
	b := src[i]
	if lexkit.IsSpace(rune(b)) {
		return
	}
	f := c.top()
	if isIdentStart(b) && l.ident(i) == n {
		w := src[i : i+n]
		switch w {
		case "struct", "union", "trait", "impl", "mod", "extern":
			c.declPending = true
		case "enum":
			c.declPending, c.enumPending = true, true
		case "let":
			f.letActive = true
		case "static", "const":
			if c.stmtStart {
				f.letActive = true
			}
		}
		c.stmtStart = c.stmtStart && w == "pub"
		c.prev = 'a'
		if w == "move" {
			c.prev = '=' // move |x: T| starts a closure like = |x: T|
		}
		return
	}
	if n > 1 {
		// a macro call that ends in its opening bracket (m!( ), or any other longer token
		if last := src[i+n-1]; last == '(' || last == '[' || last == '{' {
			c.frames = append(c.frames, frame{open: last})
		}
		c.stmtStart = false
		c.prev = src[i+n-1]
		return
	}
	c.stmtStart = false
	switch b {
	case ';':
		f.letActive = false
		c.boundary()
	case '{', '(', '[':
		nf := frame{open: b}
		if b == '{' {
			nf.decl = c.declPending || f.enum
			nf.enum = c.enumPending
			c.boundary()
		}
		c.frames = append(c.frames, nf)
	case '}', ')', ']':
		if len(c.frames) > 1 {
			c.frames = c.frames[:len(c.frames)-1]
		}
		if b == '}' {
			c.boundary()
		}
	case '|':
		if c.closure {
			c.closure = false
		} else {
			switch c.prev {
			case 0, '(', '=', ',', '{', ';', '[':
				c.closure = true
			}
		}
	}
	c.prev = b
}
