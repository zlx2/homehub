# syntax=docker/dockerfile:1.7
FROM node:24.17.0-alpine3.24 AS frontend

WORKDIR /src/services/drop
COPY services/drop/package.json services/drop/pnpm-lock.yaml services/drop/pnpm-workspace.yaml ./
RUN corepack enable && pnpm install --frozen-lockfile
COPY services/drop/frontend ./frontend
COPY services/drop/tsconfig.json services/drop/vite.config.ts ./
COPY services/drop/internal/httpapi/web ./internal/httpapi/web
RUN pnpm build

FROM golang:1.26.5-alpine3.24 AS build

WORKDIR /src/services/drop
COPY packages/go-sdk /src/packages/go-sdk
COPY services/drop/go.mod services/drop/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY services/drop/ ./
COPY --from=frontend /src/services/drop/internal/httpapi/web/app.js ./internal/httpapi/web/app.js
COPY --from=frontend /src/services/drop/internal/httpapi/web/app.css ./internal/httpapi/web/app.css
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/drop ./cmd/drop

FROM alpine:3.24

RUN mkdir -p /data /tmp \
    && chown -R 10001:10001 /data /tmp

COPY --from=build --chown=10001:10001 /out/drop /usr/local/bin/drop

USER 10001:10001
WORKDIR /data
ENV DROP_DATA_DIR=/data
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/drop"]
