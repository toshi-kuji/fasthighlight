package main

import (
	"fmt"
	"strings"
)

// role is a color role. It becomes the CSS variable --highlight-<name>.
type role struct {
	name    string
	comment string
}

// roles is the list of color roles. They are written to the CSS :root in this order.
var roles = []role{
	{"background", "code block background"},
	{"text", "plain text (names, whitespace, and anything not covered by another role)"},
	{"comment", "comment"},
	{"keyword", "keyword (if, func, etc.)"},
	{"type", "type and class name"},
	{"string", "string"},
	{"escape", "escape and interpolation inside a string (\\n, ${x}, etc.)"},
	{"number", "number"},
	{"constant", "constant (true, nil, constant names, etc.)"},
	{"function", "function name"},
	{"builtin", "builtin name (print, len, etc.)"},
	{"variable", "variable"},
	{"attribute", "attribute and property"},
	{"tag", "tag (HTML element names, YAML keys, etc.)"},
	{"operator", "operator"},
	{"punctuation", "punctuation (brackets, commas, etc.)"},
	{"preproc", "preprocessor (#include, etc.)"},
	{"decorator", "decorator and annotation (those starting with @)"},
	{"inserted", "added diff line"},
	{"deleted", "removed diff line"},
	{"heading", "heading (diff file name lines, etc.)"},
	{"muted", "muted part (shell prompts and output)"},
	{"error", "part that could not be tokenized"},
}

// classRule is the CSS of one token class.
type classRule struct {
	class   string // class name (k, s, nf, etc.)
	token   string // name of the token type (Pygments notation)
	meaning string // meaning
	role    string // color role; empty means no color is applied
	extra   string // anything applied besides the color (font-style, etc.)
}

