// Code generated from Chroma 2.27.0 lexers/embedded/rust.xml. DO NOT EDIT.

// Chroma (Copyright (C) 2017 Alec Thomas, MIT License) rust.xml: the alternatives of its regular
// expressions, copied in the same order. TestWordListsMatchChroma in compat/ compares every word with Chroma.

package rust

// keywords holds reserved words (Keyword).
const keywords = "unsafe|static|extern|return|const|crate|where|while|await|trait|super|async|match|impl|else|move|loop|pub|ref|mut|for|dyn|use|box|if|in|as"

// reserved holds reserved words (KeywordReserved).
const reserved = "abstract|override|unsized|virtual|become|typeof|final|macro|yield|priv|try|do"

// constants holds constants (KeywordConstant).
const constants = "true|false"

// types holds types (KeywordType).
const types = "isize|usize|bool|char|u128|i128|i64|i32|i16|str|u64|u32|f32|f64|u16|i8|u8"

// builtins holds built-in types and so on (NameBuiltin).
const builtins = "DoubleEndedIterator|ExactSizeIterator|IntoIterator|PartialOrd|PartialEq|ToString|Iterator|ToOwned|Default|Result|String|FnOnce|Extend|Option|FnMut|Unpin|Sized|AsRef|AsMut|Clone|None|From|Into|Sync|drop|Send|Drop|Copy|Some|Ord|Err|Box|Vec|Eq|Ok|Fn"
