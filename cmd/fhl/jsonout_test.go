package main

import (
	"bytes"
	"encoding/json"
	"math"
	"math/rand/v2"
	"strings"
	"testing"
)

// The JSON fhl writes is checked against encoding/json, which is allowed in tests: tests are not part of the
// fhl binary.

// stdJSON writes v as encoding/json does for fhl's output.
func stdJSON(t testing.TB, v any, indent bool) string {
	t.Helper()
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if indent {
		enc.SetIndent("", "  ")
	}
	if err := enc.Encode(v); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

// sameJSON reports whether got (this encoder) is what encoding/json wrote as want. The one difference
// allowed is Go 1.27's: encoding/json writes invalid UTF-8 as a raw U+FFFD, this encoder as the escape \ufffd
// (as Go 1.25 does). Any other difference fails. Inputs never contain the text \ufffd themselves (see pieces).
func sameJSON(got, want string) bool {
	return got == want || strings.ReplaceAll(got, `\ufffd`, "\ufffd") == want
}

func ownJSON(t testing.TB, v jsonValue, indent bool) string {
	t.Helper()
	var buf bytes.Buffer
	if err := encodeJSON(&buf, v, indent); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

// pieces are parts of the random strings: everything that JSON escapes or that encoding/json treats specially
// (control characters, quotes, backslashes, HTML characters, invalid UTF-8, U+2028 and U+2029, DEL, non-ASCII).
var pieces = []string{"", "a", "go", " ", "\"", "\\", "/", "<", ">", "&", "\x00", "\x01", "\x07", "\b", "\f",
	"\n", "\r", "\t", "\x1b", "\x1f", "\x7f", "\x80", "\xff", "\xc3", "\xe2\x80", "é", "日本", "😀", "\u2028",
	"\u2029", "\ufffd", "\u00a0", "<pre class=\"highlight\">", "</code></pre>", "a.md", "dir/b.md"}

func randString(r *rand.Rand) string {
	var sb strings.Builder
	for range r.IntN(6) {
		sb.WriteString(pieces[r.IntN(len(pieces))])
	}
	return sb.String()
}

func randStrings(r *rand.Rand) []string {
	switch r.IntN(4) {
	case 0:
		return nil
	case 1:
		return []string{}
	}
	s := make([]string, r.IntN(4)+1)
	for i := range s {
		s[i] = randString(r)
	}
	return s
}

var floats = []float64{0, 1, 0.5, 0.000123, 1e-6, 9.99e-7, 1e-7, 1.5e-9, 123.456, 1e20, 1e21, 1.2345e22, 3.14159,
	0.1 + 0.2, math.SmallestNonzeroFloat64, math.MaxFloat64, -0.0, -2.5e-8}

func randReport(r *rand.Rand) *report {
	rep := &report{Styles: randStrings(r), Skipped: randStrings(r)}
	if r.IntN(3) > 0 {
		rep.Files = []fileResult{}
		for range r.IntN(3) {
			out := ""
			if r.IntN(2) == 0 {
				out = randString(r)
			}
			rep.Files = append(rep.Files, fileResult{randString(r), out, r.IntN(100), r.IntN(10)})
		}
	}
	if r.IntN(3) > 0 {
		rep.Uncolored = []uncolored{}
		for range r.IntN(3) {
			rep.Uncolored = append(rep.Uncolored, uncolored{randString(r), r.IntN(1000), randString(r), randString(r)})
		}
	}
	if r.IntN(2) == 0 {
		c := randStrings(r)
		rep.Copied = &c
	}
	if r.IntN(3) > 0 {
		rep.Errors = []errorResult{}
		for range r.IntN(3) {
			rep.Errors = append(rep.Errors, errorResult{randString(r), randStrings(r)})
		}
	}
	rep.Summary = summary{r.IntN(100), r.IntN(1000), r.IntN(10), r.IntN(10), r.IntN(10),
		floats[r.IntN(len(floats))] / float64(r.IntN(3)+1)} // divided, so that MaxFloat64 does not overflow
	return rep
}

// Every kind of record fhl writes as JSON, with random contents, is written byte for byte as encoding/json
// writes it.
func TestJSONMatchesEncodingJSON(t *testing.T) {
	r := rand.New(rand.NewPCG(7, 11))
	var enc jsonEncoder // reused across records, as runCodeBlocks does
	for i := range 5000 {
		rep := randReport(r)
		if got, want := ownJSON(t, rep.json(), true), stdJSON(t, rep, true); !sameJSON(got, want) {
			t.Fatalf("report %d:\ngot  %q\nwant %q", i, got, want)
		}
		b := codeBlock{randString(r), r.IntN(500), randString(r), randString(r), randString(r), randString(r),
			r.IntN(2) == 0, ""}
		if r.IntN(2) == 0 {
			b.Reason = randString(r)
		}
		var buf bytes.Buffer
		if err := enc.encodeCodeBlock(&buf, b); err != nil {
			t.Fatal(err)
		}
		if got, want := buf.String(), stdJSON(t, b, false); !sameJSON(got, want) {
			t.Fatalf("code block %d:\ngot  %q\nwant %q", i, got, want)
		}
		var langs []langEntry
		var themes []themeEntry
		for range r.IntN(3) {
			langs = append(langs, langEntry{randString(r), randStrings(r), randStrings(r)})
			themes = append(themes, themeEntry{randString(r), randString(r), randString(r), randString(r)})
		}
		if got, want := ownJSON(t, objects(langs), true), stdJSON(t, langs, true); !sameJSON(got, want) {
			t.Fatalf("langs %d:\ngot  %q\nwant %q", i, got, want)
		}
		if got, want := ownJSON(t, objects(themes), true), stdJSON(t, themes, true); !sameJSON(got, want) {
			t.Fatalf("themes %d:\ngot  %q\nwant %q", i, got, want)
		}
	}
}

// Invalid UTF-8 is written as \ufffd, one per invalid byte, as Go 1.25's encoding/json and fhl v0.6.5 do.
func TestJSONInvalidUTF8(t *testing.T) {
	for in, want := range map[string]string{"\x80": `"\ufffd"`, "a\xffb": `"a\ufffdb"`, "\xe2\x80": `"\ufffd\ufffd"`,
		"\ufffd": "\"\ufffd\"", "\u2028\u2029": `"\u2028\u2029"`} {
		if got := string(appendJSONString(nil, in)); got != want {
			t.Errorf("%q: got %s, want %s", in, got, want)
		}
	}
}

// The real lists of languages and themes are written as encoding/json writes them.
func TestListsMatchEncodingJSON(t *testing.T) {
	var got bytes.Buffer
	if err := listLangs(&got, true); err != nil {
		t.Fatal(err)
	}
	var langs []langEntry
	if err := json.Unmarshal(got.Bytes(), &langs); err != nil {
		t.Fatal(err)
	}
	if want := stdJSON(t, langs, true); got.String() != want {
		t.Errorf("--list-langs --json differs from encoding/json")
	}
	got.Reset()
	if err := listThemes(&got, true); err != nil {
		t.Fatal(err)
	}
	var list []themeEntry
	if err := json.Unmarshal(got.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if want := stdJSON(t, list, true); got.String() != want {
		t.Errorf("--list-themes --json differs from encoding/json")
	}
}

// Any string and any float64 are written as encoding/json writes them.
//
//	go test -run='^$' -fuzz=FuzzJSONString -fuzztime=1m ./cmd/fhl
func FuzzJSONString(f *testing.F) {
	for _, s := range pieces {
		f.Add(s, 1.5)
	}
	for _, x := range floats {
		f.Add("x", x)
	}
	f.Fuzz(func(t *testing.T, s string, x float64) {
		if math.IsNaN(x) || math.IsInf(x, 0) {
			x = 0 // encoding/json cannot write them, and fhl never has them
		}
		v := struct {
			S string  `json:"s"`
			X float64 `json:"x"`
		}{s, x}
		o := jsonObject{{"s", s}, {"x", x}}
		for _, indent := range []bool{false, true} {
			if got, want := ownJSON(t, o, indent), stdJSON(t, v, indent); !sameJSON(got, want) {
				t.Fatalf("got %q, want %q", got, want)
			}
		}
	})
}
