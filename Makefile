.DEFAULT_GOAL := help

.PHONY: help status v2-config v2-up v2-check v2-down v2-logs compose-config test-iam test-iam-integration test-control test-control-integration test-portal test-sdk-go test-sdk-rust test-drop test-drop-integration test-telegram-bridge test-telegram-bridge-integration test-ai-gateway test-hermes-terminal format-iam format-control format-drop format-telegram-bridge install-bws bws-migrate secrets-sync host-baseline new-service edge-up edge-check dev-up dev-check public-check beszel-bootstrap beszel-check hermes-terminal-install hermes-terminal-check edge-down edge-logs dev-logs

COMPOSE_FILE := deploy/compose/compose.yaml
ENV_FILE := deploy/compose/.env.example
SERVICE_COMPOSE_FILES := $(sort $(wildcard services/*/compose.homehub.yaml))
COMPOSE_ARGS := --env-file $(ENV_FILE) -f $(COMPOSE_FILE) $(foreach file,$(SERVICE_COMPOSE_FILES),-f $(file))
V2_ENV_FILE := deploy/compose/.env.v2
V2_COMPOSE_FILE := deploy/compose/compose.v2.yaml
V2_COMPOSE_ARGS := --env-file $(V2_ENV_FILE) -f $(V2_COMPOSE_FILE)

help: ## Show available targets
	@awk 'BEGIN {FS = ":.*## "; printf "Usage: make <target>\n\nTargets:\n"} /^[a-zA-Z_0-9-]+:.*## / {printf "  %-18s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

status: ## Show repository status
	@git status --short --branch

v2-config: ## Validate the V2 development Compose configuration
	@docker compose $(V2_COMPOSE_ARGS) config --quiet

v2-up: ## Build and start the V2 control plane, Drop, Telegram Bridge, and React portal
	@docker compose $(V2_COMPOSE_ARGS) up -d --build --wait --wait-timeout 90

v2-check: ## Check the running V2 IAM, Control, Drop, and portal
	@curl --fail --silent http://127.0.0.1:18100/health/ready >/dev/null
	@curl --fail --silent http://127.0.0.1:18100/v1/metadata >/dev/null
	@curl --fail --silent http://127.0.0.1:18100/.well-known/jwks.json >/dev/null
	@curl --fail --silent http://127.0.0.1:18110/health/ready >/dev/null
	@curl --fail --silent http://127.0.0.1:18120/health/ready >/dev/null
	@curl --fail --silent http://127.0.0.1:8730/health/ready >/dev/null
	@curl --fail --silent http://127.0.0.1:18080/health >/dev/null

v2-down: ## Stop the V2 development stack without deleting test data
	@docker compose $(V2_COMPOSE_ARGS) down

v2-logs: ## Follow V2 development logs
	@docker compose $(V2_COMPOSE_ARGS) logs -f iam control drop telegram-bridge openfga postgres portal

compose-config: ## Validate the development Compose configuration
	@docker compose $(COMPOSE_ARGS) config --quiet

test-control: ## Run HomeHub Control unit tests in the pinned Go toolchain
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

test-iam: ## Run HomeHub IAM unit tests in the pinned Go toolchain
	@docker run --rm --network host --user $$(id -u):$$(id -g) \
		-e HOME=/tmp -e GOCACHE=/tmp/go-build \
		-e HTTP_PROXY=http://127.0.0.1:1081 -e HTTPS_PROXY=http://127.0.0.1:1081 \
		-v "$(CURDIR):/repo" -w /repo/apps/iam golang:1.26.5-alpine3.24 go test ./...

test-iam-integration: ## Exchange and verify a live V2 machine access token
	@docker run --rm --network host --user $$(id -u):$$(id -g) \
		-e HOME=/tmp -e GOCACHE=/tmp/go-build \
		-e HOMEHUB_IAM_INTEGRATION_URL=http://127.0.0.1:18100 \
		-e HOMEHUB_IAM_INTEGRATION_CREDENTIAL_FILE=/run/secrets/root_agent_token \
		-e HTTP_PROXY= -e HTTPS_PROXY= -e http_proxy= -e https_proxy= \
		-v "$(CURDIR):/repo" -v /srv/homehub-v2/runtime/root_agent_token:/run/secrets/root_agent_token:ro \
		-w /repo/apps/iam golang:1.26.5-alpine3.24 go test -count=1 -run 'Test(MachineCredentialExchange|RootCreatesBoundedWorkloadIdentity)$$' ./integration

test-portal: ## Type-check and build the React portal
	@docker build --network host -f apps/portal/Dockerfile -t homehub/portal:test .

test-sdk-go: ## Run the HomeHub Go SDK tests
	@docker run --rm --user $$(id -u):$$(id -g) -e HOME=/tmp -e GOCACHE=/tmp/go-build -v "$(CURDIR)/packages/go-sdk:/src" -w /src golang:1.26.5-alpine3.24 go test ./...

test-sdk-rust: ## Run the HomeHub Rust SDK tests
	@docker run --rm --network host --user $$(id -u):$$(id -g) \
		-e HOME=/tmp -e CARGO_HOME=/tmp/cargo -e CARGO_TARGET_DIR=/tmp/target \
		-e PATH=/usr/local/cargo/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin \
		-e HTTP_PROXY=http://127.0.0.1:1081 -e HTTPS_PROXY=http://127.0.0.1:1081 \
		-v "$(CURDIR)/packages/rust-sdk:/src" -w /src rust:1.97-alpine3.24 cargo test

test-drop: ## Run Drop V2 tests with the pinned Go toolchain
	@docker run --rm --network host --user $$(id -u):$$(id -g) \
		-e HOME=/tmp -e GOCACHE=/tmp/go-build \
		-e HTTP_PROXY=http://127.0.0.1:1081 -e HTTPS_PROXY=http://127.0.0.1:1081 \
		-v "$(CURDIR):/repo" -w /repo/services/drop golang:1.26.5-alpine3.24 go test ./...

test-drop-integration: ## Upload, read, and delete an original file through live Drop V2
	@docker run --rm --network host --user $$(id -u):$$(id -g) \
		-e HOME=/tmp -e GOCACHE=/tmp/go-build \
		-e HOMEHUB_IAM_INTEGRATION_URL=http://127.0.0.1:18100 \
		-e HOMEHUB_DROP_INTEGRATION_URL=http://127.0.0.1:18120 \
		-e HOMEHUB_IAM_INTEGRATION_CREDENTIAL_FILE=/run/secrets/root_agent_token \
		-e HTTP_PROXY= -e HTTPS_PROXY= -e http_proxy= -e https_proxy= \
		-v "$(CURDIR):/repo" -v /srv/homehub-v2/runtime/root_agent_token:/run/secrets/root_agent_token:ro \
		-w /repo/services/drop golang:1.26.5-alpine3.24 go test -count=1 -run TestLiveDropAuthorizationAndOriginalFile ./integration

test-telegram-bridge: ## Run Telegram Bridge tests with the pinned Go toolchain
	@docker run --rm --user $$(id -u):$$(id -g) -e HOME=/tmp -e GOCACHE=/tmp/go-build \
		-v "$(CURDIR)/services/telegram-bridge:/src" -w /src golang:1.26.5-alpine3.24 go test ./...

test-telegram-bridge-integration: ## Verify the live Telegram workload can create but not read Drop items
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

format-telegram-bridge: ## Format Telegram Bridge Go source
	@docker run --rm --user $$(id -u):$$(id -g) -v "$(CURDIR)/services/telegram-bridge:/src" -w /src golang:1.26.5-alpine3.24 gofmt -w ./cmd ./integration ./internal

test-ai-gateway: ## Run AI Gateway tests in the pinned Go toolchain
	@docker run --rm --user $$(id -u):$$(id -g) -e HOME=/tmp -e GOCACHE=/tmp/go-build \
		-v "$(CURDIR):/repo:ro" -w /repo/services/ai-gateway golang:1.26.5-alpine3.24 go test ./...

test-hermes-terminal: ## Type-check and build the Hermes web terminal
	@docker build --network host -f services/hermes-terminal-web/Dockerfile -t homehub/hermes-terminal-web:test .

format-drop: ## Format Drop Go source
	@docker run --rm --user $$(id -u):$$(id -g) -v "$(CURDIR)/services/drop:/src" -w /src golang:1.26.5-alpine3.24 gofmt -w ./cmd ./internal

install-bws: ## Install the pinned Bitwarden Secrets Manager CLI
	@sudo ./deploy/scripts/install-bws.sh

bws-migrate: ## Upsert existing HomeHub runtime secrets into Bitwarden
	@sudo ./deploy/scripts/migrate-secrets-to-bws.py

secrets-sync: ## Materialize HomeHub runtime secret files from Bitwarden
	@sudo ./deploy/scripts/materialize-secrets-from-bws.py

host-baseline: ## Install bounded logging and the minimal static host firewall
	@sudo ./deploy/scripts/install-host-baseline.sh

new-service: ## Generate a service: make new-service NAME=notes LANG=go VISIBILITY=owner
	@test -n "$(NAME)" || (echo "NAME is required" >&2; exit 1)
	@test -n "$(LANG)" || (echo "LANG must be go or rust" >&2; exit 1)
	@python3 ./deploy/scripts/new-service.py --name "$(NAME)" --lang "$(LANG)" --visibility "$(or $(VISIBILITY),owner)"

format-control: ## Format HomeHub Control Go source
	@docker run --rm --user $$(id -u):$$(id -g) -v "$(CURDIR)/apps/control:/src" -w /src golang:1.26.5-alpine3.24 gofmt -w ./cmd ./internal

format-iam: ## Format HomeHub IAM Go source
	@docker run --rm --user $$(id -u):$$(id -g) -v "$(CURDIR)/apps/iam:/src" -w /src golang:1.26.5-alpine3.24 gofmt -w ./authz ./cmd ./integration ./internal ./manifests

edge-up: ## Start the loopback-only Traefik development edge
	@docker compose $(COMPOSE_ARGS) up -d traefik

edge-check: ## Verify the running Traefik development edge
	@./deploy/scripts/check-running-edge.sh

dev-up: ## Build and start the HomeHub Control API and portal
	@docker compose $(COMPOSE_ARGS) up -d --build --wait --wait-timeout 60 portal

dev-check: ## Verify the portal, Control API, and development edge
	@./deploy/scripts/check-running-edge.sh
	@./deploy/scripts/check-running-dev.sh

public-check: ## Verify trusted public HTTPS and anonymous access denial
	@./deploy/scripts/check-running-public.sh

beszel-bootstrap: ## Initialize Beszel data and its local agent identity
	@sudo ./deploy/scripts/bootstrap-beszel.sh

beszel-check: ## Verify the protected server panel and local agent
	@./deploy/scripts/check-running-beszel.sh

hermes-terminal-install: ## Install or refresh the native Hermes web terminal
	@./deploy/scripts/install-hermes-terminal.sh

hermes-terminal-check: ## Verify the native terminal and protected public route
	@./deploy/scripts/check-running-hermes-terminal.sh

edge-down: ## Stop only this repository's development edge stack
	@docker compose $(COMPOSE_ARGS) down

edge-logs: ## Follow Traefik and Docker socket proxy logs
	@docker compose $(COMPOSE_ARGS) logs -f traefik docker-socket-proxy

dev-logs: ## Follow all HomeHub development service logs
	@docker compose $(COMPOSE_ARGS) logs -f traefik control portal
