package http

import "strings"

// mimeTypes maps the MIME types of Chroma 2.27.0's lexers to the language Chroma's lexers.MatchMimeType
// chooses for them, for the languages fasthighlight supports (when several lexers share a MIME type,
// Chroma chooses by priority). TestHTTPMimeTypesMatchChroma in compat/ checks it against Chroma.
var mimeTypes = map[string]string{
	"application/atom+xml":      "XML",
	"application/javascript":    "JavaScript",
	"application/json":          "JSON",
	"application/jsonl":         "JSON",
	"application/rss+xml":       "XML",
	"application/x-javascript":  "JavaScript",
	"application/x-python":      "Python",
	"application/x-python3":     "Python",
	"application/x-ruby":        "Ruby",
	"application/x-sh":          "Bash",
	"application/x-shellscript": "Bash",
	"application/xhtml+xml":     "HTML",
	"application/xml":           "XML",
	"image/svg+xml":             "XML",
	"image/x-xbitmap":           "C",
	"image/x-xpixmap":           "C",
	"text/S":                    "R",
	"text/S-plus":               "R",
	"text/css":                  "CSS",
	"text/html":                 "HTML",
	"text/inf":                  "INI",
	"text/javascript":           "JavaScript",
	"text/jsx":                  "react",
	"text/rust":                 "Rust",
	"text/typescript-jsx":       "react",
	"text/x-R":                  "R",
	"text/x-c++hdr":             "C++",
	"text/x-c++src":             "C++",
	"text/x-chdr":               "C",
	"text/x-csharp":             "C#",
	"text/x-csrc":               "C",
	"text/x-diff":               "Diff",
	"text/x-dockerfile-config":  "Docker",
	"text/x-gosrc":              "Go",
	"text/x-ini":                "INI",
	"text/x-java":               "Java",
	"text/x-javascript":         "JavaScript",
	"text/x-kotlin":             "Kotlin",
	"text/x-makefile":           "Makefile",
	"text/x-markdown":           "markdown",
	"text/x-patch":              "Diff",
	"text/x-php":                "PHP",
	"text/x-powershell":         "PowerShell",
	"text/x-python":             "Python",
	"text/x-python3":            "Python",
	"text/x-r":                  "R",
	"text/x-r-history":          "R",
	"text/x-r-profile":          "R",
	"text/x-r-source":           "R",
	"text/x-ruby":               "Ruby",
	"text/x-rust":               "Rust",
	"text/x-scss":               "SCSS",
	"text/x-sh":                 "Bash Session",
	"text/x-sql":                "SQL",
	"text/x-swift":              "Swift",
	"text/x-toml":               "TOML",
	"text/x-typescript":         "TypeScript",
	"text/x-yaml":               "YAML",
	"text/xml":                  "XML",
}

// MimeLanguage returns the name of the language whose tokenizer reads a body of content type ct, as
// Chroma's httpBodyContentTyper chooses it: the exact MIME type (case-sensitive), or else, if ct has a +,
// the type with the part from the / through the last + removed (application/ld+json is read as
// application/json). It returns "" if the language is not one fasthighlight supports, or if there is none.
func MimeLanguage(ct string) string {
	if name, ok := mimeTypes[ct]; ok {
		return name
	}
	if strings.Contains(ct, "+") {
		slash := strings.Index(ct, "/")
		plus := strings.LastIndex(ct, "+")
		return mimeTypes[ct[:slash+1]+ct[plus+1:]]
	}
	return ""
}
