package fasthighlight

import "strconv"

// The values, names, and class names of the token types in this file are the same as those in Chroma
// 2.27.0's types.go (Copyright (C) 2017 Alec Thomas, MIT License; see THIRD_PARTY_NOTICES.md).
// For the class names, only the outer wrapper (PreWrapper) uses a generic name (highlight) rather than a
// product name (Chroma uses chroma).
// token_test.go checks that they match Chroma.

// TokenType is the type of a token.
//
// Types form major categories in steps of 1000 and subcategories in steps of 100. For example, strings
// (LiteralString) are 3100 to 3199, which belong to the major category Literal (3000 to 3999).
type TokenType int

// CoalesceLimit is the upper limit (in bytes) on the length to which tokens of the same type are merged
// into one. As in Chroma's Coalesce, once a merged token reaches this length, following tokens of the
// same type are not merged (they become a separate token). In HTML, that makes a separate <span>.
const CoalesceLimit = 8192

// Token is a part of the input and its type. Value is a substring of the input and is not copied.
type Token struct {
	Type  TokenType
	Value string
}

// Types of output parts (negative values).
const (
	Background TokenType = -1 - iota
	PreWrapper
	Line
	LineNumbers
	LineNumbersTable
	LineHighlight
	LineTable
	LineTableTD
	LineLink
	CodeLine
	Error // input that could not be made into a token
	Other
	None
	Ignore
	EOFType TokenType = 0
)

// Keywords.
const (
	Keyword TokenType = 1000 + iota
	KeywordConstant
	KeywordDeclaration
	KeywordNamespace
	KeywordPseudo
	KeywordReserved
	KeywordType
)

// Names.
const (
	Name TokenType = 2000 + iota
	NameAttribute
	NameClass
	NameConstant
	NameDecorator
	NameEntity
	NameException
	NameKeyword
	NameLabel
	NameNamespace
	NameOperator
	NameOther
	NamePseudo
	NameProperty
	NameTag
)

// Built-in names.
const (
	NameBuiltin TokenType = 2100 + iota
	NameBuiltinPseudo
)

// Variable names.
const (
	NameVariable TokenType = 2200 + iota
	NameVariableAnonymous
	NameVariableClass
	NameVariableGlobal
	NameVariableInstance
	NameVariableMagic
)

// Function names.
const (
	NameFunction TokenType = 2300 + iota
	NameFunctionMagic
)

// Literals.
const (
	Literal TokenType = 3000 + iota
	LiteralDate
	LiteralOther
)

// Strings.
const (
	LiteralString TokenType = 3100 + iota
	LiteralStringAffix
	LiteralStringAtom
	LiteralStringBacktick
	LiteralStringBoolean
	LiteralStringChar
	LiteralStringDelimiter
	LiteralStringDoc
	LiteralStringDouble
	LiteralStringEscape
	LiteralStringHeredoc
	LiteralStringInterpol
	LiteralStringName
	LiteralStringOther
	LiteralStringRegex
	LiteralStringSingle
	LiteralStringSymbol
)

// Numbers.
const (
	LiteralNumber TokenType = 3200 + iota
	LiteralNumberBin
	LiteralNumberFloat
	LiteralNumberHex
	LiteralNumberInteger
	LiteralNumberIntegerLong
	LiteralNumberOct
	LiteralNumberByte
)

// Operators.
const (
	Operator TokenType = 4000 + iota
	OperatorWord
	OperatorReserved
)

// Punctuation.
const (
	Punctuation TokenType = 5000 + iota
)

// Comments.
const (
	Comment TokenType = 6000 + iota
	CommentHashbang
	CommentMultiline
	CommentSingle
	CommentSpecial
)

// Preprocessor "comments".
const (
	CommentPreproc TokenType = 6100 + iota
	CommentPreprocFile
)

// Generic (diffs, headings, and so on).
const (
	Generic TokenType = 7000 + iota
	GenericDeleted
	GenericEmph
	GenericError
	GenericHeading
	GenericInserted
	GenericOutput
	GenericPrompt
	GenericStrong
	GenericSubheading
	GenericTraceback
	GenericUnderline
)

// Text.
const (
	Text TokenType = 8000 + iota
	TextWhitespace
	TextSymbol
	TextPunctuation
)

