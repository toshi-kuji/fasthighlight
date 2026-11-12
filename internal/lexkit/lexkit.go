// Package lexkit provides the shared parts of tokenizers that reproduce Chroma's regular-expression
// rules by hand.
//
// Chroma uses regexp2 for its regular expressions, with no special options. So \s, \d, \w and \b follow
// the Unicode character classes (not just ASCII). The functions here make the same decisions as regexp2.
package lexkit

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	fh "github.com/toshi-kuji/fasthighlight"
)

// IsSpace is the same as regexp2's \s (unicode.IsSpace).
func IsSpace(r rune) bool { return unicode.IsSpace(r) }

// IsDigit is the same as regexp2's \d (Unicode Nd).
func IsDigit(r rune) bool { return unicode.IsDigit(r) }

// IsWord is the same as regexp2's \w, and as the word characters used to decide \b (L, Mn, Nd, Pc, and
// U+200C, U+200D).
func IsWord(r rune) bool {
	if r < utf8.RuneSelf {
		return 'a' <= r && r <= 'z' || 'A' <= r && r <= 'Z' || '0' <= r && r <= '9' || r == '_'
	}
	return unicode.In(r, unicode.L, unicode.Mn, unicode.Nd, unicode.Pc) || r == '‌' || r == '‍'
}

// Rune returns the first character of src[i:] and its size in bytes.
func Rune(src string, i int) (rune, int) {
	if c := src[i]; c < utf8.RuneSelf {
		return rune(c), 1
	}
	return utf8.DecodeRuneInString(src[i:])
}

// Span returns the number of bytes from the start of src[i:] that are characters satisfying f (the
// equivalent of the regular expression [...]*).
func Span(src string, i int, f func(rune) bool) int {
	j := i
	for j < len(src) {
		r, n := Rune(src, j)
		if !f(r) {
			break
		}
		j += n
	}
	return j - i
}

// WordBoundary returns true if position i of src is a \b (word boundary).
func WordBoundary(src string, i int) bool {
	before := false
	if i > 0 {
		r, _ := utf8.DecodeLastRuneInString(src[:i])
		before = IsWord(r)
	}
	after := false
	if i < len(src) {
		r, _ := Rune(src, i)
		after = IsWord(r)
	}
	return before != after
}

// LineEnd returns the position of the first newline in src[i:] (len(src) if there is none). It is the
// end of a regular expression .* (when dot_all is off, . matches every character except a newline).
func LineEnd(src string, i int) int {
	for j := i; j < len(src); j++ {
		if src[j] == '\n' {
			return j
		}
	}
	return len(src)
}

// Tokens collects tokens. As with Chroma's Coalesce, consecutive tokens of the same type are merged into
// one (but not once the merged token reaches fh.CoalesceLimit). Call Emit once per single match of a
// Chroma rule (one group, for bygroups). Calling it in finer pieces changes where tokens split at the
// limit, so they would differ from Chroma. Token values are substrings of src and are not copied.
type Tokens struct {
	src   string
	spans []span
	// parent and base are used when reading a part embedded in another language (such as inside an HTML
	// <script>). Emit adds to parent, shifted by base.
	parent *Tokens
	base   int
}

type span struct {
	t    fh.TokenType
	s, e int
}

// NewTokens returns a Tokens that collects the tokens of src.
func NewTokens(src string) *Tokens {
	return &Tokens{src: src, spans: make([]span, 0, len(src)/4+1)}
}

// Emit adds src[s:e] as type t. It does nothing if the range is empty.
func (x *Tokens) Emit(t fh.TokenType, s, e int) {
	if x.parent != nil {
		x.parent.Emit(t, x.base+s, x.base+e)
		return
	}
	if e <= s {
		return
	}
	if n := len(x.spans); n > 0 && x.spans[n-1].t == t && x.spans[n-1].e == s && x.spans[n-1].e-x.spans[n-1].s < fh.CoalesceLimit {
		x.spans[n-1].e = e
		return
	}
	x.spans = append(x.spans, span{t, s, e})
}

