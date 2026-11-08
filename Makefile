.DEFAULT_GOAL := help

.PHONY: help config ps up stop down down-volumes restart logs smoke media-storage-init media-storage-test media-test media-test-integration media-migrate-up media-migrate-version media-seed playlist-test playlist-test-integration playlist-migrate-up playlist-migrate-version auth-test auth-test-integration auth-migrate-up auth-migrate-version user-test user-test-integration user-migrate-up user-migrate-version catalog-test catalog-test-integration catalog-migrate-up catalog-migrate-version catalog-seed

COMPOSE := docker compose -f compose.yml -f compose.auth.yml -f compose.user.yml -f compose.catalog.yml -f compose.media.yml -f compose.media-service.yml -f compose.playlist.yml
MEDIA_COMPOSE := docker compose -f compose.media.yml

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
	@echo "  media-storage-init - Start MinIO and create the private media bucket"
	@echo "  media-storage-test - Verify an object survives a MinIO restart"
	@echo "  media-test            - Run Media Service unit tests"
	@echo "  media-test-integration - Run Media integration tests"
	@echo "  media-migrate-up      - Apply pending Media database migrations"
	@echo "  media-migrate-version - Show the current Media migration version"
	@echo "  media-seed            - Generate safe local demo audio and metadata"
	@echo "  playlist-test         - Run Playlist Service unit tests"
	@echo "  playlist-test-integration - Run Playlist integration tests"
	@echo "  playlist-migrate-up      - Apply pending Playlist database migrations"
	@echo "  playlist-migrate-version - Show the current Playlist migration version"
	@echo "  auth-test   - Run Auth Service unit tests"
	@echo "  auth-test-integration - Run Auth repository integration tests"
	@echo "  auth-migrate-up      - Apply pending Auth database migrations"
	@echo "  auth-migrate-version - Show the current Auth migration version"
	@echo "  user-test            - Run User Service unit tests"
	@echo "  user-test-integration - Run User repository integration tests"
	@echo "  user-migrate-up      - Apply pending User database migrations"
	@echo "  user-migrate-version - Show the current User migration version"
	@echo "  catalog-test            - Run Catalog Service unit tests"
	@echo "  catalog-test-integration - Run Catalog integration tests"
	@echo "  catalog-migrate-up      - Apply pending Catalog database migrations"
	@echo "  catalog-migrate-version - Show the current Catalog migration version"
	@echo "  catalog-seed            - Load fictional development metadata"

config:
	@$(COMPOSE) config --quiet

ps:
	@$(COMPOSE) ps

up: auth-migrate-up user-migrate-up catalog-migrate-up media-migrate-up playlist-migrate-up media-storage-init
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

media-storage-init:
	@$(MEDIA_COMPOSE) up --detach --wait --build minio
	@$(MEDIA_COMPOSE) --profile tools run --rm --build minio-init

media-storage-test: media-storage-init
	@bash scripts/test-minio.sh

media-test:
	@cd services/media-service && go test ./...

media-test-integration: media-migrate-up media-storage-init
	@$(COMPOSE) up --detach --build media-service
	@$(COMPOSE) --profile tools run --rm --build media-integration-test

media-migrate-up:
	@$(COMPOSE) --profile tools run --rm media-migrate up

media-migrate-version:
	@$(COMPOSE) --profile tools run --rm media-migrate version

media-seed: catalog-seed media-migrate-up media-storage-init
	@$(COMPOSE) --profile tools run --rm --build media-seed

playlist-test:
	@cd services/playlist-service && go test ./...

playlist-test-integration: playlist-migrate-up
	@$(COMPOSE) up --detach --build playlist-service
	@$(COMPOSE) --profile tools run --rm --build playlist-integration-test

playlist-migrate-up:
	@$(COMPOSE) --profile tools run --rm playlist-migrate up

playlist-migrate-version:
	@$(COMPOSE) --profile tools run --rm playlist-migrate version

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

catalog-test:
	@cd services/catalog-service && go test ./...

catalog-test-integration: catalog-seed
	@$(COMPOSE) up --detach --build catalog-service
	@$(COMPOSE) --profile tools run --rm --build catalog-integration-test

catalog-migrate-up:
	@$(COMPOSE) --profile tools run --rm catalog-migrate up

catalog-migrate-version:
	@$(COMPOSE) --profile tools run --rm catalog-migrate version

catalog-seed: catalog-migrate-up
	@$(COMPOSE) --profile tools run --rm catalog-seed
