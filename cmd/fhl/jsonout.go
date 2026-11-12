package main

import (
	"io"
	"math"
	"strconv"
	"unicode/utf8"
)

// fhl writes its JSON with this small encoder instead of encoding/json. With Go 1.27, encoding/json is built on
// encoding/json/v2 and adds about 0.6 MB to fhl, although fhl only writes a few kinds of records of strings,
// numbers and booleans. The output is byte for byte what encoding/json of Go 1.25 writes for the same values
// (json.NewEncoder with SetEscapeHTML(false), and SetIndent("", "  ") for the indented forms), which fhl
// v0.6.5 and earlier wrote. Go 1.27's encoding/json differs in one point: it writes invalid UTF-8 as a raw
// U+FFFD, where this encoder (like Go 1.25) writes the escape \ufffd; both read back as the same string.
// TestJSONMatchesEncodingJSON and FuzzJSONString check it against encoding/json.

// jsonValue is a value of the JSON model: string, int, bool, float64, []string, jsonObject, []jsonObject
// or nil (null).
type jsonValue = any

// jsonField is one member of an object, in the order it is written.
type jsonField struct {
	key   string
	value jsonValue
}

// jsonObject is a JSON object with its members in order.
type jsonObject []jsonField

// encodeJSON writes v to w followed by a newline, as Encoder.Encode does: indented with two spaces if indent is
// true, otherwise compact (one line).
func encodeJSON(w io.Writer, v jsonValue, indent bool) error {
	e := jsonEncoder{indent: indent}
	e.value(v)
	e.b = append(e.b, '\n')
	_, err := w.Write(e.b)
	return err
}

type jsonEncoder struct {
	b      []byte
	indent bool
	depth  int
}

// newline starts a new line at the current depth, when indenting.
func (e *jsonEncoder) newline() {
	if !e.indent {
		return
	}
	e.b = append(e.b, '\n')
	for range e.depth {
		e.b = append(e.b, "  "...)
	}
}

func (e *jsonEncoder) value(v jsonValue) {
	switch v := v.(type) {
	case nil:
		e.b = append(e.b, "null"...)
	case string:
		e.b = appendJSONString(e.b, v)
	case int:
		e.b = strconv.AppendInt(e.b, int64(v), 10)
	case bool:
		e.b = strconv.AppendBool(e.b, v)
	case float64:
		e.b = appendJSONFloat(e.b, v)
	case []string:
		if v == nil {
			e.b = append(e.b, "null"...)
			return
		}
		e.array(len(v), func(i int) { e.b = appendJSONString(e.b, v[i]) })
	case []jsonObject:
		if v == nil {
			e.b = append(e.b, "null"...)
			return
		}
		e.array(len(v), func(i int) { e.value(v[i]) })
	case jsonObject:
		e.b = append(e.b, '{')
		if len(v) == 0 {
			e.b = append(e.b, '}')
			return
		}
		e.depth++
		for i, f := range v {
			if i > 0 {
				e.b = append(e.b, ',')
			}
			e.newline()
			e.b = appendJSONString(e.b, f.key)
			e.b = append(e.b, ':')
			if e.indent {
				e.b = append(e.b, ' ')
			}
			e.value(f.value)
		}
		e.depth--
		e.newline()
		e.b = append(e.b, '}')
	default:
		panic("encodeJSON: unsupported type") // a programming error in fhl
	}
}

// array writes an array of n elements, each written by elem.
func (e *jsonEncoder) array(n int, elem func(i int)) {
	e.b = append(e.b, '[')
	if n == 0 {
		e.b = append(e.b, ']')
		return
	}
	e.depth++
	for i := range n {
		if i > 0 {
			e.b = append(e.b, ',')
		}
		e.newline()
		elem(i)
	}
	e.depth--
	e.newline()
	e.b = append(e.b, ']')
}

