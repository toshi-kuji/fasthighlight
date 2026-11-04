// Code generated from Chroma 2.27.0 lexers/embedded/php.xml. DO NOT EDIT.

// Chroma (Copyright (C) 2017 Alec Thomas, MIT License) php.xml: the alternatives of its regular
// expressions, copied in the same order. TestWordListsMatchChroma in compat/ compares every word with Chroma.

package php

// keywords holds reserved words (Keyword). Case-insensitive.
const keywords = "and|E_PARSE|old_function|E_ERROR|or|as|E_WARNING|parent|eval|PHP_OS|break|exit|case|extends|PHP_VERSION|cfunction|FALSE|print|for|require|continue|foreach|require_once|declare|return|default|static|do|switch|die|stdClass|echo|else|TRUE|elseif|var|empty|if|xor|enddeclare|include|virtual|endfor|include_once|while|endforeach|global|endif|list|endswitch|new|endwhile|not|array|E_ALL|NULL|final|php_user_filter|interface|implements|public|private|protected|abstract|clone|try|catch|throw|this|use|namespace|trait|yield|finally|match"

// constants holds constants (KeywordConstant). Case-insensitive.
const constants = "true|false|null"

// magicConstants holds special constants (NameConstant). Case-insensitive.
const magicConstants = "__NAMESPACE__|__FUNCTION__|__METHOD__|__CLASS__|__TRAIT__|__LINE__|__FILE__|__DIR__"

// magicFuncs holds special methods (NameFunctionMagic). Case-insensitive.
const magicFuncs = "__callStatic|__set_state|__construct|__debugInfo|__toString|__destruct|__invoke|__wakeup|__clone|__sleep|__isset|__unset|__call|__get|__set"
