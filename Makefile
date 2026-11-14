.DEFAULT_GOAL := help

.PHONY: help config ps up stop down down-volumes restart logs logs-ci smoke logging-smoke system-smoke-up system-smoke system-smoke-logs content-media-smoke-up content-media-smoke content-media-smoke-logs kafka-events-test-integration creator-listener-smoke _creator-listener-smoke-stack frontend-install frontend-dev frontend-build frontend-typecheck frontend-lint frontend-test media-storage-init media-storage-test media-test media-test-integration media-migrate-up media-migrate-version media-seed playlist-test playlist-test-integration playlist-migrate-up playlist-migrate-version content-test content-test-integration content-migrate-up content-migrate-version interaction-test interaction-test-integration interaction-migrate-up interaction-migrate-version comment-test comment-test-integration comment-migrate-up comment-migrate-version history-test history-test-integration history-migrate-up history-migrate-version recommendation-test recommendation-test-integration recommendation-migrate-up recommendation-migrate-version auth-test auth-test-integration auth-migrate-up auth-migrate-version auth-bootstrap-admin user-test user-test-integration user-migrate-up user-migrate-version catalog-test catalog-test-integration catalog-migrate-up catalog-migrate-version catalog-seed

COMPOSE_OVERRIDE ?=
COMPOSE := docker compose -f compose.yml -f compose.kafka.yml -f compose.observability.yml -f compose.logging.yml -f compose.auth.yml -f compose.user.yml -f compose.catalog.yml -f compose.media.yml -f compose.media-service.yml -f compose.playlist.yml -f compose.content.yml -f compose.interaction.yml -f compose.comment.yml -f compose.history.yml -f compose.recommendation.yml -f compose.frontend.yml $(COMPOSE_OVERRIDE)
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
	@echo "  logs-ci     - Print current container logs without following"
	@echo "  smoke       - Run basic service health checks"
	@echo "  logging-smoke - Start and verify Loki, Alloy and Grafana"
	@echo "  system-smoke-up - Start the services required by the full-stack API smoke test"
	@echo "  system-smoke    - Run the full-stack API user journey through Kong"
	@echo "  system-smoke-logs - Print logs for services used by the system smoke test"
	@echo "  content-media-smoke - Verify content publication and media playback through Kong"
	@echo "  content-media-smoke-logs - Print logs for the content and media smoke test"
	@echo "  creator-listener-smoke - Verify a complete real-service journey in disposable Compose"
	@echo "  kafka-events-test-integration - Verify DLQ routing and replay against Kafka"
	@echo "  frontend-install   - Install locked frontend dependencies"
	@echo "  frontend-dev       - Start the frontend development server"
	@echo "  frontend-build     - Build the production frontend bundle"
	@echo "  frontend-typecheck - Type-check the frontend"
	@echo "  frontend-lint      - Lint the frontend"
	@echo "  frontend-test      - Run frontend unit tests"
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
	@echo "  content-test             - Run Content Service unit tests"
	@echo "  content-test-integration - Run Content integration tests"
	@echo "  content-migrate-up       - Apply pending Content database migrations"
	@echo "  content-migrate-version  - Show the current Content migration version"
	@echo "  interaction-test             - Run Interaction Service unit tests"
	@echo "  interaction-test-integration - Run Interaction integration tests"
	@echo "  interaction-migrate-up       - Apply pending Interaction database migrations"
	@echo "  interaction-migrate-version  - Show the current Interaction migration version"
	@echo "  comment-test             - Run Comment Service unit tests"
	@echo "  comment-test-integration - Run Comment integration tests"
	@echo "  comment-migrate-up       - Apply pending Comment database migrations"
	@echo "  comment-migrate-version  - Show the current Comment migration version"
	@echo "  history-test             - Run History Service unit tests"
	@echo "  history-test-integration - Run History integration tests"
	@echo "  history-migrate-up       - Apply pending History database migrations"
	@echo "  history-migrate-version  - Show the current History migration version"
	@echo "  recommendation-test             - Run Recommendation Service unit tests"
	@echo "  recommendation-test-integration - Run Recommendation integration tests"
	@echo "  recommendation-migrate-up       - Apply pending Recommendation database migrations"
	@echo "  recommendation-migrate-version  - Show the current Recommendation migration version"
	@echo "  auth-test   - Run Auth Service unit tests"
	@echo "  auth-test-integration - Run Auth repository integration tests"
	@echo "  auth-migrate-up      - Apply pending Auth database migrations"
	@echo "  auth-migrate-version - Show the current Auth migration version"
	@echo "  auth-bootstrap-admin - Promote a registered user (ADMIN_EMAIL=...)"
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

up: auth-migrate-up user-migrate-up catalog-migrate-up media-migrate-up playlist-migrate-up content-migrate-up interaction-migrate-up comment-migrate-up history-migrate-up recommendation-migrate-up media-storage-init
	@$(COMPOSE) up --detach

