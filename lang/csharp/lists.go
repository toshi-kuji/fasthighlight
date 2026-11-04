// Code generated from Chroma 2.27.0 lexers/embedded/c#.xml. DO NOT EDIT.

// Chroma (Copyright (C) 2017 Alec Thomas, MIT License) c#.xml: the alternatives of its regular
// expressions, copied in the same order. TestWordListsMatchChroma in compat/ compares every word with Chroma.

package csharp

// preprocessor holds preprocessor words (CommentPreproc). Preceded by #[ \t]*.
const preprocessor = "if|endif|else|elif|define|undef|line|error|warning|region|endregion|pragma|nullable"

// keywords holds reserved words (Keyword).
const keywords = "as|await|base|break|by|case|catch|checked|continue|default|delegate|do|else|event|finally|fixed|for|foreach|goto|if|in|init|is|let|lock|new|on|out|params|readonly|ref|return|sizeof|stackalloc|switch|this|throw|try|typeof|unchecked|virtual|void|while|get|set|new|yield|add|remove|value|alias|ascending|descending|from|group|into|orderby|select|thenby|where|join|equals"

// declarations holds declaration words (KeywordDeclaration).
const declarations = "abstract|async|const|enum|explicit|extern|implicit|internal|operator|override|partial|private|protected|public|static|sealed|unsafe|volatile"

// types holds types (KeywordType). One ? may follow.
const types = "bool|byte|char|decimal|double|dynamic|float|int|long|object|sbyte|short|string|uint|ulong|ushort|var"

// constants holds constants (KeywordConstant).
const constants = "true|false|null"
