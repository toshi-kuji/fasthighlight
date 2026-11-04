// gencsslists extracts the word lists from Chroma's CSS rules and generates internal/css/lists.go.
// After upgrading Chroma, run the following in compat/.
//
//	go run ./cmd/gencsslists > ../internal/css/lists.go
package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

// lists are the alternatives to extract (Go constant name, state, rule index, description).
var lists = []struct {
	name, state string
	idx         int
	doc         string
}{
	{"vendorPrefixes", "common-values", 1, `vendor prefixes in common-values and content (KeywordPseudo). No \b`},
	{"builtinFunctions", "common-values", 3, "built-in functions in common-values (NameBuiltin). Must be followed by ("},
	{"valueConstants1", "common-values", 5, `value constants 1 in common-values (KeywordConstant). Needs \b`},
	{"valueConstants2", "common-values", 6, `value constants 2 in common-values (KeywordConstant). Needs \b`},
	{"colorNames", "common-values", 7, `color names in common-values (KeywordConstant). Needs \b`},
	{"propertyNames", "content", 5, `property names in content (Keyword). Must be followed by \s*:`},
	{"valueKeywords", "value-start", 1, `property names inside values in value-start (Keyword). Needs \b`},
	{"units", "numeric-end", 0, `units in numeric-end (KeywordType). Needs \b`},
}

func main() {
	rl, ok := lexers.Get("CSS").(*chroma.RegexLexer)
	if !ok {
		fail("Chroma's CSS lexer is not a RegexLexer")
	}
	rules, err := rl.Rules()
	if err != nil {
		fail(err.Error())
	}
	group := regexp.MustCompile(`^\(([^()"\\]*)\)`)
	var b strings.Builder
	b.WriteString("// Code generated from Chroma 2.27.0 lexers/embedded/css.xml. DO NOT EDIT.\n\n")
	b.WriteString("// Chroma (Copyright (C) 2017 Alec Thomas, MIT License) css.xml: the alternatives of its regular\n")
	b.WriteString("// expressions, copied in the same order. TestCSSWordListsMatchChroma in compat/ compares every word with Chroma.\n\n")
	b.WriteString("package css\n")
	for _, l := range lists {
		m := group.FindStringSubmatch(rules[l.state][l.idx].Pattern)
		if m == nil {
			fail(fmt.Sprintf("%s[%d]: unexpected pattern", l.state, l.idx))
		}
		fmt.Fprintf(&b, "\n// %s holds %s.\nconst %s = %q\n", l.name, l.doc, l.name, m[1])
	}
	os.Stdout.WriteString(b.String())
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, "gencsslists:", msg)
	os.Exit(1)
}
