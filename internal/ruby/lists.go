// Code generated from Chroma 2.27.0 lexers/embedded/ruby.xml. DO NOT EDIT.

// Chroma (Copyright (C) 2017 Alec Thomas, MIT License) ruby.xml: the alternatives of its regular
// expressions, copied in the same order. TestWordListsMatchChroma in compat/ compares every word with Chroma.

package ruby

// keywords holds reserved words (Keyword). Some words contain ?, such as defined?.
const keywords = "defined?|return|ensure|rescue|unless|undef|until|break|begin|elsif|super|alias|while|retry|BEGIN|raise|yield|redo|next|case|when|then|else|end|for|END|do|if|in"

// pseudo holds commonly used methods and so on (KeywordPseudo).
const pseudo = "module_function|attr_accessor|attr_reader|attr_writer|initialize|protected|include|private|extend|public|raise|false|catch|throw|attr|loop|true|new|nil"

// operatorWords holds word operators (OperatorWord).
const operatorWords = "not|and|or"

// builtinQuestions holds built-in methods ending in ? (NameBuiltin). Needs a trailing ? (no \b).
const builtinQuestions = "protected_method_defined|private_method_defined|public_method_defined|method_defined|const_defined|block_given|instance_of|respond_to|iterator|autoload|kind_of|tainted|include|frozen|equal|is_a|nil|eql"

// builtins2 holds built-in methods 2 (NameBuiltin). Not preceded by ..
const builtins2 = "protected_instance_methods|private_instance_methods|public_instance_methods|instance_variable_set|instance_variable_get|private_class_method|public_class_method|instance_variables|protected_methods|singleton_methods|included_modules|instance_methods|global_variables|private_methods|local_variables|instance_method|class_variables|public_methods|const_defined?|set_trace_func|method_missing|const_missing|instance_eval|module_eval|untrace_var|class_eval|trace_var|const_get|readlines|ancestors|constants|const_set|object_id|readline|autoload|__send__|untaint|methods|display|Integer|sprintf|inspect|require|syscall|at_exit|binding|extend|printf|lambda|__id__|String|callcc|method|select|format|system|freeze|caller|raise|Float|print|throw|taint|clone|srand|Array|abort|split|catch|chomp|sleep|open|putc|puts|fork|fail|trap|exit|scan|getc|self|send|eval|gets|exec|gsub|proc|load|loop|chop|warn|hash|test|name|to_a|rand|to_s|sub|dup|id|p"