stop:
	@$(COMPOSE) stop

down:
	@$(COMPOSE) --profile tools down
down-volumes:
	@$(COMPOSE) --profile tools down --volumes

restart:
	@$(COMPOSE) restart

logs:
	@$(COMPOSE) logs --follow

logs-ci:
	@$(COMPOSE) logs --no-color

smoke:
	@bash scripts/smoke.sh

logging-smoke:
	@$(COMPOSE) up --detach prometheus loki alloy grafana
	@bash scripts/test-logging.sh

system-smoke-up:
	@$(MAKE) auth-migrate-up user-migrate-up catalog-seed playlist-migrate-up interaction-migrate-up comment-migrate-up history-migrate-up
	@$(COMPOSE) up --detach --wait postgres redis kafka kafka-init
	@$(COMPOSE) build auth-service user-service catalog-service playlist-service interaction-service comment-service history-service
	@$(COMPOSE) up --detach --wait --wait-timeout 180 --no-deps auth-service
	@$(COMPOSE) up --detach --wait --wait-timeout 180 --no-deps user-service catalog-service playlist-service interaction-service comment-service history-service
	@$(COMPOSE) up --detach --wait --wait-timeout 180 --no-deps kong

system-smoke: system-smoke-up
	@GATEWAY_URL="$${GATEWAY_URL:-http://127.0.0.1:$${SYSTEM_TEST_GATEWAY_PORT:-18000}}" bash scripts/system-smoke.sh

system-smoke-logs:
	@$(COMPOSE) logs --no-color auth-service user-service catalog-service playlist-service interaction-service comment-service history-service kong

content-media-smoke-up:
	@$(MAKE) auth-migrate-up user-migrate-up catalog-migrate-up media-migrate-up content-migrate-up media-storage-init
	@$(COMPOSE) up --detach --wait postgres redis kafka kafka-init minio
	@$(COMPOSE) build auth-service user-service catalog-service media-service content-service
	@$(COMPOSE) up --detach --wait --wait-timeout 180 --no-deps auth-service
	@$(COMPOSE) up --detach --wait --wait-timeout 180 --no-deps user-service catalog-service media-service
	@$(COMPOSE) up --detach --wait --wait-timeout 180 --no-deps content-service
	@$(COMPOSE) up --detach --wait --wait-timeout 180 --no-deps kong

content-media-smoke: content-media-smoke-up
	@GATEWAY_URL="$${GATEWAY_URL:-http://127.0.0.1:$${SYSTEM_TEST_GATEWAY_PORT:-18000}}" bash scripts/content-media-smoke.sh

content-media-smoke-logs:
	@$(COMPOSE) logs --no-color auth-service user-service catalog-service media-service content-service kafka kong

# Public entry point creates and removes a uniquely named isolated project.
creator-listener-smoke:
	@bash scripts/run-creator-listener-smoke.sh

# Internal target: never run this against the main development project.
_creator-listener-smoke-stack:
	@case "$${COMPOSE_PROJECT_NAME:-}" in infinitune-creator-listener-*) ;; *) echo "Refusing non-isolated system smoke" >&2; exit 1;; esac
	@$(MAKE) auth-migrate-up user-migrate-up catalog-migrate-up media-migrate-up playlist-migrate-up content-migrate-up interaction-migrate-up comment-migrate-up history-migrate-up recommendation-migrate-up media-storage-init
	@$(COMPOSE) up --detach --wait postgres redis kafka kafka-init minio
	@$(COMPOSE) up --detach --wait --wait-timeout 240 --build auth-service
	@$(COMPOSE) up --detach --wait --wait-timeout 240 --build user-service catalog-service media-service playlist-service interaction-service comment-service history-service content-service recommendation-service kong

kafka-events-test-integration:
	@$(COMPOSE) --profile tools run --rm --build kafka-consumer-integration-test
	@$(COMPOSE) --profile tools run --rm --build kafka-replay-integration-test

frontend-install:
	@cd frontend && npm ci

frontend-dev:
	@cd frontend && npm run dev

frontend-build:
	@cd frontend && npm run build

frontend-typecheck:
	@cd frontend && npm run typecheck

frontend-lint:
	@cd frontend && npm run lint

frontend-test:
	@cd frontend && npm test

media-storage-init:
	@$(MEDIA_COMPOSE) up --detach --wait --build minio
	@$(MEDIA_COMPOSE) --profile tools run --rm --build minio-init

media-storage-test: media-storage-init
	@bash scripts/test-minio.sh

media-test:
	@cd services/media-service && go test ./...

media-test-integration: media-migrate-up media-storage-init
	@$(COMPOSE) up --detach --wait --no-deps --build media-service
	@$(COMPOSE) --profile tools run --rm --no-deps --build media-integration-test

