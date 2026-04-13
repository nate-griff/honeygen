# Honeygen — Copilot Instructions

## Build, Test, and Run Commands

### Backend (Go)
```powershell
Set-Location backend
go test ./...                              # full test suite
go test ./internal/api/...                 # single package
go test -run TestGenerationRun ./internal/api/...  # single test
go build ./cmd/api/...                     # build api binary
go build ./cmd/decoy-web/...               # build decoy-web binary
```

### Frontend (TypeScript/React)
```powershell
Set-Location frontend
npm run build       # production build (also validates TypeScript)
npx tsc --noEmit    # type-check without building
npm run dev         # dev server on :5173
```

### Full Stack
```powershell
docker compose up --build   # build and start all three services
docker compose config       # validate compose file
```

---

## Architecture

Three services defined in `docker-compose.yml`:

| Service | Source | Default Port | Role |
|---------|--------|-------------|------|
| `api` | `backend/cmd/api` | 8080 | Go HTTP API — world models, generation jobs, assets, events |
| `admin-web` | `frontend` | 4173 | React/Vite UI served by NGINX; proxies `/api/*` to `api` |
| `decoy-web` | `backend/cmd/decoy-web` | 8081 | Serves generated files; forwards telemetry to `api` |

### Request Flow for Generation

1. `POST /api/generation/run` with `{"world_model_id": "..."}`.
2. API loads the world model from SQLite, runs `generation.Planner.Plan()` to produce a deterministic `[]ManifestEntry`.
3. For each entry: calls external OpenAI-compatible provider → renders output → writes bytes to shared filesystem → records asset metadata in SQLite.
4. `decoy-web` mounts the same volume and serves files directly under `/generated/<world-model-id>/<job-id>/...`.
5. Every non-health decoy request is POSTed to `POST /internal/events` (authenticated with `X-Honeygen-Internal-Event-Token`).
6. Admin UI reads events and status through `/api/events` and `/api/status`.

### Storage

Two Docker named volumes:
- `sqlite-data` → SQLite DB at `/app/storage/sqlite/honeygen.db`
- `generated-assets` → files at `/app/storage/generated`

File paths are stored relative to the storage root, e.g., `generated/northbridge-financial/<job-id>/public/about.html`.

- **Decoy URL**: `/generated/northbridge-financial/<job-id>/public/about.html`
- **Admin download URL**: `/downloads/northbridge-financial/<job-id>/public/about.html` (NGINX strips the leading `generated/`)

### Backend Package Layout (`backend/internal/`)

```
app/           — APIApp struct wiring all dependencies together
api/           — HTTP handlers and router (stdlib net/http, no framework)
config/        — env-based config loading
db/            — SQLite open, migrations, index, status queries
generation/    — Planner, Service, JobStore (orchestrates the full pipeline)
provider/      — Provider interface + OpenAI implementation
rendering/     — Renderers for html, markdown, text, csv, pdf (wkhtmltopdf)
worldmodels/   — CRUD + seed (northbridge-financial seeded on first startup)
assets/        — Repository for asset metadata
events/        — Repository + Service for access events
storage/       — Filesystem abstraction over the generated assets directory
decoy/         — Decoy HTTP server and telemetry forwarding
models/        — Shared API response types (APIErrorResponse, etc.)
```

---

## Key Conventions

### API Error Shape
All API errors return a canonical JSON envelope:
```json
{"error": {"code": "snake_case_code", "message": "human readable"}}
```
Use `writeError(w, status, code, message)` from `internal/api/router.go`; never write ad-hoc error JSON.

### HTTP Router
The API uses only stdlib `net/http` — no external router. Every route is registered in `internal/api/router.go` with `allowMethod()` to enforce a single HTTP verb. Route parameters are extracted manually from `r.URL.Path`.

### Backend Tests
Tests use `httptest.NewRecorder()` + `NewRouter(application)` directly — no test server is started. The helper `newTestAPIApp(t)` creates a fully wired `APIApp` backed by a temp-directory SQLite file. Provider and renderer dependencies are replaced with stubs for unit tests:
```go
application.Provider = generationStubProvider{}
application.Renderers = rendering.NewRegistry(rendering.RegistryConfig{
    PDF: rendering.StaticPDFRenderer([]byte("%PDF-1.4\n%stub\n")),
})
```

### Provider Interface
`provider.Provider` is a small two-method interface (`Generate`, `Test`). Always inject it; never instantiate `openai.go` directly in tests. Use `provider.IsKind(err, provider.KindConfig)` to distinguish error categories.

### Database Migrations
Migrations live in `internal/db/migrations.go`. The pattern is: `CREATE TABLE IF NOT EXISTS` for new tables, then a `schemaUpgrades` slice that applies `ALTER TABLE ADD COLUMN` only if the column doesn't already exist (checked via `PRAGMA table_info`). Add new upgrades to the end of `schemaUpgrades`. All timestamps are stored as ISO 8601 TEXT via SQLite `strftime('%Y-%m-%dT%H:%M:%SZ', 'now')`.

### Frontend API Client
All API calls go through `apiRequest<T>(path, init)` in `frontend/src/api/client.ts`. API errors surface as `APIClientError` with `.status` and `.code`. Leave `VITE_API_BASE_URL` empty for the browser-based admin UI (same-origin NGINX proxy); direct tool calls should target `http://localhost:8080`.

### Generation Planner
`generation.Planner.Plan()` generates a deterministic `[]ManifestEntry` sorted by department/employee/project name. Asset paths follow the pattern `<audience>/<slug>/...`. The asset tree root directories are always: `public`, `intranet`, `shared`, `users`.

### Asset Previewability
Only `html`, `markdown`, `text`, and `csv` rendered types set `Previewable: true`. PDF and other binary types return a metadata-only response from `/api/assets/<id>/content`.

### Internal Event Token
`decoy-web` authenticates to the API using the `X-Honeygen-Internal-Event-Token` header. `api` and `decoy-web` must share the same `INTERNAL_EVENT_INGEST_TOKEN` env var.

### Seeded Demo Data
The world model with id `northbridge-financial` is seeded automatically on first API startup from `sample-data/world-models/northbridge-financial.json`. Tests rely on this seed being present via `worldModelService.EnsureSeedData`.
