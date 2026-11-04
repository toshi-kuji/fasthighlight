// genclists extracts the word lists from Chroma's C and C++ rules and generates internal/c/lists.go.
// After upgrading Chroma, run the following in compat/.
//
//	go run ./cmd/genclists > ../internal/c/lists.go
package main

import (
	"fmt"
	"os"
	"regexp/syntax"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

// Lists are the lists to extract (Go constant name, language, index of the rule in statements, description).
// The tests in compat/ use it too.
var Lists = []struct {
	Name, Lang string
	Idx        int
	Doc        string
}{
	{"cKeywords", "C", 10, "C keywords (Keyword)"},
	{"cTypes", "C", 11, `C types (KeywordType). \b[a-z]\w*_t\b is also a type`},
	{"cReserved", "C", 12, "C reserved words 1 (KeywordReserved)"},
	{"cReservedM", "C", 13, "C reserved words 2 (KeywordReserved)"},
	{"cReservedUnderscore", "C", 14, "C reserved words 3 (KeywordReserved)"},
	{"cBuiltins", "C", 15, "C built-in names (NameBuiltin)"},
	{"cppKeywords", "C++", 0, "C++-only keywords (Keyword). Tried before the other rules"},
	{"cppCKeywords", "C++", 17, "C++ keywords shared with C (Keyword)"},
	{"cppTypes", "C++", 18, "C++ types (KeywordType)"},
	{"cppReserved", "C++", 19, "C++ reserved words 1 (KeywordReserved)"},
	{"cppReservedM", "C++", 20, "C++ reserved words 2 (KeywordReserved)"},
	{"cppReservedUnderscore", "C++", 21, "C++ reserved words 3 (KeywordReserved)"},
	{"cppBuiltins", "C++", 22, "C++ built-in names (NameBuiltin)"},
}

// typeSuffix is the last alternative of the C type rule. internal/c handles it separately.
const typeSuffix = `|\b[a-z]\w*_t\b`

func main() {
	var b strings.Builder
	b.WriteString("// Code generated from Chroma 2.27.0 lexers/embedded/c.xml and c++.xml. DO NOT EDIT.\n\n")
	b.WriteString("// Chroma (Copyright (C) 2017 Alec Thomas, MIT License) c.xml and c++.xml: all the words matched by the\n")
	b.WriteString("// word-list regular expressions, written out (each one needs a trailing \\b). TestCWordListsMatchChroma in\n")
	b.WriteString("// compat/ compares every word with Chroma.\n\n")
	b.WriteString("package c\n")
	for _, l := range Lists {
		w, err := Words(Pattern(l.Lang, l.Idx))
		if err != nil {
			fail(fmt.Sprintf("%s %s[%d]: %v", l.Lang, "statements", l.Idx, err))
		}
		fmt.Fprintf(&b, "\n// %s holds %s.\nconst %s = %q\n", l.Name, l.Doc, l.Name, strings.Join(w, "|"))
	}
	os.Stdout.WriteString(b.String())
}

// Pattern returns the regular expression of the idx-th rule in statements of lang.
func Pattern(lang string, idx int) string {
	rl, ok := lexers.Get(lang).(*chroma.RegexLexer)
	if !ok {
		fail("Chroma's " + lang + " lexer is not a RegexLexer")
	}
	rules, err := rl.Rules()
	if err != nil {
		fail(err.Error())
	}
	return rules["statements"][idx].Pattern
}

// Words returns every word matched by X in a regular expression of the form X\b (the C type rule has
// typeSuffix after it). X is limited to those made only of characters, alternatives and ? (a finite
// number of matching words).
func Words(p string) ([]string, error) {
	p = strings.TrimSuffix(p, typeSuffix)
	x, ok := strings.CutSuffix(p, `\b`)
	if !ok {
		return nil, fmt.Errorf("pattern %q does not end with \\b", p)
	}
	re, err := syntax.Parse(x, syntax.Perl)
	if err != nil {
		return nil, err
	}
	words, err := expand(re)
	if err != nil {
		return nil, err
	}
	for _, w := range words {
		for _, r := range w {
			if !(r == '_' || 'a' <= r && r <= 'z' || 'A' <= r && r <= 'Z' || '0' <= r && r <= '9') {
				return nil, fmt.Errorf("word %q has a non-word character", w)
			}
		}
	}
	return words, nil
}

// expand returns every string that re matches.
func expand(re *syntax.Regexp) ([]string, error) {
	switch re.Op {
	case syntax.OpLiteral:
		return []string{string(re.Rune)}, nil
	case syntax.OpEmptyMatch:
		return []string{""}, nil
	case syntax.OpCapture:
		return expand(re.Sub[0])
	case syntax.OpCharClass:
		var out []string
		for i := 0; i < len(re.Rune); i += 2 {
			if re.Rune[i+1]-re.Rune[i] > 64 {
				return nil, fmt.Errorf("character class too large")
			}
			for r := re.Rune[i]; r <= re.Rune[i+1]; r++ {
				out = append(out, string(r))
			}
		}
		return out, nil
	case syntax.OpQuest:
		sub, err := expand(re.Sub[0])
		if err != nil {
			return nil, err
		}
		return append(sub, ""), nil
	case syntax.OpAlternate:
		var out []string
		for _, s := range re.Sub {
			sub, err := expand(s)
			if err != nil {
				return nil, err
			}
			out = append(out, sub...)
		}
		return out, nil
	case syntax.OpConcat:
		out := []string{""}
		for _, s := range re.Sub {
			sub, err := expand(s)
			if err != nil {
				return nil, err
			}
			var next []string
			for _, a := range out {
				for _, b := range sub {
					next = append(next, a+b)
				}
			}
			out = next
		}
		return out, nil
	}
	return nil, fmt.Errorf("unsupported regexp %v", re)
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, "genclists:", msg)
	os.Exit(1)
}