// classRules is the CSS for each token class. It covers all of the fasthighlight class name table (token.go)
// except the line and frame classes (line, cl, etc.) and whitespace (w).
var classRules = []classRule{
	{"err", "Error", "part that could not be tokenized", "error", ""},
	{"x", "Other", "other part", "text", ""},

	{"k", "Keyword", "keyword", "keyword", ""},
	{"kc", "Keyword.Constant", "constant keyword (true, nil, etc.)", "constant", ""},
	{"kd", "Keyword.Declaration", "declaration keyword (var, func, etc.)", "keyword", ""},
	{"kn", "Keyword.Namespace", "namespace keyword (import, package, etc.)", "keyword", ""},
	{"kp", "Keyword.Pseudo", "keyword-like word", "keyword", ""},
	{"kr", "Keyword.Reserved", "reserved word", "keyword", ""},
	{"kt", "Keyword.Type", "type keyword (int, string, etc.)", "type", ""},

	{"n", "Name", "name", "text", ""},
	{"na", "Name.Attribute", "attribute name", "attribute", ""},
	{"nb", "Name.Builtin", "builtin name", "builtin", ""},
	{"bp", "Name.Builtin.Pseudo", "special builtin name (self, this, etc.)", "builtin", ""},
	{"nc", "Name.Class", "class name", "type", ""},
	{"no", "Name.Constant", "constant name", "constant", ""},
	{"nd", "Name.Decorator", "decorator and annotation", "decorator", ""},
	{"ni", "Name.Entity", "character reference (&amp;, etc.)", "constant", ""},
	{"ne", "Name.Exception", "exception name", "type", ""},
	{"nf", "Name.Function", "function name", "function", ""},
	{"fm", "Name.Function.Magic", "special function name (__init__, etc.)", "function", ""},
	{"py", "Name.Property", "property name", "attribute", ""},
	{"nl", "Name.Label", "label", "constant", ""},
	{"nn", "Name.Namespace", "namespace and package name", "type", ""},
	{"nx", "Name.Other", "other name", "text", ""},
	{"nt", "Name.Tag", "tag (HTML element names, YAML keys, etc.)", "tag", ""},
	{"nv", "Name.Variable", "variable", "variable", ""},
	{"vc", "Name.Variable.Class", "class variable", "variable", ""},
	{"vg", "Name.Variable.Global", "global variable", "variable", ""},
	{"vi", "Name.Variable.Instance", "instance variable", "variable", ""},
	{"vm", "Name.Variable.Magic", "special variable", "variable", ""},

	{"l", "Literal", "literal (a value written out directly)", "constant", ""},
	{"ld", "Literal.Date", "date", "number", ""},

	{"s", "Literal.String", "string", "string", ""},
	{"sa", "Literal.String.Affix", "string prefix (the r in r\"…\", etc.)", "string", ""},
	{"sb", "Literal.String.Backtick", "backtick string", "string", ""},
	{"sc", "Literal.String.Char", "character", "string", ""},
	{"dl", "Literal.String.Delimiter", "string delimiter", "string", ""},
	{"sd", "Literal.String.Doc", "documentation string (docstring)", "string", ""},
	{"s2", "Literal.String.Double", "double-quoted string", "string", ""},
	{"se", "Literal.String.Escape", "escape (\\n, etc.)", "escape", ""},
	{"sh", "Literal.String.Heredoc", "heredoc", "string", ""},
	{"si", "Literal.String.Interpol", "interpolation inside a string (${x}, etc.)", "escape", ""},
	{"sx", "Literal.String.Other", "other string", "string", ""},
	{"sr", "Literal.String.Regex", "regular expression", "string", ""},
	{"s1", "Literal.String.Single", "single-quoted string", "string", ""},
	{"ss", "Literal.String.Symbol", "symbol (:name, etc.)", "constant", ""},

	{"m", "Literal.Number", "number", "number", ""},
	{"mb", "Literal.Number.Bin", "binary number", "number", ""},
	{"mf", "Literal.Number.Float", "floating-point number", "number", ""},
	{"mh", "Literal.Number.Hex", "hexadecimal number", "number", ""},
	{"mi", "Literal.Number.Integer", "integer", "number", ""},
	{"il", "Literal.Number.Integer.Long", "long integer", "number", ""},
	{"mo", "Literal.Number.Oct", "octal number", "number", ""},

	{"o", "Operator", "operator", "operator", ""},
	{"ow", "Operator.Word", "word operator (and, in, etc.)", "keyword", ""},
	{"or", "Operator.Reserved", "reserved operator", "operator", ""},

	{"p", "Punctuation", "punctuation", "punctuation", ""},

	{"c", "Comment", "comment", "comment", ""},
	{"ch", "Comment.Hashbang", "#! on the first line", "comment", ""},
	{"cm", "Comment.Multiline", "multi-line comment", "comment", ""},
	{"cp", "Comment.Preproc", "preprocessor (#include, etc.)", "preproc", ""},
	{"cpf", "Comment.PreprocFile", "preprocessor file name", "string", ""},
	{"c1", "Comment.Single", "single-line comment", "comment", ""},
	{"cs", "Comment.Special", "special comment", "comment", ""},

	{"g", "Generic", "other", "text", ""},
	{"gd", "Generic.Deleted", "removed diff line", "deleted", ""},
	{"ge", "Generic.Emph", "emphasis", "", "font-style: italic;"},
	{"gr", "Generic.Error", "error output", "error", ""},
	{"gh", "Generic.Heading", "heading", "heading", "font-weight: bold;"},
	{"gi", "Generic.Inserted", "added diff line", "inserted", ""},
	{"go", "Generic.Output", "command output", "muted", ""},
	{"gp", "Generic.Prompt", "shell prompt", "muted", ""},
	{"gs", "Generic.Strong", "strong emphasis", "", "font-weight: bold;"},
	{"gu", "Generic.Subheading", "subheading", "heading", ""},
	{"gt", "Generic.Traceback", "error traceback", "error", ""},
	{"gl", "Generic.Underline", "underline", "", "text-decoration: underline;"},
}

// palette maps each color role to a color.
type palette map[string]string

// theme is a color theme: a pair of colors for light and dark backgrounds.
type theme struct {
	name        string // symbol used in --theme (github, tokyo-night, etc.)
	label       string // official name
	lightSource string // original color theme of the light colors (shown by --list-themes)
	darkSource  string // original color theme of the dark colors
	light, dark palette
}

