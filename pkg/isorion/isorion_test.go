package isorion

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const fixture = `version: "1.0"
project: { name: webapp }
services:
  api:
    image: "nginx:1.25"
    ports: ["80:8080"]
    environment: { APP_ENV: prod }
  db:
    image: "postgres:16"
`

func TestCompile(t *testing.T) {
	res, err := Compile("app.isor", []byte(fixture), []string{"docker-compose", "yaml"}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Files) != 2 {
		t.Fatalf("want 2 files, got %d", len(res.Files))
	}
	if !strings.Contains(string(res.Files[0].Content), "nginx:1.25") {
		t.Fatalf("compose artifact:\n%s", res.Files[0].Content)
	}
}

func TestCompileBadSource(t *testing.T) {
	if _, err := Compile("bad.isor", []byte("services: ["), []string{"yaml"}, Options{}); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestValidate(t *testing.T) {
	_, infra, err := Validate("app.isor", []byte(fixture), Options{})
	if err != nil || infra.ProjectName != "webapp" {
		t.Fatalf("infra=%+v err=%v", infra, err)
	}
}

func TestImport(t *testing.T) {
	compose := "services:\n  api:\n    image: nginx:1.25\n    ports:\n      - \"80:8080\"\n"
	isor, infra, err := Import("docker-compose", []byte(compose), "docker-compose.yml")
	if err != nil {
		t.Fatal(err)
	}
	if len(infra.Services) != 1 || !strings.Contains(string(isor), "api") {
		t.Fatalf("isor doc:\n%s", isor)
	}
	// canonical .isor re-validates
	if _, _, err := Validate("out.isor", isor, Options{}); err != nil {
		t.Fatalf("imported .isor does not re-validate: %v", err)
	}
}

// ---- HTTP API (level 3) ----

func apiServer(t *testing.T) *httptest.Server {
	t.Helper()
	s := httptest.NewServer(Handler("0.1.0", "test", "today"))
	t.Cleanup(s.Close)
	return s
}

func post(t *testing.T, s *httptest.Server, path, body string) (int, map[string]any) {
	t.Helper()
	res, err := http.Post(s.URL+path, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(res.Body).Decode(&out)
	return res.StatusCode, out
}

func TestHTTPMeta(t *testing.T) {
	s := apiServer(t)
	for _, p := range []string{"/healthz", "/v1/version", "/v1/targets", "/v1/importers"} {
		res, err := http.Get(s.URL + p)
		if err != nil || res.StatusCode != 200 {
			t.Fatalf("%s: %v code=%d", p, err, res.StatusCode)
		}
		res.Body.Close()
	}
}

func TestHTTPCompile(t *testing.T) {
	s := apiServer(t)
	payload, _ := json.Marshal(compileRequest{Source: fixture, Targets: []string{"docker-compose"}})
	code, out := post(t, s, "/v1/compile", string(payload))
	if code != 200 || len(out["files"].([]any)) != 1 {
		t.Fatalf("code=%d out=%v", code, out)
	}
}

func TestHTTPCompileBadTarget(t *testing.T) {
	s := apiServer(t)
	payload, _ := json.Marshal(compileRequest{Source: fixture, Targets: []string{"nope"}})
	code, out := post(t, s, "/v1/compile", string(payload))
	if code != 422 || out["error"] == nil {
		t.Fatalf("code=%d out=%v", code, out)
	}
}

func TestHTTPValidate(t *testing.T) {
	s := apiServer(t)
	payload, _ := json.Marshal(validateRequest{Source: fixture})
	code, out := post(t, s, "/v1/validate", string(payload))
	if code != 200 || out["ok"] != true {
		t.Fatalf("code=%d out=%v", code, out)
	}
	payload, _ = json.Marshal(validateRequest{Source: "services:\n  api: {}\n"})
	code, out = post(t, s, "/v1/validate", string(payload))
	if code != 200 || out["ok"] != false || out["error"] == nil {
		t.Fatalf("bad doc: code=%d out=%v", code, out)
	}
}

func TestHTTPImport(t *testing.T) {
	s := apiServer(t)
	payload, _ := json.Marshal(importRequest{
		Format: "docker-compose",
		Data:   "services:\n  api:\n    image: nginx\n",
	})
	code, out := post(t, s, "/v1/import", string(payload))
	if code != 200 || !strings.Contains(out["isor"].(string), "api") {
		t.Fatalf("code=%d out=%v", code, out)
	}
}
