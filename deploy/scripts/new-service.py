#!/usr/bin/env python3
"""Generate a HomeHub business service and register it with the local stack."""

from __future__ import annotations

import argparse
import json
from pathlib import Path
import re
import sys
from textwrap import dedent


NAME_PATTERN = re.compile(r"^[a-z][a-z0-9-]{1,40}$")


def fail(message: str) -> None:
    print(f"new-service: {message}", file=sys.stderr)
    raise SystemExit(1)


def render(value: str, replacements: dict[str, str]) -> str:
    for source, target in replacements.items():
        value = value.replace(source, target)
    return dedent(value).lstrip()


def write_files(root: Path, files: dict[str, str], replacements: dict[str, str]) -> None:
    for relative, contents in files.items():
        path = root / render(relative, replacements)
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(render(contents, replacements), encoding="utf-8", newline="\n")


def register_catalog(repo: Path, service: dict[str, object]) -> None:
    path = repo / "deploy/catalog/services.json"
    try:
        catalog = json.loads(path.read_text(encoding="utf-8"))
        services = catalog["services"]
    except (OSError, json.JSONDecodeError, KeyError, TypeError):
        fail("service catalog is invalid")
    if not isinstance(services, list):
        fail("service catalog is invalid")
    if any(isinstance(item, dict) and item.get("id") == service["id"] for item in services):
        fail(f"service {service['id']} is already registered")
    services.append(service)
    path.write_text(json.dumps(catalog, ensure_ascii=False, indent=2) + "\n", encoding="utf-8", newline="\n")


def common_files() -> dict[str, str]:
    return {
        "README.md": """
            # __TITLE__

            Generated HomeHub service. Public traffic is authenticated by
            HomeHub Control, then this process independently verifies the
            service-bound Ed25519 identity before running business handlers.
        """,
        "openapi.yaml": """
            openapi: 3.1.0
            info:
              title: __TITLE__ API
              version: 0.1.0
            servers:
              - url: /__NAME__
            paths:
              /health/live:
                get:
                  security: []
                  responses:
                    "204": { description: Process is alive }
              /health/ready:
                get:
                  security: []
                  responses:
                    "204": { description: Service is ready }
              /api/v1/whoami:
                get:
                  responses:
                    "200": { description: Verified HomeHub identity }
                    "401": { description: Valid HomeHub identity is required }
            components:
              securitySchemes:
                homehubIdentity:
                  type: apiKey
                  in: header
                  name: X-HomeHub-Identity
            security:
              - homehubIdentity: []
        """,
    }


def go_files() -> dict[str, str]:
    files = common_files()
    files.update({
        "go.mod": """
            module homehub.local/services/__NAME__

            go 1.26.0

            toolchain go1.26.5

            require homehub.local/go-sdk v0.0.0

            replace homehub.local/go-sdk => ../../packages/go-sdk
        """,
        "Dockerfile": """
            FROM golang:1.26.5-alpine3.24 AS build
            WORKDIR /src/services/__NAME__
            COPY packages/go-sdk /src/packages/go-sdk
            COPY services/__NAME__/go.mod ./
            COPY services/__NAME__/ ./
            RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/__NAME__ ./cmd/__NAME__

            FROM scratch
            COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
            COPY --from=build /out/__NAME__ /__NAME__
            USER 65532:65532
            EXPOSE 8080
            ENTRYPOINT ["/__NAME__"]
        """,
        "cmd/__NAME__/main.go": r'''
            package main

            import (
                "context"
                "encoding/json"
                "errors"
                "fmt"
                "log/slog"
                "net/http"
                "os"
                "os/signal"
                "syscall"
                "time"

                "homehub.local/go-sdk/httpx"
                "homehub.local/go-sdk/identity"
            )

            const serviceName = "__NAME__"

            func main() {
                if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
                    response, err := http.Get("http://127.0.0.1:8080/health/ready")
                    if err != nil || response.StatusCode != http.StatusNoContent {
                        os.Exit(1)
                    }
                    _ = response.Body.Close()
                    return
                }
                logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
                if err := run(logger); err != nil {
                    logger.Error("service stopped", "service", serviceName, "error", err)
                    os.Exit(1)
                }
            }

            func run(logger *slog.Logger) error {
                address := env("__ENV___LISTEN_ADDRESS", ":8080")
                keyFile := env("__ENV___IDENTITY_PUBLIC_KEY_FILE", "/run/secrets/identity_public_key")
                verifier, err := identity.NewVerifierFromFile(keyFile, serviceName)
                if err != nil {
                    return fmt.Errorf("initialize HomeHub identity: %w", err)
                }
                protected := http.NewServeMux()
                protected.HandleFunc("GET /api/v1/whoami", func(writer http.ResponseWriter, request *http.Request) {
                    claims, _ := identity.FromContext(request.Context())
                    writer.Header().Set("Content-Type", "application/json")
                    _ = json.NewEncoder(writer).Encode(claims)
                })
                root := http.NewServeMux()
                root.HandleFunc("GET /health/live", noContent)
                root.HandleFunc("GET /health/ready", noContent)
                root.Handle("/", verifier.Authenticate([]string{"portal.view", "admin"}, protected))
                handler := httpx.RequestID(httpx.Recover(logger, httpx.SecurityHeaders(root)))
                server := &http.Server{
                    Addr: address, Handler: handler, ReadHeaderTimeout: 5 * time.Second,
                    ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second,
                }
                ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
                defer stop()
                errorsChannel := make(chan error, 1)
                go func() {
                    logger.Info("service listening", "service", serviceName, "address", address)
                    errorsChannel <- server.ListenAndServe()
                }()
                select {
                case <-ctx.Done():
                case err := <-errorsChannel:
                    if !errors.Is(err, http.ErrServerClosed) {
                        return err
                    }
                }
                shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
                defer cancel()
                return server.Shutdown(shutdown)
            }

            func noContent(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(http.StatusNoContent) }

            func env(name, fallback string) string {
                if value := os.Getenv(name); value != "" { return value }
                return fallback
            }
        ''',
    })
    return files


