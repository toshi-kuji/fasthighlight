// Code generated from Chroma 2.27.0 lexers/go.go. DO NOT EDIT.

// Chroma (Copyright (C) 2017 Alec Thomas, MIT License) go.go: the alternatives of its regular
// expressions, copied in the same order. TestWordListsMatchChroma in compat/ compares every word with Chroma.

package golang

// declarations holds declaration words (KeywordDeclaration).
const declarations = "var|func|struct|map|chan|type|interface|const"

// keywords holds reserved words (Keyword).
const keywords = "fallthrough|continue|default|select|return|switch|break|defer|range|case|goto|else|for|if|go"

// constants holds constants (KeywordConstant).
const constants = "true|false|iota|nil"

// builtins holds built-in functions and types (NameBuiltin). Must be followed by (.
const builtins = "complex128|complex64|float32|println|uintptr|complex|recover|float64|delete|uint64|uint16|string|uint32|append|int16|clear|int64|int32|close|error|float|print|uint8|panic|bool|byte|rune|real|imag|make|int8|uint|copy|cap|new|len|int|min|max"

// types holds types (KeywordType).
const types = "complex128|complex64|float32|uintptr|float64|uint16|uint32|uint64|string|error|float|int64|int32|int16|uint8|byte|rune|uint|bool|int8|int|any"

// templateKeywords holds reserved words (Keyword) of Go templates (inside a raw string).
const templateKeywords = "range|if|else|while|with|template|end|true|false|nil|and|call|html|index|js|len|not|or|print|printf|println|urlquery|eq|ne|le|lt|ge|gt|block|break|continue|define|slice"
