# Honeygen Runtime Skeleton

This worktree bootstraps the Decoy Research Platform monorepo with three Docker Compose services:

- `api` - Go API runtime with Debian-based packaging and `wkhtmltopdf`
- `admin-web` - React + TypeScript admin UI built with Vite and served by NGINX
- `decoy-web` - Go-based decoy web runtime placeholder

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

## Validation

```powershell
docker compose config
docker compose build
```
