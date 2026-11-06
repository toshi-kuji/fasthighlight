package compat

import (
	"math/rand/v2"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// pieces are fragments, per language, of the parts where the rules split finely. Documents made by
// arranging them are compared with Chroma. This tries combinations that the byte-level fuzz test rarely
// produces (backtracking, blocks, here documents, and so on).
var pieces = map[string][]string{
	"C": {
		"\n", " ", "\t", "#", "#if 0", "#if 1", "#else", "#elif", "#endif", "#include", "include", " <a.h>", "\"a.h\"",
		"#define X", "\\", "\\\n", "/*", "*/", "/* c */", "//", "// c", "\"", "'", "'a'", "'\\n'", "'\\x4", "'\\0",
		"L", "L\"", "\\x41", "\\777", "\\u1234", "{", "}", "(", ")", "[", "]", ";", ",", ".", ":", "::", "?", "*",
		"&", "->", "=", "==", "int", "int ", "char", "char16_t", "size_t", "my_t", "uint8_t", "void", "static ",
		"const", "struct", "if", "return", "goto", "NULL", "true", "__declspec", "__m128", "inline", "main", "f",
		"x", "_y", "label:", "0", "1", "0x1F", "0X1", "017", "08", "1.", ".5", "1e5", "1.5f", "2F", "e-", "UL", "u",
		"日本", "٣", "\u00a0",
	},
	"C++": {
		"\n", " ", "\t", "#", "#if 0", "#endif", "#include", " <a>", " \"a\"", "/*", "*/", "//", "\"", "'", "'a'",
		"'\\n'", "R\"(", ")\"", "R\"x(", ")x\"", "R\"x{,16}(", ")x{,16}\"", "u8\"", "u\"", "U\"", "L'", "{", "}",
		"(", ")", "[", "]", "[[", "]]", "[[nodiscard]]", ";", ",", ":", "::", "<", ">", "*", "&", "=", "class",
		"class ", "struct ", "enum ", "enum class ", "union", "template", "typename", "namespace", "public",
		"virtual", "override", "nullptr", "this", "co_await", "int", "wchar_t", "size_t", "auto", "return", "if",
		"std", "x", "label:", "0", "1", "0x1'F", "0b1'0", "1'000", "0'7", "017", "08", "1.", ".5", "1e5", "1.5f",
		"'", "LL", "日本", "٣", "\u00a0",
	},
	"Java": {
		"\n", " ", "\t", "//", "// c\n", "/*", "*/", "/** d */", "\"", "\"\"\"", "\"\"\"\n", "\\", "\\\"", "'", "'a'",
		"'\\n'", "'\\u0041'", "'''", "@", "@Override", "@a.b", "package ", "import ", "import static ", "static ",
		"module", "module ", "class ", "interface", "record ", "public ", "private ", "var ", "var", "default:",
		"default", "label:", ":", "case", "new", "return", "if", "non-sealed", "sealed", "final", "int", "void",
		"true", "null", "String", "List<String>", "x[]", "?", "a.b", ".", "x", "$y", "_", "f(", "(", ")", "{", "}",
		"[", "]", ";", ",", "<", ">", "=", "+", "0", "1", "0x1F", "0x1.8p3", "0b1", "017", "1_000", "1.", ".5",
		"1e5", "5f", "2d", "L", "e-", "p+", "日本", "٣", "\u00a0",
	},
	"C#": {
		"\n", " ", "\t", "\\", "\\\n", "//", "///", "/*", "*/", "\r", "[", "]", "[A]", "\"", "\"\"", "@\"", "$\"",
		"$@\"", "'", "'a'", "'\\n'", "#", "#if", "# region", "#pragma", "extern", "extern alias", "alias", "global::",
		"global", "::", "using ", "namespace ", "class ", "struct ", "record ", "interface ", "public", "async",
		"static", "int", "int?", "string", "var", "true", "null", "get", "value", "x", "@x", "_y", "a.b", ".", "(",
		")", "{", "}", ";", ",", "<", ">", "=", "?", "0", "1", "0x1F", "1_000", "1.5", "1e5", "2f", "3L", "e+",
		"日本", "٣", "\u00a0",
	},
	"Kotlin": {
		"\n", " ", "\t", "\\", "\\\n", "//", "// c\n", "/*", "*/", "[", "]", "[A]", "\"", "\"\"\"", "\"\"", "$", "$x",
		"${", "}", "{", "\\$", "\\n", "\\u0041", "\\q", "'", "'a'", "'\\n'", "'\\u0041'", "`", "`a b`", "`@x1`", "@",
		"@Ann", "<", ">", ",", ":", "*", "?", "in", "out", "reified", "inner", "fun ", "val ", "var ", "class ",
		"object ", "companion object", "package ", "import ", "interface ", "as", "as?", "is", "!is", "!in", "!==",
		"===", "?:", "?.", "!!", "::", "..", "->", "=", "+", "-", ".", "x", "_y", "a_b", "Foo", "Foo.", "it", "this",
		"0", "1", "0xFF", "0x_1", "0b1", "1_000", "1__0", "1.5", ".5", "1e5", "1e-5", "2f", "3L", "4uL", "e+",
		"日本", "٣", "\u00a0",
	},
	"Swift": {
		"\n", " ", "\t", "//", "/*", "*/", "/* /* */", ":param: x", ":returns:", "TODO:", "MARK:", "FIXME:", "#if",
		"#else", "#endif", "#available", "#selector", "#file", "#", "import", "import ", "class ", "class X",
		"func f", "var x", "let y", "struct ", "protocol", "extension", "@objc", "@availability(iOS)", "@",
		"\"", "\\", "\\(", ")", "(", "\\n", "\\x4", "\\u{1}", "\\0", "{", "}", "[", "]", ".", ",", ":", ";", "=", "?",
		"!", "->", "<", ">", "&", "+", "-", "*", "/", "x", "_", "self", "nil", "true", "as", "is", "in", "Int",
		"String", "print", "map", "$0", "$", "0", "1", "0x1F", "0b1", "0o7", "1_000", "1.", "1.5", "1.5e3",
		"1e5", "e+", "日本", "٣", "\u00a0",
	},
	"Go": {
		"\n", " ", "\t", "//", "//go:build", "// c", "/*", "*/", "/*\\\n", "\\", "\\\n", "\"", "\\\"", "\\\\", "'",
		"'a'", "'\\n'", "'\\x41'", "'\\101'", "'\\u00e9'", "'''", "`", "{{", "}}", "{{-", "-}}", "{{/*", "*/}}",
		"{{ .Name }}", "$", "$x", ".", ".A", "|", ":=", "(", ")", "[", "]", "{", "}", ",", ";", ":", "=", "==",
		"<-", "<<=", "&^", "...", "+", "-", "*", "&", "package", "import", "func", "func ", "type", "struct", "var",
		"go", "if", "range", "true", "nil", "len", "len(", "string", "int", "any", "x", "_", "f(", "f (", "0",
		"1", "017", "08", "0x1F", "0b1", "1_000", "1.", ".5", "1.5", "1e5", "1e+5", "2i", "1.5i", "1e-5i", "e+",
		"日本", "٣", "\u00a0",
	},
	"Rust": {
		"\n", " ", "\t", "#!", "#![", "#[", "]", "[", "#", "//", "//!", "///", "////", "/*", "*/", "/**", "/*!",
		"\"", "\\", "\\\"", "\\n", "\\x41", "\\x80", "\\u{1F600}", "\\0", "{", "}", "{{", "}}", "{x}", "b\"",
		"b'a'", "r\"", "r#\"", "\"#", "br\"", "'", "'a", "'a'", "'\\n'", "'static", "'_", "$", "$x", "$(", "$),",
		"fn ", "fn", "mod ", "let", "struct ", "enum", "type", "default type", "self", "Self", "impl", "pub",
		"break", "continue 'a", "loop", "true", "i32", "u8", "str", "Vec", "Option", "Some", "::", ":", "->", "&",
		"..", "..=", ".", ",", ";", "(", ")", "=", "+", "-", "!", "?", "@", "MAX", "MAX_A", "_AB", "A1", "x", "_y",
		"r#x", "println!", "println!(\"", "vec!", "vec![", "format!(", "0", "1", "1_000", "0x1F", "0o7", "0b1",
		"1.", "1.5", "1.5.", "1e5", "u8", "i64", "f32", "usize", "e+", "日本", "٣", "\u00a0",
	},
	"SQL": {
		"\n", " ", "\t", "--", "-- c\n", "/*", "*/", "'", "''", "\"", "\"\"", "SELECT", "select", "FROM", "where",
		"END", "END-EXEC", "end-exec", " TEMP", "TEMP", "INT", "int", "VARCHAR", "x", "_y", "a$b", "Key", "ſelect",
		"Key", "İNT", "(", ")", "[", "]", ",", ";", ":", ".", "*", "=", "<>", "!", "@", "#", "`", "?", "-",
		"+", "0", "1", "12", "1.5", "$", "{", "日本", "٣", " ",
	},
	"R": {
		"\n", " ", "\t", "#", "# c", "'", "\"", "\\", "\\'", "\\\"", "\\\n", "`", "`a b`", "`a\\`b`", "if", "else",
		"function", "function(", "for", "in", "x", "x.y", ".x", "._", "_x", ".", "..", "...", "..1", "^", "[", "]",
		"[[", "]]", "(", ")", "{", "}", ";", ",", "<-", "<<-", "->", "->>", "-", "+", "*", "/", "%", "%in%",
		"%%", "=", "==", "!=", "!", "&", "&&", "|", "||", "?", "~", "$", "@", ":", "::", ":::", "TRUE", "T",
		"F", "NA", "NA_integer_", "NULL", "Inf", "pi", "letters", "0", "1", "2.5", ".5", "1e5", "1e-5", "5L",
		"2i", "0x1F", "0x1p2", "-1", "+.", "f (", "日本", "٣", " ",
	},
	"PHP": {
		"\n", " ", "\t", "<?php", "?>", "<?", "#", "# c\n", "#[", "]", "//", "// c\n", "/*", "*/", "/**", "/**/",
		"'", "\\'", "\"", "\\\"", "\\n", "\\x4", "\\101", "\\\\", "$", "$x", "$$x", "${", "}", "{", "{$", "{${", "}}",
		"[0]", "[ ]", "->", "->y", "::", "::C", "`", "<<<", "<<<EOT\n", "<<<'EOT'\n", "EOT", "EOT;\n", "\nEOT\n",
		"class ", "new class", "function", "function (", "function &", "const X", "__construct", "__CLASS__",
		"echo", "ECHO", "if", "true", "Null", "x", "_y", "\\A\\B", "é", "(", ")", ";", ",", "=", "==", ".", "?",
		"??", "!", "@", "0", "1", "017", "0o7", "0x1F", "0X1f", "0b1", "1_000", "1.", "1.5", ".5", "1e5", "1E-5",
		"日本", "٣", " ", "K",
	},
	"Ruby": {
		"\n", " ", "\t", "#", "# c\n", "#!", "=begin ", "\n=end", "'", "\\'", "\"", "\\\"", "\\n", "\\x4", "#{", "}",
		"#@x", "#@@x", "#$x", "`", ":", "::", ":sym", ":@x", ":+", ":[]", ":'s'", ":\"", "key:", "%w[", "]", "%q(",
		")", "%Q{", "%r{", "%r/", "/i", "%(", "%<", ">", "% ", "%|", "|", "/", "/re/", " /", "= /", "(", ",", "[",
		"<<EOS\n", "<<-'EOS'\n", "<<\"\"\n", "EOS", "__END__", "__FILE__", "def ", "def +", "def self.", "class ",
		"class << self", "module A::B", "defined?", "defined? x", "if", "when ", "and ", "or", "not", "nil?",
		"chomp!", "puts", "attr_reader", "new", "true", "@x", "@@y", "$z", "$!", "$-w", "?a", "?\\n", "?\\M-a",
		"Foo", "A", "x", "x?", "y!", "_z", "obj.m", ".+", "&.", "<=>", "**", "=~", "!~", "&&", "||", "...", "..",
		"=", "==", "+=", "0", "1", "017", "0x1F", "0b1", "1_000", "1e5", "1.5", "日本", "٣", "\u00a0",
	},
	"Python": {
		"\n", " ", "\t", "  ", "#", "#!", "\\", "\\\n", "\"", "'", "\"\"\"", "'''", "f", "F", "r", "R", "b", "u",
		"rb", "Br", "rf", "fR", "{", "}", "{{", "}}", "(", ")", "[", "]", ":", "=", "!r", "!", ".", ",", ";", "%",
		"%s", "%(k)d", "%-5.2f", "{0}", "{x.y[1]}", "{:>10}", "{:0=+#8,.2f}", "\\n", "\\x4", "\\N{X}", "\\u12345",
		"\\777", "def ", "class ", "from ", "import ", " import ", " as ", "as", "None", "True", "self", "print",
		"__init__", "__name__", "ValueError", "yield", "yield from", "async", "async for", "for", "in", "is", "not",
		"and", "lambda", "match", "case", "if", "else", "x", "_y", "@", "@a.b", "@a . b", "0", "1", "1_0", "1.",
		".5", "1e5", "e-", "2j", "0x1F", "0o7", "0b1", "_", "==", "!=", "<<", ":=", "->", "**", "{,2}", "日本", "٣",
		"²", "\u00a0", "{:}", "{:}>", "<", ">", "^", "*", "%0*d", "%.*f", "%(", "N{", "{a!s}", "{a=}", "= ", "{x[}",
		"[]", "-", "+", "\\\\", "\\'", "\\\"", "\\U0001F600", "\\N{", "é", "\u0301", "Ⅻ", "\t", "\r",
	},
	"YAML": {
		"\n", " ", "  ", "\t", "- ", "? ", ": ", ":", "#", " #c", " //c", "//", "---", "...", "|", "|-", ">+", "\n  ",
		"\n    ", "key", "a b", "\"", "'", "\\", "\\\"", "{", "}", "[", "]", ",", "!!str", "&a", "*a", "%include x",
		"0x1f", "0o7", "12", "1.", ".5", "1e5", "e+", "+", "-", ".inf", ".nan", "2001-12-14", "T21:59:43", ".10",
		"Z", " -5", "true", "no", "y", "Yes", "@", "`", "http://x", "日本", "٣", " ",
	},
	"markdown": {
		"\n", " ", "  ", "\t", "#", "# ", "## ", "###", "####### ", "-", "- ", "* ", "*", "**", "_", "__", "~", "~~",
		"+ ", "1. ", "12.", "> ", ">", "- [ ] ", "- [x] ", "* [X] ", "[ ]", "[", "]", "(", ")", "](", "!", "![",
		"[a](b)", "`", "``", "```", "```\n", "```py\n", "```go\n", "```md\n", "```yaml\n", "```json\n", "```nolang\n",
		"```py", "\n```\n", "\n```", "---\n", "---", "<!--", "-->", "<!-- c -->", "\\", "\\*", "@", "#tag", "@a/b:c",
		"a", "word", "x = 1", "print(1)", "a: 1", "\"", "'", "日本", "٣", "\u00a0", "\u3000",
	},
	"JSON": {
		"\n", " ", "{", "}", "[", "]", ":", ",", "\"", "\"k\"", "\\", "\\\"", "true", "false", "null", "nullx",
		"-", "0", "01", "12", ".5", "e5", "E-3", "1.5e+2", "//c\n", "//", "/", "日本", "٣", "\u00a0",
	},
	"TOML": {
		"\n", " ", "#", "[", "]", "[[", "{", "}", "=", ",", ".", "\"", "'", "\\", "key", "a-b_c", "true", "false",
		"+", "-", "1", "1_000", "_", "3.14", "1979-05-27T07:32:00", "Z", "-08:00", ".9+", "$", "日本", "٣",
	},
	"Diff": {
		"\n", " ", "+", "-", "---", "+++", "@@ -1 +1 @@", "<", "< ", ">", "> ", "!", "=", "diff", "Index", "index",
		"1", "1,2", "a", "c", "d", "3", "x", "日本",
	},
	"CSS": {
		"\n", " ", "{", "}", ";", ":", "::", ",", ".", "#", "@", "@media", "a", "div", "--v", "color", "margin",
		"border-bottom-color", "inline", "inline-block", "-webkit-", "-moz-", "url(", "url(\"a\")", ")", "(", "rgb(",
		"calc(", "foo(", "1", "1.5", ".5", "-", "+", "px", "%", "em", "#fff", "#abcdefg", "!important", "\"", "'",
		"\\", "/*", "*/", "*", "/", "red", "x-large", "repeat no-repeat", "sans-serif", "日本", "٣",
	},
	"XML": {
		"\n", " ", "<", ">", "/", "/>", "</", "=", "\"", "'", "&", ";", "&amp;", "a", "b:c", "x.y", "-", "<!--",
		"-->", "<!", "<?", "?>", "<![CDATA[", "]]>", "text", "日本",
	},
	"TypeScript": {
		"\n", " ", "\t", "x", "y1", "$a", "_b", "a_b", "var", "let", "function", "return", "if", "in", "of", "keyof",
		"this", "true", "null", "class", "const", "type", "interface", "as", "string", "number", "any", "module",
		" \"m\" ", "declare", "super", "super(", "Math", "window", "=>", "(", ")", "() {", "{", "}", "[", "]", ";", ",",
		".", "...", "=", "===", "!==", ">>>=", "<<", "&&", "||", "?", ":", " : ", "+", "++", "-", "--", "*", "/", "<",
		">", "</", "/>", "<>", "</>", "<div", "</div>", "<a.b", " a=", "={", "=\"v\"", "='v'", "-x", "\"", "'", "`",
		"${", "$", "\\", "\\`", "\\n", "\\\n", "//", "/*", "*/", "<!--", "-->", "0", "1.5", "1.5e3f", "1e3",
		"0x1F", "12", "/re/g", "/[/]/", "/a/gx", "@dec", "#", "日本", "٣", "\u00a0",
	},
	"JavaScript": {
		"\n", " ", "\t", "x", "y1", "$a", "_b", "a_b", "var", "let", "function", "return", "if", "in", "instanceof",
		"of", "this", "true", "null", "NaN", "class", "await", "Math", "Error", "=>", "(", ")", "{", "}", "[", "]", ";",
		",", ".", "...", "=", "==", "===", "!==", ">>>=", "<<", "**", "&&", "||", "??", "?.", "?", ":", "+", "++", "-",
		"--", "*", "/", "%", "<", ">", "\"", "'", "`", "${", "$", "\\", "\\`", "\\\n", "//", "/*", "*/", "#!",
		"#! /", "<!--", "0", "1.", "1.5", "1e3", "1e", "0x1F", "0b1", "0o7", "1_000", "/re/g", "/[/]/", "/a/gx", "[",
		"\\u00e9", "日本", "٣", "\u00a0",
	},
	"HTML": {
		"\n", " ", "<", ">", "/", "/>", "</", "=", "\"", "'", "&", ";", "&amp;", "div", "p", "a b", "x:y", "-",
		"<!--", "-->", "<!", "<?", "?>", "<![CDATA[", "<![cdata[", "]]>", "<script>", "</script>", "<SCRIPT", "script",
		"<style>", "</style>", "style", "< / script >", "var x = 1;", "`a${b}`", "{", "}", "a{color:red}", "/* c */",
		"// c", "text", "日本", "İ", "<SCRİPT>", "</SCRİPT>", "<ſtyle>", "</ſtyle>", "<\u212abd>",
	},
	"Bash": {
		"\n", " ", "\t", "#", "#!", "/bin/sh", "if", "then", "fi", "do", "done", "echo", "type", "typeset", "x", "VAR",
		"=", "+=", "\"", "'", "$'", "$\"", "\\", "$", "$(", "$((", "))", ")", "(", "${", "${#", "}", ":-", ":", "`",
		"<<", "<<-", "<<<", "EOF", "'EOF'", "&&", "||", "&", "|", ";", "<", ">", "[", "]", "{", "1", "16#", "2#1",
		"**", "*", "-", "%", "$@", "$1", "$_a", "日本", "٣",
	},
	"PowerShell": {
		"\n", " ", "\t", "#", "##", "#requires", "#Requires ", "-Version", "-Modules", "-x", "<#", "#>", "&lt;#", "#&gt;",
		".", ".synopsis", ".Notes", ".link", "(", ")", "$(", "@(", "@{", "{", "}", "[", "]", "[int]", "[a.b[]]", "@\"\n",
		"\n\"@", "@'\n", "\n'@", "\"", "\"\"", "'", "''", "`", "`n", "`N", "`$", "`\"", "$", "$x", "$_", "${", "${env:x}",
		"$env:", "$global:x", "global:", "script:", "@@", "@", "$true", "$TRUE", "$null", "$false", "function ",
		"filter", "workflow", "class ", "configuration", ":", "::", "x", "a", "C:", "Get-Item", "foreach", "ForEach",
		"if", "in", "else", "param", "Parameter", "alias", "mandatory", "helpuri", "?", "%", "-eq", "-and", "-NotLike",
		"-f", "-file", "ls ", "cd", "sort", "r ", "1", "1.5", ".5", "1e5", "-1", "+2", "10kb", "3MB", "d", "m", "f",
		"=", "+", "*", "|", ";", ",", "<", ">", "&", "!", "-", "_", "K", "ſ", "İ", "日本", "٣", " ",
	},
	"GraphQL": {
		"\n", " ", "\t", ",", "#", "# c", "query", "mutation", "type", "types", "input", "enum", "on", "extend", "schema",
		"directive", "...", "..", ".", "FIELD", "FIELD_DEFINITION", "QUERY", "x", "Name", "_a", "1", "12", "0", ".5",
		".5i", "e", "E", "+", "-", "i", "@", "@dep", ":", "$", "$v", "(", ")", "{", "}", "[", "]", "!", "|", "=", "\"",
		"\"\"", "\"\"\"", "\\", "\\\"", "\\x41", "\\u0041", "\\U0001F600", "\\101", "\\8", "true", "null", "日本", "é",
		"٣", " ",
	},
	"Protocol Buffer": {
		"\n", " ", "\t", "/", "//", "/*", "*/", "*", "\\", "\\\n", "\"", "'", "a", "a.b", "_x", "=", " = ", "+", "-",
		";", ",", "{", "}", "[", "]", "(", ")", "<", ">", "package", "package ", "message ", "extend ", "enum ",
		"service ", "rpc ", "oneof ", "group ", "import", "syntax", "option", "default", "map", "extensions", "int32",
		"string", "true", "false", "inf", "-inf", "nan", "0", "1", "07", "08", "0x1F", "0x", "1.", ".5", "1e5", "e",
		"E", "f", "F", "L", "u", "日本", "é", "٣", " ",
	},
}

// TestRandomPiecesMatchChroma checks that any arrangement of the fragments gives exactly the same token
// sequence as Chroma. FASTHIGHLIGHT_RANDOM_N changes the number of runs (the default is 2000 per language).
// Setting FASTHIGHLIGHT_RANDOM_LANG to a language name (for example Python) tries only that language.
func TestRandomPiecesMatchChroma(t *testing.T) {
	n := 2000
	if s := os.Getenv("FASTHIGHLIGHT_RANDOM_N"); s != "" {
		n, _ = strconv.Atoi(s)
	}
	only := os.Getenv("FASTHIGHLIGHT_RANDOM_LANG")
	for name, ps := range pieces {
		if only != "" && name != only {
			continue
		}
		l := lookup(t, name)
		r := rand.New(rand.NewPCG(7, uint64(len(name))))
		for range n {
			var b strings.Builder
			for range 1 + r.IntN(24) {
				b.WriteString(ps[r.IntN(len(ps))])
			}
			code := b.String()
			if incomparable(l, code) != "" {
				continue
			}
			ours := chromaMode(l)(code)
			theirs, slow := chromaTokensTimed(t, l, code)
			if !slices.Equal(ours, theirs) {
				if slow {
					t.Logf("%s: skipped %q: tokens differ, but Chroma may have hit its regexp timeout", name, code)
					continue
				}
				t.Fatalf("%s: tokens differ from Chroma for %q\n ours %v\nchroma %v", name, code, ours, theirs)
			}
		}
	}
}
