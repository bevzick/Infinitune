#!/bin/bash

set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."

dotenv_value() {
    local key="$1"
    local line value

    while IFS= read -r line || [[ -n "$line" ]]; do
        line="${line%$'\r'}"
        if [[ "$line" != "$key="* ]]; then
            continue
        fi

        value="${line#*=}"
        value="$(sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//' <<<"$value")"
        if [[ ${#value} -ge 2 ]]; then
            if [[ "$value" == \"*\" ]] || [[ "$value" == \'*\' ]]; then
                value="${value:1:${#value}-2}"
            fi
        fi
        printf '%s' "$value"
        return 0
    done < .env

    return 1
}

# Docker Compose reads .env automatically, while a directly executed shell
# script does not. Import only the values this check consumes and preserve
# explicitly exported variables because they take precedence over .env in
# Compose as well.
if [[ -f .env ]]; then
    for key in COMPOSE_PROJECT_NAME PROMETHEUS_PORT LOKI_PORT ALLOY_PORT GRAFANA_PORT GRAFANA_ADMIN_USER GRAFANA_ADMIN_PASSWORD; do
        if [[ ! -v "$key" ]] && value="$(dotenv_value "$key")"; then
            printf -v "$key" '%s' "$value"
            export "$key"
        fi
    done
fi

prometheus_url="${PROMETHEUS_URL:-http://127.0.0.1:${PROMETHEUS_PORT:-9091}}"
loki_url="${LOKI_URL:-http://127.0.0.1:${LOKI_PORT:-3100}}"
alloy_url="${ALLOY_URL:-http://127.0.0.1:${ALLOY_PORT:-12345}}"
grafana_url="${GRAFANA_URL:-http://127.0.0.1:${GRAFANA_PORT:-3000}}"
grafana_user="${GRAFANA_ADMIN_USER:-admin}"
grafana_password="${GRAFANA_ADMIN_PASSWORD:-admin}"
compose_project="${COMPOSE_PROJECT_NAME:-infinitune}"

for command in curl docker; do
    if ! command -v "$command" >/dev/null 2>&1; then
        echo "$command is required for logging smoke checks" >&2
        exit 1
    fi
done

wait_for_endpoint() {
    local name="$1"
    local url="$2"

    for _ in $(seq 1 30); do
        if curl --fail --silent --output /dev/null "$url"; then
            return 0
        fi
        sleep 1
    done

    echo "$name did not become ready at $url" >&2
    return 1
}

echo "Waiting for Loki..."
wait_for_endpoint "Loki" "$loki_url/ready"

echo "Waiting for Alloy..."
wait_for_endpoint "Alloy" "$alloy_url/-/ready"

echo "Waiting for Grafana..."
wait_for_endpoint "Grafana" "$grafana_url/api/health"

echo "Checking Grafana database..."
grafana_health="$(curl --fail --silent --show-error "$grafana_url/api/health")"
if ! grep --quiet '"database"[[:space:]]*:[[:space:]]*"ok"' <<<"$grafana_health"; then
    echo "Grafana health response does not report a healthy database" >&2
    exit 1
fi

echo "Checking provisioned Grafana dashboard..."
dashboard_response="$(curl --fail --silent --show-error \
    --user "$grafana_user:$grafana_password" \
    "$grafana_url/api/dashboards/uid/infinitune-service-logs")"
if ! grep --quiet '"uid"[[:space:]]*:[[:space:]]*"infinitune-service-logs"' <<<"$dashboard_response"; then
    echo "Infinitune service logs dashboard is not provisioned" >&2
    exit 1
fi

echo "Checking provisioned Grafana alert rules..."
alert_response="$(curl --fail --silent --show-error \
    --user "$grafana_user:$grafana_password" \
    "$grafana_url/api/v1/provisioning/alert-rules")"
for alert_uid in infinitune-http-5xx-burst infinitune-service-panic infinitune-grpc-failure-burst; do
    if ! grep --quiet "\"uid\"[[:space:]]*:[[:space:]]*\"$alert_uid\"" <<<"$alert_response"; then
        echo "Grafana alert rule $alert_uid is not provisioned" >&2
        exit 1
    fi
done

echo "Checking centralized Docker logs..."
mapfile -t prometheus_containers < <(docker ps \
    --filter "label=com.docker.compose.project=$compose_project" \
    --filter "label=com.docker.compose.service=prometheus" \
    --format '{{.ID}}')
if [[ ${#prometheus_containers[@]} -ne 1 ]]; then
    echo "Expected one running Prometheus container for Compose project $compose_project" >&2
    exit 1
fi

# Produce a fresh Docker log so this check does not depend on retained Loki data
# or on whether `docker compose up` had to start an existing container.
docker restart "${prometheus_containers[0]}" >/dev/null
wait_for_endpoint "Prometheus" "$prometheus_url/-/ready"

for _ in $(seq 1 30); do
    query_response="$(curl --fail --silent --show-error --get \
        "$loki_url/loki/api/v1/query_range" \
        --data-urlencode "query={compose_project=\"$compose_project\", service=\"prometheus\"}" \
        --data-urlencode "limit=1")"
    if grep --quiet '"result"[[:space:]]*:[[:space:]]*\[{' <<<"$query_response"; then
        echo "Centralized logging smoke checks passed!"
        exit 0
    fi
    sleep 1
done

echo "Prometheus Docker logs did not reach Loki through Alloy" >&2
exit 1
