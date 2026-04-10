# Honeygen Runtime Skeleton

This worktree bootstraps the Decoy Research Platform monorepo with three Docker Compose services:

- `api` - Go API runtime with Debian-based packaging and `wkhtmltopdf`
- `admin-web` - React + TypeScript admin UI built with Vite and served by NGINX
- `decoy-web` - Go-based decoy web runtime placeholder with a lean Debian runtime

## Repository layout

- `backend/` - Go module, API/decoy-web entrypoints, and multi-target Dockerfile
- `frontend/` - Vite-based React admin shell and static-serving Dockerfile
- `docs/architecture.md` - High-level runtime architecture summary

## Environment

Copy `.env.example` to `.env` to override defaults.

## Run locally

```powershell
docker compose up --build
```

Published ports:

- API: `http://localhost:8080`
- Admin UI: `http://localhost:4173`
- Decoy web: `http://localhost:8081`

Mounted named volumes:

- `sqlite-data` -> SQLite database files
- `generated-assets` -> generated PDFs and other exported assets

## API runtime notes

- The API reads defaults, then an optional JSON config file from `CONFIG_PATH`, then environment variable overrides.
- Docker Compose mounts `backend/config/` to `/app/config`; place `backend/config/config.json` there to use the default `CONFIG_PATH`.
- Health endpoints:
  - `GET /healthz` -> plain-text container healthcheck response
  - `GET /api/health` -> JSON service health summary
  - `GET /api/status` -> JSON dashboard summary with database readiness, provider mode, counts, and latest job info
  - `GET /api/world-models` -> JSON summaries for saved world models
  - `POST /api/world-models` -> create a saved world model
  - `GET /api/world-models/:id` -> fetch a saved world model
  - `PUT /api/world-models/:id` -> update a saved world model
  - `POST /api/generation/run` -> synchronously plan and generate assets for a saved world model
  - `GET /api/generation/jobs` -> list persisted generation jobs
  - `GET /api/generation/jobs/:id` -> fetch one generation job with persisted logs/summary
  - `GET /api/assets` -> list generated assets with simple filters and pagination
  - `GET /api/assets/tree` -> browse generated assets as a directory tree
  - `GET /api/assets/:id` -> fetch one asset's metadata
  - `GET /api/assets/:id/content` -> return inline preview content for safe text/html/markdown/csv assets
- First-run API startup seeds the `Northbridge Financial Advisory` demo model.
- The matching sample payload lives at `sample-data/world-models/northbridge-financial.json`.

## Validation

```powershell
docker compose config
docker compose build
Set-Location backend
go test ./...
```