media-migrate-up:
	@$(COMPOSE) --profile tools run --rm media-migrate up

media-migrate-version:
	@$(COMPOSE) --profile tools run --rm media-migrate version

media-seed: catalog-seed media-migrate-up media-storage-init
	@$(COMPOSE) --profile tools run --rm --build media-seed

playlist-test:
	@cd services/playlist-service && go test ./...

playlist-test-integration: playlist-migrate-up
	@$(COMPOSE) up --detach --wait --no-deps --build playlist-service
	@$(COMPOSE) --profile tools run --rm --no-deps --build playlist-integration-test

playlist-migrate-up:
	@$(COMPOSE) --profile tools run --rm playlist-migrate up

playlist-migrate-version:
	@$(COMPOSE) --profile tools run --rm playlist-migrate version

content-test:
	@cd services/content-service && go test ./...

content-test-integration: content-migrate-up
	@$(COMPOSE) up --detach --wait --no-deps --build content-service
	@$(COMPOSE) --profile tools run --rm --no-deps --build content-integration-test

content-migrate-up:
	@$(COMPOSE) --profile tools run --rm content-migrate up

content-migrate-version:
	@$(COMPOSE) --profile tools run --rm content-migrate version

interaction-test:
	@cd services/interaction-service && go test ./...

interaction-test-integration: interaction-migrate-up comment-migrate-up
	@$(COMPOSE) up --detach --wait --no-deps --build comment-service interaction-service
	@$(COMPOSE) --profile tools run --rm --no-deps --build interaction-integration-test

interaction-migrate-up:
	@$(COMPOSE) --profile tools run --rm interaction-migrate up

interaction-migrate-version:
	@$(COMPOSE) --profile tools run --rm interaction-migrate version

comment-test:
	@cd services/comment-service && go test ./...

comment-test-integration: comment-migrate-up
	@$(COMPOSE) up --detach --wait --no-deps --build comment-service
	@$(COMPOSE) --profile tools run --rm --no-deps --build comment-integration-test

comment-migrate-up:
	@$(COMPOSE) --profile tools run --rm comment-migrate up

comment-migrate-version:
	@$(COMPOSE) --profile tools run --rm comment-migrate version

history-test:
	@cd services/history-service && go test ./...

history-test-integration: history-migrate-up
	@$(COMPOSE) up --detach --wait --no-deps --build history-service
	@$(COMPOSE) --profile tools run --rm --no-deps --build history-integration-test

history-migrate-up:
	@$(COMPOSE) --profile tools run --rm history-migrate up

history-migrate-version:
	@$(COMPOSE) --profile tools run --rm history-migrate version

recommendation-test:
	@cd services/recommendation-service && go test ./...

recommendation-test-integration: recommendation-migrate-up
	@$(COMPOSE) up --detach --wait --no-deps --build recommendation-service
	@$(COMPOSE) --profile tools run --rm --no-deps --build recommendation-integration-test

recommendation-migrate-up:
	@$(COMPOSE) --profile tools run --rm recommendation-migrate up

recommendation-migrate-version:
	@$(COMPOSE) --profile tools run --rm recommendation-migrate version

auth-test:
	@cd services/auth-service && go test ./...

auth-test-integration: auth-migrate-up
	@$(COMPOSE) up --detach --wait redis
	@$(COMPOSE) --profile tools run --rm --no-deps --build auth-integration-test

auth-migrate-up:
	@$(COMPOSE) --profile tools run --rm auth-migrate up

auth-migrate-version:
	@$(COMPOSE) --profile tools run --rm auth-migrate version

auth-bootstrap-admin: auth-migrate-up
	@ADMIN_EMAIL="$(ADMIN_EMAIL)" $(COMPOSE) --profile tools run --rm --no-deps --build auth-admin-bootstrap

user-test:
	@cd services/user-service && go test ./...

user-test-integration: user-migrate-up
	@$(COMPOSE) --profile tools run --rm --no-deps --build user-integration-test

user-migrate-up:
	@$(COMPOSE) --profile tools run --rm user-migrate up

user-migrate-version:
	@$(COMPOSE) --profile tools run --rm user-migrate version

catalog-test:
	@cd services/catalog-service && go test ./...

catalog-test-integration: catalog-seed
	@$(COMPOSE) up --detach --wait --no-deps --build catalog-service
	@$(COMPOSE) --profile tools run --rm --no-deps --build catalog-integration-test

catalog-migrate-up:
	@$(COMPOSE) --profile tools run --rm catalog-migrate up

catalog-migrate-version:
	@$(COMPOSE) --profile tools run --rm catalog-migrate version

catalog-seed: catalog-migrate-up
	@$(COMPOSE) --profile tools run --rm catalog-seed
