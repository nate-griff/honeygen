# API Reference

Base API URL: `http://localhost:8080`

For the browser-based admin UI, the recommended local setting is `VITE_API_BASE_URL=` so requests go through `http://localhost:4173/api/...`.

## Health and status

| Method | Path | Purpose |
| --- | --- | --- |
| GET | `/healthz` | Plain-text container health probe |
| GET | `/api/health` | JSON service health |
| GET | `/api/status` | Dashboard summary: service, database, provider, counts, recent events, latest job |

## Provider

| Method | Path | Purpose |
| --- | --- | --- |
| POST | `/api/provider/test` | Validate external provider connectivity and auth |

Example:

```powershell
Invoke-RestMethod -Method Post http://localhost:8080/api/provider/test
```

Successful response:

```json
{
  "ready": true,
  "mode": "external",
  "base_url": "https://provider.example/v1",
  "model": "gpt-4o-mini"
}
```

## World models

| Method | Path | Purpose |
| --- | --- | --- |
| GET | `/api/world-models` | List saved world models |
| POST | `/api/world-models` | Create a world model from a JSON payload |
| GET | `/api/world-models/:id` | Fetch one expanded world model |
| PUT | `/api/world-models/:id` | Replace one world model |

Notes:

- the seeded demo model id is `northbridge-financial`
- payload validation requires `organization`, `branding`, `departments`, `employees`, `projects`, and `document_themes`

## Generation

| Method | Path | Purpose |
| --- | --- | --- |
| POST | `/api/generation/run` | Run synchronous generation for one world model |
| GET | `/api/generation/jobs` | List generation jobs |
| GET | `/api/generation/jobs/:id` | Fetch one job, including summary logs |

Request body:

```json
{
  "world_model_id": "northbridge-financial"
}
```

Important behavior:

- generation is synchronous from the caller's perspective
- a completed job includes summary counts and log entries
- provider failures are surfaced as job failure output and API errors

Supported `/api/generation/jobs` query params:

- `world_model_id`
- `limit`
- `offset`

Failure note:

- if provider generation fails after a job has been created, `POST /api/generation/run` can return `502` with the failed job object instead of the standard `{ "error": ... }` envelope

## Assets

| Method | Path | Purpose |
| --- | --- | --- |
| GET | `/api/assets` | List assets, optionally filtered by `world_model_id` and `generation_job_id` |
| GET | `/api/assets/tree` | Return a hierarchical asset tree |
| GET | `/api/assets/:id` | Fetch one asset record |
| GET | `/api/assets/:id/content` | Return inline content for previewable assets |

Supported query params:

- `world_model_id`
- `generation_job_id`
- `limit`
- `offset`

Preview behavior:

- HTML, Markdown, text, and CSV can be previewed inline
- PDFs, DOCX, and XLSX return metadata with `previewable: false`

## Events

| Method | Path | Purpose |
| --- | --- | --- |
| GET | `/api/events` | List persisted decoy access events |
| GET | `/api/events/:id` | Fetch one event |
| POST | `/internal/events` | Internal ingestion endpoint used by `decoy-web` |

Supported `/api/events` query params:

- `world_model_id`
- `path`
- `source_ip`
- `status_code`
- `limit`
- `offset`

The internal ingestion endpoint is protected by `X-Honeygen-Internal-Event-Token`.

## Deployments

| Method | Path | Purpose |
| --- | --- | --- |
| GET | `/api/deployments` | List all deployments (returns `items` array and `running` status map) |
| POST | `/api/deployments` | Create a deployment |
| GET | `/api/deployments/:id` | Fetch one deployment |
| DELETE | `/api/deployments/:id` | Delete a deployment (stops it first if running) |
| POST | `/api/deployments/:id/start` | Start serving the deployment |
| POST | `/api/deployments/:id/stop` | Stop serving the deployment |

Create request body:

```json
{
  "generation_job_id": "...",
  "world_model_id": "...",
  "protocol": "http",
  "port": 9000,
  "root_path": "generated/northbridge-financial/<job-id>"
}
```

Supported `protocol` values: `"http"`, `"ftp"`, `"nfs"`.

The same generation job output can be deployed across multiple protocols on different ports. Docker Compose exposes the port range 9000–9020 by default for deployments.

## Error shape

Most API failures return:

```json
{
  "error": {
    "code": "validation_error",
    "message": "world_model_id is required"
  }
}
```

Common codes include:

- `validation_error`
- `not_found`
- `method_not_allowed`
- `provider_invalid`
- `provider_auth_failed`
- `provider_unreachable`
- `provider_invalid_response`
- `provider_unavailable`
- `generation_failed`
- `assets_unavailable`
- `events_unavailable`
- `start_failed`
- `stop_failed`
