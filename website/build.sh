#!/usr/bin/env bash
# build.sh builds the documentation site from the Markdown in website/ with fhl --static-site.
#
# Usage: website/build.sh [OUT]   (OUT defaults to _site at the top of the repository)
#
# OUT is deleted and written again. So that a wrong OUT does not delete other files, an OUT that already
# exists must be an empty folder, the default _site, or a folder that build.sh wrote (it holds .build-sh-site).
#
# Steps:
#  1. Build fhl from this repository, so the site always uses the current engine.
#  2. Copy only the Markdown of website/ and the logo into a temporary work folder, and replace the
#     markers (<!-- LANGS --> and so on) with lists made from fhl's own output.
#  3. Convert the work folder with fhl --static-site.
#  4. Append the site styles (website/style.css, if it exists) and the theme sample CSS to the
#     highlight.css that every page reads.
#  5. Copy the Google Search Console verification file (website/google*.html) as it is, and write
#     sitemap.xml, listing every page under SITE_URL (default: the GitHub Pages address).
#  6. Copy the icon (website/favicon.png) and link it from every page, and add the site name to the
#     title of every page but the top one (as search results and tabs show the title alone).
#     Also copy the share image (website/og.png) and add Open Graph and Twitter card tags, so that a
#     link to a page shows the image, the title and a description on social media.
#     Then put the header common to every page (links, and the button that switches between light
#     and dark) at the top of each page.
#  7. Build the demo page (website/demo): fasthighlight compiled to WebAssembly, only for this page,
#     with the CSS of every theme, one sample per language and the latest release tag filled in,
#     and add it to sitemap.xml.
#     It is a page of its own, built after steps 4 to 6, which leave it alone.
#
# Light and dark: the site follows the viewer's device, and the button overrides it. The choice is
# saved in localStorage ("theme") and set as data-theme="light" or "dark" on <html> before the page
# is drawn; without a saved choice there is no data-theme, and the device setting decides.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
out="${1:-$root/_site}"
marker=".build-sh-site"
if [ -e "$out" ] && ! { [ -d "$out" ] && { [ -z "$(ls -A "$out")" ] || [ -f "$out/$marker" ] ||
	[ "$(cd "$out" && pwd -P)" = "$(cd "$root" && pwd -P)/_site" ]; }; }; then
	echo "build.sh: $out exists and was not written by build.sh; give a new or empty folder, or delete it first" >&2
	exit 1
fi
site_url="${SITE_URL:-https://toshi-kuji.github.io/fasthighlight/}"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

# 1. fhl
fhl="$work/bin/fhl"
(cd "$root" && go build -o "$fhl" ./cmd/fhl)

# 2. The work folder: the Markdown pages and the logo only (not build.sh or style.css).
src="$work/src"
mkdir -p "$src/assets"
cp "$root"/website/*.md "$src/"
cp "$root/assets/logo-light.webp" "$root/assets/logo-dark.webp" "$src/assets/"

"$fhl" --list-langs --json >"$work/langs.json"
"$fhl" --list-themes --json >"$work/themes.json"
# The light and the dark CSS of each theme (for the samples), and the default CSS (for the lists of
# variables and classes).
for name in $(python3 -c 'import json,sys; print(" ".join(t["name"] for t in json.load(open(sys.argv[1]))))' "$work/themes.json"); do
	"$fhl" --print-css --theme "light:$name" >"$work/css-light-$name.css"
	"$fhl" --print-css --theme "dark:$name" >"$work/css-dark-$name.css"
done
"$fhl" --print-css >"$work/css-default.css"

python3 - "$work" "$src" <<'PY'
import json, re, sys

work, src = sys.argv[1], sys.argv[2]

def style(path):
    return open(path, encoding="utf-8").read()

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

# <!-- THEME-SAMPLES -->: the same code in each theme, in its light or dark colors as the site is shown.
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
css = style(f"{work}/css-default.css")
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

# themes.css: the variables of each theme under .theme-NAME instead of :root: the light ones, and the
# dark ones where the site is dark (the same conditions as the site colors in style.css).
def variables(mode, name, selector):
    c = style(f"{work}/css-{mode}-{name}.css")
    start = c.index(":root {")
    return c[start:c.index("\n}\n", start) + 2].replace(":root {", f"{selector} {{", 1)

out = ["", "/* theme samples (website/build.sh): each theme's variables under .theme-NAME */"]
dark = []
for t in themes:
    out.append(variables("light", t["name"], f".theme-{t['name']}"))
    dark.append(variables("dark", t["name"], f':root:not([data-theme="light"]) .theme-{t["name"]}'))
