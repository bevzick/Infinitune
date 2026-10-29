#!/bin/bash

set -euo pipefail

echo "Running smoke checks..."

echo "Checking PostgreSQL..."
docker compose exec --no-tty postgres \
    sh -c 'pg_isready -U "$POSTGRES_USER" -d "$POSTGRES_DB"'

echo "Checking Redis..."
docker compose exec --no-tty redis redis-cli ping

echo "Checking Kong..."
docker compose exec --no-tty kong kong health

echo "All smoke checks passed!"
