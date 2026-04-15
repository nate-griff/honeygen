# Decoy Research Platform — Project Specification

## 1. Overview

Build a Docker Compose-based decoy research platform for controlled research and classroom use. The platform should generate a believable synthetic organizational file environment from a structured persona/world model, expose that environment through static decoy services, and provide an admin interface for configuration, file inspection, and interaction review.

This project is a prototype research tool, not a production security appliance.

## 2. Purpose

The platform should allow an operator to:

1. Define a synthetic organization/persona.
2. Configure an external LLM/content provider.
3. Generate a coherent decoy file tree and files.
4. Expose generated assets through a static decoy service.
5. Inspect assets and review interaction events through a web admin GUI.
6. Re-run generation later to refresh or expand content.

## 3. Product Goals

### Primary Goals
- Deliver a complete end-to-end prototype.
- Support configuration-driven setup.
- Generate believable synthetic files and directory structure.
- Expose generated files via at least one decoy service.
- Capture and persist interaction events.
- Provide an admin GUI for management and review.
- Package and run with Docker Compose.

### Non-Goals for MVP
- Full system or shell emulation.
- SSH session emulation.
- Production-grade hardening or enterprise auth.
- Multi-tenant SaaS architecture.
- Highly realistic Office file fidelity.
- Complex distributed deployment.

## 4. Technology Requirements

### Required Stack
- **Backend:** Go
- **Frontend:** TypeScript + React
- **Database:** SQLite for MVP
- **Deployment:** Docker Compose
- **Storage:** Local volume-backed file storage
- **LLM Integration:** External provider only

### LLM Provider Constraints
- Do not run a local model inside the main application container.
- Support an OpenAI-compatible API endpoint for MVP.
- Allow future adapters for llama.cpp/OpenRouter-compatible providers.

## 5. High-Level Architecture

The system should be modular and compose of the following logical parts:

### 5.1 API Service
Responsible for:
- configuration handling
- persona/world model CRUD
- generation orchestration
- asset indexing
- event ingestion and querying
- admin API

### 5.2 Generation Module
Responsible for:
- creating/normalizing the world model
- planning file trees and assets
- generating source content
- rendering output files
- writing files to storage
- persisting asset metadata and generation job state

For MVP, this may live inside the API service.

### 5.3 Decoy Web Service
Responsible for:
- serving generated files over HTTP
- optionally serving a simple intranet or portal landing page
- logging access events

### 5.4 Admin Frontend
Responsible for:
- persona/world configuration
- generation controls
- file browsing and preview
- event browsing
- system status display

## 6. MVP Scope

The MVP must include:

1. Docker Compose deployment.
2. Go backend API.
3. TypeScript/React admin UI.
4. SQLite persistence.
5. Persona/world model CRUD.
6. Generation jobs and status tracking.
7. Synthetic file generation for at least:
   - markdown
   - text
   - csv
   - html
   - pdf
8. Static decoy web service serving generated files.
9. Interaction/event logging.
10. File browsing and event viewing in the admin UI.

## 7. Configuration Requirements

Support:
- `.env` for operational settings and secrets
- structured config file (`config.yaml` or `config.json`) for scenario defaults

### Required Config Domains
- backend API settings
- frontend/public URLs if needed
- database path/settings
- storage root
- decoy web port
- admin web port
- LLM provider endpoint
- LLM provider API key
- generation defaults
- logging options

## 8. Persona / World Model Requirements

The system must support a structured synthetic organization definition.

### Minimum World Model Fields
- organization name
- industry
- size bucket
- region/location
- domain theme
- departments
- employee roster
- project names
- document themes
- branding/tone hints

### Example World Model
```json
{
  "organization": {
    "name": "Example Financial Group",
    "industry": "Financial Services",
    "size": "mid-size",
    "region": "United States",
    "domain_theme": "examplefinancial.local"
  },
  "branding": {
    "tone": "formal",
    "colors": ["#0A2A43", "#D9E2EC"]
  },
  "departments": [
    "Finance",
    "HR",
    "IT",
    "Operations",
    "Sales"
  ],
  "employees": [
    {
      "name": "Jordan Lee",
      "role": "CFO",
      "department": "Finance"
    },
    {
      "name": "Avery Patel",
      "role": "IT Manager",
      "department": "IT"
    }
  ],
  "projects": [
    "Project Falcon",
    "Q3 Cost Optimization",
    "Client Portal Refresh"
  ],
  "document_themes": [
    "budgets",
    "policies",
    "meeting notes",
    "vendor lists",
    "roadmaps"
  ]
}
```

