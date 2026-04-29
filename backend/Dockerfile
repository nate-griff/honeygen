ARG GO_VERSION=1.24
FROM golang:${GO_VERSION}-bookworm AS build-base
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/cmd ./cmd
COPY backend/internal ./internal

FROM build-base AS build-api
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/api ./cmd/api

FROM build-base AS build-decoy-web
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/decoy-web ./cmd/decoy-web

FROM debian:bookworm-slim AS runtime-base
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /app
RUN adduser --system --group appuser \
    && mkdir -p /app/storage/sqlite /app/storage/generated \
    && chown -R appuser:appuser /app
USER appuser

FROM runtime-base AS api
USER root
RUN apt-get update \
    && apt-get install -y --no-install-recommends samba wget wkhtmltopdf \
    && rm -rf /var/lib/apt/lists/*
USER appuser
COPY --chown=appuser:appuser --from=build-api /out/api /app/api
COPY --chown=appuser:appuser sample-data /app/sample-data
ENV HTTP_ADDR=:8080 \
    SQLITE_PATH=/app/storage/sqlite/honeygen.db \
    GENERATED_ASSETS_DIR=/app/storage/generated
EXPOSE 8080
ENTRYPOINT ["/app/api"]

FROM runtime-base AS decoy-web
COPY --chown=appuser:appuser --from=build-decoy-web /out/decoy-web /app/decoy-web
ENV HTTP_ADDR=:8081 \
    GENERATED_ASSETS_DIR=/app/storage/generated
EXPOSE 8081
ENTRYPOINT ["/app/decoy-web"]