def rust_files() -> dict[str, str]:
    files = common_files()
    files.update({
        "Cargo.toml": """
            [package]
            name = "__NAME__"
            version = "0.1.0"
            edition = "2024"
            rust-version = "1.97"
            publish = false

            [dependencies]
            axum = "0.8"
            homehub-sdk = { path = "../../packages/rust-sdk" }
            serde_json = "1"
            tokio = { version = "1", features = ["macros", "net", "rt-multi-thread", "signal"] }
        """,
        "Dockerfile": """
            FROM rust:1.97-alpine3.24 AS build
            WORKDIR /src
            COPY packages/rust-sdk ./packages/rust-sdk
            COPY services/__NAME__ ./services/__NAME__
            WORKDIR /src/services/__NAME__
            RUN cargo build --release

            FROM alpine:3.24
            COPY --from=build /src/services/__NAME__/target/release/__NAME__ /usr/local/bin/__NAME__
            USER 65532:65532
            EXPOSE 8080
            ENTRYPOINT ["/usr/local/bin/__NAME__"]
        """,
        "src/main.rs": r'''
            use std::env;
            use std::net::TcpStream;
            use std::sync::Arc;

            use axum::extract::{Extension, Request, State};
            use axum::http::{HeaderMap, StatusCode};
            use axum::middleware::{self, Next};
            use axum::response::Response;
            use axum::routing::get;
            use axum::{Json, Router};
            use homehub_sdk::identity::{Claims, HEADER_NAME, Verifier};

            const SERVICE_NAME: &str = "__NAME__";

            #[tokio::main]
            async fn main() {
                if env::args().nth(1).as_deref() == Some("healthcheck") {
                    if TcpStream::connect("127.0.0.1:8080").is_err() { std::process::exit(1); }
                    return;
                }
                let address = env::var("__ENV___LISTEN_ADDRESS").unwrap_or_else(|_| "0.0.0.0:8080".to_owned());
                let key_file = env::var("__ENV___IDENTITY_PUBLIC_KEY_FILE")
                    .unwrap_or_else(|_| "/run/secrets/identity_public_key".to_owned());
                let verifier = Arc::new(Verifier::from_public_key_file(key_file, SERVICE_NAME).expect("valid HomeHub identity key"));
                let protected = Router::new()
                    .route("/api/v1/whoami", get(whoami))
                    .route_layer(middleware::from_fn_with_state(verifier, authenticate));
                let app = Router::new()
                    .route("/health/live", get(no_content))
                    .route("/health/ready", get(no_content))
                    .merge(protected);
                let listener = tokio::net::TcpListener::bind(&address).await.expect("bind listener");
                axum::serve(listener, app).with_graceful_shutdown(shutdown()).await.expect("serve requests");
            }

            async fn authenticate(
                State(verifier): State<Arc<Verifier>>, headers: HeaderMap, mut request: Request, next: Next,
            ) -> Result<Response, StatusCode> {
                let token = headers.get(HEADER_NAME).and_then(|value| value.to_str().ok()).unwrap_or("");
                let claims = verifier.verify(token).map_err(|_| StatusCode::UNAUTHORIZED)?;
                if !claims.has_any_scope(&["portal.view", "admin"]) { return Err(StatusCode::FORBIDDEN); }
                request.extensions_mut().insert(claims);
                Ok(next.run(request).await)
            }

            async fn whoami(Extension(claims): Extension<Claims>) -> Json<Claims> { Json(claims) }
            async fn no_content() -> StatusCode { StatusCode::NO_CONTENT }
            async fn shutdown() { let _ = tokio::signal::ctrl_c().await; }
        ''',
    })
    return files


