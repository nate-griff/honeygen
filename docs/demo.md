# Demo Guide

This is the shortest reliable path from clone to proof that the platform works end to end.

## Prerequisites

- Docker Desktop
- an external OpenAI-compatible provider endpoint
- a provider API key and model name

## 1. Prepare configuration

```powershell
Copy-Item .env.example .env
```

Update `.env`:

- set `INTERNAL_EVENT_INGEST_TOKEN` to any non-empty shared secret
- set `LLM_BASE_URL`
- set `LLM_API_KEY`
- set `LLM_MODEL`

Optional:

- set `VITE_API_BASE_URL=` for the browser-based admin demo flow
- `VITE_API_BASE_URL` only affects the admin UI bundle; direct API calls from tools should target `http://localhost:8080` explicitly

## 2. Start the stack

```powershell
docker compose up --build
```

Wait for:

- API health at `http://localhost:8080/healthz`
- admin UI at `http://localhost:4173`
- decoy landing page at `http://localhost:8081`

## 3. Verify the demo model exists

```powershell
Invoke-RestMethod http://localhost:8080/api/world-models
```

Look for:

- `id: "northbridge-financial"`
- `name: "Northbridge Financial Advisory"`

## 4. Verify provider connectivity

```powershell
Invoke-RestMethod -Method Post http://localhost:8080/api/provider/test
```

Expected:

- `ready` is `true`
- `mode` is `external`

## 5. Generate assets

Admin UI path:

- open **Generation**
- select **Northbridge Financial Advisory**
- click **Run generation**

API path:

```powershell
Invoke-RestMethod `
  -Method Post `
  -Uri http://localhost:8080/api/generation/run `
  -ContentType 'application/json' `
  -Body '{"world_model_id":"northbridge-financial"}'
```

Expected:

- job status becomes `completed`
- `summary.asset_count` is greater than `0`

## 6. Inspect generated files

- open **File Browser**
- choose the latest generation job
- click an HTML, Markdown, CSV, or text asset for inline preview
- click a PDF asset and confirm it is download-only

Useful API checks:

```powershell
Invoke-RestMethod 'http://localhost:8080/api/assets?world_model_id=northbridge-financial&limit=10'
Invoke-RestMethod 'http://localhost:8080/api/assets/tree?world_model_id=northbridge-financial'
```

## 7. Trigger decoy events

- open `http://localhost:8081`
- click one of the generated links shown on the landing page
- if you have run generation multiple times, prefer the File Browser or `/api/assets` to find the newest job-specific path

Or fetch a generated asset directly:

```text
http://localhost:8081/generated/northbridge-financial/<generation-job-id>/public/about.html
```

## 8. Verify event capture

In the admin UI:

- **Dashboard** should show recent activity
- **Event Log** should show the request path and request details; `metadata` is usually empty in the default decoy flow

In the API:

```powershell
Invoke-RestMethod 'http://localhost:8080/api/status'
Invoke-RestMethod 'http://localhost:8080/api/events?world_model_id=northbridge-financial&limit=10'
```

Expected:

- `counts.recent_events` increases
- an event exists for `/generated/northbridge-financial/...`

## 9. Verify persistence

Restart containers without removing volumes:

```powershell
docker compose restart
```

Then confirm:

- the latest generation job still exists
- generated asset URLs still return `200`
- event history is still present

## Demo tradeoffs to call out

- binary assets, especially PDFs, are download-only in the admin preview flow
- deterministic demos require a deterministic external OpenAI-compatible endpoint; the app does not include a built-in local fallback provider
- PDF output is serviceable for decoy content, but fidelity is limited by the `wkhtmltopdf` conversion step
