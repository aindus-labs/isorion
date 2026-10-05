package lang

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func evalStr(t *testing.T, expr string, env map[string]Value) Value {
	t.Helper()
	v, err := EvalExpr(expr, env)
	if err != nil {
		t.Fatalf("EvalExpr(%q): %v", expr, err)
	}
	return v
}

func TestExprBasics(t *testing.T) {
	env := map[string]Value{"env": "production", "n": int64(3), "ports": []any{"a", "b"}}

	cases := []struct {
		expr string
		want Value
	}{
		{`$env == "production"`, true},
		{`$env == "staging"`, false},
		{`$env != "staging"`, true},
		{`$n > 2`, true},
		{`$n >= 3`, true},
		{`$n < 2`, false},
		{`$n + 1`, int64(4)},
		{`$n * 2`, int64(6)},
		{`2 + 2`, int64(4)},
		{`"a" + "b"`, "ab"},
		{`$env == "production" && $n > 1`, true},
		{`$env == "x" || $n > 1`, true},
		{`!($env == "x")`, true},
		{`len($ports)`, int64(2)},
		{`upper("abc")`, "ABC"},
		{`default(null, "d")`, "d"},
		{`default("v", "d")`, "v"},
		{`$mode == fast`, false}, // bare ident = string literal, $mode undefined → error expected below
	}
	for _, c := range cases[:len(cases)-1] {
		got := evalStr(t, c.expr, env)
		if got != c.want {
			t.Errorf("%q = %v, want %v", c.expr, got, c.want)
		}
	}
	// undefined variable must error
	if _, err := EvalExpr(cases[len(cases)-1].expr, env); err == nil {
		t.Errorf("expected error for undefined variable")
	}
}

func TestIfElseMapping(t *testing.T) {
	src := []byte(`
variables:
  env: "production"
services:
  api:
    image: nginx:1.25
    @if $env == "production":
      replicas: 5
    @else:
      replicas: 1
`)
	doc, err := Load("test.isor", src)
	if err != nil {
		t.Fatal(err)
	}
	services := doc.Body["services"].(map[string]Value)
	api := services["api"].(map[string]Value)
	if api["replicas"] != int64(5) {
		t.Fatalf("replicas = %v, want 5", api["replicas"])
	}
}

func TestElseBranch(t *testing.T) {
	src := []byte(`
variables:
  env: "dev"
a:
  @if $env == "production":
    x: 1
  @elif $env == "staging":
    x: 2
  @else:
    x: 3
`)
	doc, err := Load("t.isor", src)
	if err != nil {
		t.Fatal(err)
	}
	a := doc.Body["a"].(map[string]Value)
	if a["x"] != int64(3) {
		t.Fatalf("x = %v, want 3", a["x"])
	}
}

func TestForMapping(t *testing.T) {
	src := []byte(`
variables:
  regions: ["eu", "us"]
services:
  @for $r in $regions:
    "api-${r}":
      image: img:${r}
`)
	doc, err := Load("t.isor", src)
	if err != nil {
		t.Fatal(err)
	}
	services := doc.Body["services"].(map[string]Value)
	if _, ok := services["api-eu"]; !ok {
		t.Fatal("api-eu missing")
	}
	us := services["api-us"].(map[string]Value)
	if us["image"] != "img:us" {
		t.Fatalf("image = %v", us["image"])
	}
}

func TestForSequence(t *testing.T) {
	src := []byte(`
variables:
  ports: [80, 443]
list:
  - "first"
  - @for $p in $ports:
      - "port-${p}"
`)
	doc, err := Load("t.isor", src)
	if err != nil {
		t.Fatal(err)
	}
	list := doc.Body["list"].([]any)
	if len(list) != 3 || list[1] != "port-80" || list[2] != "port-443" {
		t.Fatalf("list = %v", list)
	}
}

func TestInterpolation(t *testing.T) {
	src := []byte(`
variables:
  host: "example.com"
  n: 3
a:
  url: "https://${host}/api"
  count: ${$n * 2}
  raw: $n
  esc: "100$$off"
`)
	doc, err := Load("t.isor", src)
	if err != nil {
		t.Fatal(err)
	}
	a := doc.Body["a"].(map[string]Value)
	if a["url"] != "https://example.com/api" {
		t.Fatalf("url = %v", a["url"])
	}
	if a["count"] != int64(6) {
		t.Fatalf("count = %v", a["count"])
	}
	if a["raw"] != int64(3) {
		t.Fatalf("raw = %v", a["raw"])
	}
	if a["esc"] != "100$off" {
		t.Fatalf("esc = %v", a["esc"])
	}
}

