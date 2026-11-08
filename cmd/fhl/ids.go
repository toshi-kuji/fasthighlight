package main

import (
	"bytes"
	"fmt"
	"unicode"
	"unicode/utf8"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
)

// headingIDs makes the ids of headings. It works as goldmark's own ids for ASCII text (letters and digits
// in lower case, spaces, - and _ as -, other characters dropped), and also keeps letters and digits of other
// scripts, as GitHub does: "## インストール" becomes "インストール" (not "heading"), so that the ids of
// headings in Japanese and other languages do not change when headings are added. A heading with no letters
// or digits at all becomes "heading". A second heading with the same id gets -1 added, then -2, and so on.
type headingIDs struct {
	used map[string]bool
	n    map[string]int
}

func newHeadingIDs() *headingIDs {
	return &headingIDs{used: map[string]bool{}, n: map[string]int{}}
}

// parseContext returns a goldmark parser context that uses headingIDs.
func parseContext() parser.Context {
	return parser.NewContext(parser.WithIDs(newHeadingIDs()))
}

// Generate is parser.IDs's Generate.
func (s *headingIDs) Generate(value []byte, kind ast.NodeKind) []byte {
	value = bytes.TrimSpace(value) // spaces at the ends do not count
	var id []byte
	for len(value) > 0 {
		r, size := utf8.DecodeRune(value)
		value = value[size:]
		switch {
		case r < utf8.RuneSelf && ('a' <= r && r <= 'z' || '0' <= r && r <= '9'):
			id = append(id, byte(r))
		case 'A' <= r && r <= 'Z':
			id = append(id, byte(r+'a'-'A'))
		case r >= utf8.RuneSelf && (unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsMark(r)):
			id = utf8.AppendRune(id, unicode.ToLower(r))
		case unicode.IsSpace(r) || r == '-' || r == '_':
			id = append(id, '-')
		}
	}
	if len(id) == 0 {
		if kind == ast.KindHeading {
			id = []byte("heading")
		} else {
			id = []byte("id")
		}
	}
	key := string(id)
	if !s.used[key] {
		s.used[key] = true
		return id
	}
	for i := s.n[key] + 1; ; i++ {
		v := fmt.Sprintf("%s-%d", key, i)
		if !s.used[v] {
			s.used[v] = true
			s.n[key] = i
			return []byte(v)
		}
	}
}

// Put is parser.IDs's Put: it marks an id given by hand as used.
func (s *headingIDs) Put(value []byte) {
	s.used[string(value)] = true
}
