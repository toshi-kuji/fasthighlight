#!/usr/bin/env bash
# build.sh builds the documentation site from the Markdown in website/ with fhl --static-site.
#
# Usage: website/build.sh [OUT]   (OUT defaults to _site at the top of the repository)
#
# Steps:
#  1. Build fhl from this repository, so the site always uses the current engine.
#  2. Copy only the Markdown of website/ and the logo into a temporary work folder, and replace the
#     markers (<!-- LANGS --> and so on) with lists made from fhl's own output.
#  3. Convert the work folder with fhl --static-site.
#  4. Append the site styles (website/style.css, if it exists) and the theme sample CSS to the
#     highlight.css that every page reads.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
out="${1:-$root/_site}"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

# 1. fhl
fhl="$work/bin/fhl"
(cd "$root" && go build -o "$fhl" ./cmd/fhl)

# 2. The work folder: the Markdown pages and the logo only (not build.sh or style.css).
src="$work/src"
mkdir -p "$src/assets"
cp "$root"/website/*.md "$src/"
cp "$root/assets/logo.webp" "$src/assets/"

"$fhl" --list-langs --json >"$work/langs.json"
"$fhl" --list-themes --json >"$work/themes.json"
# The light CSS of each theme (the site is light only), and the default CSS (for the lists of
# variables and classes), read from standalone pages of an empty document.
: >"$work/empty.md"
for name in $(python3 -c 'import json,sys; print(" ".join(t["name"] for t in json.load(open(sys.argv[1]))))' "$work/themes.json"); do
	"$fhl" -q --single-html -o - --theme="light:$name" "$work/empty.md" >"$work/css-light-$name.html"
done
"$fhl" -q --single-html -o - "$work/empty.md" >"$work/css-default.html"

python3 - "$work" "$src" <<'PY'
import json, re, sys

work, src = sys.argv[1], sys.argv[2]

def style(path):
    html = open(path, encoding="utf-8").read()
    return html[html.index("<style>\n") + len("<style>\n"):html.index("</style>")]

def cell(s):
    return s.replace("|", "\\|")

langs = json.load(open(f"{work}/langs.json"))
themes = json.load(open(f"{work}/themes.json"))

# <!-- LANGS -->: the supported languages.
rows = ["| Language | Names in code blocks | File names |", "| --- | --- | --- |"]
for l in langs:
    names = ", ".join(f"`{a}`" for a in l["aliases"])
    files = ", ".join(f"`{f}`" for f in l["filenames"])
    rows.append(f"| {cell(l['name'])} | {cell(names)} | {cell(files)} |")
markers = {"LANGS": "\n".join(rows), "LANG-COUNT": str(len(langs))}

# <!-- THEMES -->: the themes.
rows = ["| `THEME` | Name | Light | Dark |", "| --- | --- | --- | --- |"]
for t in themes:
    rows.append(f"| `{t['name']}` | {cell(t['label'])} | {cell(t['light'])} | {cell(t['dark'])} |")
markers["THEMES"] = "\n".join(rows)

# <!-- THEME-SAMPLES -->: the same code in each theme, in its light colors (the site is light only).
# Each block is in a <div> whose class (theme-NAME) carries that theme's variables (themes.css,
# appended to highlight.css).
sample = '''```go
// Greet says hello to name.
func Greet(name string) string {
	if name == "" {
		return "hello, world\\n"
	}
	return fmt.Sprintf("hello, %s (%d)", name, 42)
}
```'''
parts = []
for t in themes:
    title = f"`{t['name']}`" if t["label"] == t["name"] else f"{t['label']} (`{t['name']}`)"
    parts.append(f"### {title}")
    parts.append(f'<div class="theme-sample theme-{t["name"]}">\n\n{sample}\n\n</div>')
markers["THEME-SAMPLES"] = "\n\n".join(parts)

# <!-- CSS-VARS --> and <!-- TOKEN-CLASSES -->: from the comments of the default highlight.css.
css = style(f"{work}/css-default.html")
root = css[css.index(":root {"):css.index("\n}\n")]
rows = ["| Variable | What it colors |", "| --- | --- |"]
for name, comment in re.findall(r"(--highlight-[a-z]+): [^;]+; /\* (.*?) \*/", root):
    rows.append(f"| `{name}` | {cell(comment)} |")
markers["CSS-VARS"] = "\n".join(rows)
rows = ["| Class | Meaning | Token type |", "| --- | --- | --- |"]
for cls, meaning, token in re.findall(r"/\* ([a-z0-9]+): (.*) \(([A-Za-z.]+)\) \*/", css):
    rows.append(f"| `{cls}` | {cell(meaning)} | {token} |")
markers["TOKEN-CLASSES"] = "\n".join(rows)

for page in ("index.md", "themes.md"):
    path = f"{src}/{page}"
    text = open(path, encoding="utf-8").read()
    for name, value in markers.items():
        text = text.replace(f"<!-- {name} -->", value)
    open(path, "w", encoding="utf-8").write(text)

# themes.css: the light variables of each theme, under .theme-NAME instead of :root.
out = ["", "/* theme samples (website/build.sh): each theme's light variables under .theme-NAME */"]
for t in themes:
    c = style(f"{work}/css-light-{t['name']}.html")
    start = c.index(":root {")
    block = c[start:c.index("\n}\n", start) + 2]
    out.append(block.replace(":root {", f".theme-{t['name']} {{", 1))
open(f"{work}/themes.css", "w", encoding="utf-8").write("\n".join(out) + "\n")

left = [m for m in re.findall(r"<!-- ([A-Z-]+) -->", open(f"{src}/index.md").read() + open(f"{src}/themes.md").read())]
if left:
    sys.exit(f"build.sh: markers left: {left}")
PY

# 3. Convert. --force, because the output is rebuilt every time.
rm -rf "$out"
"$fhl" -q --static-site --raw-html=unsafe --theme=light --force -o "$out/" "$src"

# 4. The site styles and the theme samples go after the code colors in the one highlight.css.
if [ -f "$root/website/style.css" ]; then
	{ echo; cat "$root/website/style.css"; } >>"$out/highlight.css"
fi
cat "$work/themes.css" >>"$out/highlight.css"

echo "build.sh: wrote $out"
