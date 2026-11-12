package fasthighlight_test

import (
	"os/exec"
	"strings"
	"testing"
)

// The root package and the language packages do not depend on goldmark. Programs that only highlight code
// import just these, so goldmark must not end up in them; only the markdown package may use goldmark.
// This guards against a goldmark import added to them by mistake, which nothing else would notice.
func TestNoGoldmarkInHighlighting(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps",
		"github.com/toshi-kuji/fasthighlight", "github.com/toshi-kuji/fasthighlight/lang/...").CombinedOutput()
	if err != nil {
		t.Fatalf("go list: %v\n%s", err, out)
	}
	for _, p := range strings.Fields(string(out)) {
		if strings.HasPrefix(p, "github.com/yuin/goldmark") {
			t.Errorf("%s is a dependency of the root package or a language package", p)
		}
	}
}

// The markdown package and fhl use goldmark v1 only, and markdown/v2 uses goldmark v2 only, so a program that
// imports one of them gets only that goldmark (in its binary, and in its go.sum: go mod tidy also records the
// modules the tests of imported packages need, hence -test).
func TestOneGoldmarkPerPackage(t *testing.T) {
	for _, tt := range []struct{ pkg, wrong string }{
		{"github.com/toshi-kuji/fasthighlight/markdown", "v2"},
		{"github.com/toshi-kuji/fasthighlight/cmd/fhl", "v2"},
		{"github.com/toshi-kuji/fasthighlight/markdown/v2", "v1"},
	} {
		out, err := exec.Command("go", "list", "-deps", "-test", tt.pkg).CombinedOutput()
		if err != nil {
			t.Fatalf("go list: %v\n%s", err, out)
		}
		found := false
		for _, p := range strings.Fields(string(out)) {
			v2 := strings.HasPrefix(p, "github.com/yuin/goldmark/v2")
			v1 := !v2 && (p == "github.com/yuin/goldmark" || strings.HasPrefix(p, "github.com/yuin/goldmark/"))
			if v1 || v2 {
				found = true
			}
			if v1 && tt.wrong == "v1" || v2 && tt.wrong == "v2" {
				t.Errorf("%s depends on %s", tt.pkg, p)
			}
		}
		if !found {
			t.Errorf("%s does not depend on goldmark at all; is the package list right?", tt.pkg)
		}
	}
}
