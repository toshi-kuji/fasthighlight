package markdown

import (
	"testing"

	"github.com/toshi-kuji/fasthighlight/internal/mdcore"
)

// Parallelism is passed to mdcore as is, so the constants must have mdcore's numbering.
func TestParallelismMatchesMdcore(t *testing.T) {
	if mdcore.Mode(ParallelAuto) != mdcore.Auto || mdcore.Mode(ParallelNever) != mdcore.Never ||
		mdcore.Mode(ParallelAlways) != mdcore.Always {
		t.Fatal("Parallelism constants differ from mdcore.Mode")
	}
}
