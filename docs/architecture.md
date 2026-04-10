# Runtime Architecture

## Services

- **api** - Go HTTP service that owns SQLite access, generated asset writes, and future provider orchestration.
- **admin-web** - Static React admin application built once with Vite and served with NGINX.
- **decoy-web** - Go HTTP service reserved for decoy site delivery and generated asset reads.

## Storage

The runtime reserves two named Docker volumes from day one:

- `sqlite-data` for SQLite database files
- `generated-assets` for PDFs, screenshots, and other generated artifacts

## Container strategy

- `backend/Dockerfile` builds two binaries from the same Go module and ships them in Debian Bookworm runtime images so `wkhtmltopdf` can be installed reliably.
- `frontend/Dockerfile` builds static assets in a Node image and serves the compiled output from NGINX.
- `docker-compose.yml` standardizes service names, ports, environment variables, and build targets.
