package main

import (
	"bytes"
	"html"
	"net/url"
	"path/filepath"
	"strings"

	fh "github.com/toshi-kuji/fasthighlight"
	"github.com/toshi-kuji/fasthighlight/markdown"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	ghtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// uncolored is one code block that is not highlighted.
type uncolored struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Lang   string `json:"lang"`
	Reason string `json:"reason"` // "unsupported" or "no-language"
}

// converted is the result of converting one input.
type converted struct {
	body      []byte // contents of <body>
	title     string // first heading; empty if there is none
	blocks    int
	uncolored []uncolored
}

// newMarkdown builds fhl's goldmark. It adds GFM and the fasthighlight extension and sets the handling of raw HTML to rawHTML.
// For pages that open as they are (--single-html and --static-site, ids), it gives headings ids, so that links
// within a page work. For --static-site (site), it also rewrites links to Markdown files to the .html pages.
// newMarkdown returns the goldmark that converts the pages. pages, for --static-site, holds the cleaned paths
// of the Markdown files that the site converts, the only ones that links are rewritten to (nil otherwise).
func newMarkdown(rawHTML string, p markdown.Parallelism, ids bool, pages map[string]bool) goldmark.Markdown {
	opts := []goldmark.Option{goldmark.WithExtensions(extension.GFM, &markdown.Extension{Parallelism: p})}
	if ids {
		opts = append(opts, goldmark.WithParserOptions(parser.WithAutoHeadingID()))
	}
	if pages != nil {
		opts = append(opts, goldmark.WithParserOptions(
			parser.WithASTTransformers(util.Prioritized(mdLinkRewriter{pages: pages}, 1000)),
		))
	}
	switch rawHTML {
	case "unsafe":
		opts = append(opts, goldmark.WithRendererOptions(ghtml.WithUnsafe()))
	case "escape":
		opts = append(opts, goldmark.WithRendererOptions(
			renderer.WithNodeRenderers(util.Prioritized(rawHTMLEscaper{}, 100))))
	}
	return goldmark.New(opts...)
}

// mdLinkRewriter rewrites relative links to Markdown files (.md, .markdown, .mkd) to the .html pages that
// --static-site writes for them, keeping any ?query and #fragment (guide.md#setup becomes guide.html#setup).
// Only links to the pages of the site are rewritten: a link to a Markdown file outside the folder, in a folder
// whose name starts with a dot, or that does not exist, keeps pointing at the file.
type mdLinkRewriter struct {
	pages map[string]bool // the cleaned paths of the Markdown files that the site converts
}

// pageKey holds, in the parser.Context, the path of the Markdown file being converted.
var pageKey = parser.NewContextKey()

func (r mdLinkRewriter) Transform(doc *ast.Document, _ text.Reader, pc parser.Context) {
	page, _ := pc.Get(pageKey).(string)
	isPage := func(rel string) bool { return r.pages[filepath.Join(filepath.Dir(page), filepath.FromSlash(rel))] }
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if l, ok := n.(*ast.Link); ok && entering {
			l.Destination = []byte(rewriteMDLink(string(l.Destination), isPage))
		}
		return ast.WalkContinue, nil
	})
}

// rewriteMDLink returns dest with the extension of a relative link to a Markdown file changed to .html, when
// isPage reports that the file it names (relative to the page, with any %XX decoded) is a page of the site.
// Other links (with a scheme such as https:, starting with / or #, or to other files) are returned as they are.
func rewriteMDLink(dest string, isPage func(rel string) bool) string {
	if dest == "" || strings.HasPrefix(dest, "/") || strings.HasPrefix(dest, "#") {
		return dest
	}
	path, rest := dest, ""
	if i := strings.IndexAny(dest, "?#"); i >= 0 {
		path, rest = dest[:i], dest[i:]
	}
	if i := strings.IndexAny(path, ":"); i >= 0 && !strings.Contains(path[:i], "/") {
		return dest // a scheme (https:, mailto:, etc.)
	}
	if !isMarkdown(path) {
		return dest
	}
	file, err := url.PathUnescape(path)
	if err != nil || !isPage(file) {
		return dest
	}
	return strings.TrimSuffix(path, filepath.Ext(path)) + ".html" + rest
}

// rawHTMLEscaper turns raw HTML in Markdown into escaped, literal text (--raw-html escape).
// HTML blocks are wrapped in a <pre> without a class to keep the newlines. Inline tags are turned into text in place.
type rawHTMLEscaper struct{}

func (rawHTMLEscaper) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindHTMLBlock, renderHTMLBlock)
	reg.Register(ast.KindRawHTML, renderRawHTML)
}

func renderHTMLBlock(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.HTMLBlock)
	if entering {
		_, _ = w.WriteString("<pre>")
		for i := 0; i < n.Lines().Len(); i++ {
			line := n.Lines().At(i)
			_, _ = w.Write(util.EscapeHTML(line.Value(src)))
		}
		return ast.WalkContinue, nil
	}
	if n.HasClosure() {
		_, _ = w.Write(util.EscapeHTML(n.ClosureLine.Value(src)))
	}
	_, _ = w.WriteString("</pre>\n")
	return ast.WalkContinue, nil
}

func renderRawHTML(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkSkipChildren, nil
	}
	n := node.(*ast.RawHTML)
	for i := 0; i < n.Segments.Len(); i++ {
		seg := n.Segments.At(i)
		_, _ = w.Write(util.EscapeHTML(seg.Value(src)))
	}
	return ast.WalkSkipChildren, nil
}

