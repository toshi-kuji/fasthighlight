// Code generated from Chroma 2.27.0 lexers/embedded/typescript.xml. DO NOT EDIT.

// Chroma (Copyright (C) 2017 Alec Thomas, MIT License) typescript.xml: the alternatives of its regular
// expressions, copied in the same order. TestWordListsMatchChroma in compat/ compares every word with Chroma.

package typescript

// keywords holds reserved words (Keyword). No \b before.
const keywords = "for|in|of|while|do|break|return|yield|continue|switch|case|default|if|else|throw|try|catch|finally|new|delete|typeof|instanceof|keyof|asserts|is|infer|await|void|this"

// declarations holds declaration words (KeywordDeclaration). No \b before.
const declarations = "var|let|with|function"

// reserved holds reserved words (KeywordReserved). No \b before.
const reserved = "abstract|async|boolean|class|const|debugger|enum|export|extends|from|get|global|goto|implements|import|interface|namespace|package|private|protected|public|readonly|require|set|static|super|type|using"

// constants holds constants (KeywordConstant). No \b before.
const constants = "true|false|null|NaN|Infinity|undefined"

// builtins holds built-in names (NameBuiltin). No \b before.
const builtins = "Array|Boolean|Date|Error|Function|Math|Number|Object|Packages|RegExp|String|decodeURI|decodeURIComponent|encodeURI|encodeURIComponent|eval|isFinite|isNaN|parseFloat|parseInt|document|this|window"

// types holds types (KeywordType). Needs \b before.
const types = "string|bool|number|any|never|object|symbol|unique|unknown|bigint"

// reserved2 holds reserved words 2 (KeywordReserved). Needs \b before.
const reserved2 = "constructor|declare|interface|as"
