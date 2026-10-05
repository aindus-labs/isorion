# ISORION HTTP API — v1

JSON-over-HTTP interface served by `isorion serve -addr :8080`
(implementation: `pkg/isorion.Handler`, stdlib `net/http` only).
Stateless compiler: no auth, no persistence — put it behind your own
proxy/mTLS if exposed. Request bodies are capped at 4 MiB.

## Meta

| Route | Response |
|---|---|
| `GET /healthz` | `200` `ok` (plain text) |
| `GET /v1/version` | `{"version","commit","date"}` |
| `GET /v1/targets` | `{"targets": ["ansible","docker-compose",…]}` |
| `GET /v1/importers` | `{"importers": ["ansible","docker-compose",…]}` |

## `POST /v1/compile`

Compile `.isor` source to one or more targets (composite `base+tx` ok).

```json
// request
{ "source": "version: \"1.0\"\nservices: {api: {image: nginx}}",
  "filename": "app.isor",            // optional, diagnostics only
  "targets": ["docker-compose"],
  "profile": "prod",                 // optional
  "set": {"tag": "2.5"} }            // optional --set k=v

// 200
{ "files": [ {"path": "docker-compose.yml", "content": "…"} ],
  "warnings": ["…"] }                // capability + constraint warnings
```

Errors: `400` invalid JSON / missing targets · `422` compile error
(`{"error": "file:line:col: …"}`).

## `POST /v1/import`

Lift an existing infra artifact back to canonical `.isor`.

```json
// request
{ "format": "docker-compose", "data": "services:\n  api: …",
  "filename": "docker-compose.yml" } // optional

// 200
{ "isor": "version: \"1.0\"\n…", "services": ["api"] }
```

Errors: `400` invalid JSON · `422` unknown format / parse error.

## `POST /v1/validate`

Validation verdict is payload, not transport — always `200`:

```json
// request: { "source": "…", "filename": "…", "profile": "…", "set": {} }
// valid   → { "ok": true,  "version": "1.0", "services": ["api"],
//             "warnings": [] }
// invalid → { "ok": false, "error": "file:line:col: model: …" }
```