### World Model Requirements
- Store world models in the database.
- Allow create/read/update operations.
- Permit reuse of a saved world model for regeneration.
- Versioning is optional for MVP but architecture should not prevent it later.

## 9. File Tree and Asset Generation Requirements

The system must generate a believable decoy file tree based on the world model.

### Example Directory Patterns
- `/public/`
- `/intranet/`
- `/shared/Finance/`
- `/shared/HR/`
- `/shared/IT/`
- `/users/<name>/Documents/`
- `/users/<name>/Desktop/`
- `/users/<name>/Projects/`

### Required Asset Types for MVP
- `.md`
- `.txt`
- `.csv`
- `.html`
- `.pdf`

### Optional Stretch Types
- `.docx`
- `.xlsx`
- `.png`
- `.jpg`

### Required Document Categories
- policy document
- internal memo
- meeting notes
- project summary
- vendor/contact CSV
- FAQ/help page
- intranet/about page
- employee roster excerpt

### Generation Strategy
- LLM generates source content.
- Renderers convert source content to final files where needed.
- Prefer generating text/markdown/html/csv first, then rendering PDFs from markdown or HTML.

### Asset Metadata Requirements
Each generated asset must store:
- unique id
- generation job id
- logical path
- source type
- rendered type
- mime type
- size
- checksum
- tags or category metadata
- previewable flag
- created timestamp

## 10. Generation Workflow Requirements

The system must support the following workflow:

1. Create or load world model.
2. Build planned file tree and asset manifest.
3. Generate content for each planned asset.
4. Render final artifacts.
5. Write files to disk.
6. Persist metadata to DB.
7. Expose files through the decoy web service.

### Generation Job States
- pending
- running
- failed
- completed

### Generation Requirements
- Store generation logs and expose them through API/UI.
- Allow generation to be triggered manually from UI or API.
- Allow future scheduled generation, but scheduling is not required for MVP.

## 11. Decoy Web Service Requirements

The platform must provide an HTTP decoy service that serves generated content.

### Required Behavior
- Serve files from mounted generated storage.
- Support either:
  - direct static file serving, or
  - a lightweight portal/index page linking into generated content.
- Log every request as an event.

### Required Logged Fields
- timestamp
- source IP
- HTTP method
- requested path
- query string
- user agent
- referer if present
- response status
- bytes sent

### MVP Constraints
- No login/auth simulation required.
- No shell, command execution, or interactive remote system simulation.
- Keep behavior static and file-oriented.

## 12. Admin UI Requirements

The admin UI must include the following screens/features.

### 12.1 Dashboard
- system health/status
- file count
- recent generation job summary
- recent event list

### 12.2 World Model Editor
- create/edit a world model
- save and reuse world models
- populate with reasonable defaults

### 12.3 Generation Controls
- trigger generation
- view generation job status
- view generation errors/logs

### 12.4 File Browser
- browse generated tree
- inspect metadata
- preview text/markdown/html where safe
- download files

### 12.5 Event Log
- paginated interaction list
- filters by path/date/IP/status if feasible
- event detail view

## 13. API Requirements

Implement REST-style JSON endpoints.

### Health / Status
- `GET /api/health`
- `GET /api/status`

### World Models
- `GET /api/world-models`
- `POST /api/world-models`
- `GET /api/world-models/:id`
- `PUT /api/world-models/:id`

### Generation
- `POST /api/generation/run`
- `GET /api/generation/jobs`
- `GET /api/generation/jobs/:id`

### Assets
- `GET /api/assets`
- `GET /api/assets/tree`
- `GET /api/assets/:id`
- `GET /api/assets/:id/content`

### Events
- `GET /api/events`
- `GET /api/events/:id`

### Provider Validation
- `POST /api/provider/test`

### API Conventions
- JSON request/response bodies
- consistent error shape
- pagination where appropriate
- simple filtering support for events/assets

