# Honeygen Decoy Research Platform

Honeygen generates a believable decoy document set from a saved world model, serves that content through a decoy web service, and records access events in an admin UI.

## Dependencies

| Area | Dependency | Why it is needed |
| --- | --- | --- |
| Container workflow | Docker Engine / Docker Desktop with Docker Compose v2 | Recommended way to run the full stack locally |
| Backend local development | Go 1.24+ | Required by `backend/go.mod`; `github.com/xuri/excelize/v2` now requires Go 1.24 |
| Frontend local development | Node.js and npm | Required to build and run the Vite/React admin UI from `frontend\` |
| Database | SQLite via `modernc.org/sqlite` | Persistent storage for world models, jobs, assets, events, and deployments |
| Backend Go module deps | `github.com/google/uuid`, `github.com/xuri/excelize/v2`, `modernc.org/sqlite`, `goftp.io/server/v2`, `github.com/willscott/go-nfs` | IDs, XLSX rendering, SQLite storage, FTP deployments, and NFS deployments |
| PDF rendering | `wkhtmltopdf` | Required to render PDF assets from generated HTML |
| SMB deployments | Samba `smbd` | Required only for `protocol: "smb"` deployments; Honeygen manages `smbd` as a subprocess and exposes the `honeygen` guest share |
| Frontend app deps | React, React Router, Vite, TypeScript, `marked`, `dompurify` | Admin UI, routing, markdown rendering, and safe HTML sanitization |

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
    - Deployment ports: `9000-9020` are exposed by default for HTTP, FTP, NFS, and SMB deployments created from the UI
    - FTP passive data ports: `9011-9020` are reserved inside the default deployment port range so FTP listings and downloads work through Docker

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
- Use the download button for binary files such as PDFs, DOCX files, and XLSX files.

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
- `DEPLOYMENT_PORTS=9000-9020`
- `FTP_PUBLIC_HOST=127.0.0.1`
- `FTP_PASSIVE_PORTS=9011-9020`
- `SQLITE_PATH=/app/storage/sqlite/honeygen.db`
- `GENERATED_ASSETS_DIR=/app/storage/generated`
- `INTERNAL_API_BASE_URL=http://api:8080`

## Deployment protocol quick reference

These commands were validated against local deployments on ports `9001` (SMB), `9002` (FTP), and `9003` (NFS).

| Protocol | Tested access pattern | Notes |
| --- | --- | --- |
| SMB | `smbclient //127.0.0.1/honeygen -p 9001 -N -c 'ls'` | Share name is always `honeygen`; guest read-only access |
| FTP | `curl.exe --user anonymous:anonymous ftp://localhost:9002/` | Anonymous read-only FTP; passive-mode Windows clients work. By default the passive range is `9011-9020`, so avoid assigning deployment listeners in that slice when you need FTP |
| NFS | `wsl -d Debian -u root -- mount -t nfs -o nfsvers=3,noacl,tcp,port=9003,mountport=9003,nolock,noresvport 127.0.0.1:/mount /mnt/honeygen-nfs-test` | Standard Windows Explorer does not mount custom-port NFS shares; use WSL/Linux tools |

Windows-specific notes:

- **FTP**: PowerShell/.NET and other passive-mode clients can download files successfully from `ftp://localhost:9002/...`. Windows `ftp.exe` and other active-mode clients do not work reliably through Docker NAT because the server cannot connect back to the client loopback address.
- **SMB**: the built-in Windows SMB client always targets port `445`, so it cannot reach Honeygen's custom-port SMB listener on `9001`. On the same Windows host, `\\\\localhost\\honeygen` reaches the Windows SMB stack instead of the Honeygen container. Use WSL/Linux clients such as `smbclient`, or run the SMB service on a separate host/IP that can expose port `445`.

Event log telemetry now covers all deployment protocols:

- **HTTP / decoy-web**: `http_request`
- **FTP**: `ftp_list`, `ftp_download`
- **NFS**: `nfs_mount`, `nfs_list`, `nfs_read`
- **SMB**: `smb_connect`, `smb_disconnect`, `smb_opendir`, `smb_open`

Non-HTTP events populate canonical generated-file paths plus `metadata.protocol`, `metadata.operation`, and `metadata.deployment_id`. Fields like `status_code`, `user_agent`, and `referer` remain HTTP-only, and source IP is best-effort based on what each protocol exposes.

## Tradeoffs and current limits

- **External provider only:** Honeygen does not ship a built-in local generation provider. For deterministic demos or CI, point `LLM_*` at a deterministic external OpenAI-compatible endpoint or test double.
- **Limited binary preview support:** inline preview is only available for safe text-like assets. PDFs and other binaries are metadata/download only.
- **Reduced PDF fidelity:** PDF output is produced from generated HTML through `wkhtmltopdf`, so advanced CSS/layout fidelity is lower than a full browser print pipeline.
- **Protocol telemetry fidelity differs by protocol:** HTTP still has the richest metadata. FTP/NFS/SMB now emit access events into the same event log, but they only expose the fields their respective server libraries provide.

## More docs

- `docs/architecture.md`
- `docs/api.md`
- `docs/data-model.md`
- `docs/demo.md`
