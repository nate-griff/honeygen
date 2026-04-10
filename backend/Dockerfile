FROM golang:1.22-bookworm AS build-base
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal

FROM build-base AS build-api
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/api ./cmd/api

FROM build-base AS build-decoy-web
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/decoy-web ./cmd/decoy-web

FROM debian:bookworm-slim AS runtime-base
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates wkhtmltopdf \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /app
RUN adduser --system --group appuser \
    && mkdir -p /app/storage/sqlite /app/storage/generated \
    && chown -R appuser:appuser /app
USER appuser

FROM runtime-base AS api
COPY --from=build-api /out/api /app/api
ENV APP_ENV=development \
    HTTP_ADDR=:8080 \
    SQLITE_PATH=/app/storage/sqlite/honeygen.db \
    GENERATED_ASSETS_DIR=/app/storage/generated
EXPOSE 8080
ENTRYPOINT ["/app/api"]

FROM runtime-base AS decoy-web
COPY --from=build-decoy-web /out/decoy-web /app/decoy-web
ENV APP_ENV=development \
    HTTP_ADDR=:8081 \
    GENERATED_ASSETS_DIR=/app/storage/generated
EXPOSE 8081
ENTRYPOINT ["/app/decoy-web"]