// Sub returns a Tokens for reading [base, base+len(src)) of x's src as a different language.
// Positions in the returned Tokens are positions within src, and Emit adds them to x with merging (as with
// Chroma's using, tokens of the embedded language are merged with the surrounding tokens too).
func (x *Tokens) Sub(src string, base int) *Tokens {
	return &Tokens{src: src, parent: x, base: base}
}

// Error adds the one character that matches no rule as an Error (as Chroma does). It returns the number of
// bytes it advanced.
func (x *Tokens) Error(i int) int {
	_, n := Rune(x.src, i)
	x.Emit(fh.Error, i, i+n)
	return n
}

// Result returns the collected tokens.
func (x *Tokens) Result() []fh.Token {
	toks := make([]fh.Token, len(x.spans))
	for k, s := range x.spans {
		toks[k] = fh.Token{Type: s.t, Value: x.src[s.s:s.e]}
	}
	return toks
}

// QuotedLen returns the length matched by the regular expression q(\\\\|\\q|[^q])*q (q is " or '), or 0 if
// it does not match. src[i] is q.
//
// It reproduces the regular expression's backtracking. \\ is read as one unit of 2 characters, and \q as
// one unit including the quote; the match closes at a q with no backslash before it. If there is no such
// q, the regular expression backtracks and the match closes at the q of the last \q. If there is no q at
// all, it does not match.
func QuotedLen(src string, i int, q byte) int {
	lastEscaped := -1
	for j := i + 1; j < len(src); {
		switch c := src[j]; {
		case c == '\\' && j+1 < len(src) && src[j+1] == '\\':
			j += 2
		case c == '\\' && j+1 < len(src) && src[j+1] == q:
			lastEscaped = j + 1
			j += 2
		case c == q:
			return j + 1 - i
		default:
			j++
		}
	}
	if lastEscaped >= 0 {
		return lastEscaped + 1 - i
	}
	return 0
}

// Run tries, starting at the beginning of src[p:], runs of characters satisfying f (min characters or
// more) from the longest down, and returns true if k returns true at the end position of any of them (a
// greedy repetition with backtracking in a regular expression; the part that follows is tried with k, in
// continuation-passing style).
func Run(src string, p, min int, f func(rune) bool, k func(int) bool) bool {
	e, count := p, 0
	for e < len(src) {
		r, n := Rune(src, e)
		if !f(r) {
			break
		}
		e += n
		count++
	}
	for ; count >= min; count-- {
		if k(e) {
			return true
		}
		if count == 0 {
			break
		}
		_, n := utf8.DecodeLastRuneInString(src[p:e])
		e -= n
	}
	return false
}

// WordLists holds several lists of alternatives of a regular expression of the form (a|b|…)\b. If a word
// consists only of word characters, the \b after it is equivalent to the run of word characters ending
// there, so the run is looked up in a table. Words that contain non-word characters (such as non-sealed)
// are compared one by one from the start.
type WordLists struct {
	words     map[string]uint32 // word -> bits of the lists that contain it
	irregular [][]string        // for each list, the words that contain non-word characters
	longest   int               // the length of the longest word in words: a longer run is in no list
}

// NewWordLists builds a WordLists from lists of alternatives separated by "|" (up to 32 lists).
func NewWordLists(lists ...string) *WordLists {
	w := &WordLists{words: map[string]uint32{}, irregular: make([][]string, len(lists))}
	for k, alts := range lists {
		list := strings.Split(alts, "|")
		for idx, word := range list {
			n := Span(word, 0, IsWord)
			if n == len(word) {
				w.words[word] |= 1 << k
				w.longest = max(w.longest, len(word))
				continue
			}
			// If the word-character part (first) is in an earlier alternative, that one matches first,
			// so this word never matches.
			first := word[:n]
			if i := slices.Index(list, first); i < 0 || idx < i {
				w.irregular[k] = append(w.irregular[k], word)
			}
		}
	}
	return w
}

