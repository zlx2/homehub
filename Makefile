.DEFAULT_GOAL := help

.PHONY: help status compose-config edge-up edge-check edge-down edge-logs

COMPOSE_FILE := deploy/compose/compose.yaml
ENV_FILE := deploy/compose/.env.example

help: ## Show available targets
	@awk 'BEGIN {FS = ":.*## "; printf "Usage: make <target>\n\nTargets:\n"} /^[a-zA-Z_0-9-]+:.*## / {printf "  %-18s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

status: ## Show repository status
	@git status --short --branch

compose-config: ## Validate the development Compose configuration
	@docker compose --env-file $(ENV_FILE) -f $(COMPOSE_FILE) config --quiet

edge-up: ## Start the loopback-only Traefik development edge
	@docker compose --env-file $(ENV_FILE) -f $(COMPOSE_FILE) up -d traefik

edge-check: ## Verify the running Traefik development edge
	@./deploy/scripts/check-running-edge.sh

edge-down: ## Stop only this repository's development edge stack
	@docker compose --env-file $(ENV_FILE) -f $(COMPOSE_FILE) down

edge-logs: ## Follow Traefik and Docker socket proxy logs
	@docker compose --env-file $(ENV_FILE) -f $(COMPOSE_FILE) logs -f traefik docker-socket-proxy
