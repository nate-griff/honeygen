You are building a full working prototype from the attached PROJECT_SPEC.md.

Your job is to produce a runnable monorepo implementation of the project, not just scaffolding or design notes. Optimize for end-to-end completeness and demo readiness by the time you finish.

Core rules:
1. Build the actual system, not just stubs.
2. Prioritize a working MVP over elegance.
3. Make reasonable implementation decisions without asking for clarification unless absolutely blocked.
4. If a feature is ambiguous, choose the simplest implementation that satisfies the spec.
5. Do not overengineer plugin systems, auth, or infrastructure.
6. Keep the architecture modular enough for future expansion, but do not let future-proofing delay shipping.
7. When time is limited, complete the critical user flows first:
   - docker compose up works
   - admin UI loads
   - world model CRUD works
   - generation can be triggered
   - files are generated and stored
   - decoy web serves files
   - access events are logged and viewable
8. If optional features threaten progress, stub them cleanly and move on.
9. Prefer stable, boring, widely used libraries.
10. Add clear README instructions so the project can be run immediately after checkout.

Required stack:
- Backend: Go
- Frontend: React + TypeScript
- Database: SQLite
- Deployment: Docker Compose

Implementation priorities, in order:
1. Monorepo structure
2. Docker Compose
3. Go backend with SQLite and required API endpoints
4. World model persistence and CRUD
5. Generation pipeline
6. File storage and asset indexing
7. Decoy web static file serving
8. Event logging
9. React admin UI
10. Documentation and cleanup

Generation guidance:
- Use an OpenAI-compatible provider abstraction for LLM calls.
- Also include a local mock/fallback provider so the system can still demonstrate generation without a live key.
- The fallback provider should generate plausible deterministic sample content so the app remains usable in demo mode.
- Generate text/markdown/html/csv first.
- Render PDFs from markdown or HTML using the simplest reliable method available in containers.
- If DOCX/XLSX are not practical, do not block MVP on them.

Frontend guidance:
- Create a clean but simple UI.
- Required pages:
  - Dashboard
  - World Model Editor/List
  - Generation Controls / Job Status
  - File Browser
  - Event Log
- The UI does not need to be beautiful, but it must be functional and coherent.

Backend guidance:
- Implement the required API endpoints from the spec.
- Use structured JSON logs.
- Use migrations or reliable startup-time schema creation.
- Store generated files in mounted volumes.
- Persist metadata in SQLite.
- Support preview for text, markdown, html, and metadata-only display for binary files.

Decoy web guidance:
- Keep it static and safe.
- Serve generated files over HTTP.
- Log all requests with required metadata.
- Make sure those logs reach the backend database or are otherwise visible in the admin UI.

Project quality bar:
- A teammate should be able to clone the repo, copy `.env.example` to `.env`, run `docker compose up`, and use the system.
- The README should include:
  - setup
  - config
  - how to trigger generation
  - how to access the admin UI
  - how to access the decoy web service
  - how to verify events are logged

Execution strategy:
- Start by creating the repo skeleton and Docker setup.
- Then build the backend data model and APIs.
- Then implement generation and storage.
- Then wire in the decoy web service.
- Then build the frontend against the live API.
- Then finish docs and polish.
- Continuously prefer integrated working code over isolated components.

Important delivery expectations:
- Do not stop at a partial scaffold.
- Do not spend most of the effort on docs while leaving the app incomplete.
- Do not leave major required routes unimplemented.
- Do not leave the frontend disconnected from the backend.
- Do not depend entirely on external APIs for demoability; include a mock/fallback generation mode.

Definition of done:
- The project runs with Docker Compose.
- The admin UI works.
- World models can be created and edited.
- Generation jobs can run.
- Generated files can be browsed.
- The decoy server serves generated files.
- Accesses to the decoy server create event records visible in the UI.
- The repo includes enough documentation to run and demo the system.

If you must make tradeoffs:
- Choose completeness over sophistication.
- Choose a working fallback provider over fragile live-provider-only behavior.
- Choose a simple UI over an unfinished fancy one.
- Choose a simple static decoy implementation over an ambitious unfinished multi-service design.

At the end, ensure the repository contains:
- working code
- Dockerfiles
- docker-compose.yml
- .env.example
- README.md
- sample/demo world model data
- basic documentation in /docs