### Example Error Shape
```json
{
  "error": {
    "code": "GENERATION_FAILED",
    "message": "PDF rendering failed for asset policy-123"
  }
}
```

## 14. Persistence Requirements

Use SQLite for MVP.

### Minimum Tables

#### `world_models`
- id
- name
- description
- json_blob
- created_at
- updated_at

#### `generation_jobs`
- id
- world_model_id
- status
- started_at
- completed_at
- error_message
- summary_json

#### `assets`
- id
- generation_job_id
- path
- source_type
- rendered_type
- mime_type
- size_bytes
- tags_json
- previewable
- checksum
- created_at

#### `events`
- id
- timestamp
- source_ip
- method
- path
- query
- user_agent
- referer
- status_code
- bytes_sent

#### `settings` (optional)
- key
- value_json
- updated_at

## 15. File Storage Layout

Use a mounted volume and keep generated content organized by world model and generation job.

Example:
```text
/data/
  generated/
    <world-model-id>/
      <generation-job-id>/
        public/
        intranet/
        shared/
        users/
  previews/
  exports/
```

## 16. Repository Structure

Use a monorepo structure similar to:

```text
repo-root/
  docker-compose.yml
  .env.example
  README.md
  docs/
    architecture.md
    api.md
    data-model.md
    development-plan.md
  backend/
    cmd/
      api/
    internal/
      config/
      db/
      models/
      world/
      generation/
      rendering/
      assets/
      events/
      api/
      provider/
    migrations/
    go.mod
  frontend/
    src/
      app/
      components/
      pages/
      api/
      types/
    package.json
  decoy-web/
  scripts/
  sample-data/
```

## 17. Non-Functional Requirements

### Simplicity
The prototype should remain understandable and maintainable.

### Modularity
Architecture should support:
- future provider adapters
- future decoy service modules
- future DB migration to Postgres

### Reliability
The stack should be able to run unattended overnight for demo/testing purposes.

### Observability
Services should log structured JSON logs.

### Safety
- Do not implement real shell execution.
- Do not execute untrusted content.
- Keep the decoy service static and file-serving only.
- Validate admin API input.
- Avoid exposing secrets through frontend.

## 18. Docker Compose Requirements

The project must include a working `docker-compose.yml`.

### Minimum Services
- `api`
- `admin-web`
- `decoy-web`

### Required Volumes
- generated asset storage
- SQLite database storage

### Required Environment Variables
- provider endpoint
- provider API key
- DB path
- storage root
- API port
- admin web port
- decoy web port
- CORS/admin config as needed

## 19. Deliverables

The completed prototype should include:

1. Working monorepo.
2. Docker Compose setup.
3. Backend API.
4. Admin frontend.
5. World model CRUD.
6. Generation workflow.
7. Static decoy web service.
8. Event logging.
9. Seed/demo persona.
10. Project documentation.

## 20. Acceptance Criteria

The project is successful if:

1. `docker compose up` starts the system successfully.
2. Admin UI is accessible.
3. A user can create or edit a world model.
4. A generation job can be triggered.
5. Generated files appear in the file browser.
6. PDF generation works for at least some assets.
7. The decoy web service serves generated files.
8. Accessing the decoy service creates visible event records.
9. Data persists across restarts.
10. The README explains setup, usage, and demo steps.

## 21. Suggested Demo Defaults

Provide at least one default demo world model, such as:
- “Northbridge Financial Advisory”
- mid-sized US financial services firm
- departments: finance, HR, IT, operations, compliance
- 8–12 employees
- 20–50 generated assets
- a public portal and intranet-style content set

This should make the system demoable immediately after setup.

## 22. Implementation Guidance

Prioritize:
1. End-to-end completeness
2. Stable simple choices
3. Working defaults
4. Clean interfaces
5. Local runnability

Avoid:
- overengineering
- prematurely complex plugin systems
- blocking MVP on optional file types
- interactive system emulation
- complex auth unless required for local demo

## 23. Stretch Goals

Only attempt after MVP works end-to-end:
- Postgres support
- scheduled regeneration
- file aging and fake timestamps
- DOCX/XLSX generation
- placeholder image generation
- export functions
- richer decoy modules
- improved portal theming
- basic admin auth