func TestSequenceIf(t *testing.T) {
	src := []byte(`
variables:
  tls: true
ports:
  - "80:80"
  - @if $tls:
      - "443:443"
`)
	doc, err := Load("t.isor", src)
	if err != nil {
		t.Fatal(err)
	}
	ports := doc.Body["ports"].([]any)
	if len(ports) != 2 || ports[1] != "443:443" {
		t.Fatalf("ports = %v", ports)
	}
}

func TestProfile(t *testing.T) {
	src := []byte(`variables:
  env: "dev"
  replicas: 1
profiles:
  prod:
    env: "production"
    replicas: ${$replicas + 4}
app:
  name: "${env}-svc"
  replicas: $replicas
`)
	doc, err := LoadWith("t.isor", src, Options{Profile: "prod"})
	if err != nil {
		t.Fatal(err)
	}
	app := doc.Body["app"].(map[string]Value)
	if app["name"] != "production-svc" {
		t.Fatalf("name = %v", app["name"])
	}
	if app["replicas"] != int64(5) {
		t.Fatalf("replicas = %v", app["replicas"])
	}
	if doc.Profile != "prod" {
		t.Fatalf("profile = %q", doc.Profile)
	}
}

func TestProfileUnknown(t *testing.T) {
	src := []byte("profiles:\n  dev:\n    x: 1\napp: {}\n")
	_, err := LoadWith("t.isor", src, Options{Profile: "nope"})
	if err == nil || !strings.Contains(err.Error(), `unknown profile "nope"`) ||
		!strings.Contains(err.Error(), "available: dev") {
		t.Fatalf("err = %v", err)
	}
}

func TestSetOverrides(t *testing.T) {
	src := []byte(`variables:
  env: "dev"
  n: 1
app:
  name: "${env}"
  count: $n
`)
	doc, err := LoadWith("t.isor", src, Options{Set: map[string]string{"env": "staging", "n": "7"}})
	if err != nil {
		t.Fatal(err)
	}
	app := doc.Body["app"].(map[string]Value)
	if app["name"] != "staging" {
		t.Fatalf("name = %v", app["name"])
	}
	// --set literals are YAML-typed: "7" → int64(7)
	if app["count"] != int64(7) && app["count"] != 7 {
		t.Fatalf("count = %v (%T)", app["count"], app["count"])
	}
}

func TestErrorLineColumn(t *testing.T) {
	//         1: variables:
	//         2:   env: "dev"
	//         3: app:
	//         4:   image: "$missing"
	src := []byte("variables:\n  env: \"dev\"\napp:\n  image: \"$missing\"\n")
	_, err := Load("bad.isor", src)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "bad.isor:4:3:") || !strings.Contains(msg, "undefined variable $missing") {
		t.Fatalf("err = %q", msg)
	}
	var le *LineError
	if !errors.As(err, &le) || le.Pos.Line != 4 {
		t.Fatalf("LineError not preserved: %v", err)
	}
}

func TestDirectiveErrorLine(t *testing.T) {
	src := []byte("app:\n  a: 1\n  @elif $x:\n    b: 2\n")
	_, err := Load("bad2.isor", src)
	if err == nil || !strings.Contains(err.Error(), "bad2.isor:3:3:") {
		t.Fatalf("err = %v", err)
	}
}

func TestHiddenFields(t *testing.T) {
	src := []byte(`app:
  _draft: "internal"
  name: "x"
  deep:
    _tmp: 1
    visible: 2
`)
	doc, err := Load("t.isor", src)
	if err != nil {
		t.Fatal(err)
	}
	app := doc.Body["app"].(map[string]Value)
	if _, ok := app["_draft"]; ok {
		t.Fatal("_draft leaked into body")
	}
	deep := app["deep"].(map[string]Value)
	if _, ok := deep["_tmp"]; ok || deep["visible"] != int64(2) {
		t.Fatalf("deep = %v", deep)
	}
}

func TestVarConstraint(t *testing.T) {
	src := []byte(`variables:
  env: "prod"
  "env!": "in: dev|staging|production"
  n: 5
  "n!": "$it >= 1 && $it <= 10"
x: $env
`)
	// union check fails: "prod" not in list
	if _, err := Load("t.isor", src); err == nil || !strings.Contains(err.Error(), "env = prod not in") {
		t.Fatalf("expected union violation, err = %v", err)
	}
	// fix env, keep n! — should pass
	src2 := strings.Replace(string(src), `env: "prod"`, `env: "staging"`, 1)
	if _, err := Load("t.isor", []byte(src2)); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	// n=50 violates range
	src3 := strings.Replace(src2, "n: 5", "n: 50", 1)
	if _, err := Load("t.isor", []byte(src3)); err == nil || !strings.Contains(err.Error(), "constraint violated") {
		t.Fatalf("expected expr violation, err = %v", err)
	}
}

