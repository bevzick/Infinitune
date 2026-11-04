.DEFAULT_GOAL := help

.PHONY: help config ps up stop down down-volumes restart logs smoke auth-test auth-test-integration auth-migrate-up auth-migrate-version user-test user-test-integration user-migrate-up user-migrate-version

COMPOSE := docker compose -f compose.yml -f compose.auth.yml -f compose.user.yml

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
	@echo "  auth-test   - Run Auth Service unit tests"
	@echo "  auth-test-integration - Run Auth repository integration tests"
	@echo "  auth-migrate-up      - Apply pending Auth database migrations"
	@echo "  auth-migrate-version - Show the current Auth migration version"
	@echo "  user-test            - Run User Service unit tests"
	@echo "  user-test-integration - Run User repository integration tests"
	@echo "  user-migrate-up      - Apply pending User database migrations"
	@echo "  user-migrate-version - Show the current User migration version"

config:
	@$(COMPOSE) config --quiet

ps:
	@$(COMPOSE) ps

up: auth-migrate-up user-migrate-up
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

auth-test:
	@cd services/auth-service && go test ./...

auth-test-integration: auth-migrate-up
	@$(COMPOSE) --profile tools run --rm --build auth-integration-test

auth-migrate-up:
	@$(COMPOSE) --profile tools run --rm auth-migrate up

auth-migrate-version:
	@$(COMPOSE) --profile tools run --rm auth-migrate version

user-test:
	@cd services/user-service && go test ./...

user-test-integration: user-migrate-up
	@$(COMPOSE) --profile tools run --rm --build user-integration-test

user-migrate-up:
	@$(COMPOSE) --profile tools run --rm user-migrate up

user-migrate-version:
	@$(COMPOSE) --profile tools run --rm user-migrate version