out.append("@media (prefers-color-scheme: dark) {")
out += dark
out.append("}")
for t in themes:
    out.append(variables("dark", t["name"], f':root[data-theme="dark"] .theme-{t["name"]}'))
open(f"{work}/themes.css", "w", encoding="utf-8").write("\n".join(out) + "\n")

left = [m for m in re.findall(r"<!-- ([A-Z-]+) -->", open(f"{src}/index.md").read() + open(f"{src}/themes.md").read())]
if left:
    sys.exit(f"build.sh: markers left: {left}")
PY

# 3. Convert. --force, because the output is rebuilt every time. The code colors follow the device, and
#    data-theme (set by the button) overrides it.
rm -rf "$out"
mkdir -p "$out"
: >"$out/$marker"
"$fhl" -q --static-site --raw-html unsafe --force -o "$out/" \
	--theme both --theme-selector dark:':root[data-theme="dark"]' --theme-selector light:':root[data-theme="light"]' \
	"$src"

# 4. The site styles and the theme samples go after the code colors in the one highlight.css.
if [ -f "$root/website/style.css" ]; then
	{ echo; cat "$root/website/style.css"; } >>"$out/highlight.css"
fi
cat "$work/themes.css" >>"$out/highlight.css"

# 5. The Search Console verification file, byte for byte (Google compares its contents), and sitemap.xml.
shopt -s nullglob
for f in "$root"/website/google*.html; do
	cp "$f" "$out/"
done
shopt -u nullglob
python3 - "$out" "$site_url" <<'PY'
import os, sys
from xml.sax.saxutils import escape

out, base = sys.argv[1], sys.argv[2].rstrip("/") + "/"
pages = []
for dirpath, _, files in os.walk(out):
    for name in files:
        rel = os.path.relpath(os.path.join(dirpath, name), out).replace(os.sep, "/")
        if not name.endswith(".html") or (dirpath == out and name.startswith("google")):
            continue
        if rel == "index.html":
            rel = ""
        elif rel.endswith("/index.html"):
            rel = rel[: -len("index.html")]
        pages.append(rel)
lines = ['<?xml version="1.0" encoding="UTF-8"?>', '<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">']
for rel in sorted(pages):
    lines.append(f"  <url><loc>{escape(base + rel)}</loc></url>")
lines.append("</urlset>")
open(os.path.join(out, "sitemap.xml"), "w", encoding="utf-8").write("\n".join(lines) + "\n")
PY

# 6. The icon, the page titles and the header. fhl writes <title> from the first heading and a <link> to
#    highlight.css (with a relative path); the icon link goes after it, with the same path, and the
#    header after the tags of <head>, at the start of the page.
cp "$root/website/favicon.png" "$root/website/og.png" "$out/"
python3 - "$out" "$site_url" <<'PY'
import html as h, os, re, sys

out, base, site = sys.argv[1], sys.argv[2].rstrip("/") + "/", "fasthighlight"
description = ("Fast syntax highlighting written in Go, without regular expressions. A goldmark extension, "
               "and the command fhl that turns Markdown into HTML or a basic static site.")

