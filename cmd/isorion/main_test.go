package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/isorion-lang/isorion/pkg/lang"
)

const fixtureIsor = `version: "1.0"
project: { name: testapp }
services:
  api:
    image: "nginx:1.25"
    replicas: 2
    ports: ["8080:80"]
`

func writeIsor(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "app.isor")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// --- splitArgs -------------------------------------------------------------

func TestSplitArgs(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want []string // joined flags+pos as flagset would see
	}{
		{"flags after positional", []string{"f.isor", "-t", "compose"},
			[]string{"-t", "compose", "f.isor"}},
		{"bool flag does not swallow", []string{"f.isor", "--stdout", "-t", "yaml"},
			[]string{"--stdout", "-t", "yaml", "f.isor"}},
		{"equals form", []string{"f.isor", "-t=yaml"},
			[]string{"-t=yaml", "f.isor"}},
		{"flag value starting with dash is positional", []string{"-o", "x", "f.isor"},
			[]string{"-o", "x", "f.isor"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fa, pa := splitArgs(c.in)
			got := append(append([]string{}, fa...), pa...)
			if strings.Join(got, " ") != strings.Join(c.want, " ") {
				t.Fatalf("got %v, want %v", got, c.want)
			}
		})
	}
}

// --- parseSets -------------------------------------------------------------

func TestParseSets(t *testing.T) {
	m, err := parseSets([]string{"a=1", "$b=x y"})
	if err != nil {
		t.Fatal(err)
	}
	if m["a"] != "1" || m["b"] != "x y" {
		t.Fatalf("unexpected map %v", m)
	}
	if _, err := parseSets([]string{"nope"}); err == nil {
		t.Fatal("expected error for name without '='")
	}
}

// --- cmdValidate / cmdFusion -----------------------------------------------

func TestValidateOK(t *testing.T) {
	dir := t.TempDir()
	if err := cmdValidate([]string{writeIsor(t, dir, fixtureIsor)}); err != nil {
		t.Fatal(err)
	}
}

func TestValidateBadService(t *testing.T) {
	dir := t.TempDir()
	f := writeIsor(t, dir, `version: "1.0"
services:
  api: { replicas: 2 }
`)
	err := cmdValidate([]string{f})
	if err == nil || !strings.Contains(err.Error(), "image or build") {
		t.Fatalf("expected model error, got %v", err)
	}
	if !strings.Contains(err.Error(), f) {
		t.Fatalf("error should carry the file position, got %v", err)
	}
}

func TestFusionWritesFiles(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	f := writeIsor(t, dir, fixtureIsor)
	if err := cmdFusion([]string{f, "-o", out, "-t", "docker-compose"}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(out, "docker-compose", "docker-compose.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "nginx:1.25") || !strings.Contains(string(b), "8080:80") {
		t.Fatalf("unexpected compose output:\n%s", b)
	}
}

func TestFusionStdoutRequiresOneTarget(t *testing.T) {
	dir := t.TempDir()
	f := writeIsor(t, dir, fixtureIsor)
	err := cmdFusion([]string{f, "-t", "yaml,docker-compose", "--stdout"})
	if err == nil || !strings.Contains(err.Error(), "exactly one target") {
		t.Fatalf("expected single-target error, got %v", err)
	}
}

func TestFusionUnknownTarget(t *testing.T) {
	dir := t.TempDir()
	f := writeIsor(t, dir, fixtureIsor)
	err := cmdFusion([]string{f, "-o", filepath.Join(dir, "o"), "-t", "nope"})
	if err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("expected unknown target error, got %v", err)
	}
}

func TestFusionSetAndProfile(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	f := writeIsor(t, dir, `version: "1.0"
variables: { tag: "1.0" }
services:
  api: { image: "nginx:${tag}" }
`)
	if err := cmdFusion([]string{f, "-o", out, "-t", "yaml", "--set", "tag=2.5"}); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(out, "yaml", "resolved.yaml"))
	if !strings.Contains(string(b), "nginx:2.5") {
		t.Fatalf("--set not applied:\n%s", b)
	}
}

// --- cmdDiff ----------------------------------------------------------------

func TestDiffDetectsDrift(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	f := writeIsor(t, dir, fixtureIsor)
	if err := cmdFusion([]string{f, "-o", out, "-t", "yaml"}); err != nil {
		t.Fatal(err)
	}
	// identical output -> no difference
	if err := cmdDiff([]string{f, "-o", out, "-t", "yaml"}); err != nil {
		t.Fatalf("expected no diff, got %v", err)
	}
	// drift -> errDiffer
	p := filepath.Join(out, "yaml", "resolved.yaml")
	b, _ := os.ReadFile(p)
	os.WriteFile(p, append(b, "# drift\n"...), 0o644)
	if err := cmdDiff([]string{f, "-o", out, "-t", "yaml"}); !errors.Is(err, errDiffer) {
		t.Fatalf("expected errDiffer, got %v", err)
	}
	// missing output -> errDiffer (new files)
	if err := cmdDiff([]string{f, "-o", filepath.Join(dir, "none"), "-t", "yaml"}); !errors.Is(err, errDiffer) {
		t.Fatalf("expected errDiffer for new files, got %v", err)
	}
}

func TestDiffHelpers(t *testing.T) {
	if n := countLines([]byte("a\nb\n")); n != 3 {
		t.Fatalf("countLines = %d, want 3", n)
	}
	add, del := diffStats([]byte("x\ny\nz"), []byte("x\nw\nz"))
	if add != 1 || del != 1 {
		t.Fatalf("diffStats = +%d/-%d, want +1/-1", add, del)
	}
	d := unifiedDiff("f", []byte("a\nb"), []byte("a\nc"))
	if !strings.Contains(d, "-b") || !strings.Contains(d, "+c") {
		t.Fatalf("bad unified diff:\n%s", d)
	}
}

// --- cmdFmt -----------------------------------------------------------------

func TestFmtCanonicalizes(t *testing.T) {
	dir := t.TempDir()
	f := writeIsor(t, dir, `services:
    api:
        image: nginx
version: "1.0"
`)
	if err := cmdFmt([]string{f, "-w"}); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(f)
	s := string(got)
	if !strings.HasPrefix(s, "version:") {
		t.Fatalf("version should be first:\n%s", s)
	}
	if strings.Contains(s, "    api") { // 4-space indent gone
		t.Fatalf("indent not normalized:\n%s", s)
	}
	// idempotent
	if err := cmdFmt([]string{f, "-w"}); err != nil {
		t.Fatal(err)
	}
}

func TestFmtUnquotesDirectives(t *testing.T) {
	in := `"@if $env == \"prod\"":
  replicas: 3`
	got := unquoteKeys(in)
	if strings.Contains(got, `"@if`) {
		t.Fatalf("directive key still quoted: %q", got)
	}
	if !strings.Contains(got, `@if $env == "prod":`) {
		t.Fatalf("unexpected output: %q", got)
	}
}

// --- posify -----------------------------------------------------------------

func TestPosifyPositions(t *testing.T) {
	doc, err := lang.LoadWith("x.isor", []byte(fixtureIsor), lang.Options{})
	if err != nil {
		t.Fatal(err)
	}
	err = posify(doc, errors.New("services.api: bad"))
	if err == nil || !strings.Contains(err.Error(), "x.isor:") {
		t.Fatalf("expected positioned error, got %v", err)
	}
	if got := posify(doc, errors.New("unrelated")).Error(); !strings.HasPrefix(got, "model: ") {
		t.Fatalf("expected model: prefix, got %q", got)
	}
}