// typeNames are the names of the types. They are the same as Chroma's TokenType.String().
var typeNames = map[TokenType]string{
	Background:       "Background",
	PreWrapper:       "PreWrapper",
	Line:             "Line",
	LineNumbers:      "LineNumbers",
	LineNumbersTable: "LineNumbersTable",
	LineHighlight:    "LineHighlight",
	LineTable:        "LineTable",
	LineTableTD:      "LineTableTD",
	LineLink:         "LineLink",
	CodeLine:         "CodeLine",
	Error:            "Error",
	Other:            "Other",
	None:             "None",
	Ignore:           "Ignore",
	EOFType:          "EOFType",

	Keyword:            "Keyword",
	KeywordConstant:    "KeywordConstant",
	KeywordDeclaration: "KeywordDeclaration",
	KeywordNamespace:   "KeywordNamespace",
	KeywordPseudo:      "KeywordPseudo",
	KeywordReserved:    "KeywordReserved",
	KeywordType:        "KeywordType",

	Name:                  "Name",
	NameAttribute:         "NameAttribute",
	NameClass:             "NameClass",
	NameConstant:          "NameConstant",
	NameDecorator:         "NameDecorator",
	NameEntity:            "NameEntity",
	NameException:         "NameException",
	NameKeyword:           "NameKeyword",
	NameLabel:             "NameLabel",
	NameNamespace:         "NameNamespace",
	NameOperator:          "NameOperator",
	NameOther:             "NameOther",
	NamePseudo:            "NamePseudo",
	NameProperty:          "NameProperty",
	NameTag:               "NameTag",
	NameBuiltin:           "NameBuiltin",
	NameBuiltinPseudo:     "NameBuiltinPseudo",
	NameVariable:          "NameVariable",
	NameVariableAnonymous: "NameVariableAnonymous",
	NameVariableClass:     "NameVariableClass",
	NameVariableGlobal:    "NameVariableGlobal",
	NameVariableInstance:  "NameVariableInstance",
	NameVariableMagic:     "NameVariableMagic",
	NameFunction:          "NameFunction",
	NameFunctionMagic:     "NameFunctionMagic",

	Literal:                  "Literal",
	LiteralDate:              "LiteralDate",
	LiteralOther:             "LiteralOther",
	LiteralString:            "LiteralString",
	LiteralStringAffix:       "LiteralStringAffix",
	LiteralStringAtom:        "LiteralStringAtom",
	LiteralStringBacktick:    "LiteralStringBacktick",
	LiteralStringBoolean:     "LiteralStringBoolean",
	LiteralStringChar:        "LiteralStringChar",
	LiteralStringDelimiter:   "LiteralStringDelimiter",
	LiteralStringDoc:         "LiteralStringDoc",
	LiteralStringDouble:      "LiteralStringDouble",
	LiteralStringEscape:      "LiteralStringEscape",
	LiteralStringHeredoc:     "LiteralStringHeredoc",
	LiteralStringInterpol:    "LiteralStringInterpol",
	LiteralStringName:        "LiteralStringName",
	LiteralStringOther:       "LiteralStringOther",
	LiteralStringRegex:       "LiteralStringRegex",
	LiteralStringSingle:      "LiteralStringSingle",
	LiteralStringSymbol:      "LiteralStringSymbol",
	LiteralNumber:            "LiteralNumber",
	LiteralNumberBin:         "LiteralNumberBin",
	LiteralNumberFloat:       "LiteralNumberFloat",
	LiteralNumberHex:         "LiteralNumberHex",
	LiteralNumberInteger:     "LiteralNumberInteger",
	LiteralNumberIntegerLong: "LiteralNumberIntegerLong",
	LiteralNumberOct:         "LiteralNumberOct",
	LiteralNumberByte:        "LiteralNumberByte",

	Operator:         "Operator",
	OperatorWord:     "OperatorWord",
	OperatorReserved: "OperatorReserved",

	Punctuation: "Punctuation",

	Comment:            "Comment",
	CommentHashbang:    "CommentHashbang",
	CommentMultiline:   "CommentMultiline",
	CommentSingle:      "CommentSingle",
	CommentSpecial:     "CommentSpecial",
	CommentPreproc:     "CommentPreproc",
	CommentPreprocFile: "CommentPreprocFile",

	Generic:           "Generic",
	GenericDeleted:    "GenericDeleted",
	GenericEmph:       "GenericEmph",
	GenericError:      "GenericError",
	GenericHeading:    "GenericHeading",
	GenericInserted:   "GenericInserted",
	GenericOutput:     "GenericOutput",
	GenericPrompt:     "GenericPrompt",
	GenericStrong:     "GenericStrong",
	GenericSubheading: "GenericSubheading",
	GenericTraceback:  "GenericTraceback",
	GenericUnderline:  "GenericUnderline",

	Text:            "Text",
	TextWhitespace:  "TextWhitespace",
	TextSymbol:      "TextSymbol",
	TextPunctuation: "TextPunctuation",
}

