// Code generated from Chroma 2.27.0 lexers/embedded/python.xml. DO NOT EDIT.

// Chroma (Copyright (C) 2017 Alec Thomas, MIT License) python.xml: the alternatives of its regular
// expressions, copied in the same order. TestPythonWordListsMatchChroma in compat/ compares every word with Chroma.

package python

// keywords holds reserved words in keywords (Keyword). Needs \b.
const keywords = "yield from|nonlocal|continue|finally|except|lambda|assert|global|return|raise|yield|while|break|await|async|pass|else|elif|with|try|for|del|as|if|match|case"

// keywordConstants holds constants in keywords and expr-keywords (KeywordConstant). Needs \b.
const keywordConstants = "False|True|None"

// exprKeywords holds reserved words inside expressions in expr-keywords (Keyword). Needs \b.
const exprKeywords = "yield from|async for|lambda|yield|await|else|for|if"

// operatorWords holds word operators in expr (OperatorWord). Needs \b.
const operatorWords = "in|is|and|or|not"

// builtins holds built-in functions and so on in builtins (NameBuiltin). Not preceded by . and needs \b.
const builtins = "staticmethod|classmethod|memoryview|__import__|issubclass|isinstance|frozenset|bytearray|enumerate|reversed|property|compile|complex|delattr|hasattr|setattr|globals|getattr|divmod|filter|locals|format|object|sorted|slice|print|bytes|range|input|tuple|round|super|float|eval|list|dict|repr|type|vars|hash|next|bool|open|iter|oct|pow|min|zip|max|map|bin|len|set|any|dir|all|abs|str|sum|chr|int|hex|ord|id"

// builtinPseudo holds self and so on in builtins (NameBuiltinPseudo). Not preceded by . and needs \b.
const builtinPseudo = "self|Ellipsis|NotImplemented|cls"

// exceptions holds exceptions in builtins (NameException). Not preceded by . and needs \b.
const exceptions = "PendingDeprecationWarning|ConnectionAbortedError|ConnectionRefusedError|UnicodeTranslateError|ConnectionResetError|ModuleNotFoundError|NotImplementedError|FloatingPointError|StopAsyncIteration|UnicodeDecodeError|DeprecationWarning|UnicodeEncodeError|NotADirectoryError|ProcessLookupError|ZeroDivisionError|IsADirectoryError|FileNotFoundError|UnboundLocalError|KeyboardInterrupt|ChildProcessError|EnvironmentError|IndentationError|InterruptedError|BlockingIOError|ArithmeticError|ConnectionError|BrokenPipeError|FileExistsError|ResourceWarning|PermissionError|RuntimeWarning|ReferenceError|AttributeError|AssertionError|UnicodeWarning|RecursionError|StopIteration|BaseException|OverflowError|SyntaxWarning|FutureWarning|GeneratorExit|ImportWarning|UnicodeError|TimeoutError|WindowsError|RuntimeError|BytesWarning|SystemError|UserWarning|MemoryError|ImportError|LookupError|BufferError|SyntaxError|SystemExit|ValueError|IndexError|NameError|Exception|TypeError|TabError|EOFError|KeyError|VMSError|Warning|OSError|IOError"

// magicFuncs holds special methods in magicfuncs (NameFunctionMagic). Needs \b.
const magicFuncs = "__instancecheck__|__subclasscheck__|__getattribute__|__length_hint__|__rfloordiv__|__ifloordiv__|__itruediv__|__contains__|__floordiv__|__rtruediv__|__reversed__|__setitem__|__complex__|__rdivmod__|__delattr__|__rmatmul__|__ilshift__|__prepare__|__delitem__|__rrshift__|__imatmul__|__rlshift__|__setattr__|__truediv__|__getitem__|__missing__|__getattr__|__irshift__|__rshift__|__format__|__invert__|__matmul__|__divmod__|__delete__|__aenter__|__lshift__|__await__|__bytes__|__anext__|__aiter__|__aexit__|__round__|__float__|__enter__|__index__|__iadd__|__ipow__|__rpow__|__iter__|__init__|__ixor__|__rmul__|__rmod__|__imul__|__imod__|__iand__|__hash__|__rsub__|__exit__|__rxor__|__bool__|__call__|__rand__|__next__|__radd__|__isub__|__repr__|__set__|__add__|__new__|__neg__|__xor__|__and__|__mul__|__mod__|__sub__|__len__|__str__|__ror__|__ior__|__pos__|__del__|__get__|__dir__|__abs__|__int__|__pow__|__eq__|__gt__|__le__|__lt__|__ne__|__or__|__ge__"

// magicVars holds special attributes in magicvars (NameVariableMagic). Needs \b.
const magicVars = "__annotations__|__kwdefaults__|__qualname__|__objclass__|__defaults__|__closure__|__globals__|__weakref__|__module__|__class__|__bases__|__slots__|__file__|__code__|__name__|__func__|__dict__|__self__|__mro__|__doc__"