# Before the page is drawn: the saved choice, if any, as data-theme on <html>. localStorage can be
# unavailable (blocked, private windows); then the device setting decides.
THEME_INIT = ('<script>try{var t=localStorage.getItem("theme");'
              'if(t==="light"||t==="dark")document.documentElement.dataset.theme=t}catch(e){}</script>')

# The button shows the mode it switches to (a moon in light, a sun in dark). It is hidden until the script
# runs, as it does nothing without it.
MOON = ('<svg class="icon-moon" viewBox="0 0 24 24" width="18" height="18" aria-hidden="true">'
        '<path d="M20 14.5A8 8 0 0 1 9.5 4a8 8 0 1 0 10.5 10.5z"/></svg>')
SUN = ('<svg class="icon-sun" viewBox="0 0 24 24" width="18" height="18" aria-hidden="true">'
       '<circle cx="12" cy="12" r="4.5"/><path d="M12 1.5v3M12 19.5v3M1.5 12h3M19.5 12h3M4.6 4.6l2.1 2.1'
       'M17.3 17.3l2.1 2.1M4.6 19.4l2.1-2.1M17.3 6.7l2.1-2.1"/></svg>')
TOGGLE = """<script>(function(){
var r=document.documentElement,b=document.querySelector(".theme-toggle"),m=matchMedia("(prefers-color-scheme: dark)");
function now(){return r.dataset.theme||(m.matches?"dark":"light")}
function show(){var d=now()==="dark",l=d?"Switch to light mode":"Switch to dark mode";b.dataset.current=d?"dark":"light";b.setAttribute("aria-label",l);b.title=l}
b.addEventListener("click",function(){var t=now()==="dark"?"light":"dark";r.dataset.theme=t;try{localStorage.setItem("theme",t)}catch(e){}show()});
m.addEventListener("change",show);show();b.hidden=false})()</script>"""

def header(prefix):
    return (f'<header class="site-header"><nav><a href="{prefix}index.html">fasthighlight</a> · '
            f'<a href="{prefix}contents.html">Contents</a></nav>'
            f'<button type="button" class="theme-toggle" hidden>{MOON}{SUN}</button></header>\n{TOGGLE}')

for dirpath, _, files in os.walk(out):
    for name in files:
        if not name.endswith(".html") or (dirpath == out and name.startswith("google")):
            continue
        path = os.path.join(dirpath, name)
        html = open(path, encoding="utf-8").read()
        rel = os.path.relpath(path, out).replace(os.sep, "/")
        if rel != "index.html":
            html = re.sub(r"<title>(.*?)</title>", lambda m: f"<title>{m.group(1)} – {site}</title>", html, count=1)
        # Find the style sheet link after the title is changed, so the position below is still right.
        css = re.search(r'<link rel="stylesheet" href="([^"]*)highlight\.css">\n', html)
        if not css:
            sys.exit(f"build.sh: no highlight.css link in {path}")
        title = h.unescape(re.search(r"<title>(.*?)</title>", html).group(1))
        url = base + (rel[: -len("index.html")] if rel.endswith("index.html") else rel)
        meta = {
            "og:type": "website", "og:site_name": site, "og:title": title, "og:description": description,
            "og:url": url, "og:image": base + "og.png", "og:image:width": "1200", "og:image:height": "630",
            "og:image:alt": "The fasthighlight logo",
        }
        tags = [f'<link rel="icon" type="image/png" href="{css.group(1)}favicon.png">']
        tags += [f'<meta property="{k}" content="{h.escape(v)}">' for k, v in meta.items()]
        tags.append('<meta name="twitter:card" content="summary_large_image">')
        tags.append(THEME_INIT)
        tags.append(header(css.group(1)))
        html = html[:css.end()] + "\n".join(tags) + "\n" + html[css.end():]
        if not re.search(r'<link rel="stylesheet" href="[^"<>]*highlight\.css">\n<link rel="icon"', html):
            sys.exit(f"build.sh: the head of {path} came out broken")
        open(path, "w", encoding="utf-8").write(html)
PY