def compose_file() -> str:
    return """
        services:
          __NAME__:
            image: homehub/__NAME__:local
            build:
              context: ../..
              dockerfile: services/__NAME__/Dockerfile
              network: host
            restart: unless-stopped
            depends_on:
              control:
                condition: service_healthy
            environment:
              __ENV___LISTEN_ADDRESS: 0.0.0.0:8080
              __ENV___IDENTITY_PUBLIC_KEY_FILE: /run/secrets/identity_public_key
              HTTP_PROXY: ""
              HTTPS_PROXY: ""
              NO_PROXY: localhost,127.0.0.1,__NAME__
            networks:
              - edge
            secrets:
              - source: identity_public_key
                target: identity_public_key
            read_only: true
            tmpfs:
              - /tmp:uid=65532,gid=65532,mode=0700
            security_opt:
              - no-new-privileges:true
            cap_drop:
              - ALL
            healthcheck:
              test: ["CMD", "__BINARY__", "healthcheck"]
              interval: 10s
              timeout: 3s
              retries: 5
              start_period: 10s
            labels:
              traefik.enable: "true"
              traefik.docker.network: homehub-edge
              traefik.http.routers.homehub-__NAME__-public.entrypoints: public-https
              traefik.http.routers.homehub-__NAME__-public.rule: Path(`/__NAME__`) || PathPrefix(`/__NAME__/`)
              traefik.http.routers.homehub-__NAME__-public.priority: "200"
              traefik.http.routers.homehub-__NAME__-public.middlewares: homehub-strip-untrusted-identity@file,homehub-forward-auth@file,homehub-__NAME__-strip,homehub-security-headers@file,homehub-compress@file
              traefik.http.routers.homehub-__NAME__-public.tls: "true"
              traefik.http.middlewares.homehub-__NAME__-strip.stripprefix.prefixes: /__NAME__
              traefik.http.services.homehub-__NAME__.loadbalancer.server.port: "8080"
              homehub.id: __NAME__
              homehub.visibility: __VISIBILITY__
              homehub.share-enabled: "__SHARE_ENABLED__"
    """


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--name", required=True)
    parser.add_argument("--lang", choices=("go", "rust"), required=True)
    parser.add_argument("--visibility", choices=("owner", "shared", "internal"), default="owner")
    parser.add_argument("--repo-root", type=Path)
    args = parser.parse_args()

    name = args.name.strip().lower()
    if not NAME_PATTERN.fullmatch(name):
        fail("name must use lowercase letters, digits, and hyphens")
    repo = (args.repo_root or Path(__file__).resolve().parents[2]).resolve()
    service_root = repo / "services" / name
    if service_root.exists():
        fail(f"{service_root} already exists")
    title = " ".join(part.capitalize() for part in name.split("-"))
    env_name = name.upper().replace("-", "_")
    share_enabled = args.visibility == "shared"
    replacements = {
        "__NAME__": name,
        "__TITLE__": title,
        "__ENV__": env_name,
        "__VISIBILITY__": args.visibility,
        "__SHARE_ENABLED__": str(share_enabled).lower(),
        "__BINARY__": f"/{name}" if args.lang == "go" else f"/usr/local/bin/{name}",
    }
    files = go_files() if args.lang == "go" else rust_files()
    files["compose.homehub.yaml"] = compose_file()
    write_files(service_root, files, replacements)
    register_catalog(repo, {
        "id": name,
        "name": title,
        "description": f"HomeHub {args.lang.title()} business service.",
        "icon": title[0],
        "route": f"/{name}/",
        "visibility": args.visibility,
        "share_enabled": share_enabled,
        "identity_enabled": True,
        "health_url": f"http://{name}:8080/health/ready",
    })
    print(f"Generated {args.lang} service {name} in {service_root.relative_to(repo)}")
    print("Run make compose-config, then review and commit the generated service.")


if __name__ == "__main__":
    main()
