package eval

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Only the evaluation may read the truth: no serving package mentions eval/truth.
func TestServingCodeNeverReadsTruth(t *testing.T) {
	for _, dir := range []string{"index", "drift", "rank", "pack", "mcp", "baseline", "words"} {
		files, _ := filepath.Glob(filepath.Join("..", dir, "*.go"))
		for _, f := range files {
			if strings.HasSuffix(f, "_test.go") {
				continue
			}
			b, _ := os.ReadFile(f)
			if strings.Contains(string(b), "eval/truth") || strings.Contains(string(b), "truth/") {
				t.Errorf("%s refers to the truth", f)
			}
		}
	}
}

func TestHeldoutLock(t *testing.T) {
	if err := CheckLock("../../eval/tasks/heldout.json", "../../config/heldout.lock"); err != nil {
		t.Fatal(err)
	}
}
