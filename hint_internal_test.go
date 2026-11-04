package fasthighlight

import "testing"

// In the built-in catalog, the 16 programming languages (including CSS) are HintHeavy and the rest are not
// specified.
func TestBuiltinParallelHints(t *testing.T) {
	program := map[string]bool{
		"Bash": true, "C": true, "C#": true, "C++": true, "CSS": true, "Go": true, "Java": true, "JavaScript": true,
		"Kotlin": true, "PHP": true, "Python": true, "R": true, "Ruby": true, "Rust": true, "Swift": true, "TypeScript": true,
	}
	for _, l := range Builtins() {
		hint := HintDefault
		if program[l.Name] {
			hint = HintHeavy
		}
		if l.ParallelHint != hint {
			t.Errorf("%s: ParallelHint = %d, want %d", l.Name, l.ParallelHint, hint)
		}
	}
}

// A language whose ParallelHint is not one of the defined hints cannot be registered.
func TestRegisterParallelHint(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("Register accepted ParallelHint = 7")
		}
	}()
	Register(&Language{Name: "bad-parallel-hint", Lex: func(string, int) []Token { return nil }, ParallelHint: 7})
}
