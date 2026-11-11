// genlists extracts the word lists from Chroma's rules and generates lists.go in each language package.
// The supported languages are in configs. After upgrading Chroma, run the following in compat/.
//
//	go run ./cmd/genlists java > ../internal/java/lists.go
//	go run ./cmd/genlists kotlin > ../internal/kotlin/lists.go
//	go run ./cmd/genlists csharp > ../internal/csharp/lists.go
//	go run ./cmd/genlists swift > ../internal/swift/lists.go
//	go run ./cmd/genlists go > ../internal/golang/lists.go
//	go run ./cmd/genlists rust > ../internal/rust/lists.go
//	go run ./cmd/genlists sql > ../lang/sql/lists.go
//	go run ./cmd/genlists r > ../internal/r/lists.go
//	go run ./cmd/genlists php > ../internal/php/lists.go
//	go run ./cmd/genlists ruby > ../internal/ruby/lists.go
//	go run ./cmd/genlists typescript > ../internal/typescript/lists.go
//	go run ./cmd/genlists powershell > ../lang/powershell/lists.go
package main

import (
	"fmt"
	"os"
	"path"
	"regexp/syntax"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

// List is a list to extract (Go constant name, state, rule index, the parts before and after, description).
type List struct {
	Name, State string
	// Lexer is the Chroma language to take the rules from (Config.Lexer if omitted).
	Lexer string
	Idx   int
	// Prefix and Suffix are the parts before and after the (…) of the alternatives.
	Prefix, Suffix string
	Doc            string
}

// Config is the configuration of one language.
type Config struct {
	Lexer, Package, XML string
	Lists               []List
}

// Configs are the supported languages. The tests in compat/ use it too.
var Configs = map[string]Config{
	"java": {"Java", "java", "embedded/java.xml", []List{
		{"keywords", "root", "", 4, "", `\b`, "reserved words (Keyword)"},
		{"declarations", "root", "", 7, "", `\b`, "declaration words (KeywordDeclaration). Some words contain -, such as non-sealed"},
		{"types", "root", "", 8, "", `\b`, "types (KeywordType)"},
		{"constants", "root", "", 10, "", `\b`, "constants (KeywordConstant)"},
	}},
	"kotlin": {"Kotlin", "kotlin", "embedded/kotlin.xml", []List{
		{"keywords", "root", "", 23, "", `\b`, "reserved words (Keyword). as? never matches, because the earlier as matches first"},
	}},
	"swift": {"Swift", "swift", "embedded/swift.xml", []List{
		{"keywords", "keywords", "", 0, "", `\b`, "reserved words (Keyword). Some words start with #, such as #selector"},
		{"reserved", "keywords", "", 2, "", `\b`, "reserved words (KeywordReserved). Some words start with @, such as @objc"},
		{"constants", "keywords", "", 3, "", `\b`, "constants (KeywordConstant). as and is are matched first by the earlier reserved words"},
		{"declarations", "keywords", "", 8, "", `\b`, "declaration words (KeywordDeclaration)"},
		{"builtins", "root", "", 6, "", `\b`, "built-in types and so on (NameBuiltin)"},
		{"builtinPseudo", "root", "", 7, "", `\b`, "built-in functions (NameBuiltinPseudo)"},
	}},
	"go": {"Go", "golang", "go.go", []List{
		{"declarations", "root", "", 6, "", `\b`, "declaration words (KeywordDeclaration)"},
		{"keywords", "root", "", 7, "", `\b`, "reserved words (Keyword)"},
		{"constants", "root", "", 8, "", `\b`, "constants (KeywordConstant)"},
		{"builtins", "root", "", 9, "", `\b(\()`, "built-in functions and types (NameBuiltin). Must be followed by ("},
		{"types", "root", "", 10, "", `\b`, "types (KeywordType)"},
		{"templateKeywords", "expression", "Go Text Template", 2, "", `\b`, "reserved words (Keyword) of Go templates (inside a raw string)"},
	}},
	"rust": {"Rust", "rust", "embedded/rust.xml", []List{
		{"keywords", "base", "", 10, "", `\b`, "reserved words (Keyword)"},
		{"reserved", "base", "", 11, "", `\b`, "reserved words (KeywordReserved)"},
		{"constants", "base", "", 12, "", `\b`, "constants (KeywordConstant)"},
		{"types", "base", "", 19, "", `\b`, "types (KeywordType)"},
		{"builtins", "base", "", 21, "", `\b`, "built-in types and so on (NameBuiltin)"},
	}},
	"sql": {"SQL", "sql", "embedded/sql.xml", []List{
		{"keywords", "root", "", 5, "", `\b`, "reserved words (Keyword). Case-insensitive"},
		{"builtins", "root", "", 6, "", `\b`, "type names (NameBuiltin). Case-insensitive"},
	}},
	"r": {"R", "r", "embedded/r.xml", []List{
		{"keywords", "keywords", "", 0, "", `(?![\w.])`, "reserved words (KeywordReserved). Not followed by a word character or ."},
	}},
	"php": {"PHP", "php", "embedded/php.xml", []List{
		{"keywords", "root", "", 18, "", `\b`, "reserved words (Keyword). Case-insensitive"},
		{"constants", "root", "", 19, "", `\b`, "constants (KeywordConstant). Case-insensitive"},
		{"magicConstants", "magicconstants", "", 0, "", `\b`, "special constants (NameConstant). Case-insensitive"},
		{"magicFuncs", "magicfuncs", "", 0, "", `\b`, "special methods (NameFunctionMagic). Case-insensitive"},
	}},
	"ruby": {"Ruby", "ruby", "embedded/ruby.xml", []List{
		{"keywords", "root", "", 3, "", `\b`, "reserved words (Keyword). Some words contain ?, such as defined?"},
		{"pseudo", "root", "", 8, "", `\b`, "commonly used methods and so on (KeywordPseudo)"},
		{"operatorWords", "root", "", 9, "", `\b`, "word operators (OperatorWord)"},
		{"builtinQuestions", "root", "", 10, "", `\?`, "built-in methods ending in ? (NameBuiltin). Needs a trailing ? (no \\b)"},
		{"builtins2", "root", "", 12, `(?<!\.)`, `\b`, "built-in methods 2 (NameBuiltin). Not preceded by ."},
	}},
	"typescript": {"TypeScript", "typescript", "embedded/typescript.xml", []List{
		{"keywords", "root", "", 6, "", `\b`, "reserved words (Keyword). No \\b before"},
		{"declarations", "root", "", 7, "", `\b`, "declaration words (KeywordDeclaration). No \\b before"},
		{"reserved", "root", "", 8, "", `\b`, "reserved words (KeywordReserved). No \\b before"},
		{"constants", "root", "", 9, "", `\b`, "constants (KeywordConstant). No \\b before"},
		{"builtins", "root", "", 10, "", `\b`, "built-in names (NameBuiltin). No \\b before"},
		{"types", "root", "", 12, `\b`, `\b`, "types (KeywordType). Needs \\b before"},
		{"reserved2", "root", "", 13, `\b`, `\b`, "reserved words 2 (KeywordReserved). Needs \\b before"},
	}},
	"csharp": {"C#", "csharp", "embedded/c#.xml", []List{
		{"preprocessor", "root", "", 14, `#[ \t]*`, `\b`, "preprocessor words (CommentPreproc). Preceded by #[ \\t]*"},
		{"keywords", "root", "", 16, "", `\b`, "reserved words (Keyword)"},
		{"declarations", "root", "", 18, "", `\b`, "declaration words (KeywordDeclaration)"},
		{"types", "root", "", 19, "", `\b\??`, "types (KeywordType). One ? may follow"},
		{"constants", "root", "", 20, "", `\b`, "constants (KeywordConstant)"},
	}},
	"powershell": {"PowerShell", "powershell", "embedded/powershell.xml", []List{
		{"builtins", "root", "", 17, "", `\b`, "attribute names and so on (NameBuiltin). Case-insensitive; some words appear twice"},
		{"attributes", "root", "", 19, "", `\b`, "parameter attribute arguments (NameAttribute). Case-insensitive"},
		{"attributes2", "root", "", 20, "", `\b`, "cmdlet binding arguments (NameAttribute). Case-insensitive"},
		{"keywords", "root", "", 21, "", `\b`, "reserved words (Keyword). Case-insensitive; some words contain ?, %, # or :"},
		{"operators", "root", "", 22, "-", `\b`, "operators after - (Operator). Case-insensitive"},
		{"aliases", "root", "", 23, "", `\s`, "built-in aliases (NameBuiltin). Case-insensitive; followed by \\s, not \\b"},
		{"docWords", "multline", "", 2, `(\s*\.)`, `(\s*$)`, "comment-based help keywords after . (LiteralStringDoc). Case-insensitive"},
	}},
}

func main() {
	if len(os.Args) != 2 {
		fail("usage: genlists java|csharp|kotlin|swift|go|rust|sql|r|php|ruby|typescript|powershell")
	}
	cfg, ok := Configs[os.Args[1]]
	if !ok {
		fail("unknown language " + os.Args[1])
	}
	var b strings.Builder
	fmt.Fprintf(&b, "// Code generated from Chroma 2.27.0 lexers/%s. DO NOT EDIT.\n\n", cfg.XML)
	fmt.Fprintf(&b, "// Chroma (Copyright (C) 2017 Alec Thomas, MIT License) %s: the alternatives of its regular\n", path.Base(cfg.XML))
	b.WriteString("// expressions, copied in the same order. TestWordListsMatchChroma in compat/ compares every word with Chroma.\n\n")
	fmt.Fprintf(&b, "package %s\n", cfg.Package)
	for _, l := range cfg.Lists {
		lexer := cfg.Lexer
		if l.Lexer != "" {
			lexer = l.Lexer
		}
		w, err := Words(lexer, l)
		if err != nil {
			fail(err.Error())
		}
		fmt.Fprintf(&b, "\n// %s holds %s.\nconst %s = %q\n", l.Name, l.Doc, l.Name, strings.Join(w, "|"))
	}
	os.Stdout.WriteString(b.String())
}

// Words returns the alternatives of the rule l of lexer, in order.
func Words(lexer string, l List) ([]string, error) {
	rl, ok := lexers.Get(lexer).(*chroma.RegexLexer)
	if !ok {
		return nil, fmt.Errorf("Chroma's %s lexer is not a RegexLexer", lexer)
	}
	rules, err := rl.Rules()
	if err != nil {
		return nil, err
	}
	p := rules[l.State][l.Idx].Pattern
	inner, ok := strings.CutPrefix(p, l.Prefix+"(")
	if ok {
		inner, ok = strings.CutSuffix(inner, ")"+l.Suffix)
	}
	if !ok {
		return nil, fmt.Errorf("%s %s[%d]: unexpected pattern %q", lexer, l.State, l.Idx, p)
	}
	re, err := syntax.Parse(inner, syntax.Perl)
	if err != nil {
		return nil, err
	}
	return expand(re)
}

// expand returns every string that re (made only of characters, alternatives and ?) matches, in the order
// the regular expression tries them.
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
	case syntax.OpAlternate, syntax.OpConcat:
		out := []string{""}
		if re.Op == syntax.OpAlternate {
			out = nil
		}
		for _, s := range re.Sub {
			sub, err := expand(s)
			if err != nil {
				return nil, err
			}
			if re.Op == syntax.OpAlternate {
				out = append(out, sub...)
				continue
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
	fmt.Fprintln(os.Stderr, "genlists:", msg)
	os.Exit(1)
}
