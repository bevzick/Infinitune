.DEFAULT_GOAL := help

.PHONY: help config ps up stop down down-volumes restart logs smoke auth-migrate-up auth-migrate-version

COMPOSE := docker compose -f compose.yml -f compose.auth.yml

help:
	@echo "Usage: make <target>"
	@echo
	@echo "Available targets:"
	@echo "  help        - Show this help message"
	@echo "  config      - Validate Docker Compose configuration"
	@echo "  ps          - Show status of containers"
	@echo "  up          - Start containers in background"
	@echo "  stop        - Stop running containers"
	@echo "  down        - Stop and remove containers and networks"
	@echo "    -volumes  - Stop and remove containers, networks and volumes"
	@echo "  restart     - Restart running containers"
	@echo "  logs        - Follow logs from all containers"
	@echo "  smoke       - Run basic service health checks"
	@echo "  auth-migrate-up      - Apply pending Auth database migrations"
	@echo "  auth-migrate-version - Show the current Auth migration version"

config:
	@$(COMPOSE) config --quiet

ps:
	@$(COMPOSE) ps

up: auth-migrate-up
	@$(COMPOSE) up --detach

stop:
	@$(COMPOSE) stop

down:
	@$(COMPOSE) down
down-volumes:
	@make down --volumes

restart:
	@make up
	@make down

logs:
	@$(COMPOSE) logs --follow

smoke:
	@bash scripts/smoke.sh

auth-migrate-up:
	@$(COMPOSE) --profile tools run --rm auth-migrate up

auth-migrate-version:
	@$(COMPOSE) --profile tools run --rm auth-migrate version
