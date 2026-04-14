# Architecture

## Runtime shape

The platform runs as three services in `docker-compose.yml`:

- **api** - Go HTTP API for world models, generation jobs, assets, status, provider checks, and event persistence
- **admin-web** - React admin UI built with Vite and served by NGINX
- **decoy-web** - Go HTTP service that serves generated files and forwards non-health access telemetry back to the API

Beyond these three fixed services, the API can spawn additional listeners for deployments — dedicated HTTP file servers, FTP servers, or NFS servers — each bound to its own port within the 9000–9020 range exposed by Docker Compose.

## Request flow

1. A user opens the admin UI at `http://localhost:4173`.
2. The browser-safe default is for the UI to read API data through the admin container's same-origin NGINX `/api/*` proxy. `VITE_API_BASE_URL` only affects the built admin UI; direct tool clients should call the API URL themselves.
3. A generation run posts `world_model_id` to `POST /api/generation/run`.
4. The API:
   - loads the saved world model from SQLite
   - builds a deterministic asset manifest from that model
   - calls the configured external OpenAI-compatible provider once per manifest entry
   - renders content into HTML, Markdown, text, CSV, PDF, DOCX, or XLSX
   - writes files to shared storage
   - records asset and generation metadata in SQLite
5. The decoy service reads the generated files from the shared volume and serves them under `/generated/...`.
6. Every non-health decoy request is posted to `POST /internal/events` with `X-Honeygen-Internal-Event-Token`.
7. The admin UI reads those persisted events through `/api/status` and `/api/events`.

## Generation

The generation planner builds a deterministic asset manifest from the world model. In v1.1, the planner supports **file variation**: a pool of 16 document templates with style hints, driven by a seeded RNG so that repeated runs against the same world model produce diverse but reproducible output. World models can include `generation_settings` with `file_count_target` and `file_count_variance` to control the number of generated files per run.

Rendering now includes **DOCX** (pure Go) and **XLSX** (excelize/v2) alongside the existing HTML, Markdown, text, CSV, and PDF renderers. DOCX and XLSX assets are non-previewable, like PDFs.

## Deployments

Deployments serve generation job output on dedicated ports, managed by an in-process `DeploymentManager` within the API service. Each deployment binds a generation job's file tree to a port using one of the supported protocols:

- **HTTP** — `http.FileServer` serving the job's output directory
- **FTP** — `goftp/server/v2` exposing the files over FTP
- **NFS** — `go-nfs` providing NFSv3 access to the files

The same generation job output can be deployed across multiple protocols simultaneously. Event forwarding to `POST /internal/events` works the same way as `decoy-web`, so all access is captured in the event log.

Docker Compose exposes the port range 9000–9020 by default for deployment listeners.

## Storage

Two named volumes back the stack:

- `sqlite-data` -> SQLite database at `/app/storage/sqlite/honeygen.db`
- `generated-assets` -> generated files under `/app/storage/generated`

The API stores file metadata in SQLite and file bytes on disk. The decoy service and admin container both mount the generated-assets volume read-only.

## Data paths

- persisted file paths are stored relative to the storage root, for example:

  ```text
  generated/northbridge-financial/<job-id>/public/about.html
  ```

- decoy URLs serve those files as:

  ```text
  /generated/northbridge-financial/<job-id>/public/about.html
  ```

- admin download links map the same file tree to `/downloads/...`, stripping the stored leading `generated/` prefix

## Seed/demo model

On first API startup, the world-model service seeds:

- id: `northbridge-financial`
- name: `Northbridge Financial Advisory`
- source file: `sample-data/world-models/northbridge-financial.json`

This gives a teammate a working demo baseline without creating a world model by hand.

## Service boundaries

### API

Owns:

- config loading
- SQLite migrations
- demo seed insertion
- provider test endpoint
- generation orchestration
- asset metadata and preview APIs
- event persistence and querying

### Admin web

Owns:

- dashboard
- world model CRUD UI
- generation controls
- asset tree and safe preview UI
- event log UI

NGINX also proxies `/api/*` to `api` and exposes `/downloads/*` as attachment-only file downloads from the generated asset volume.

### Decoy web

Owns:

- `/healthz`
- landing page at `/`
- direct generated file serving under `/generated/*`
- request telemetry forwarding back to the API

## Operational notes

- `api` and `decoy-web` must share the same `INTERNAL_EVENT_INGEST_TOKEN`.
- Provider mode is `external` only when `LLM_BASE_URL`, `LLM_API_KEY`, and `LLM_MODEL` are all set.
- PDF rendering depends on `wkhtmltopdf` inside the backend image. DOCX and XLSX rendering are pure Go with no external dependencies.
- Persistence survives container restart as long as Compose volumes are kept.
