# Decoy Research Platform — Implementation Backlog

## Milestone 0 — Project Setup
- [x] Create monorepo structure
- [x] Add `README.md`
- [x] Add `.env.example`
- [x] Add `docker-compose.yml`
- [x] Add base docs directory
- [x] Define local development ports and service names
- [x] Add `.gitignore` files for backend/frontend/artifacts

## Milestone 1 — Backend Foundation
- [x] Initialize Go module
- [x] Create `cmd/api` entrypoint
- [x] Add config loading from `.env`
- [x] Add support for structured config file (`config.json`)
- [x] Add structured logging (`log/slog`)
- [x] Add health endpoint (`GET /api/health`)
- [x] Add status endpoint (`GET /api/status`)
- [x] Set up DB connection layer
- [x] Add SQLite support
- [x] Add DB migration system (`internal/db/migrations.go`)
- [x] Create initial migrations
- [x] Add graceful shutdown handling

## Milestone 2 — Data Model and Persistence
- [x] Create `world_models` table
- [x] Create `generation_jobs` table
- [x] Create `assets` table
- [x] Create `events` table
- [x] Create optional `settings` table
- [x] Add repository/data access layer for all core entities
- [x] Add models/types for world model
- [x] Add models/types for assets
- [x] Add models/types for events
- [x] Add models/types for generation jobs

## Milestone 3 — World Model CRUD
- [x] Define world model JSON schema/types
- [x] Implement `GET /api/world-models`
- [x] Implement `POST /api/world-models`
- [x] Implement `GET /api/world-models/:id`
- [x] Implement `PUT /api/world-models/:id`
- [x] Add validation for required world model fields
- [x] Add sample seed world model (`northbridge-financial`)
- [x] Add API tests for world model endpoints

## Milestone 4 — Provider Integration
- [x] Define provider interface abstraction (`provider.Provider`)
- [x] Implement OpenAI-compatible provider client (`provider/openai.go`)
- [x] Add provider config validation (`ProviderConfig.Ready()`)
- [x] Implement `POST /api/provider/test`
- [x] Add request/response logging safeguards
- [x] Add timeout/retry handling for provider calls
- [ ] Add mock provider for local testing (stub exists for tests only, no runtime fallback)

## Milestone 5 — Generation Planning
- [x] Implement world model normalization
- [x] Implement file tree planner
- [x] Implement asset manifest planner
- [x] Define required default directories (`public`, `intranet`, `shared`, `users`)
- [x] Define default document categories
- [x] Add planning logic for employee folders
- [x] Add planning logic for department/shared folders
- [x] Add generation job creation and lifecycle state transitions

## Milestone 6 — Source Content Generation
- [x] Add prompt templates for organization summary
- [x] Add prompt templates for policy docs
- [x] Add prompt templates for meeting notes
- [x] Add prompt templates for project summaries
- [x] Add prompt templates for vendor/contact CSV data
- [x] Add prompt templates for FAQ/help docs
- [x] Add prompt templates for intranet/about pages
- [x] Implement source content generation service
- [x] Add logging of generation steps
- [x] Persist generation errors to job state

## Milestone 7 — Rendering Pipeline
- [x] Implement markdown writer
- [x] Implement text writer
- [x] Implement CSV writer
- [x] Implement HTML writer
- [x] Implement PDF rendering from markdown or HTML (via wkhtmltopdf)
- [x] Add output file naming conventions
- [x] Add checksum generation
- [x] Add previewable flag logic
- [x] Persist generated asset metadata
- [x] Write generated files to mounted storage

## Milestone 8 — Generation API
- [x] Implement `POST /api/generation/run`
- [x] Implement `GET /api/generation/jobs`
- [x] Implement `GET /api/generation/jobs/:id`
- [x] Return job state and error info
- [x] Return generation summary metadata
- [x] Support generation by world model id
- [x] Add API tests for generation endpoints

## Milestone 9 — Asset Browsing API
- [x] Implement `GET /api/assets`
- [x] Implement `GET /api/assets/tree`
- [x] Implement `GET /api/assets/:id`
- [x] Implement `GET /api/assets/:id/content`
- [x] Add pagination/filtering for asset listing
- [x] Add safe preview handling for text/html/markdown
- [x] Add download URL or proxy support (NGINX `/downloads/` proxy)
- [x] Add API tests for asset endpoints

## Milestone 10 — Event Logging
- [x] Implement event model and persistence
- [x] Define event ingestion path from decoy service (`POST /internal/events`)
- [x] Implement `GET /api/events`
- [x] Implement `GET /api/events/:id`
- [x] Add pagination/filtering for event logs
- [x] Add API tests for event endpoints

