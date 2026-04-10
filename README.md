# Honeygen Decoy Research Platform

Honeygen generates a believable decoy document set from a saved world model, serves that content through a decoy web service, and records access events in an admin UI.

## What you can do

- run the full stack with Docker Compose
- use the seeded demo world model (`northbridge-financial`)
- test your external provider connection
- generate decoy assets
- browse generated files and safe previews
- open the decoy service and trigger access events
- confirm those events in the admin UI

## Quick start

1. Copy the example environment file.

   ```powershell
   Copy-Item .env.example .env
   ```

2. Edit `.env` and set at least:

   - `INTERNAL_EVENT_INGEST_TOKEN` to a shared secret used by `api` and `decoy-web`
   - `LLM_BASE_URL` to your external OpenAI-compatible API base URL
   - `LLM_API_KEY` to a valid API key
   - `LLM_MODEL` to the model you want the provider to use

   Notes:

   - For the browser-based admin UI, set `VITE_API_BASE_URL=` so requests use the admin container's same-origin `/api/*` proxy.
   - PowerShell, curl, and other direct API clients should call `http://localhost:8080/...` themselves; `VITE_API_BASE_URL` only affects the built admin UI.

3. Start the stack.

   ```powershell
   docker compose up --build
   ```

4. Open:

   - Admin UI: http://localhost:4173
   - API health: http://localhost:8080/healthz
   - Decoy web: http://localhost:8081

## Demo walkthrough

### 1. Confirm the seeded world model

- Open the admin UI.
- The default demo model is **Northbridge Financial Advisory**.
- The API seeds it automatically on first startup with id `northbridge-financial`.
- Source payload: `sample-data/world-models/northbridge-financial.json`

### 2. Confirm provider readiness

The Generation page shows whether provider settings are configured. Use the API to verify live connectivity and auth:

```powershell
Invoke-RestMethod -Method Post http://localhost:8080/api/provider/test
```

Expected result: `ready: true`, `mode: "external"`.

If the provider is not ready, generation will fail until the `LLM_*` settings are fixed.

### 3. Run generation

From the admin UI:

- go to **Generation**
- keep **Northbridge Financial Advisory** selected
- click **Run generation**

Or call the API directly:

```powershell
Invoke-RestMethod `
  -Method Post `
  -Uri http://localhost:8080/api/generation/run `
  -ContentType 'application/json' `
  -Body '{"world_model_id":"northbridge-financial"}'
```

Expected result:

- the job reaches `completed`
- `summary.manifest_count` and `summary.asset_count` are populated
- the Dashboard and File Browser begin showing generated assets

### 4. Browse generated content

- Open **File Browser** in the admin UI.
- Filter to the latest generation job if needed.
- Select an HTML, Markdown, CSV, or text asset to view inline content.
- Use the download button for binary files such as PDFs.

### 5. Trigger decoy traffic

- Open the decoy landing page at http://localhost:8081
- Click one of the generated links shown on the landing page, or open a generated asset path returned by the API
- The landing page is intentionally simple and may not point at the newest generation job after multiple runs; use the File Browser if you need the latest exact path.
- Example pattern:

  ```text
  http://localhost:8081/generated/northbridge-financial/<generation-job-id>/public/about.html
  ```

### 6. Verify event capture

- Open **Dashboard** to see recent event activity
- Open **Event Log** for full request details
- Or query the API directly:

  ```powershell
  Invoke-RestMethod 'http://localhost:8080/api/events?world_model_id=northbridge-financial&limit=10'
  ```

Expected result:

- a new event appears with a path under `/generated/northbridge-financial/...`
- the Dashboard `recent_events` count increases
- the Event Log shows status code, source IP, path, and request details; `metadata` is usually empty in the default decoy flow

## Persistence check

Generated assets and SQLite data live in named Docker volumes:

- `sqlite-data`
- `generated-assets`

To verify persistence without deleting volumes:

```powershell
docker compose restart
```

After restart:

- the latest generation job should still appear
- generated files should still load from the decoy service
- previously recorded events should still appear in the Dashboard and Event Log

## Configuration reference

### Required for a real demo

- `INTERNAL_EVENT_INGEST_TOKEN`
- `LLM_BASE_URL`
- `LLM_API_KEY`
- `LLM_MODEL`

### Common local defaults

- `API_PORT=8080`
- `ADMIN_WEB_PORT=4173`
- `DECOY_WEB_PORT=8081`
- `SQLITE_PATH=/app/storage/sqlite/honeygen.db`
- `GENERATED_ASSETS_DIR=/app/storage/generated`
- `INTERNAL_API_BASE_URL=http://api:8080`

## Tradeoffs and current limits

- **External provider only:** Honeygen does not ship a built-in local generation provider. For deterministic demos or CI, point `LLM_*` at a deterministic external OpenAI-compatible endpoint or test double.
- **Limited binary preview support:** inline preview is only available for safe text-like assets. PDFs and other binaries are metadata/download only.
- **Reduced PDF fidelity:** PDF output is produced from generated HTML through `wkhtmltopdf`, so advanced CSS/layout fidelity is lower than a full browser print pipeline.

## More docs

- `docs/architecture.md`
- `docs/api.md`
- `docs/data-model.md`
- `docs/demo.md`
