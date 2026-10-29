.DEFAULT_GOAL := help

.PHONY = help config ps up stop down down-volumes restart logs

COMPOSE := docker compose

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

config:
	@$(COMPOSE) config --quiet

ps:
	@$(COMPOSE) ps

up:
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
