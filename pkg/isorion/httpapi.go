package isorion

import (
	"encoding/json"
	"net/http"
	"strings"
)

// Handler returns the read-only-compile HTTP API (level 3 of the
// interface contract): same operations as the CLI, as JSON over HTTP.
// Stdlib only — mount it on your own server or via `isorion serve`.
//
//	GET  /healthz            → "ok"
//	GET  /v1/version         → {"version","commit","date"}
//	GET  /v1/targets         → {"targets":[...]}
//	GET  /v1/importers       → {"importers":[...]}
//	POST /v1/compile         → {source,filename,targets[],profile,set{}} → {files[],warnings[]}
//	POST /v1/import          → {format,filename,data} → {isor}
//	POST /v1/validate        → {source,filename,profile,set{}} → {ok}
func Handler(version, commit, date string) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /v1/version", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"version": version, "commit": commit, "date": date,
		})
	})
	mux.HandleFunc("GET /v1/targets", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"targets": Targets()})
	})
	mux.HandleFunc("GET /v1/importers", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"importers": Importers()})
	})
	mux.HandleFunc("POST /v1/compile", compileEndpoint)
	mux.HandleFunc("POST /v1/import", importEndpoint)
	mux.HandleFunc("POST /v1/validate", validateEndpoint)
	return mux
}

type compileRequest struct {
	Source   string            `json:"source"`
	Filename string            `json:"filename,omitempty"`
	Targets  []string          `json:"targets"`
	Profile  string            `json:"profile,omitempty"`
	Set      map[string]string `json:"set,omitempty"`
}

type compileResponse struct {
	Files    []outFile `json:"files"`
	Warnings []string  `json:"warnings"`
}

type outFile struct {
	Path    string `json:"path"`
	Content string `json:"content"` // UTF-8 artifact text
}

func compileEndpoint(w http.ResponseWriter, r *http.Request) {
	var req compileRequest
	if !decode(w, r, &req) {
		return
	}
	if len(req.Targets) == 0 {
		httpErr(w, http.StatusBadRequest, "targets is required")
		return
	}
	res, err := Compile(nonEmpty(req.Filename, "input.isor"), []byte(req.Source),
		req.Targets, Options{Profile: req.Profile, Set: req.Set})
	if err != nil {
		httpErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	out := compileResponse{Files: []outFile{}, Warnings: res.Warnings}
	for _, f := range res.Files {
		out.Files = append(out.Files, outFile{Path: f.Path, Content: string(f.Content)})
	}
	writeJSON(w, http.StatusOK, out)
}

type importRequest struct {
	Format   string `json:"format"`
	Filename string `json:"filename,omitempty"`
	Data     string `json:"data"`
}

func importEndpoint(w http.ResponseWriter, r *http.Request) {
	var req importRequest
	if !decode(w, r, &req) {
		return
	}
	isor, infra, err := Import(req.Format, []byte(req.Data), nonEmpty(req.Filename, req.Format))
	if err != nil {
		httpErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"isor":     string(isor),
		"services": infra.ServiceNames,
	})
}

type validateRequest struct {
	Source   string            `json:"source"`
	Filename string            `json:"filename,omitempty"`
	Profile  string            `json:"profile,omitempty"`
	Set      map[string]string `json:"set,omitempty"`
}

func validateEndpoint(w http.ResponseWriter, r *http.Request) {
	var req validateRequest
	if !decode(w, r, &req) {
		return
	}
	_, infra, err := Validate(nonEmpty(req.Filename, "input.isor"), []byte(req.Source),
		Options{Profile: req.Profile, Set: req.Set})
	if err != nil {
		// 200 with ok:false — validation outcome is the payload, not transport.
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "version": infra.Version, "services": infra.ServiceNames,
	})
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if r.Body == nil || r.ContentLength == 0 && !strings.Contains(r.Header.Get("Transfer-Encoding"), "chunked") {
		httpErr(w, http.StatusBadRequest, "empty JSON body")
		return false
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20)).Decode(v); err != nil {
		httpErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func httpErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func nonEmpty(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
