// Code generated from Chroma 2.27.0 lexers/embedded/java.xml. DO NOT EDIT.

// Chroma (Copyright (C) 2017 Alec Thomas, MIT License) java.xml: the alternatives of its regular
// expressions, copied in the same order. TestWordListsMatchChroma in compat/ compares every word with Chroma.

package java

// keywords holds reserved words (Keyword).
const keywords = "assert|break|case|catch|continue|default|do|else|finally|for|if|goto|instanceof|new|return|switch|this|throw|try|while"

// declarations holds declaration words (KeywordDeclaration). Some words contain -, such as non-sealed.
const declarations = "abstract|const|enum|exports|extends|final|implements|native|non-sealed|open|opens|permits|private|protected|provides|public|requires|sealed|static|strictfp|super|synchronized|throws|to|transient|transitive|uses|volatile|with|yield"

// types holds types (KeywordType).
const types = "boolean|byte|char|double|float|int|long|short|void"

// constants holds constants (KeywordConstant).
const constants = "true|false|null"
