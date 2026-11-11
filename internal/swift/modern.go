package swift

// modernBuiltins lists standard library names of current Swift that Chroma's word list (lists.go, from the
// Swift 1 and 2 era) does not have, such as Codable (Swift 4) and Identifiable (Swift 5.1). In production
// (Options.ChromaBuiltins false) they are NameBuiltin, like Hashable and Int, so that the standard types
// share one color. Common words that programs also use for their own types (Task, Result, Duration, Clock)
// are left out. This list is written by hand; it is not generated from Chroma.
const modernBuiltins = "Codable|Encodable|Decodable|Identifiable|Sendable|Error|Never|" +
	"Sequence|IteratorProtocol|Collection|BidirectionalCollection|RandomAccessCollection|" +
	"MutableCollection|RangeReplaceableCollection|StringProtocol|Substring|Set|AnyHashable|" +
	"CaseIterable|OptionSet|CustomStringConvertible|CustomDebugStringConvertible|" +
	"LosslessStringConvertible|TextOutputStream|TextOutputStreamable|" +
	"ExpressibleByStringLiteral|ExpressibleByIntegerLiteral|ExpressibleByFloatLiteral|" +
	"ExpressibleByBooleanLiteral|ExpressibleByArrayLiteral|ExpressibleByDictionaryLiteral|" +
	"ExpressibleByNilLiteral|ExpressibleByStringInterpolation|" +
	"Numeric|SignedNumeric|AdditiveArithmetic|BinaryInteger|FixedWidthInteger|SignedInteger|" +
	"UnsignedInteger|FloatingPoint|BinaryFloatingPoint|Float16|" +
	"ClosedRange|PartialRangeFrom|PartialRangeUpTo|PartialRangeThrough|RangeExpression|" +
	"KeyPath|WritableKeyPath|ReferenceWritableKeyPath|PartialKeyPath|AnyKeyPath|" +
	"Actor|AnyActor|MainActor|UnsafeRawPointer|UnsafeMutableRawPointer|UnsafeRawBufferPointer|" +
	"UnsafeMutableRawBufferPointer|OpaquePointer|AsyncSequence|AsyncIteratorProtocol|AsyncStream|" +
	"AsyncThrowingStream|CheckedContinuation|UnsafeContinuation|TaskGroup|ThrowingTaskGroup"
