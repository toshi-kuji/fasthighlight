// Package chromadata embeds the Chroma 2.27.0 XML (typescript, javascript, github) used by the comparison benchmarks.
package chromadata

import "embed"

// FS holds data/typescript.xml, data/javascript.xml and data/github.xml.
//
//go:embed data
var FS embed.FS
