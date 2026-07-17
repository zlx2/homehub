.DEFAULT_GOAL := help

.PHONY: help status compose-config test-control test-demo-decider test-demo-counter format-control format-demo-decider edge-up edge-check dev-up dev-check public-check beszel-bootstrap beszel-check demos-check edge-down edge-logs dev-logs

COMPOSE_FILE := deploy/compose/compose.yaml
ENV_FILE := deploy/compose/.env.example

help: ## Show available targets
	@awk 'BEGIN {FS = ":.*## "; printf "Usage: make <target>\n\nTargets:\n"} /^[a-zA-Z_0-9-]+:.*## / {printf "  %-18s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

status: ## Show repository status
	@git status --short --branch

compose-config: ## Validate the development Compose configuration
	@docker compose --env-file $(ENV_FILE) -f $(COMPOSE_FILE) config --quiet

test-control: ## Run HomeHub Control unit tests in the pinned Go toolchain
	@docker run --rm --user $$(id -u):$$(id -g) -e HOME=/tmp -e GOCACHE=/tmp/go-build -v "$(CURDIR)/apps/control:/src" -w /src golang:1.26.5-alpine3.24 go test ./...

test-demo-decider: ## Run the Go demo service unit tests
	@docker run --rm --user $$(id -u):$$(id -g) -e HOME=/tmp -e GOCACHE=/tmp/go-build -v "$(CURDIR)/services/demo-decider:/src" -w /src golang:1.26.5-alpine3.24 go test ./...

test-demo-counter: ## Run the Rust demo service unit tests
	@docker build --network host --target test -f services/demo-counter/Dockerfile \
		--build-arg HTTP_PROXY=http://127.0.0.1:1081 \
		--build-arg HTTPS_PROXY=http://127.0.0.1:1081 \
		--build-arg http_proxy=http://127.0.0.1:1081 \
		--build-arg https_proxy=http://127.0.0.1:1081 .

format-control: ## Format HomeHub Control Go source
	@docker run --rm --user $$(id -u):$$(id -g) -v "$(CURDIR)/apps/control:/src" -w /src golang:1.26.5-alpine3.24 gofmt -w ./cmd ./internal

format-demo-decider: ## Format the Go demo service source
	@docker run --rm --user $$(id -u):$$(id -g) -v "$(CURDIR)/services/demo-decider:/src" -w /src golang:1.26.5-alpine3.24 gofmt -w .

edge-up: ## Start the loopback-only Traefik development edge
	@docker compose --env-file $(ENV_FILE) -f $(COMPOSE_FILE) up -d traefik

edge-check: ## Verify the running Traefik development edge
	@./deploy/scripts/check-running-edge.sh

dev-up: ## Build and start the HomeHub Control API and portal
	@docker compose --env-file $(ENV_FILE) -f $(COMPOSE_FILE) up -d --build --wait --wait-timeout 60 portal

dev-check: ## Verify the portal, Control API, and development edge
	@./deploy/scripts/check-running-edge.sh
	@./deploy/scripts/check-running-dev.sh

public-check: ## Verify trusted public HTTPS and anonymous access denial
	@./deploy/scripts/check-running-public.sh

beszel-bootstrap: ## Initialize Beszel data and its local agent identity
	@sudo ./deploy/scripts/bootstrap-beszel.sh

beszel-check: ## Verify the protected server panel and local agent
	@./deploy/scripts/check-running-beszel.sh

demos-check: ## Verify demo health and anonymous access denial
	@./deploy/scripts/check-running-demos.sh

edge-down: ## Stop only this repository's development edge stack
	@docker compose --env-file $(ENV_FILE) -f $(COMPOSE_FILE) down

edge-logs: ## Follow Traefik and Docker socket proxy logs
	@docker compose --env-file $(ENV_FILE) -f $(COMPOSE_FILE) logs -f traefik docker-socket-proxy

dev-logs: ## Follow all HomeHub development service logs
	@docker compose --env-file $(ENV_FILE) -f $(COMPOSE_FILE) logs -f traefik control portal
