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