func TestSecrets(t *testing.T) {
	src := []byte(`secrets:
  db_password: "ENC[age:xyz]"
services:
  api:
    env:
      PASS: ${secret("db_password")}
      PLAIN: "x"
`)
	doc, err := Load("t.isor", src)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Secrets["db_password"] != "ENC[age:xyz]" {
		t.Fatalf("secrets = %v", doc.Secrets)
	}
	env := doc.Body["services"].(map[string]Value)["api"].(map[string]Value)["env"].(map[string]Value)
	name, ok := SecretName(env["PASS"])
	if !ok || name != "db_password" {
		t.Fatalf("PASS = %v", env["PASS"])
	}
	if env["PLAIN"] != "x" {
		t.Fatalf("PLAIN = %v", env["PLAIN"])
	}
	// plaintext rejected
	bad := []byte("secrets:\n  k: \"hunter2\"\nx: 1\n")
	if _, err := Load("t.isor", bad); err == nil || !strings.Contains(err.Error(), "ENC[") {
		t.Fatalf("expected ENC rejection, err = %v", err)
	}
	// undeclared secret rejected
	bad2 := []byte("x: ${secret(\"nope\")}\n")
	if _, err := Load("t.isor", bad2); err == nil || !strings.Contains(err.Error(), "undefined secret") {
		t.Fatalf("expected undefined secret, err = %v", err)
	}
}

func TestInclude(t *testing.T) {
	dir := t.TempDir()
	frag := `variables:
  frag_var: "from-frag"
frag_section:
  val: $frag_var
  parent_var: $parent
`
	os.WriteFile(filepath.Join(dir, "frag.isor"), []byte(frag), 0o644)
	main := `variables:
  parent: "from-parent"
app:
  @include frag.isor:
`
	doc, err := Load(filepath.Join(dir, "main.isor"), []byte(main))
	if err != nil {
		t.Fatal(err)
	}
	app := doc.Body["app"].(map[string]Value)
	frag2 := app["frag_section"].(map[string]Value)
	if frag2["val"] != "from-frag" || frag2["parent_var"] != "from-parent" {
		t.Fatalf("frag_section = %v", frag2)
	}
}

func TestIncludeSequence(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "items.yaml"), []byte("- a\n- b\n"), 0o644)
	main := "list:\n  - first\n  - @include items.yaml:\n  - last\n"
	doc, err := Load(filepath.Join(dir, "main.isor"), []byte(main))
	if err != nil {
		t.Fatal(err)
	}
	list := doc.Body["list"].([]any)
	if len(list) != 4 || list[0] != "first" || list[2] != "b" || list[3] != "last" {
		t.Fatalf("list = %v", list)
	}
}

func TestIncludeCycle(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.isor"), []byte("app:\n  @include b.isor:\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "b.isor"), []byte("app:\n  @include a.isor:\n"), 0o644)
	_, err := Load(filepath.Join(dir, "a.isor"), []byte("app:\n  @include b.isor:\n"))
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("expected cycle error, got %v", err)
	}
}

func TestIncludeSHA(t *testing.T) {
	dir := t.TempDir()
	frag := []byte("x: 1\n")
	os.WriteFile(filepath.Join(dir, "f.isor"), frag, 0o644)
	sum := sha256.Sum256(frag)
	good := fmt.Sprintf("app:\n  @include f.isor @sha256 %s:\n", hex.EncodeToString(sum[:]))
	if _, err := Load(filepath.Join(dir, "m.isor"), []byte(good)); err != nil {
		t.Fatalf("valid pin rejected: %v", err)
	}
	bad := fmt.Sprintf("app:\n  @include f.isor @sha256 %064x:\n", 0)
	if _, err := Load(filepath.Join(dir, "m.isor"), []byte(bad)); err == nil || !strings.Contains(err.Error(), "sha256 mismatch") {
		t.Fatalf("expected sha mismatch, got %v", err)
	}
}

func TestExtends(t *testing.T) {
	dir := t.TempDir()
	base := `version: "1.0"
services:
  api:
    image: nginx
    replicas: 3
    env:
      A: 1
      B: 2
`
	os.WriteFile(filepath.Join(dir, "base.isor"), []byte(base), 0o644)
	child := `@extends base.isor:
services:
  api:
    replicas: 7
    env:
      B: 20
`
	doc, err := Load(filepath.Join(dir, "child.isor"), []byte(child))
	if err != nil {
		t.Fatal(err)
	}
	if doc.Version != "1.0" {
		t.Fatalf("version not inherited: %q", doc.Version)
	}
	api := doc.Body["services"].(map[string]Value)["api"].(map[string]Value)
	if api["image"] != "nginx" || api["replicas"] != int64(7) {
		t.Fatalf("api = %v", api)
	}
	env := api["env"].(map[string]Value)
	if env["A"] != int64(1) || env["B"] != int64(20) {
		t.Fatalf("env deep-merge = %v", env)
	}
}