// WordCache remembers, for each position, the run of word characters and the lists that contain it (because
// the same position is looked up many times).
type WordCache struct {
	Lists *WordLists
	pos   int
	n     int
	mask  uint32
	run   SpanCache // the last run of word characters, so that a long run is measured once
}

// Match returns the length if a word of list k matches at position i of src (including the trailing \b), or 0
// if it does not.
func (c *WordCache) Match(src string, i, k int) int {
	for _, w := range c.Lists.irregular[k] {
		if strings.HasPrefix(src[i:], w) && WordBoundary(src, i+len(w)) {
			return len(w)
		}
	}
	n, mask := c.Word(src, i)
	if mask&(1<<k) != 0 {
		return n
	}
	return 0
}

// Word returns the length of the run of word characters starting at position i of src, and the bits of the
// lists that contain that run.
func (c *WordCache) Word(src string, i int) (int, uint32) {
	if c.pos != i+1 { // pos is the position + 1 (so that the zero value means "not looked up yet")
		c.pos, c.mask = i+1, 0
		c.n = c.run.Span(src, i, IsWord)
		if c.n > 0 && c.n <= c.Lists.longest {
			c.mask = c.Lists.words[src[i:i+c.n]]
		}
	}
	return c.n, c.mask
}

// FoldEq reports whether the input character r matches the pattern character w in regexp2's
// case-insensitive comparison (?i). regexp2 matches the characters connected to w by unicode.SimpleFold
// (the uppercase of w, and for k the Kelvin sign U+212A, for s the long s U+017F). Unlike unicode.ToLower,
// U+0130 İ does not match i.
func FoldEq(r, w rune) bool {
	if r == w {
		return true
	}
	// Two ASCII characters match only as the two cases of one letter: the other members of an ASCII letter's
	// fold orbit (the Kelvin sign, ſ) are not ASCII. This skips unicode.SimpleFold for most comparisons.
	if r < utf8.RuneSelf && w < utf8.RuneSelf {
		return r|0x20 == w|0x20 && 'a' <= r|0x20 && r|0x20 <= 'z'
	}
	for f := unicode.SimpleFold(w); f != w; f = unicode.SimpleFold(f) {
		if f == r {
			return true
		}
	}
	return false
}

// FoldASCII returns the ASCII lowercase letter that r matches under (?i) (A-Z, the Kelvin sign, ſ and so on).
// If there is no such character, it returns r unchanged. Use it when comparing against words made only of
// ASCII lowercase letters.
func FoldASCII(r rune) rune {
	if r < utf8.RuneSelf {
		if 'A' <= r && r <= 'Z' {
			return r + 'a' - 'A'
		}
		return r
	}
	for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
		if 'a' <= f && f <= 'z' {
			return f
		}
	}
	return r
}

// FoldWordLists holds several lists of alternatives of a case-insensitive (?i) regular expression of the
// form (a|b|…)\b. Words are assumed to consist only of ASCII letters. The run of word characters in the
// input is lowercased with FoldASCII and looked up in a table.
type FoldWordLists struct {
	words     map[string]uint32 // lowercased word -> bits of the lists that contain it
	irregular [][]string        // for each list, the words (lowercased) that contain non-word characters
	longest   int               // the length of the longest word in words (ASCII, so also in characters)
}

// NewFoldWordLists builds a FoldWordLists from lists of alternatives separated by "|" (up to 32 lists).
func NewFoldWordLists(lists ...string) *FoldWordLists {
	w := &FoldWordLists{words: map[string]uint32{}, irregular: make([][]string, len(lists))}
	for k, alts := range lists {
		list := strings.Split(strings.ToLower(alts), "|")
		for idx, word := range list {
			n := Span(word, 0, IsWord)
			if n == len(word) {
				w.words[word] |= 1 << k
				w.longest = max(w.longest, len(word))
				continue
			}
			if i := slices.Index(list, word[:n]); i < 0 || idx < i {
				w.irregular[k] = append(w.irregular[k], word)
			}
		}
	}
	return w
}

