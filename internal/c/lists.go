// Code generated from Chroma 2.27.0 lexers/embedded/c.xml and c++.xml. DO NOT EDIT.

// Chroma (Copyright (C) 2017 Alec Thomas, MIT License) c.xml and c++.xml: all the words matched by the
// word-list regular expressions, written out (each one needs a trailing \b). TestCWordListsMatchChroma in
// compat/ compares every word with Chroma.

package c

// cKeywords holds C keywords (Keyword).
const cKeywords = "restricted|volatile|continue|register|default|typedef|struct|extern|switch|sizeof|static|return|union|while|const|break|goto|enum|else|case|auto|for|asm|if|do"

// cTypes holds C types (KeywordType). \b[a-z]\w*_t\b is also a type.
const cTypes = "bool|int|long|float|short|double|char8_t|char16_t|char32_t|char|unsigned|signed|void|uint_fast8_t|uint_fast16_t|uint_fast32_t|uint_fast64_t|uint_least8_t|uint_least16_t|uint_least32_t|uint_least64_t|uint8_t|uint16_t|uint32_t|uint64_t|int_fast8_t|int_fast16_t|int_fast32_t|int_fast64_t|int_least8_t|int_least16_t|int_least32_t|int_least64_t|int8_t|int16_t|int32_t|int64_t"

// cReserved holds C reserved words 1 (KeywordReserved).
const cReserved = "typename|__inline|restrict|_inline|thread|inline|naked"

// cReservedM holds C reserved words 2 (KeywordReserved).
const cReservedM = "__m128d|__m128i|__m128|__m64"

// cReservedUnderscore holds C reserved words 3 (KeywordReserved).
const cReservedUnderscore = "__forceinline|__identifier|__unaligned|__declspec|__fastcall|__finally|__stdcall|__wchar_t|__assume|__except|__int32|__cdecl|__int16|__leave|__based|__raise|__int64|__noop|__int8|__w64|__try|__asm"

// cBuiltins holds C built-in names (NameBuiltin).
const cBuiltins = "true|false|NULL"

// cppKeywords holds C++-only keywords (Keyword). Tried before the other rules.
const cppKeywords = "reinterpret_cast|static_assert|thread_local|dynamic_cast|static_cast|const_cast|co_return|protected|namespace|consteval|constexpr|typename|co_await|co_yield|operator|restrict|explicit|template|override|noexcept|requires|decltype|alignof|private|alignas|virtual|mutable|nullptr|concept|export|friend|typeid|throws|public|delete|final|throw|catch|using|this|new|try"

// cppCKeywords holds C++ keywords shared with C (Keyword).
const cppCKeywords = "restricted|volatile|continue|register|default|typedef|struct|extern|switch|sizeof|static|return|union|while|const|break|goto|enum|else|case|auto|for|asm|if|do"

// cppTypes holds C++ types (KeywordType).
const cppTypes = "bool|int|long|float|short|double|char8_t|char16_t|char32_t|char|wchar_t|unsigned|signed|void|uint_fast8_t|uint_fast16_t|uint_fast32_t|uint_fast64_t|uint_least8_t|uint_least16_t|uint_least32_t|uint_least64_t|uint8_t|uint16_t|uint32_t|uint64_t|int_fast8_t|int_fast16_t|int_fast32_t|int_fast64_t|int_least8_t|int_least16_t|int_least32_t|int_least64_t|int8_t|int16_t|int32_t|int64_t"

// cppReserved holds C++ reserved words 1 (KeywordReserved).
const cppReserved = "typename|__inline|restrict|_inline|thread|inline|naked"

// cppReservedM holds C++ reserved words 2 (KeywordReserved).
const cppReservedM = "__m128d|__m128i|__m128|__m64"

// cppReservedUnderscore holds C++ reserved words 3 (KeywordReserved).
const cppReservedUnderscore = "__forceinline|__identifier|__unaligned|__declspec|__fastcall|__stdcall|__finally|__except|__assume|__int32|__cdecl|__int64|__based|__leave|__int16|__raise|__noop|__int8|__w64|__try|__asm"

// cppBuiltins holds C++ built-in names (NameBuiltin).
const cppBuiltins = "true|false|NULL"