// themes is the list of selectable color themes. --list-themes writes them in this order.
//
// Apart from default, the colors were copied from the original color themes (in the VS Code theme format): for each role,
// the color of a representative TextMate scope (keyword.control for keyword, string.quoted.double for string, etc.) was taken.
// The scope is looked up as VS Code does (the rule whose selector is the longest prefix of the scope; on a tie,
// the later rule), trying the next candidate scope when no rule matches and falling back to the plain text
// color; the background and plain text come from editor.background and editor.foreground. One Light and One
// Dark (Atom themes written in LESS) were read from the same rules of styles/syntax/_base.less. The original
// themes, their versions and their licenses are listed in THIRD_PARTY_NOTICES.md.
var themes = []*theme{
	{
		name:        "default",
		label:       "default",
		lightSource: "fhl's own colors",
		darkSource:  "fhl's own colors",
		light: palette{
			"background": "#f6f7f9", "text": "#1e2430", "comment": "#5d6875", "keyword": "#a3195b",
			"type": "#6a3fb5", "string": "#17663a", "escape": "#8a5a00", "number": "#9a4700",
			"constant": "#9a4700", "function": "#1f55b8", "builtin": "#00707f", "variable": "#7a4b1f",
			"attribute": "#7a4b1f", "tag": "#a3195b", "operator": "#5b4a8a", "punctuation": "#1e2430",
			"preproc": "#7a5a00", "decorator": "#6a3fb5", "inserted": "#17663a", "deleted": "#b3261e",
			"heading": "#1f55b8", "muted": "#5d6875", "error": "#b3261e",
		},
		dark: palette{
			"background": "#1a1d23", "text": "#dde3ec", "comment": "#8f9aa8", "keyword": "#ff85b3",
			"type": "#c7a6ff", "string": "#8bd49c", "escape": "#f0c674", "number": "#f5a96b",
			"constant": "#f5a96b", "function": "#82b4ff", "builtin": "#5fd4df", "variable": "#e8c39e",
			"attribute": "#e8c39e", "tag": "#ff85b3", "operator": "#b8a9e8", "punctuation": "#dde3ec",
			"preproc": "#e6c36b", "decorator": "#c7a6ff", "inserted": "#8bd49c", "deleted": "#ff8a80",
			"heading": "#82b4ff", "muted": "#8f9aa8", "error": "#ff8a80",
		},
	},
	{
		name:        "github",
		label:       "GitHub",
		lightSource: "GitHub Light Default (primer/github-vscode-theme)",
		darkSource:  "GitHub Dark Default (primer/github-vscode-theme)",
		light: palette{
			"background": "#ffffff", "text": "#1f2328", "comment": "#6e7781", "keyword": "#cf222e",
			"type": "#953800", "string": "#0a3069", "escape": "#cf222e", "number": "#0550ae",
			"constant": "#0550ae", "function": "#8250df", "builtin": "#0550ae", "variable": "#953800",
			"attribute": "#0550ae", "tag": "#116329", "operator": "#cf222e", "punctuation": "#1f2328",
			"preproc": "#cf222e", "decorator": "#cf222e", "inserted": "#116329", "deleted": "#82071e",
			"heading": "#0550ae", "muted": "#0550ae", "error": "#82071e",
		},
		dark: palette{
			"background": "#0d1117", "text": "#e6edf3", "comment": "#8b949e", "keyword": "#ff7b72",
			"type": "#ffa657", "string": "#a5d6ff", "escape": "#ff7b72", "number": "#79c0ff",
			"constant": "#79c0ff", "function": "#d2a8ff", "builtin": "#79c0ff", "variable": "#ffa657",
			"attribute": "#79c0ff", "tag": "#7ee787", "operator": "#ff7b72", "punctuation": "#e6edf3",
			"preproc": "#ff7b72", "decorator": "#ff7b72", "inserted": "#7ee787", "deleted": "#ffa198",
			"heading": "#79c0ff", "muted": "#79c0ff", "error": "#ffa198",
		},
	},
	{
		name:        "vscode",
		label:       "VS Code",
		lightSource: "VS Code Light+ (microsoft/vscode)",
		darkSource:  "VS Code Dark+ (microsoft/vscode)",
		light: palette{
			"background": "#ffffff", "text": "#000000", "comment": "#008000", "keyword": "#af00db",
			"type": "#267f99", "string": "#a31515", "escape": "#ee0000", "number": "#098658",
			"constant": "#0000ff", "function": "#795e26", "builtin": "#795e26", "variable": "#001080",
			"attribute": "#e50000", "tag": "#800000", "operator": "#000000", "punctuation": "#000000",
			"preproc": "#0000ff", "decorator": "#0000ff", "inserted": "#098658", "deleted": "#a31515",
			"heading": "#800000", "muted": "#008000", "error": "#cd3131",
		},
		dark: palette{
			"background": "#1e1e1e", "text": "#d4d4d4", "comment": "#6a9955", "keyword": "#c586c0",
			"type": "#4ec9b0", "string": "#ce9178", "escape": "#d7ba7d", "number": "#b5cea8",
			"constant": "#569cd6", "function": "#dcdcaa", "builtin": "#dcdcaa", "variable": "#9cdcfe",
			"attribute": "#9cdcfe", "tag": "#569cd6", "operator": "#d4d4d4", "punctuation": "#d4d4d4",
			"preproc": "#569cd6", "decorator": "#569cd6", "inserted": "#b5cea8", "deleted": "#ce9178",
			"heading": "#569cd6", "muted": "#6a9955", "error": "#f44747",
		},
	},
	{
		name:        "solarized",
		label:       "Solarized",
		lightSource: "Solarized Light (as shipped with VS Code)",
		darkSource:  "Solarized Dark (as shipped with VS Code)",
		light: palette{
			"background": "#fdf6e3", "text": "#657b83", "comment": "#93a1a1", "keyword": "#859900",
			"type": "#cb4b16", "string": "#2aa198", "escape": "#cb4b16", "number": "#d33682",
			"constant": "#b58900", "function": "#268bd2", "builtin": "#268bd2", "variable": "#657b83",
			"attribute": "#93a1a1", "tag": "#268bd2", "operator": "#859900", "punctuation": "#657b83",
			"preproc": "#b58900", "decorator": "#586e75", "inserted": "#859900", "deleted": "#dc322f",
			"heading": "#268bd2", "muted": "#93a1a1", "error": "#dc322f",
		},
		dark: palette{
			"background": "#002b36", "text": "#839496", "comment": "#586e75", "keyword": "#859900",
			"type": "#cb4b16", "string": "#2aa198", "escape": "#cb4b16", "number": "#d33682",
			"constant": "#b58900", "function": "#268bd2", "builtin": "#268bd2", "variable": "#839496",
			"attribute": "#93a1a1", "tag": "#268bd2", "operator": "#859900", "punctuation": "#839496",
			"preproc": "#b58900", "decorator": "#93a1a1", "inserted": "#859900", "deleted": "#dc322f",
			"heading": "#268bd2", "muted": "#586e75", "error": "#dc322f",
		},
	},
	{
		name:        "one",
		label:       "One",
		lightSource: "One Light (atom/one-light-syntax)",
		darkSource:  "One Dark (atom/one-dark-syntax)",
		light: palette{
			"background": "#fafafa", "text": "#383a42", "comment": "#a0a1a7", "keyword": "#a626a4",
			"type": "#c18401", "string": "#50a14f", "escape": "#0184bc", "number": "#986801",
			"constant": "#986801", "function": "#4078f2", "builtin": "#0184bc", "variable": "#e45649",
			"attribute": "#986801", "tag": "#e45649", "operator": "#383a42", "punctuation": "#383a42",
			"preproc": "#a626a4", "decorator": "#a626a4", "inserted": "#50a14f", "deleted": "#e45649",
			"heading": "#e45649", "muted": "#a0a1a7", "error": "#ff1414",
		},
		dark: palette{
			"background": "#282c34", "text": "#abb2bf", "comment": "#5c6370", "keyword": "#c678dd",
			"type": "#e5c07b", "string": "#98c379", "escape": "#56b6c2", "number": "#d19a66",
			"constant": "#d19a66", "function": "#61afef", "builtin": "#56b6c2", "variable": "#e06c75",
			"attribute": "#d19a66", "tag": "#e06c75", "operator": "#abb2bf", "punctuation": "#abb2bf",
			"preproc": "#c678dd", "decorator": "#c678dd", "inserted": "#98c379", "deleted": "#e06c75",
			"heading": "#e06c75", "muted": "#5c6370", "error": "#e05252",
		},
	},
	{
		name:        "catppuccin",
		label:       "Catppuccin",
		lightSource: "Catppuccin Latte (catppuccin/vscode)",
		darkSource:  "Catppuccin Mocha (catppuccin/vscode)",
		light: palette{
			"background": "#eff1f5", "text": "#4c4f69", "comment": "#7c7f93", "keyword": "#8839ef",
			"type": "#df8e1d", "string": "#40a02b", "escape": "#ea76cb", "number": "#fe640b",
			"constant": "#d20f39", "function": "#1e66f5", "builtin": "#1e66f5", "variable": "#4c4f69",
			"attribute": "#df8e1d", "tag": "#1e66f5", "operator": "#179299", "punctuation": "#7c7f93",
			"preproc": "#df8e1d", "decorator": "#fe640b", "inserted": "#40a02b", "deleted": "#d20f39",
			"heading": "#4c4f69", "muted": "#7c7f93", "error": "#4c4f69",
		},
		dark: palette{
			"background": "#1e1e2e", "text": "#cdd6f4", "comment": "#9399b2", "keyword": "#cba6f7",
			"type": "#f9e2af", "string": "#a6e3a1", "escape": "#f5c2e7", "number": "#fab387",
			"constant": "#f38ba8", "function": "#89b4fa", "builtin": "#89b4fa", "variable": "#cdd6f4",
			"attribute": "#f9e2af", "tag": "#89b4fa", "operator": "#94e2d5", "punctuation": "#9399b2",
			"preproc": "#f9e2af", "decorator": "#fab387", "inserted": "#a6e3a1", "deleted": "#f38ba8",
			"heading": "#cdd6f4", "muted": "#9399b2", "error": "#cdd6f4",
		},
	},
	{
		name:        "tokyo-night",
		label:       "Tokyo Night",
		lightSource: "Tokyo Night Light (enkia/tokyo-night-vscode-theme)",
		darkSource:  "Tokyo Night (enkia/tokyo-night-vscode-theme)",
		light: palette{
			"background": "#e6e7ed", "text": "#343b59", "comment": "#888b94", "keyword": "#65359d",
			"type": "#343b58", "string": "#385f0d", "escape": "#363c4d", "number": "#965027",
			"constant": "#965027", "function": "#2959aa", "builtin": "#006c86", "variable": "#343b58",
			"attribute": "#65359d", "tag": "#8c4351", "operator": "#006c86", "punctuation": "#006c86",
			"preproc": "#33635c", "decorator": "#65359d", "inserted": "#449dab", "deleted": "#914c54",
			"heading": "#343b59", "muted": "#888b94", "error": "#942f2f",
		},
		dark: palette{
			"background": "#1a1b26", "text": "#a9b1d6", "comment": "#51597d", "keyword": "#bb9af7",
			"type": "#c0caf5", "string": "#9ece6a", "escape": "#89ddff", "number": "#ff9e64",
			"constant": "#ff9e64", "function": "#7aa2f7", "builtin": "#0db9d7", "variable": "#c0caf5",
			"attribute": "#bb9af7", "tag": "#f7768e", "operator": "#89ddff", "punctuation": "#89ddff",
			"preproc": "#73daca", "decorator": "#bb9af7", "inserted": "#449dab", "deleted": "#914c54",
			"heading": "#a9b1d6", "muted": "#51597d", "error": "#ff5370",
		},
	},
}

