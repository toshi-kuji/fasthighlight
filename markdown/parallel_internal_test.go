package markdown

import (
	"testing"

	fh "github.com/toshi-kuji/fasthighlight"
)

// ParallelAuto goes parallel only when the total size of the contents of the code blocks to highlight exceeds
// 32KB and the number of blocks is at least the largest number the languages' ParallelHint calls for.
func TestParallelAuto(t *testing.T) {
	for _, tt := range []struct {
		n, need, size int
		want          bool
	}{
		{2, 3, 100 * 1024, false},
		{3, 3, 32 * 1024, false},
		{3, 3, 32*1024 + 1, true},
		{3, 4, 100 * 1024, false}, // 3 blocks, mixed with JSON and the like
		{4, 4, 32 * 1024, false},
		{4, 4, 32*1024 + 1, true},
		{25, 4, 40 * 1024, true},
	} {
		if got := parallel(ParallelAuto, tt.n, tt.need, tt.size); got != tt.want {
			t.Errorf("parallel(%d, %d, %d) = %v, want %v", tt.n, tt.need, tt.size, got, tt.want)
		}
	}
}

// needBlocks gives 3 for HintHeavy and 4 for HintLight and HintDefault.
func TestNeedBlocks(t *testing.T) {
	for _, tt := range []struct {
		hint fh.ParallelHint
		want int
	}{
		{fh.HintHeavy, 3},
		{fh.HintLight, 4},
		{fh.HintDefault, 4},
	} {
		if got := needBlocks(&fh.Language{ParallelHint: tt.hint}); got != tt.want {
			t.Errorf("needBlocks(%d) = %d, want %d", tt.hint, got, tt.want)
		}
	}
}
