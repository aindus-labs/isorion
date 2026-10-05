package parse

import (
	"os"
	"path/filepath"
	"testing"
)

// FuzzImport feeds arbitrary bytes to every registered importer. The
// contract being tested: no importer may panic — malformed input must
// come back as an error, not a crash. Seeds come from the emitter
// goldens so the corpus covers every emitted shape.
func FuzzImport(f *testing.F) {
	// golden paths live under pkg/emit/testdata — resolve relatively.
	goldenRoot := filepath.Join("..", "emit", "testdata", "golden")
	goldens := map[string]string{
		"compose.yml":     filepath.Join(goldenRoot, "docker-compose", "docker-compose.yml"),
		"deployment.yaml": filepath.Join(goldenRoot, "kubernetes", "api-deployment.yaml"),
		"service.yaml":    filepath.Join(goldenRoot, "kubernetes", "api-service.yaml"),
		"playbook.yml":    filepath.Join(goldenRoot, "ansible", "playbook.yml"),
		"values.yaml":     filepath.Join(goldenRoot, "helm", "values.yaml"),
		"chart.yaml":      filepath.Join(goldenRoot, "helm", "Chart.yaml"),
		"main.tf":         filepath.Join(goldenRoot, "terraform", "main.tf"),
		"unit.service":    filepath.Join(goldenRoot, "systemd", "webapp-api.service"),
		"huborion.isor":   filepath.Join(goldenRoot, "huborion", "webapp.huborion.isor"),
		"resolved.yaml":   filepath.Join(goldenRoot, "yaml", "resolved.yaml"),
	}
	for name, path := range goldens {
		data, err := os.ReadFile(path)
		if err != nil {
			f.Fatalf("seed %s (%s): %v", name, path, err)
		}
		f.Add(name, data)
	}
	// degenerate seeds
	f.Add("empty", []byte{})
	f.Add("junk", []byte("\x00\xff\xfe{{{ not yaml"))
	f.Add("truncated", []byte("services:\n  api:\n    image: \"ng"))

	f.Fuzz(func(t *testing.T, filename string, data []byte) {
		for _, name := range Names() {
			imp, err := Get(name)
			if err != nil {
				t.Fatalf("Get(%q): %v", name, err)
			}
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("importer %q panicked on %q: %v", name, filename, r)
					}
				}()
				_, _ = imp.Parse(data, filename) // errors are fine; panics are not
			}()
		}
	})
}
