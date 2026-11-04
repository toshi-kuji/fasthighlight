package fasthighlight_test

import (
	"sort"
	"strings"
	"testing"

	. "github.com/toshi-kuji/fasthighlight"
	_ "github.com/toshi-kuji/fasthighlight/lang/all"
	"github.com/toshi-kuji/fasthighlight/lang/javascript"
	"github.com/toshi-kuji/fasthighlight/lang/typescript"
)

// A language can be looked up by language name. Agreement with Chroma's lexers.Get is checked in compat/.
func TestLookupRegistered(t *testing.T) {
	tests := []struct {
		name string
		want *Language
	}{
		{"ts", typescript.Language},
		{"TS", typescript.Language},
		{"tsx", typescript.Language},
		{"typescript", typescript.Language},
		{"TypeScript", typescript.Language},
		{"mts", typescript.Language},        // as an extension
		{"app.ts", typescript.Language},     // as a file name
		{"src/app.ts", typescript.Language}, // for a path, the last part
		{"app.ts.bak", typescript.Language}, // backup suffix
		{"app.js~", javascript.Language},
		{"js", javascript.Language},
		{"JavaScript", javascript.Language},
		{"mjs", javascript.Language},
		{"cjs", javascript.Language},
		{"", nil},
		{"text", nil},
		{"fortran", nil},
	}
	for _, tt := range tests {
		if got := LookupRegistered(tt.name); got != tt.want {
			t.Errorf("LookupRegistered(%q) = %v, want %v", tt.name, name(got), name(tt.want))
		}
	}
}

func name(l *Language) string {
	if l == nil {
		return "nil"
	}
	return l.Name
}

// The registered languages are in name order.
func TestLanguages(t *testing.T) {
	var names []string
	for _, l := range Languages() {
		names = append(names, l.Name)
	}
	var want []string
	for _, l := range Builtins() {
		want = append(want, l.Name)
	}
	if got, want := strings.Join(names, ","), strings.Join(want, ","); got != want {
		t.Errorf("Languages() = %s, want %s", got, want)
	}
	if !sort.StringsAreSorted(names) {
		t.Errorf("Languages() is not sorted by name: %v", names)
	}
}

// Registering a language whose name or alias collides panics, and does not change the registered languages.
func TestRegisterDuplicate(t *testing.T) {
	before := len(Languages())
	dup := &Language{Name: "Dup", Aliases: []string{"dup-only", "ts"}, Lex: func(string, int) []Token { return nil }}
	func() {
		defer func() {
			if recover() == nil {
				t.Errorf("Register did not panic on a duplicate alias")
			}
		}()
		Register(dup)
	}()
	if LookupRegistered("dup-only") != nil || LookupRegistered("ts") != typescript.Language {
		t.Errorf("a failed Register changed the registry")
	}
	if len(Languages()) != before {
		t.Errorf("a failed Register added a language")
	}
}

// LookupSupported looks among the built-in languages, whether imported or not, and not among languages
// registered directly; LookupRegistered looks among the registered ones, including those.
func TestLookupSupported(t *testing.T) {
	if l := LookupSupported("rs"); l == nil || l.Package != "github.com/toshi-kuji/fasthighlight/lang/rust" {
		t.Errorf("LookupSupported(rs) = %v", l)
	}
	if l := LookupSupported("main.rs.bak"); l == nil || l.Name != "Rust" {
		t.Errorf("LookupSupported(main.rs.bak) = %v", l)
	}
	if l := LookupSupported("hs"); l != nil {
		t.Errorf("LookupSupported(hs) = %v, want nil", l.Name)
	}
	custom := &Language{Name: "fhtest-custom", Lex: func(string, int) []Token { return nil }}
	Register(custom)
	if LookupRegistered("fhtest-custom") != custom {
		t.Error("LookupRegistered does not find a language registered directly")
	}
	if LookupSupported("fhtest-custom") != nil {
		t.Error("LookupSupported finds a language registered directly")
	}
}

// The built-in catalog (Builtins) matches the languages registered by lang/all. Languages that tests
// register directly (without a Package) are left out.
func TestBuiltinsAreRegistered(t *testing.T) {
	builtins := Builtins()
	var registered []*Language
	for _, l := range Languages() {
		if l.Package != "" {
			registered = append(registered, l)
		}
	}
	if len(builtins) != len(registered) {
		t.Fatalf("Builtins() has %d languages, lang/all registers %d", len(builtins), len(registered))
	}
	for i, l := range builtins {
		if registered[i] != l {
			t.Errorf("%s: the registered language is not the Builtin entry", l.Name)
		}
		if l.Lex == nil || l.Package == "" {
			t.Errorf("%s: Lex or Package is missing", l.Name)
		}
		if LookupRegistered(l.Name) != l {
			t.Errorf("%s: not found by name", l.Name)
		}
	}
}