// String returns the name of the type.
func (t TokenType) String() string {
	if s, ok := typeNames[t]; ok {
		return s
	}
	return "TokenType(" + strconv.Itoa(int(t)) + ")"
}

// classNames are the CSS class names for each type. They are the same as Chroma's StandardTypes (only the
// outer wrapper, PreWrapper, differs). As in Chroma, some types are not listed (such as NameKeyword).
var classNames = map[TokenType]string{
	Background:       "bg",
	PreWrapper:       "highlight", // Chroma uses "chroma". We use a generic name, not a product name
	Line:             "line",
	LineNumbers:      "ln",
	LineNumbersTable: "lnt",
	LineHighlight:    "hl",
	LineTable:        "lntable",
	LineTableTD:      "lntd",
	LineLink:         "lnlinks",
	CodeLine:         "cl",
	Text:             "",
	TextWhitespace:   "w",
	Error:            "err",
	Other:            "x",

	Keyword:            "k",
	KeywordConstant:    "kc",
	KeywordDeclaration: "kd",
	KeywordNamespace:   "kn",
	KeywordPseudo:      "kp",
	KeywordReserved:    "kr",
	KeywordType:        "kt",

	Name:                 "n",
	NameAttribute:        "na",
	NameBuiltin:          "nb",
	NameBuiltinPseudo:    "bp",
	NameClass:            "nc",
	NameConstant:         "no",
	NameDecorator:        "nd",
	NameEntity:           "ni",
	NameException:        "ne",
	NameFunction:         "nf",
	NameFunctionMagic:    "fm",
	NameProperty:         "py",
	NameLabel:            "nl",
	NameNamespace:        "nn",
	NameOther:            "nx",
	NameTag:              "nt",
	NameVariable:         "nv",
	NameVariableClass:    "vc",
	NameVariableGlobal:   "vg",
	NameVariableInstance: "vi",
	NameVariableMagic:    "vm",

	Literal:     "l",
	LiteralDate: "ld",

	LiteralString:          "s",
	LiteralStringAffix:     "sa",
	LiteralStringBacktick:  "sb",
	LiteralStringChar:      "sc",
	LiteralStringDelimiter: "dl",
	LiteralStringDoc:       "sd",
	LiteralStringDouble:    "s2",
	LiteralStringEscape:    "se",
	LiteralStringHeredoc:   "sh",
	LiteralStringInterpol:  "si",
	LiteralStringOther:     "sx",
	LiteralStringRegex:     "sr",
	LiteralStringSingle:    "s1",
	LiteralStringSymbol:    "ss",

	LiteralNumber:            "m",
	LiteralNumberBin:         "mb",
	LiteralNumberFloat:       "mf",
	LiteralNumberHex:         "mh",
	LiteralNumberInteger:     "mi",
	LiteralNumberIntegerLong: "il",
	LiteralNumberOct:         "mo",

	Operator:         "o",
	OperatorWord:     "ow",
	OperatorReserved: "or",

	Punctuation: "p",

	Comment:            "c",
	CommentHashbang:    "ch",
	CommentMultiline:   "cm",
	CommentPreproc:     "cp",
	CommentPreprocFile: "cpf",
	CommentSingle:      "c1",
	CommentSpecial:     "cs",

	Generic:           "g",
	GenericDeleted:    "gd",
	GenericEmph:       "ge",
	GenericError:      "gr",
	GenericHeading:    "gh",
	GenericInserted:   "gi",
	GenericOutput:     "go",
	GenericPrompt:     "gp",
	GenericStrong:     "gs",
	GenericSubheading: "gu",
	GenericTraceback:  "gt",
	GenericUnderline:  "gl",
}

// Class returns the CSS class name of the type. For a type not in the table, ok is false.
func (t TokenType) Class() (class string, ok bool) {
	class, ok = classNames[t]
	return class, ok
}

// Parent returns the next category up: for a subcategory member, the start of the subcategory; for the
// start of a subcategory, the start of the major category; for the start of a major category, 0.
func (t TokenType) Parent() TokenType {
	if t%100 != 0 {
		return t / 100 * 100
	}
	if t%1000 != 0 {
		return t / 1000 * 1000
	}
	return 0
}

// Category returns the start of the major category.
func (t TokenType) Category() TokenType { return t / 1000 * 1000 }

// SubCategory returns the start of the subcategory.
func (t TokenType) SubCategory() TokenType { return t / 100 * 100 }