// appendJSONString appends s as a JSON string, escaped as encoding/json does without HTML escaping: ", \ and
// control characters are escaped (\b, \f, \n, \r, \t, or \u00XX), invalid UTF-8 becomes \ufffd, and U+2028 and
// U+2029 are escaped.
func appendJSONString(b []byte, s string) []byte {
	const hex = "0123456789abcdef"
	b = append(b, '"')
	start := 0
	for i := 0; i < len(s); {
		if c := s[i]; c < utf8.RuneSelf {
			if c >= 0x20 && c != '"' && c != '\\' {
				i++
				continue
			}
			b = append(b, s[start:i]...)
			switch c {
			case '"', '\\':
				b = append(b, '\\', c)
			case '\b':
				b = append(b, '\\', 'b')
			case '\f':
				b = append(b, '\\', 'f')
			case '\n':
				b = append(b, '\\', 'n')
			case '\r':
				b = append(b, '\\', 'r')
			case '\t':
				b = append(b, '\\', 't')
			default:
				b = append(b, '\\', 'u', '0', '0', hex[c>>4], hex[c&0xf])
			}
			i++
			start = i
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			b = append(b, s[start:i]...)
			b = append(b, `\ufffd`...)
			i += size
			start = i
			continue
		}
		if r == '\u2028' || r == '\u2029' {
			b = append(b, s[start:i]...)
			b = append(b, '\\', 'u', '2', '0', '2', hex[r&0xf])
			i += size
			start = i
			continue
		}
		i += size
	}
	b = append(b, s[start:]...)
	return append(b, '"')
}

// appendJSONFloat appends f as encoding/json writes a float64: the shortest form, with an exponent only for
// very small or very large values (and a two-digit exponent written without a leading zero).
func appendJSONFloat(b []byte, f float64) []byte {
	format := byte('f')
	if abs := math.Abs(f); abs != 0 && (abs < 1e-6 || abs >= 1e21) {
		format = 'e'
	}
	b = strconv.AppendFloat(b, f, format, -1, 64)
	if format == 'e' {
		// clean up e-09 to e-9
		if n := len(b); n >= 4 && b[n-4] == 'e' && b[n-3] == '-' && b[n-2] == '0' {
			b[n-2] = b[n-1]
			b = b[:n-1]
		}
	}
	return b
}

func (f fileResult) json() jsonObject {
	o := jsonObject{{"input", f.Input}}
	if f.Output != "" {
		o = append(o, jsonField{"output", f.Output})
	}
	return append(o, jsonField{"blocks", f.Blocks}, jsonField{"uncolored", f.Uncolored})
}

func (r errorResult) json() jsonObject {
	o := jsonObject{{"message", r.Message}}
	if len(r.Inputs) > 0 {
		o = append(o, jsonField{"inputs", r.Inputs})
	}
	return o
}

func (u uncolored) json() jsonObject {
	return jsonObject{{"file", u.File}, {"line", u.Line}, {"lang", u.Lang}, {"reason", u.Reason}}
}

func (s summary) json() jsonObject {
	return jsonObject{{"files", s.Files}, {"blocks", s.Blocks}, {"uncolored", s.Uncolored},
		{"skipped", s.Skipped}, {"errors", s.Errors}, {"seconds", s.Seconds}}
}

// objects turns a slice of records into []jsonObject, keeping a nil slice nil (null).
func objects[T interface{ json() jsonObject }](s []T) []jsonObject {
	if s == nil {
		return nil
	}
	out := make([]jsonObject, len(s))
	for i, x := range s {
		out[i] = x.json()
	}
	return out
}

func (r *report) json() jsonObject {
	o := jsonObject{{"files", objects(r.Files)}, {"styles", r.Styles}, {"skipped", r.Skipped},
		{"uncolored", objects(r.Uncolored)}}
	if r.Copied != nil {
		o = append(o, jsonField{"copied", *r.Copied})
	}
	return append(o, jsonField{"errors", objects(r.Errors)}, jsonField{"summary", r.Summary.json()})
}

func (b codeBlock) json() jsonObject {
	o := jsonObject{{"file", b.File}, {"line", b.Line}, {"lang", b.Lang}, {"open", b.Open}, {"code", b.Code},
		{"close", b.Close}, {"colored", b.Colored}}
	if b.Reason != "" {
		o = append(o, jsonField{"reason", b.Reason})
	}
	return o
}

func (l langEntry) json() jsonObject {
	return jsonObject{{"name", l.Name}, {"aliases", l.Aliases}, {"filenames", l.Filenames}}
}

func (t themeEntry) json() jsonObject {
	return jsonObject{{"name", t.Name}, {"label", t.Label}, {"light", t.Light}, {"dark", t.Dark}}
}
