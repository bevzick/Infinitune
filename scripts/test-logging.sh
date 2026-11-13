#!/bin/bash

set -euo pipefail

loki_url="${LOKI_URL:-http://127.0.0.1:${LOKI_PORT:-3100}}"
alloy_url="${ALLOY_URL:-http://127.0.0.1:${ALLOY_PORT:-12345}}"
grafana_url="${GRAFANA_URL:-http://127.0.0.1:${GRAFANA_PORT:-3000}}"
grafana_user="${GRAFANA_ADMIN_USER:-admin}"
grafana_password="${GRAFANA_ADMIN_PASSWORD:-admin}"
compose_project="${COMPOSE_PROJECT_NAME:-infinitune}"

if ! command -v curl >/dev/null 2>&1; then
    echo "curl is required for logging smoke checks" >&2
    exit 1
fi

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
query_response="$(curl --fail --silent --show-error --get \
    "$loki_url/loki/api/v1/query_range" \
    --data-urlencode "query={compose_project=\"$compose_project\"}" \
    --data-urlencode "limit=1")"
if ! grep --quiet '"result"[[:space:]]*:[[:space:]]*\[{' <<<"$query_response"; then
    echo "Loki has no logs for Compose project $compose_project" >&2
    exit 1
fi

echo "Centralized logging smoke checks passed!"