## Milestone 11 — Decoy Web Service
- [x] Create decoy web service (`cmd/decoy-web`)
- [x] Serve generated files from mounted storage
- [x] Add request logging middleware
- [x] Capture source IP
- [x] Capture method/path/query/user-agent/referer/status/bytes sent
- [x] Persist or forward events to backend (via `X-Honeygen-Internal-Event-Token`)
- [x] Add health endpoint for decoy service
- [ ] Add optional default portal/index page
- [x] Verify generated files are publicly browsable

## Milestone 12 — Frontend Foundation
- [x] Initialize React + TypeScript app (Vite)
- [x] Add basic routing (React Router)
- [x] Add API client layer (`api/client.ts` with `apiRequest<T>`)
- [x] Add layout/navigation shell (`AppShell`)
- [x] Add environment config (`VITE_API_BASE_URL`)
- [x] Add UI styling framework or base styles (custom CSS)
- [x] Add error/loading state patterns (`ErrorAlert`, `APIClientError`)

## Milestone 13 — Dashboard UI
- [x] Create dashboard page
- [x] Show service/system status
- [x] Show total asset count
- [x] Show latest generation job
- [x] Show recent events
- [x] Add empty/error states

## Milestone 14 — World Model UI
- [x] Create world model list page
- [x] Create world model editor form
- [x] Add create/edit/save flow
- [x] Add sensible default values
- [x] Add client-side validation
- [x] Add ability to select a world model for generation

## Milestone 15 — Generation UI
- [x] Create generation controls section/page
- [x] Trigger generation job from UI
- [x] Show job progress/status
- [x] Show generation errors
- [x] Show generation completion summary

## Milestone 16 — File Browser UI
- [x] Create asset tree browser
- [x] Create asset detail panel
- [x] Show metadata for selected asset
- [x] Preview text/markdown/html assets
- [x] Add download/open action
- [x] Show file counts and tree structure cleanly

## Milestone 17 — Event Log UI
- [x] Create event list page
- [x] Add event detail view
- [ ] Add filters (path/date/IP/status)
- [x] Show path, IP, method, status, timestamp
- [x] Add pagination support

## Milestone 18 — Integration and Demo Readiness
- [x] Wire frontend to live backend APIs
- [x] Confirm end-to-end generation works
- [x] Confirm decoy web serves latest generated dataset
- [x] Confirm decoy access creates event rows visible in UI
- [x] Add default demo persona/world model (`northbridge-financial` seeded at startup)
- [ ] Generate demo dataset on first run or via one-click action
- [x] Verify persistence across restarts

## Milestone 19 — Docker and Runtime Hardening
- [x] Add backend Dockerfile
- [x] Add frontend Dockerfile (multi-stage with NGINX)
- [x] Add decoy service Dockerfile (shares backend image)
- [x] Confirm container networking works
- [x] Confirm mounted volumes work (`sqlite-data`, `generated-assets`)
- [x] Confirm `.env` wiring works
- [x] Add startup instructions
- [x] Fix any compose race/startup dependency issues

## Milestone 20 — Documentation
- [x] Write setup instructions (`README.md`)
- [x] Write local development instructions
- [x] Write architecture summary (`docs/architecture.md`)
- [x] Document API endpoints (`docs/api.md`)
- [ ] Document config variables (partial — `.env.example` has comments)
- [x] Document demo workflow (`docs/demo.md`)
- [ ] Add troubleshooting section
- [ ] Add screenshots

## Milestone 21 — Final QA
- [x] Run stack with `docker compose up`
- [x] Verify admin UI loads
- [x] Verify world model CRUD works
- [x] Verify generation works
- [x] Verify PDF output exists
- [x] Verify asset browser works
- [x] Verify decoy web serves content
- [x] Verify events appear in UI
- [x] Verify restart persistence
- [x] Fix critical broken flows

## v1 Fixes (from first hands-on testing)
- [x] Fix null JSON serialization crash (nil slices → `null` instead of `[]`)
- [x] Add defensive null guards in frontend API layer (`?? []`)
- [x] Add descriptive comments to `.env.example`
- [x] Add provider config logging at API startup
- [x] Add live-update polling on Dashboard (5s auto-refresh)
- [x] Add runtime LLM configuration UI (Settings page with save/test)
- [x] Add world model generation from natural language description
- [x] Update BACKLOG.md to reflect actual progress

## Stretch Goals
- [ ] Add Postgres support
- [ ] Add scheduled regeneration
- [ ] Add fake timestamp aging
- [ ] Add DOCX generation
- [ ] Add XLSX generation
- [ ] Add placeholder image generation
- [ ] Add export/download for event logs
- [ ] Add improved decoy landing portal
- [ ] Add basic admin authentication
- [ ] Add mock/fallback provider for demo mode without live LLM
- [ ] Add event log filters (path, date range, IP, status code)
- [ ] Add config variables documentation page
- [ ] Add troubleshooting guide
- [ ] Auto-generate demo dataset on first startup