# 7. The demo: demo/index.html (the themes and samples filled in), demo/fasthighlight.wasm and Go's
#    demo/wasm_exec.js, which loads it.
mkdir -p "$out/demo"
(cd "$root" && GOOS=js GOARCH=wasm go build -trimpath -ldflags "-s -w" -o "$out/demo/fasthighlight.wasm" ./website/demo/wasm)
goroot="$(cd "$root" && go env GOROOT)"
if [ -f "$goroot/lib/wasm/wasm_exec.js" ]; then
	install -m 644 "$goroot/lib/wasm/wasm_exec.js" "$out/demo/"
else
	install -m 644 "$goroot/misc/wasm/wasm_exec.js" "$out/demo/"
fi
# The version shown on the demo, linked to its release page: the latest release tag ("dev" when no tags
# were fetched).
version="$(git -C "$root" tag --list 'v*' --sort=-version:refname 2>/dev/null | head -n 1)"
version="${version:-dev}"
python3 - "$work" "$root/website/demo" "$out" "$site_url" "$version" <<'PY'
import json, sys
from html import escape as hesc
from xml.sax.saxutils import escape

work, src, out, base, version = sys.argv[1:6]
read = lambda path: open(path, encoding="utf-8").read()

# The CSS of every theme and the languages, as step 2 wrote them.
themes = {t["name"]: {"label": t["label"], "light": read(f"{work}/css-light-{t['name']}.css"),
                      "dark": read(f"{work}/css-dark-{t['name']}.css")}
          for t in json.load(open(f"{work}/themes.json"))}
langs = json.load(open(f"{work}/langs.json"))

# One sample per language: each name a supported language, once, with a lang that names it.
samples = json.loads(read(src + "/samples.json"))
by_name = {l["name"]: l for l in langs}
problems, seen = [], set()
for s in samples:
    l = by_name.get(s.get("name"))
    if l is None:
        problems.append(f"{s.get('name')!r} is not a supported language")
    elif s["name"] in seen:
        problems.append(f"{s['name']!r} has more than one sample")
    elif s.get("lang", "").lower() not in [a.lower() for a in l["aliases"]] + [l["name"].lower()]:
        problems.append(f"the lang of {s['name']!r}, {s.get('lang')!r}, is not one of its names or aliases")
    seen.add(s.get("name"))
problems += [f"{n!r} has no sample" for n in sorted(set(by_name) - seen)]
if problems:
    sys.exit("build.sh: website/demo/samples.json: " + "; ".join(problems))

page = read(src + "/index.html")
for marker, value in (("/*THEMES*/", themes), ("/*SAMPLES*/", samples)):
    if page.count(marker) != 1:
        sys.exit(f"build.sh: website/demo/index.html needs one {marker}")
    # Nothing in the data may end the <script> it goes into or start an HTML comment in it.
    js = json.dumps(value, ensure_ascii=False).replace("</", "<\\/").replace("<!--", "<\\!--")
    page = page.replace(marker, js)
page = page.replace("__LANG_COUNT__", str(len(langs)))
if page.count("__VERSION__") != 1:
    sys.exit("build.sh: website/demo/index.html needs one __VERSION__")
# A release version links to its release page; "dev" (no tags fetched) has no page, so it stays plain text.
if version.startswith("v"):
    url = "https://github.com/toshi-kuji/fasthighlight/releases/tag/" + version
    version_html = f'<a href="{hesc(url)}">{hesc(version)}</a>'
else:
    version_html = hesc(version)
page = page.replace("__VERSION__", version_html)
open(out + "/demo/index.html", "w", encoding="utf-8").write(page)

sitemap = out + "/sitemap.xml"
url = "  <url><loc>" + escape(base.rstrip("/") + "/demo/") + "</loc></url>\n"
xml = read(sitemap)  # read before opening it for writing, which empties it
open(sitemap, "w", encoding="utf-8").write(xml.replace("</urlset>", url + "</urlset>", 1))
PY

echo "build.sh: wrote $out"