// convertMarkdown converts the Markdown src to HTML. The newlines of src must already be normalized.
// Leading front matter (the part enclosed in ---) is skipped. Line numbers are counted in the original src.
func convertMarkdown(md goldmark.Markdown, name string, src []byte) (*converted, error) {
	body, offset := stripFrontMatter(src)
	pc := parseContext()
	pc.Set(pageKey, name)
	doc := md.Parser().Parse(text.NewReader(body), parser.WithContext(pc))
	c := &converted{}
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n := n.(type) {
		case *ast.Heading:
			if c.title == "" {
				c.title = strings.TrimSpace(plainText(n, body))
			}
		case *ast.FencedCodeBlock:
			c.blocks++
			lang := markdown.LanguageName(n, body)
			line := fenceLine(body, n)
			if line > 0 {
				line += offset
			}
			switch {
			case lang == "":
				c.uncolored = append(c.uncolored, uncolored{File: name, Line: line, Reason: "no-language"})
			case fh.LookupRegistered(lang) == nil && !fh.IsPlainText(lang): // plain text is uncolored on purpose
				c.uncolored = append(c.uncolored, uncolored{File: name, Line: line, Lang: lang, Reason: "unsupported"})
			}
		}
		return ast.WalkContinue, nil
	})
	var out bytes.Buffer
	if err := md.Renderer().Render(&out, body, doc); err != nil {
		return nil, err
	}
	c.body = out.Bytes()
	return c, nil
}

// convertCode converts the code into the HTML of one code block of language lang (the name put in data-lang).
func convertCode(lang, code string) (*converted, error) {
	var out bytes.Buffer
	if err := fh.Highlight(&out, lang, code); err != nil {
		return nil, err
	}
	return &converted{body: out.Bytes(), blocks: 1}, nil
}

// stripFrontMatter removes the front matter at the start of the document and returns the rest and the number of
// removed lines. Front matter starts with a --- line, ends at the next --- line, and its first line is a YAML
// key ("title: x"). Anything else that starts with --- is Markdown (a thematic break, or the underline of a
// heading after it) and is kept: so is a --- block with no closing line.
func stripFrontMatter(src []byte) ([]byte, int) {
	first, rest, ok := bytes.Cut(src, []byte("\n"))
	if !ok || !isDashes(first) {
		return src, 0
	}
	if key, _, _ := bytes.Cut(rest, []byte("\n")); !isYAMLKey(key) {
		return src, 0
	}
	lines := 1
	for len(rest) > 0 {
		var line []byte
		line, rest, ok = bytes.Cut(rest, []byte("\n"))
		lines++
		if isDashes(line) {
			if !ok {
				return nil, lines - 1
			}
			return rest, lines
		}
		if !ok {
			break
		}
	}
	return src, 0
}

// isYAMLKey reports whether line starts a YAML mapping entry: a key at the start of the line (not indented,
// and not starting like a Markdown block or a YAML comment), then a colon at the end of the line or before a
// space or tab.
func isYAMLKey(line []byte) bool {
	line = bytes.TrimRight(line, " \t\r")
	if len(line) == 0 || bytes.IndexByte([]byte(" \t#-*+>|`=<[!{"), line[0]) >= 0 {
		return false
	}
	i := bytes.IndexByte(line, ':')
	return i > 0 && (i == len(line)-1 || line[i+1] == ' ' || line[i+1] == '\t')
}

func isDashes(line []byte) bool {
	return string(bytes.TrimRight(line, " \t")) == "---"
}

// fenceLine returns the line number of the start of a code block (the ``` line), or 0 if unknown. It counts the same way as Scan in the main package.
func fenceLine(src []byte, b *ast.FencedCodeBlock) int {
	switch {
	case b.Info != nil:
		return 1 + bytes.Count(src[:b.Info.Segment.Start], []byte("\n"))
	case b.Lines().Len() > 0:
		return bytes.Count(src[:b.Lines().At(0).Start], []byte("\n"))
	}
	return 0
}

// plainText concatenates only the text inside the node (to turn a heading into the <title>).
func plainText(n ast.Node, src []byte) string {
	var b strings.Builder
	_ = ast.Walk(n, func(c ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch c := c.(type) {
		case *ast.Text:
			b.Write(c.Segment.Value(src))
			if c.SoftLineBreak() || c.HardLineBreak() {
				b.WriteByte(' ')
			}
		case *ast.String:
			b.Write(c.Value)
		case *ast.RawHTML:
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	})
	return b.String()
}

// sitePage turns the body into a page of --static-site: the declaration, <html lang>, the character encoding,
// the viewport, the title and a link to the CSS (cssHref, relative to the page), then the body.
func sitePage(lang, title, cssHref string, body []byte) []byte {
	var b bytes.Buffer
	b.WriteString("<!DOCTYPE html>\n<html lang=\"")
	b.WriteString(html.EscapeString(lang))
	b.WriteString("\">\n<meta charset=\"utf-8\">\n<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n<title>")
	b.WriteString(html.EscapeString(title))
	b.WriteString("</title>\n<link rel=\"stylesheet\" href=\"")
	b.WriteString(html.EscapeString(cssHref))
	b.WriteString("\">\n")
	b.Write(body)
	return b.Bytes()
}

// singleHTML turns the body into a standalone HTML document with the CSS embedded.
func singleHTML(title, css string, body []byte) []byte {
	var b bytes.Buffer
	b.WriteString("<!DOCTYPE html>\n<meta charset=\"utf-8\">\n<title>")
	b.WriteString(html.EscapeString(title))
	b.WriteString("</title>\n<style>\n")
	b.WriteString(css)
	b.WriteString("</style>\n")
	b.Write(body)
	return b.Bytes()
}