// mode is which of the light and dark colors are written.
type mode int

const (
	modeBoth  mode = iota // light colors, switched to the dark ones by prefers-color-scheme
	modeLight             // light colors only
	modeDark              // dark colors only
)

// modeNames are the symbols of the modes in --theme, in the order of the mode constants.
var modeNames = []string{"both", "light", "dark"}

// lookupTheme looks up a color theme by its symbol. It returns nil if there is none.
func lookupTheme(name string) *theme {
	for _, t := range themes {
		if t.name == name {
			return t
		}
	}
	return nil
}

// themeForm and themeDetail tell how to write the value of --theme. They are part of the error messages about --theme.
const (
	themeForm   = "MODE or MODE:THEME"
	themeDetail = "(MODE is light, dark or both; THEME is one of fhl --list-themes, default if omitted), e.g. --theme light:github"
	themeUsage  = themeForm + " " + themeDetail
)

// parseTheme reads the value of --theme (MODE or MODE:THEME). It returns the error message if the value is wrong.
func parseTheme(v string) (*theme, mode, string) {
	ms, ts, _ := strings.Cut(v, ":")
	m := -1
	for i, n := range modeNames {
		if ms == n {
			m = i
		}
	}
	if m < 0 {
		if lookupTheme(ms) != nil {
			return nil, 0, fmt.Sprintf("fhl: --theme %s needs the mode first, e.g. --theme light:%s (MODE is light, dark or both)", v, ms)
		}
		return nil, 0, fmt.Sprintf("fhl: --theme must be %s, not %q %s", themeForm, v, themeDetail)
	}
	if ts == "" {
		ts = "default"
	}
	t := lookupTheme(ts)
	if t == nil {
		return nil, 0, fmt.Sprintf("fhl: unknown theme %q in --theme %s; see fhl --list-themes", ts, v)
	}
	return t, mode(m), ""
}

