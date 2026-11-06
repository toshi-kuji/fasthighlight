// genscsslists extracts the word lists from Chroma's SCSS rules and generates lang/scss/lists.go.
// After upgrading Chroma, run the following in compat/.
//
//	go run ./cmd/genscsslists > ../lang/scss/lists.go
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

// List is a list to extract (Go constant name, state, rule index, the parts before and after the (…) of
// the alternatives, description). The tests in compat/ use the same table.
type List struct {
	Name, State    string
	Idx            int
	Prefix, Suffix string
	Doc            string
}

// Lists are the lists of SCSS.
var Lists = []List{
	{"rootProperties", "root", 16, `\b(`, `)(?=\s*:)`, `property names in root (NameAttribute). Needs \b before and \s*: after`},
	{"valueProperties", "value", 5, `\b(`, `)(?=\s*:)`, `property names in value (NameAttribute). Needs \b before and \s*: after`},
	{"entities", "value", 7, `\b(`, `)\b`, `value keywords in value (NameEntity). Needs \b before and after. Some words contain ; or U+2011`},
	{"colors", "value", 8, `(?i)\b(`, `)\b`, `color names in value (NameConstant). Needs \b before and after`},
}

// Words returns the alternatives of l in Chroma's SCSS rules, in order.
func Words(l List) ([]string, error) {
	rl, ok := lexers.Get("SCSS").(*chroma.RegexLexer)
	if !ok {
		return nil, fmt.Errorf("Chroma's SCSS lexer is not a RegexLexer")
	}
	rules, err := rl.Rules()
	if err != nil {
		return nil, err
	}
	p := rules[l.State][l.Idx].Pattern
	inner, ok := strings.CutPrefix(p, l.Prefix)
	if ok {
		inner, ok = strings.CutSuffix(inner, l.Suffix)
	}
	if !ok {
		return nil, fmt.Errorf("%s[%d]: unexpected pattern %q", l.State, l.Idx, p)
	}
	words := strings.Split(inner, "|")
	for _, w := range words {
		// Only literal characters: letters, digits, -, ; and U+2011 (none of them special in a regular expression).
		for _, r := range w {
			if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == ';' || r == '‑') {
				return nil, fmt.Errorf("%s[%d]: unexpected character %q in %q", l.State, l.Idx, r, w)
			}
		}
	}
	return words, nil
}

func main() {
	var b strings.Builder
	b.WriteString("// Code generated from Chroma 2.27.0 lexers/embedded/scss.xml. DO NOT EDIT.\n\n")
	b.WriteString("// Chroma (Copyright (C) 2017 Alec Thomas, MIT License) scss.xml: the alternatives of its regular\n")
	b.WriteString("// expressions, copied in the same order. TestSCSSWordListsMatchChroma in compat/ compares every word with Chroma.\n\n")
	b.WriteString("package scss\n")
	for _, l := range Lists {
		w, err := Words(l)
		if err != nil {
			fail(err.Error())
		}
		fmt.Fprintf(&b, "\n// %s holds %s.\nconst %s = %q\n", l.Name, l.Doc, l.Name, strings.Join(w, "|"))
	}
	os.Stdout.WriteString(b.String())
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, "genscsslists:", msg)
	os.Exit(1)
}
