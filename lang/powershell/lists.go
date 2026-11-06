// Code generated from Chroma 2.27.0 lexers/embedded/powershell.xml. DO NOT EDIT.

// Chroma (Copyright (C) 2017 Alec Thomas, MIT License) powershell.xml: the alternatives of its regular
// expressions, copied in the same order. TestWordListsMatchChroma in compat/ compares every word with Chroma.

package powershell

// builtins holds attribute names and so on (NameBuiltin). Case-insensitive; some words appear twice.
const builtins = "parameter|validatenotnullorempty|validatescript|validaterange|validateset|validaterange|validatepattern|validatelength|validatecount|validatenotnullorempty|validatescript|cmdletbinding|alias"

// attributes holds parameter attribute arguments (NameAttribute). Case-insensitive.
const attributes = "mandatory|parametersetname|position|helpmessage|valuefrompipeline|valuefrompipelinebypropertyname|valuefromremainingarguments|dontshow"

// attributes2 holds cmdlet binding arguments (NameAttribute). Case-insensitive.
const attributes2 = "confirmimpact|defaultparametersetname|helpuri|supportspaging|supportsshouldprocess|positionalbinding"

// keywords holds reserved words (Keyword). Case-insensitive; some words contain ?, %, # or :.
const keywords = "while|until|trap|switch|return|ref|process|param|parameter|if|in|global:|foreach|for|finally|filter|end|elseif|else|dynamicparam|do|default|continue|break|begin|%|?|#script|#private|#local|#global|try|catch|throw"

// operators holds operators after - (Operator). Case-insensitive.
const operators = "and|as|band|bnot|bor|bxor|casesensitive|ccontains|ceq|cge|cgt|cle|clike|clt|cmatch|cne|cnotcontains|cnotlike|cnotmatch|contains|creplace|eq|exact|f|file|ge|gt|icontains|ieq|ige|igt|ile|ilike|ilt|imatch|ine|inotcontains|inotlike|inotmatch|ireplace|is|isnot|le|like|lt|match|ne|not|notcontains|notlike|notmatch|or|regex|replace|wildcard"

// aliases holds built-in aliases (NameBuiltin). Case-insensitive; followed by \s, not \b.
const aliases = "ac|asnp|cat|cd|cfs|chdir|clc|clear|clhy|cli|clp|cls|clv|cnsn|compare|copy|cp|cpi|cpp|curl|cvpa|dbp|del|diff|dir|dnsn|ebp|echo|epal|epcsv|epsn|erase|etsn|exsn|fc|fhx|fl|foreach|ft|fw|gal|gbp|gc|gci|gcm|gcs|gdr|ghy|gi|gjb|gl|gm|gmo|gp|gps|gpv|group|gsn|gsnp|gsv|gu|gv|gwmi|h|history|icm|iex|ihy|ii|ipal|ipcsv|ipmo|ipsn|irm|ise|iwmi|iwr|kill|lp|ls|man|md|measure|mi|mount|move|mp|mv|nal|ndr|ni|nmo|npssc|nsn|nv|ogv|oh|popd|ps|pushd|pwd|r|rbp|rcjb|rcsn|rd|rdr|ren|ri|rjb|rm|rmdir|rmo|rni|rnp|rp|rsn|rsnp|rujb|rv|rvpa|rwmi|sajb|sal|saps|sasv|sbp|sc|select|set|shcm|si|sl|sleep|sls|sort|sp|spjb|spps|spsv|start|sujb|sv|swmi|tee|trcm|type|wget|where|wjb|write"

// docWords holds comment-based help keywords after . (LiteralStringDoc). Case-insensitive.
const docWords = "component|description|example|externalhelp|forwardhelpcategory|forwardhelptargetname|functionality|inputs|link|notes|outputs|parameter|remotehelprunspace|role|synopsis"