// css builds the CSS of the color theme in mode m. The same theme and mode always give the same CSS.
func (t *theme) css(m mode) string {
	var b strings.Builder
	b.WriteString("/*\n")
	b.WriteString(" * color theme of fhl code blocks: " + t.label + ", " + modeNames[m] + " (--theme " + modeNames[m] + ":" + t.name + ")\n")
	switch m {
	case modeBoth:
		b.WriteString(" * Light colors, switched to dark ones when the viewer's device prefers dark (prefers-color-scheme).\n")
	case modeLight:
		b.WriteString(" * Light colors only.\n")
	case modeDark:
		b.WriteString(" * Dark colors only.\n")
	}
	b.WriteString(" * The colors are collected in the CSS variables of :root (--highlight-*); change the values to change the colors.\n")
	b.WriteString(" * A code block is <pre class=\"highlight\" data-lang=\"language name\"><code class=\"language-language name\">, and tokens are <span> elements with short class names.\n")
	b.WriteString(" */\n")
	vars := func(indent string, p palette) {
		for _, r := range roles {
			b.WriteString(indent + "--highlight-" + r.name + ": " + p[r.name] + "; /* " + r.comment + " */\n")
		}
	}
	first := t.light
	if m == modeDark {
		first = t.dark
	}
	b.WriteString(":root {\n")
	vars("  ", first)
	b.WriteString("}\n")
	if m == modeBoth {
		b.WriteString("@media (prefers-color-scheme: dark) {\n  :root {\n")
		vars("    ", t.dark)
		b.WriteString("  }\n}\n")
	}
	b.WriteString("\n/* code block frame: background and plain text color */\n")
	b.WriteString("pre.highlight { background-color: var(--highlight-background); color: var(--highlight-text); }\n")
	for _, c := range classRules {
		b.WriteString("/* " + c.class + ": " + c.meaning + " (" + c.token + ") */\n")
		b.WriteString("pre.highlight ." + c.class + " {")
		if c.role != "" {
			b.WriteString(" color: var(--highlight-" + c.role + ");")
		}
		if c.extra != "" {
			b.WriteString(" " + c.extra)
		}
		b.WriteString(" }\n")
	}
	return b.String()
}
