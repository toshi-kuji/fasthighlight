// genpythonlists extracts the word lists from Chroma's Python rules and generates internal/python/lists.go.
// After upgrading Chroma, run the following in compat/.
//
//	go run ./cmd/genpythonlists > ../internal/python/lists.go
package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

// Lists are the alternatives to extract (Go constant name, state, rule index, description). The tests in
// compat/ use it too.
var Lists = []struct {
	Name, State string
	Idx         int
	Doc         string
}{
	{"keywords", "keywords", 0, `reserved words in keywords (Keyword). Needs \b`},
	{"keywordConstants", "keywords", 1, `constants in keywords and expr-keywords (KeywordConstant). Needs \b`},
	{"exprKeywords", "expr-keywords", 0, `reserved words inside expressions in expr-keywords (Keyword). Needs \b`},
	{"operatorWords", "expr", 20, `word operators in expr (OperatorWord). Needs \b`},
	{"builtins", "builtins", 0, `built-in functions and so on in builtins (NameBuiltin). Not preceded by . and needs \b`},
	{"builtinPseudo", "builtins", 1, `self and so on in builtins (NameBuiltinPseudo). Not preceded by . and needs \b`},
	{"exceptions", "builtins", 2, `exceptions in builtins (NameException). Not preceded by . and needs \b`},
	{"magicFuncs", "magicfuncs", 0, `special methods in magicfuncs (NameFunctionMagic). Needs \b`},
	{"magicVars", "magicvars", 0, `special attributes in magicvars (NameVariableMagic). Needs \b`},
}

func main() {
	rl, ok := lexers.Get("Python").(*chroma.RegexLexer)
	if !ok {
		fail("Chroma's Python lexer is not a RegexLexer")
	}
	rules, err := rl.Rules()
	if err != nil {
		fail(err.Error())
	}
	var b strings.Builder
	b.WriteString("// Code generated from Chroma 2.27.0 lexers/embedded/python.xml. DO NOT EDIT.\n\n")
	b.WriteString("// Chroma (Copyright (C) 2017 Alec Thomas, MIT License) python.xml: the alternatives of its regular\n")
	b.WriteString("// expressions, copied in the same order. TestPythonWordListsMatchChroma in compat/ compares every word with Chroma.\n\n")
	b.WriteString("package python\n")
	for _, l := range Lists {
		w, err := Words(rules[l.State][l.Idx].Pattern)
		if err != nil {
			fail(fmt.Sprintf("%s[%d]: %v", l.State, l.Idx, err))
		}
		fmt.Fprintf(&b, "\n// %s holds %s.\nconst %s = %q\n", l.Name, l.Doc, l.Name, strings.Join(w, "|"))
	}
	os.Stdout.WriteString(b.String())
}

var pattern = regexp.MustCompile(`^(?:\(\?<!\\\.\))?\(([a-zA-Z_ |]*)\)\\b$`)

// Words extracts the alternatives, in order, from a regular expression of the form (?<!\.)(a|b|c)\b.
func Words(p string) ([]string, error) {
	m := pattern.FindStringSubmatch(p)
	if m == nil {
		return nil, fmt.Errorf("unexpected pattern %q", p)
	}
	return strings.Split(m[1], "|"), nil
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, "genpythonlists:", msg)
	os.Exit(1)
}
