package emit

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

// Golden files: `go test ./pkg/emit -update` regenerates them, CI asserts.
var updateGolden = flag.Bool("update", false, "regenerate golden files")

// TestGolden asserts every emitter is byte-deterministic against the
// recorded golden output for the shared fixture.
func TestGolden(t *testing.T) {
	doc, infra := loadFixture(t)
	for _, name := range Names() {
		files := emitTarget(t, name, doc, infra)
		for _, f := range files {
			gp := filepath.Join("testdata", "golden", name, filepath.Base(f.Path))
			if *updateGolden {
				if err := os.MkdirAll(filepath.Dir(gp), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(gp, f.Content, 0o644); err != nil {
					t.Fatal(err)
				}
				continue
			}
			want, err := os.ReadFile(gp)
			if err != nil {
				t.Errorf("%s: %v (regenerate with `go test ./pkg/emit -update`)", gp, err)
				continue
			}
			if !bytes.Equal(f.Content, want) {
				t.Errorf("%s: output differs from golden (regenerate with `go test ./pkg/emit -update`)\n--- got ---\n%s\n--- want ---\n%s",
					gp, f.Content, want)
			}
		}
	}
}
