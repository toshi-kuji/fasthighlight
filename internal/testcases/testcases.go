// Package testcases holds the inputs shared by the tests of the main package and the comparison tests in compat/.
package testcases

import (
	"fmt"
	"os"
	"regexp"
)

// Block is one code block. Lang is "ts" or "js".
type Block struct{ Lang, Code string }

// InputBlocks extracts the ts and js code blocks from the document used for measurement (testdata/input.md).
func InputBlocks(path string) ([]Block, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	re := regexp.MustCompile("(?ms)^```(ts|js)\n(.*?)^```")
	var bs []Block
	for _, m := range re.FindAllStringSubmatch(string(src), -1) {
		bs = append(bs, Block{m[1], m[2]})
	}
	if len(bs) != 3 {
		return nil, fmt.Errorf("ts/js blocks = %d, want 3", len(bs))
	}
	return bs, nil
}

// EdgeCases are edge cases.
var EdgeCases = []Block{
	{"ts", "const x = 1\n"},
	{"ts", "const a = b / c / d\nconst r = /ab+c/gi.test(s)\n"},
	{"ts", "const s = `a${b}c${f({ k: 1 })}d`\n"},
	{"ts", "// line\n/* block\n */ let y = 'it\\'s' + \"q\\\"q\"\n"},
	{"ts", "type P = { name: string; age?: number }\nfunction f(p: P): void {}\n"},
	{"ts", "x === y !== z && a ?? b?.c ... => >>>= 0x1F 3.14e2\n"},
	{"ts", "@decorator class A { constructor() {} }\n"},
	{"ts", "const 名前 = 'テスト'\n"},
	{"ts", "const m = new Map<string, number>()\nconst s = `line\\n${m.size}`\n"},
	{"ts", "unterminated 'abc\n"},
	{"ts", "`unterminated ${a\n"},
	{"ts", "/bad regex\nok\n"},
	{"js", "#!/usr/bin/env node\nconst n = [1, 1.5, 1e3, 0b101, 0o17, 0xFF, 1_000]\n"},
	{"js", "const f = (...a) => a.map((x) => x * 2)\n"},
	{"js", "let 変数 = /[/]x/u; let my_var = $el\n"},
	{"js", "a = b\n/re/g.test(c)\n"},
	{"js", ""},
	// Inputs without a trailing newline (to check how Chroma's ensure_nl is handled).
	{"ts", "const x = 1"},
	{"ts", "x "},
	{"ts", "// c"},
	{"ts", "'abc"},
	{"ts", "`abc"},
	{"ts", "/* c"},
	{"js", "a\n/re/g"},
	{"js", "f()\n"},
	{"js", "const s = `a${f({ k: 1 })}b`\n"}, // intended difference for template literals (JavaScript)

	// JSON
	{"json", "{\n  \"name\": \"fasthighlight\",\n  \"version\": 1.5e3,\n  \"tags\": [\"go\", true, null, -0.25],\n  \"nested\": {\"a\": {}}\n}\n"},
	{"json", "// comment\n[1, 2, 3]\n"},
	{"json", "{\"esc\": \"a\\\"b\\\\\", \"k\": 01, \"e\": 1e+5, \"bad\": 1.}\n"},
	{"json", "{\"a\" 1 : , }\n} ] trueish true1 nullx\n"},
	{"json", "\"unterminated \\\" string"},
	{"json", "[1٣, \"日本\", 2\u00a0]\n"},
	{"json", "// no newline at end"},
	{"json", "{\"a\": 1"},
	// TOML
	{"toml", "# comment\n[server]\nhost = \"localhost\"\nport = 8080\nratio = -1_000.5\nenabled = true\n"},
	{"toml", "[[products]]\nname = 'Hammer'\nsku = 738_594_937\ndate = 1979-05-27T07:32:00Z\nlocal = 1979-05-27T07:32:00-08:00\n"},
	{"toml", "points = [ { x = 1, y = 2 }, { x = 7, y = 8 } ]\nstr = \"a\\\"b\"\nbad = 1_ +x 1.e3 trueish\n"},
	{"toml", "multi = \"\"\"\nline\n\"\"\"\nkey.sub-part = 0x1F\n"},
	{"toml", "frac = 1979-05-27T07:32:00.9+Z\nodd = $ % ^"},
	// diff
	{"diff", "diff --git a/x.go b/x.go\nindex 1234..5678 100644\n--- a/x.go\n+++ b/x.go\n@@ -1,3 +1,3 @@\n context\n-old\n+new\n"},
	{"diff", "2,3c2,3\n< old line\n---\n> new line\n5d4\n! changed\n=== heading\nIndex: file\nplain text\n\n"},
	{"diff", "1,a2\n12c\n-no newline at end"},
	{"diff", "+"},
	// YAML
	{"yaml", "---\n# config\nname: fasthighlight\nversion: 1.2\nenabled: true\nlist:\n  - one\n  - \"two\"\n  - 'three'\n...\n"},
	{"yaml", "server:\n  host: localhost  # comment\n  port: 8080\n  url: http://example.com // note\n  date: 2001-12-14t21:59:43.10-05:00\n  when: 2001-12-14 21:59:43.10 -5\n"},
	{"yaml", "script: |\n  echo one\n\n  echo two\n    nested\nnext: >-\n  folded\n  text\nafter: x\n"},
	{"yaml", "anchors:\n  base: &base {a: 1}\n  copy: *base\n  tag: !!str 123\n? complex key\n: value\n%include file.yaml\n"},
	{"yaml", "nums: [0x1f, 0o17, +12, -3.5e+2, .inf, .nan, 1.5a, 12:30]\nflags: [yes, No, ON, off, y, n, Yes1]\n\"quoted key\": v\n\"bad\": \"x\": y\n"},
	{"yaml", "key: \"unterminated\nnext: 'it''s'\nmore: \"esc \\\" quote\"\n- dash\n-x\n"},
	{"yaml", "a: |\nnot indented\n"},
	{"yaml", "k: v  #c\nweird: { [ ] } @ ` % 日本: 値\n"},
	// C and C++
	{"c", "#include <stdio.h>\n#include \"local.h\"\n#define MAX(a, b) ((a) > (b) ? (a) : (b)) \\\n  /* continued */\n\n/* block\n   comment */\nstatic const char *names[] = {\"a\\n\", \"b\\x41\", L\"wide\"};\n\nint\nmain(int argc, char **argv)\n{\n    unsigned long n = 0x1FUL + 017 + 1.5e-3f + .5 + 'a' + '\\n' + L'\\0';\n    size_t len = sizeof(names) / sizeof *names;\n    for (int i = 0; i < argc; i++) {\n        if (!argv[i]) goto done;\n        printf(\"%s\\n\", argv[i]); // print\n    }\ndone:\n    return n > 0 ? 1 : 0;\n}\n"},
	{"c", "#if 0\nint dead(void);\n#if 1\nnested\n#endif\n#else\nint live(void);\n#endif\nvoid f(void);\nstruct point { int x, y; };\ntypedef enum { A, B } kind_t;\n  /* c */ #pragma once\n*/ @ $\n"},
	{"cpp", "#include <vector>\n#include \"x.hpp\"\nnamespace app {\ntemplate <typename T>\nclass Box final : public Base<T> {\npublic:\n    explicit Box(T v) noexcept : v_(v) {}\n    [[nodiscard]] constexpr T get() const { return v_; }\n    virtual ~Box() override = default;\nprivate:\n    T v_;\n};\n\nenum class Color : uint8_t { Red, Green };\nauto s = R\"(raw string)\";\nauto n = 0x1'F + 0b1010'0101 + 1'000'000 + 017 + u8\"utf8\" + U'x';\nstd::vector<int> v{1, 2, 3};\nint main() {\n    for (auto &x : v) std::cout << x << '\\n';\n    co_return nullptr;\n}\n}\n"},
	{"cpp", "struct S;\nunion U { int a; };\nclass\n  [[deprecated]] Old {};\nwchar_t w = L'\\x41';\nlabel: goto label;\n#if 0\n#endif\n"},
	// Java and C#
	{"java", "package com.example.app;\n\nimport java.util.*;\nimport static java.lang.Math.max;\n\n/** Doc comment. */\n@Service\npublic final class App<T extends Comparable<T>> implements Runnable {\n    private static final long MAX = 42;\n    private final List<String> names = new ArrayList<>();\n\n    public static void main(String[] args) throws Exception {\n        var app = new App<Integer>();\n        double d = 1.5;\n        char c = 'x', e = '\\n', u = '\\u0041';\n        String s = \"hello \\\"world\\\"\";\n        String t = \"\"\"\n            text block \"quoted\"\n            \"\"\";\n        outer:\n        for (int i = n; i < args.length; i++) {\n            if (args[i] == null) break outer; // comment\n        }\n        switch (s) {\n        default:\n            System.out.println(s.length());\n        }\n    }\n}\n"},
	{"java", "public record Point(int x, int y) {}\nsealed interface Shape permits Circle {}\nnon-sealed class Circle implements Shape {}\nmodule com.example { requires java.base; }\n@interface Ann {}\n"},
	{"cs", "using System;\nusing System.Collections.Generic;\n\nnamespace Demo.App\n{\n    /// <summary>Docs.</summary>\n    [Serializable]\n    public sealed class Program : Base\n    {\n        private static readonly int? Count = 0x1F + 1_000 + 1.5e3f + 2d;\n        public string Name { get; set; } = \"x\";\n\n        static async Task Main(string[] args)\n        {\n            var path = @\"C:\\dir\\\"\"file\"\"\";\n            var msg = $\"Hello {args.Length}\";\n            char c = 'a', n = '\\n';\n            // comment\n            /* block\n               comment */\n            foreach (var a in args) Console.WriteLine(a);\n            await Task.Delay(1);\n        }\n    }\n}\n"},
	{"cs", "#region Setup\n#if DEBUG\nextern alias Other;\nglobal::System.Console.WriteLine(null ?? true);\n#endif\ninterface IShape { }\nrecord struct P(int X);\n\"unterminated\nnext @ $ `\n"},
	// Kotlin and Swift
	{"kotlin", "package com.example.app\n\nimport kotlinx.coroutines.*\n\n/** Docs. */\n@Suppress(\"unused\")\ndata class User(val name: String, var age: Int = 0) : Base() {\n    companion object {\n        const val MAX = 0xFF_FF + 1_000L + 0b1010 + 2u\n    }\n\n    fun <T : Comparable<T>> greet(other: T?): String {\n        val d = 1.5e-3 + .5f + 2F\n        val c = 'x'; val e = '\\n'; val u = '\\u0041'\n        // comment\n        val s = \"Hello, $name! ${age + 1} \\$ \\t\"\n        val raw = \"\"\"\n            raw $name ${\"x\"} \"quoted\"\n        \"\"\".trimIndent()\n        return other?.toString() ?: s!! as? String ?: \"\"\n    }\n}\n"},
	{"kotlin", "fun `weird name`() = Unit\nval my_var = list.filter { it !in xs && it !== null }\nobject Singleton : Runnable by impl\ninterface Foo<in T, out R>\nval x = a::b..c\n"},
	{"swift", "import Foundation\nimport UIKit\n\n/// Docs. MARK: section\n@objc class ViewController: UIViewController {\n    // TODO: fix\n    var count = 0x1F + 0b1010 + 0o17 + 1_000 + 1.5e3 + 2.\n    let name: String? = nil\n\n    func greet(_ who: String) -> String {\n        let msg = \"Hello, \\(who)! \\(count + 1) \\n \\u{1F600}\"\n        /* block /* nested */ comment */\n        if let x = optional, x != 0 {\n            print(msg, $0)\n        }\n        return msg\n    }\n}\n#if DEBUG\nlet debug = true\n#endif\n"},
	{"swift", "struct Point { var x: Int }\nenum Color { case red }\nprotocol Shape {}\nextension Point: Shape {}\ntypealias P = Point\n@availability(iOS, introduced: 8.0)\nlet a = b?.c ?? d!\nlet r = 1...5\n日本 = 1 $ \\\n"},
	// Go and Rust
	{"go", "//go:build linux\n\n// Package main is an example.\npackage main\n\nimport (\n\t\"fmt\"\n\t\"strings\"\n)\n\ntype Point struct {\n\tX, Y int\n}\n\nconst tmpl = `Hello, {{.Name}}! {{- if .Ok }}yes{{ end -}} {{/* c */}}`\n\nfunc (p *Point) String() string {\n\treturn fmt.Sprintf(\"(%d, %d)\\n\", p.X, p.Y)\n}\n\nfunc main() {\n\tvar xs []string = make([]string, 0, 10)\n\tch := make(chan int)\n\tgo func() { ch <- 1 }()\n\tx := 0x1F + 017 + 0b101 + 1_000 + 1.5e-3 + 2i + .5\n\tr := 'a'; e := '\\n'; u := '\\u00e9'\n\tfor i, s := range xs {\n\t\tif s != \"\" && i >= 0 {\n\t\t\tcontinue\n\t\t}\n\t}\n\tdefer close(ch)\n\t_ = strings.TrimSpace(\"x\")\n\t_, _, _, _ = x, r, e, u\n}\n"},
	{"go", "var a = len (b)\nx := `raw\nmulti {{ printf \"%d\" 1 | html }} {{$x := .A}}`\ns := \"unterminated\n/* c */ y &^= 1 << 2\n"},
	{"rust", "//! Crate docs.\n#![allow(dead_code)]\n\nuse std::collections::HashMap;\n\n/// A point.\n#[derive(Debug, Clone)]\npub struct Point<'a> {\n    x: i32,\n    name: &'a str,\n}\n\nimpl<'a> Point<'a> {\n    pub fn new(x: i32) -> Self {\n        let mut m: HashMap<String, Vec<u8>> = HashMap::new();\n        let n = 0x1F_u8 + 0o17 + 0b1010 + 1_000i64 + 1.5e-3f64 + 2.;\n        let c = 'x'; let e = '\\n'; let b = b'a'; let u = '\\u{1F600}';\n        let s = \"hello \\\"world\\\"\"; let r = r#\"raw \"quoted\"\"#; let bs = b\"bytes\\x80\";\n        println!(\"{} {x:?} {{}}\", n);\n        vec![1, 2, 3];\n        /* block /* nested */ */\n        'outer: loop { break 'outer; }\n        MAX_VALUE; Self { x, name: \"\" }\n    }\n}\n"},
	{"rust", "#!/usr/bin/env rust-script\nmod tests {\n    use super::*;\n    #[test]\n    fn it_works() -> Result<(), String> { Ok(()) }\n}\nmacro_rules! m { ($x:expr) => { $x }; }\nlet r = 1..=5; let t = 1..2; let f = 1.5.max(2.0);\n日本 \\ `\n"},
	// SQL and R
	{"sql", "-- comment\nSELECT u.id, COUNT(*) AS n, 'it''s' AS s, \"Quoted\"\"Name\"\nFROM users u\nLEFT JOIN orders o ON o.user_id = u.id\nWHERE u.created_at >= '2024-01-01' AND u.age BETWEEN 18 AND 65\nGROUP BY u.id HAVING n > 10\nORDER BY n DESC LIMIT 100;\n/* block /* nested */ comment */\nCREATE TABLE t (id INTEGER PRIMARY KEY, name VARCHAR(255) NOT NULL, price numeric);\n"},
	{"sql", "select * from T where x like '%a%' or y is null; insert into t values (1, 'x'); end-exec $x { }\n"},
	{"r", "# comment\nlibrary(dplyr)\nx <- c(1, 2.5, 3e10, 0x1F, 5L, 2i, .5, -1)\nsquare <- function(n) {\n  if (n > 0 && !is.na(n)) return(n^2) else NULL\n}\ndf %>% filter(a == TRUE, b != NA_integer_) %in% y\n`my var` <- 'single \\'q\\'' ; z = \"double\\n\"\nfor (i in 1:10) print(i)\nlst[[1]]$name; T; F; ...; ..1; Inf; pi\n"},
	{"r", "x -> y; a ->> b; f <<- 1; s4@slot; ns::fun; ns:::hidden; ~ y | z\n.hidden <- 1; ._ (2); _x\n\"unterminated\n"},
	// PHP
	{"php", "<?php\nnamespace App\\Models;\n\nuse Illuminate\\Support\\Str;\n\n#[Attribute(Attribute::TARGET_CLASS)]\nfinal class User extends Model implements JsonSerializable\n{\n    const MAX = 0x1F + 017 + 1_000 + 1.5e3;\n    private static ?array $cache = null;\n\n    /** Doc comment. */\n    public function __construct(private string $name, int ...$ids) {}\n\n    public function greet(): string\n    {\n        // comment\n        # also comment\n        $s = \"Hello, $this->name! {$this->name} ${name} \\n\\x41\";\n        $t = 'single \\'q\\'';\n        $fn = function ($x) use (&$s) { return $x * 2; };\n        $out = <<<EOT\n        heredoc $name\n        EOT;\n        return strtoupper($s) . PHP_EOL . __CLASS__;\n    }\n}\n?>\n"},
	{"php", "echo TRUE, False, NULL; $$var; ${'x'}; new class {}; Function foo() {}\nif ($a <=> $b ?: $c ?? $d) { exit; }\n$x = `ls`; /**/ /* c */\n"},
	// Ruby
	{"ruby", "#!/usr/bin/env ruby\n# frozen_string_literal: true\nrequire 'json'\n\nmodule App::Models\n  class User < Base\n    attr_accessor :name, :age\n    MAX = 0x1F + 017 + 0b1010 + 1_000\n\n    def initialize(name, age = 0)\n      @name = name\n      @@count ||= 0\n      $stdout.puts \"Hello, #{name}! #@name \\n\"\n    end\n\n    def self.find(id) = new(id)\n\n    def to_s\n      %w[a b c].map { |x| x.upcase }.join(', ') + 'single \\'q\\''\n    end\n\n    def valid? = !@name.nil? && @name =~ /^[a-z]+$/i\n  end\nend\n\nputs User.new('x').to_s if __FILE__ == $0\n"},
	{"ruby", "x = { key: 1, \"str\" => :sym, :\"q\" => %Q{a #{b} c}, r: %r{^a/b$}i }\ncase y\nwhen /re/ then ?a\nelse `ls -l`\nend\n=begin\nblock comment\n=end\n<<~EOS\nheredoc\nEOS\nobj&.method!; arr[0] <=> b; a ||= []\n"},
	// Python
	{"python", "#!/usr/bin/env python3\n\"\"\"Module docstring.\"\"\"\nfrom __future__ import annotations\nimport os.path as osp, sys\nfrom . import x\n\n\n@dataclass(frozen=True)\nclass Point(Base):\n    x: int = 0\n\n    def __init__(self, *args, **kw) -> None:\n        super().__init__()\n        self.y = 0x1F + 0o17 + 0b1_01 + 1_000.5e-3 + 3j\n\n    async def run(self):\n        await asyncio.sleep(1)  # comment\n        yield from range(10)\n"},
	{"python", "name = 'world'\nprint(f\"Hello, {name!r:>10}! {{literal}} {x = } {d['k']:{w}.{p}f}\")\nprint(rf'raw {x} \\d', b'\\x00\\n', r\"\\n\", u'\\N{DASH}')\nprint('%(key)s and %-5.2f %%' % d, '{0.attr[1]!s:<8}'.format(o))\ns = \"\"\"multi\nline \\t {x}\n\"\"\"\n"},
	{"python", "match cmd:\n    case [\"go\", direction] if direction in dirs:\n        pass\n    case _:\n        raise ValueError from None\nlambda x: x is not None and x or True\nglobal g; nonlocal n\nwith open(p) as f, ctx():\n    try:\n        del a[1:2]\n    except (KeyError, OSError) as e:\n        x.print(e); ...\n    finally:\n        assert x != y, x @ y\n"},
	{"python", "x = 1 \\\n    + 2\nif x := f(): ...\nimport \\\n  os\nclass\\\n  A: pass\ndef \\\n f(): pass\n日本 = 名前 + café\ndef g():\n    r'''raw docstring'''\n"},
	// Shell
	{"bash", "#!/bin/bash\n# comment\nset -euo pipefail\nNAME=\"world\"\necho \"Hello, ${NAME}!\" | tee out.txt\n"},
	{"bash", "for f in *.go; do\n  if [ -f \"$f\" ]; then\n    gofmt -l \"$f\" && echo ok || echo ng\n  fi\ndone\n"},
	{"bash", "x=$((1 + 2 * 3))\ny=$(date +%s)\nz=`uname -a`\ncount+=1\nexport PATH=\"$HOME/bin:$PATH\"\n"},
	{"bash", "cat <<EOF > file\nline $HOME\nEOF\nread -r line <<< \"$x\"\necho 'single $no' $'esc\\n' \"$@\" $# $? $1 ${#arr[@]} ${x:-def}\n"},
	{"bash", "function f() {\n  local a=1\n  return $a\n}\ncase $1 in\n  start) echo s;;\n  *) echo other;;\nesac\n"},
	{"bash", "echo \"unterminated $x\necho 'unclosed\n\\\necho done 2>&1 ; echo 10 20x\n"},
	{"bash", "echo $((16#ff + 2#)) \"a\\\"b\" \\\\ trailing\\"},
	{"bash", "do   \nif(true)then\ntypeset -i n; times; type x"},
	// CSS
	{"css", "/* reset */\nbody, h1 > p:hover::before {\n  margin: 0 auto;\n  color: #ff0;\n  font-family: \"Helvetica Neue\", sans-serif;\n}\n"},
	{"css", ".card { width: calc(100% - 2rem); transform: rotate(45deg) translateX(-1.5em); background: url(\"a.png\") no-repeat, url(b.svg); }\n"},
	{"css", "@media (max-width: 600px) {\n  .a { display: none !important; }\n}\n@import url('x.css');\n:root { --main-color: rebeccapurple; }\n"},
	{"css", "a{color:red;inline-blockx:1;border-bottom-color:blue;z-index:-1;flex:1 1 0%;margin:.5px 10PX}\n"},
	{"css", "@font-face { font-family: X; src: local(\"X\") }\n#id.cls[data-x=\"y\"] { content: '\\'q' }\n"},
	{"css", "} { @ # \"unterminated\n'x\n/* unclosed"},
	{"css", "a { b: 1px; c: rgb(1, 2, 3) foo(bar(baz)) -webkit-box; }\nx { y:1"},
	// HTML
	{"html", "<!DOCTYPE html>\n<html lang=\"ja\">\n<head>\n  <meta charset=utf-8>\n  <title>Test &amp; demo</title>\n  <style>\n    body { color: #333; }\n  </style>\n</head>\n<body>\n  <!-- comment -->\n  <p class='x' hidden>Hello<br/></p>\n  <script>\n    const n = 1;\n    console.log(`n=${n}`);\n  </script>\n</body>\n</html>\n"},
	{"html", "<SCRİPT>x</SCRİPT><ſtyle>a{}</ſtyle>\n"},
	{"html", "<SCRIPT type=\"module\">if (a < b) { x = /re/g }</SCRIPT >\n<Style>a{color:red}</ style>\n<![CDATA[ x ]]><?php echo 1 ?>\n"},
	{"html", "<div\n  data-x = \"multi\nline\"\n  onclick='f()' >text & more &lt;</div>\n<a href=x>y</a>\n"},
	{"html", "<script>unterminated\nnext line\n<p>after</p>\n< b>x</ b>\n<x a=>\n"},
	{"html", "<script></script><style></style><script>var s = `a${f({ k: 1 })}b`</script>\n"},
	{"html", "<!-- unclosed\n<p"},
	// XML
	{"xml", "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<!DOCTYPE note>\n<!-- a - comment -->\n<note id=\"1\" lang='ja'>\n  <to>Tove &amp; Jani</to>\n  <![CDATA[ <raw> ]]>\n  <empty/>\n</note>\n"},
	{"xml", "<svg xmlns:xlink=\"http://www.w3.org/1999/xlink\" width=100 >\n<path d=\"M0 0\"/></svg>\n"},
	{"xml", "< a b = \"multi\nline\" c=unquoted>text &bad entity; & </ a >\n<x y=\"unterminated>\n"},
	{"xml", "<a \"bad\" >< / >x\n<!-- unclosed"},
	// Markdown
	{"md", "---\ntitle: x\ntags: [a, b]\n---\n# Title\n\nSome *em* and **strong** and ~~del~~ text, `code`, [link](http://x), ![img](a.png), @user #tag.\n<!-- note -->\n\n## Sub\n\n- item one\n* item two\n  - [ ] task\n- [x] done\n1. first *a* b\n> quote\n\n```go\nfunc main() {}\n```\n\n```\nplain\n```\n\n```nolang\nx\n```\n\\* not em\n"},
	{"markdown", "```md\n# inner\n```py\nprint(1)\n```\n```yaml\na: 1\n```\nunclosed ```sh\nls\n"},
	{"md", "# Title\n**Note**: x\n- **a** b\n#\nnot a heading\n-\nnot a list\n>\nnot a quote\n"},
	// HTTP (the body is read in the language of its Content-Type)
	{"http", "POST /api/items?id=1 HTTP/1.1\nHost: example.com\nContent-Type: application/json; charset=utf-8\nContent-Length: 34\n\n{\"name\": \"x\", \"tags\": [1, true]}\n"},
	{"http", "HTTP/1.1 200 OK\nContent-Type: application/xml\nX-Long:   value with  spaces \n\n<?xml version=\"1.0\"?>\n<a b=\"c\">d &amp; e</a>\n"},
	{"http", "HTTP/2 404 Not Found\ncontent-type: application/x-www-form-urlencoded\n\na=1&b=2\n"},
	{"http", "GET  /path   HTTP/1.0\nX-Empty:   \nKey :value\nNo-Space:value\n  continued line\n\t\n bad header line\nContent-Type :  text/html  ;q=1\n\n<p>hi</p>\n<script>var a = 1;</script>\n"},
	{"http", "PUT /x HTTP/1.1\nHost: a\n\nbody text\nmore\n"},
	{"http", "HTTP/1.1 201 Created\nContent-Type: application/problem+json\n\n{\"type\": \"about:blank\"}\n"},
	{"http", "HTTP/1.1 200 OK\nContent-Type: text/css\n\na { color: red }\n"},
	{"http", "HTTP/1.1 200 OK\nCONTENT-TYPE: application/javascript\n\nfunction f() { return 1 }\n"},
	{"http", "HTTP/1.1 200 OK\nContent-Type: text/x-python\n\ndef f():\n    return 1\n"},
	{"http", "HTTP/1.1 200 OK\nContent-Type: text/x-ini\n\n[core]\nbare = true\n"},
	{"http", "not http\nGET / HTTP/1.1\nbad line\n\n"},
	{"http", "OPTIONS * HTTP/1.1\n\nHTTP/3 2000x\n"},
	{"http", "HTTP/1.1 200 OK"},
	{"http", "GET / HTTP/1.1\nContent-Type: application/javascript\n\nvar a // c"},
	{"http", "GET / HTTP/1.1\nContent-Type: application/json\n\n[1, 2"},
	// INI
	{"ini", "; comment\n# also\n[section]\nkey = value\nname=x\n  indented = y\n[bad] x\n[a]b]\nmulti = line one\n  line two\n\tline three\n \nempty =\n=novalue\njust a key\n"},
	{"ini", "[unterminated\nkey = v ; not a comment\na = b\n"},
	{"ini", "a=b"},
	{"ini", "[s]"},
}