// FoldWordCache remembers, for each position, the result of looking up a FoldWordLists.
type FoldWordCache struct {
	Lists *FoldWordLists
	pos   int // position + 1 (so that the zero value means "not looked up yet")
	n     int
	mask  uint32
	key   []byte
	run   SpanCache // the last run of word characters, as in WordCache
}

// Match returns the length if a word of list k matches at position i of src (including the trailing \b),
// or 0 if it does not.
func (c *FoldWordCache) Match(src string, i, k int) int {
	for _, w := range c.Lists.irregular[k] {
		if n := FoldPrefix(src, i, w); n > 0 && WordBoundary(src, i+n) {
			return n
		}
	}
	n, mask := c.Word(src, i)
	if mask&(1<<k) != 0 {
		return n
	}
	return 0
}

// Word returns the length of the run of word characters starting at position i of src, and the bits of the
// lists that contain it.
func (c *FoldWordCache) Word(src string, i int) (int, uint32) {
	if c.pos == i+1 {
		return c.n, c.mask
	}
	c.pos, c.key, c.mask = i+1, c.key[:0], 0
	c.n = c.run.Span(src, i, IsWord)
	// Only a run of at most longest characters can be a word, and the words are ASCII: build the lowercased
	// key, and stop at the first character that does not fold to ASCII (then the run is no word). Stopping
	// early matters for long runs of non-ASCII letters, where folding each character is slow.
	for j, chars := i, 0; j < i+c.n; chars++ {
		if chars == c.Lists.longest {
			return c.n, 0
		}
		r, w := Rune(src, j)
		f := FoldASCII(r)
		if f >= utf8.RuneSelf {
			return c.n, 0
		}
		c.key = append(c.key, byte(f))
		j += w
	}
	if c.n > 0 {
		c.mask = c.Lists.words[string(c.key)]
	}
	return c.n, c.mask
}

// FoldPrefix returns the number of bytes if the start of src[i:] matches word (lowercase) case-insensitively
// (FoldEq), or 0 if it does not.
func FoldPrefix(src string, i int, word string) int {
	j := i
	for _, w := range word {
		if j >= len(src) {
			return 0
		}
		r, n := Rune(src, j)
		if !FoldEq(r, w) {
			return 0
		}
		j += n
	}
	return j - i
}

// EntityCache finds the end of an entity, &\S*?;, and remembers the last scan. Without it, a long run
// of "&" with no ";" would be scanned to its end again from every "&". The zero value is ready to use.
type EntityCache struct {
	from, stop int  // the last scan started after the "&" at from and stopped at stop
	found      bool // stop is the ";" that ends the entity
}

// End returns the end of &\S*?; at i (src[i] == '&'), or 0 if there is none. Every "&" between from and
// stop gives the same answer: the scan stops only at ";" or whitespace, and an ASCII "&" always starts a
// rune, so a scan from a later "&" reads the same runes up to stop.
func (c *EntityCache) End(src string, i int) int {
	if !(c.from <= i && i < c.stop) {
		j := i + 1
		for j < len(src) {
			r, w := Rune(src, j)
			if r == ';' || IsSpace(r) {
				break
			}
			j += w
		}
		c.from, c.stop, c.found = i, j, j < len(src) && src[j] == ';'
	}
	if c.found {
		return c.stop + 1
	}
	return 0
}

// SpanCache is Span that remembers the last run it found. Lexers ask for the run at every position of a run
// they do not take as one token (a run of blank lines, for example), and scanning the rest of the run each time
// would take time quadratic in its length. One SpanCache must always be used with the same src and f. The zero
// value is ready to use.
type SpanCache struct{ from, end int }

// Span returns Span(src, i, f). i must start a rune.
func (c *SpanCache) Span(src string, i int, f func(rune) bool) int {
	if c.from <= i && i < c.end {
		return c.end - i
	}
	n := Span(src, i, f)
	if n > 0 {
		c.from, c.end = i, i+n
	}
	return n
}
