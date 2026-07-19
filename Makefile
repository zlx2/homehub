.DEFAULT_GOAL := help

.PHONY: help status config up check down logs build build-ai-gateway test-iam test-iam-integration test-control test-control-integration test-portal test-sdk-go test-sdk-rust test-drop test-drop-integration test-telegram-bridge test-telegram-bridge-integration format-iam format-control format-drop format-ai-gateway format-telegram-bridge install-bws secrets-sync host-baseline

ENV_FILE := deploy/compose/.env
COMPOSE_FILE := deploy/compose/compose.yaml
COMPOSE_ARGS := --env-file $(ENV_FILE) -f $(COMPOSE_FILE)

help: ## Show available targets
	@awk 'BEGIN {FS = ":.*## "; printf "Usage: make <target>\n\nTargets:\n"} /^[a-zA-Z_0-9-]+:.*## / {printf "  %-32s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

status: ## Show repository status
	@git status --short --branch

config: ## Validate the Compose configuration
	@docker compose $(COMPOSE_ARGS) config --quiet

up: ## Build and start the full stack
	@docker compose $(COMPOSE_ARGS) up -d --build --wait --wait-timeout 90

check: ## Health-check all core services
	@curl --fail --silent http://127.0.0.1:18100/health/ready >/dev/null
	@curl --fail --silent http://127.0.0.1:18100/v1/metadata >/dev/null
	@curl --fail --silent http://127.0.0.1:18100/.well-known/jwks.json >/dev/null
	@curl --fail --silent http://127.0.0.1:18110/health/ready >/dev/null
	@curl --fail --silent http://127.0.0.1:18120/health/ready >/dev/null
	@curl --fail --silent http://127.0.0.1:8730/health/ready >/dev/null
	@curl --fail --silent http://127.0.0.1:18080/health >/dev/null

down: ## Stop all services without deleting data
	@docker compose $(COMPOSE_ARGS) down

logs: ## Follow all service logs
	@docker compose $(COMPOSE_ARGS) logs -f iam control ai-gateway drop telegram-bridge openfga postgres portal

build: ## Compile every application without running tests
	@docker compose $(COMPOSE_ARGS) build
	@docker run --rm --network host --user $$(id -u):$$(id -g) \
		-e HOME=/tmp -e CARGO_HOME=/tmp/cargo -e CARGO_TARGET_DIR=/tmp/target \
		-e HTTP_PROXY=http://127.0.0.1:1081 -e HTTPS_PROXY=http://127.0.0.1:1081 \
		-v "$(CURDIR)/packages/rust-sdk:/src" -w /src rust:1.97-alpine3.24 cargo build

build-ai-gateway: ## Compile the AI Gateway
	@docker build --network host -f services/ai-gateway/Dockerfile -t homehub/ai-gateway:build .

test-control: ## Run Control unit tests
	@docker run --rm --user $$(id -u):$$(id -g) -e HOME=/tmp -e GOCACHE=/tmp/go-build \
		-v "$(CURDIR):/repo" -w /repo/apps/control golang:1.26.5-alpine3.24 go test ./...

test-control-integration: ## Verify live Control audience and permission enforcement
	@docker run --rm --network host --user $$(id -u):$$(id -g) \
		-e HOME=/tmp -e GOCACHE=/tmp/go-build \
		-e HOMEHUB_IAM_INTEGRATION_URL=http://127.0.0.1:18100 \
		-e HOMEHUB_CONTROL_INTEGRATION_URL=http://127.0.0.1:18110 \
		-e HOMEHUB_IAM_INTEGRATION_CREDENTIAL_FILE=/run/secrets/root_agent_token \
		-e HTTP_PROXY= -e HTTPS_PROXY= -e http_proxy= -e https_proxy= \
		-v "$(CURDIR):/repo" -v /srv/homehub-v2/runtime/root_agent_token:/run/secrets/root_agent_token:ro \
		-w /repo/apps/control golang:1.26.5-alpine3.24 go test -count=1 -run TestLiveControlAuthorization ./integration

test-iam: ## Run IAM unit tests
	@docker run --rm --network host --user $$(id -u):$$(id -g) \
		-e HOME=/tmp -e GOCACHE=/tmp/go-build \
		-e HTTP_PROXY=http://127.0.0.1:1081 -e HTTPS_PROXY=http://127.0.0.1:1081 \
		-v "$(CURDIR):/repo" -w /repo/apps/iam golang:1.26.5-alpine3.24 go test ./...

test-iam-integration: ## Exchange and verify a live machine access token
	@docker run --rm --network host --user $$(id -u):$$(id -g) \
		-e HOME=/tmp -e GOCACHE=/tmp/go-build \
		-e HOMEHUB_IAM_INTEGRATION_URL=http://127.0.0.1:18100 \
		-e HOMEHUB_IAM_INTEGRATION_CREDENTIAL_FILE=/run/secrets/root_agent_token \
		-e HTTP_PROXY= -e HTTPS_PROXY= -e http_proxy= -e https_proxy= \
		-v "$(CURDIR):/repo" -v /srv/homehub-v2/runtime/root_agent_token:/run/secrets/root_agent_token:ro \
		-w /repo/apps/iam golang:1.26.5-alpine3.24 go test -count=1 -run 'Test(MachineCredentialExchange|RootCreatesBoundedWorkloadIdentity)$$' ./integration

test-portal: ## Type-check and build the React portal
	@docker build --network host -f apps/portal/Dockerfile -t homehub/portal:test .

test-sdk-go: ## Run the Go SDK tests
	@docker run --rm --user $$(id -u):$$(id -g) -e HOME=/tmp -e GOCACHE=/tmp/go-build -v "$(CURDIR)/packages/go-sdk:/src" -w /src golang:1.26.5-alpine3.24 go test ./...

test-sdk-rust: ## Run the Rust SDK tests
	@docker run --rm --network host --user $$(id -u):$$(id -g) \
		-e HOME=/tmp -e CARGO_HOME=/tmp/cargo -e CARGO_TARGET_DIR=/tmp/target \
		-e PATH=/usr/local/cargo/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin \
		-e HTTP_PROXY=http://127.0.0.1:1081 -e HTTPS_PROXY=http://127.0.0.1:1081 \
		-v "$(CURDIR)/packages/rust-sdk:/src" -w /src rust:1.97-alpine3.24 cargo test

test-drop: ## Run Drop unit tests
	@docker run --rm --network host --user $$(id -u):$$(id -g) \
		-e HOME=/tmp -e GOCACHE=/tmp/go-build \
		-e HTTP_PROXY=http://127.0.0.1:1081 -e HTTPS_PROXY=http://127.0.0.1:1081 \
		-v "$(CURDIR):/repo" -w /repo/services/drop golang:1.26.5-alpine3.24 go test ./...

test-drop-integration: ## Upload, read, and delete a file through live Drop
	@docker run --rm --network host --user $$(id -u):$$(id -g) \
		-e HOME=/tmp -e GOCACHE=/tmp/go-build \
		-e HOMEHUB_IAM_INTEGRATION_URL=http://127.0.0.1:18100 \
		-e HOMEHUB_DROP_INTEGRATION_URL=http://127.0.0.1:18120 \
		-e HOMEHUB_IAM_INTEGRATION_CREDENTIAL_FILE=/run/secrets/root_agent_token \
		-e HTTP_PROXY= -e HTTPS_PROXY= -e http_proxy= -e https_proxy= \
		-v "$(CURDIR):/repo" -v /srv/homehub-v2/runtime/root_agent_token:/run/secrets/root_agent_token:ro \
		-w /repo/services/drop golang:1.26.5-alpine3.24 go test -count=1 -run TestLiveDropAuthorizationAndOriginalFile ./integration

test-telegram-bridge: ## Run Telegram Bridge unit tests
	@docker run --rm --user $$(id -u):$$(id -g) -e HOME=/tmp -e GOCACHE=/tmp/go-build \
		-v "$(CURDIR)/services/telegram-bridge:/src" -w /src golang:1.26.5-alpine3.24 go test ./...

test-telegram-bridge-integration: ## Verify Telegram workload can create but not read Drop
	@docker run --rm --network host --user 65532:65532 \
		-e HOME=/tmp -e GOCACHE=/tmp/go-build \
		-e HOMEHUB_IAM_INTEGRATION_URL=http://127.0.0.1:18100 \
		-e HOMEHUB_DROP_INTEGRATION_URL=http://127.0.0.1:18120 \
		-e HOMEHUB_TELEGRAM_INTEGRATION_CREDENTIAL_FILE=/run/secrets/telegram_bridge_credential \
		-e HOMEHUB_IAM_INTEGRATION_CREDENTIAL_FILE=/run/secrets/root_agent_token \
		-e HTTP_PROXY= -e HTTPS_PROXY= -e http_proxy= -e https_proxy= \
		-v "$(CURDIR):/repo" \
		-v /srv/homehub-v2/runtime/telegram_bridge_credential:/run/secrets/telegram_bridge_credential:ro \
		-v /srv/homehub-v2/runtime/root_agent_token:/run/secrets/root_agent_token:ro \
		-w /repo/services/telegram-bridge golang:1.26.5-alpine3.24 go test -count=1 -run TestLiveBridgeIdentityCreatesButCannotReadDrop ./integration

format-iam: ## Format IAM Go source
	@docker run --rm --user $$(id -u):$$(id -g) -v "$(CURDIR)/apps/iam:/src" -w /src golang:1.26.5-alpine3.24 gofmt -w ./authz ./cmd ./integration ./internal ./manifests

format-control: ## Format Control Go source
	@docker run --rm --user $$(id -u):$$(id -g) -v "$(CURDIR)/apps/control:/src" -w /src golang:1.26.5-alpine3.24 gofmt -w ./cmd ./internal

format-drop: ## Format Drop Go source
	@docker run --rm --user $$(id -u):$$(id -g) -v "$(CURDIR)/services/drop:/src" -w /src golang:1.26.5-alpine3.24 gofmt -w ./cmd ./internal

format-ai-gateway: ## Format AI Gateway Go source
	@docker run --rm --user $$(id -u):$$(id -g) -v "$(CURDIR):/repo" -w /repo/services/ai-gateway golang:1.26.5-alpine3.24 gofmt -w ./cmd ./internal

format-telegram-bridge: ## Format Telegram Bridge Go source
	@docker run --rm --user $$(id -u):$$(id -g) -v "$(CURDIR)/services/telegram-bridge:/src" -w /src golang:1.26.5-alpine3.24 gofmt -w ./cmd ./integration ./internal

install-bws: ## Install Bitwarden Secrets Manager CLI
	@sudo ./deploy/scripts/install-bws.sh

secrets-sync: ## Materialize runtime secret files from Bitwarden
	@sudo ./deploy/scripts/materialize-secrets-from-bws.py

host-baseline: ## Install host firewall and logging
	@sudo ./deploy/scripts/install-host-baseline.sh
