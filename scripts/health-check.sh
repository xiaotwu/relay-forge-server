#!/usr/bin/env bash
set -euo pipefail

# =============================================================================
# RelayForge Health Check Script
#
# Checks the health of all RelayForge services and reports their status.
#
# Usage:
#   ./scripts/health-check.sh
#   ./scripts/health-check.sh --json
#   ./scripts/health-check.sh --base-url https://relayforge.example.com
#
# Environment variables:
#   HEALTH_BASE_URL  - Base URL for services (default: http://localhost)
#   HEALTH_TIMEOUT   - Request timeout in seconds (default: 5)
# =============================================================================

# Defaults
BASE_URL="${HEALTH_BASE_URL:-http://localhost}"
TIMEOUT="${HEALTH_TIMEOUT:-5}"
OUTPUT_JSON=false
EXIT_CODE=0

# Parse arguments
while [[ $# -gt 0 ]]; do
    case "$1" in
        --json)
            OUTPUT_JSON=true
            shift
            ;;
        --base-url)
            BASE_URL="$2"
            shift 2
            ;;
        --timeout)
            TIMEOUT="$2"
            shift 2
            ;;
        --help|-h)
            echo "Usage: $0 [--json] [--base-url URL] [--timeout SECONDS]"
            exit 0
            ;;
        *)
            echo "Unknown option: $1" >&2
            exit 1
            ;;
    esac
done

# Service definitions: name|url|expected_status
declare -a SERVICES=(
    "api|${BASE_URL}:8080/healthz|200"
    "realtime|${BASE_URL}:8081/healthz|200"
    "media|${BASE_URL}:8082/healthz|200"
)

# Results storage
declare -a RESULTS=()

check_service() {
    local name="$1"
    local url="$2"
    local expected_status="$3"

    local start_time
    start_time=$(date +%s%N)

    local http_code
    local body
    local curl_exit

    body=$(curl \
        --silent \
        --output /dev/stderr \
        --write-out "%{http_code}" \
        --connect-timeout "$TIMEOUT" \
        --max-time "$TIMEOUT" \
        "$url" 2>/dev/null) && curl_exit=0 || curl_exit=$?

    http_code="$body"

    local end_time
    end_time=$(date +%s%N)
    local duration_ms=$(( (end_time - start_time) / 1000000 ))

    local status="healthy"
    if [[ $curl_exit -ne 0 ]]; then
        status="unreachable"
        http_code="000"
        EXIT_CODE=1
    elif [[ "$http_code" != "$expected_status" ]]; then
        status="unhealthy"
        EXIT_CODE=1
    fi

    if [[ "$OUTPUT_JSON" == "true" ]]; then
        RESULTS+=("{\"name\":\"${name}\",\"url\":\"${url}\",\"status\":\"${status}\",\"http_code\":${http_code},\"response_ms\":${duration_ms}}")
    else
        local icon
        case "$status" in
            healthy)     icon="[OK]"   ;;
            unhealthy)   icon="[WARN]" ;;
            unreachable) icon="[FAIL]" ;;
        esac
        printf "  %-8s %-12s  HTTP %-3s  %4dms  %s\n" "$icon" "$name" "$http_code" "$duration_ms" "$url"
    fi
}

# Check PostgreSQL connectivity
check_postgres() {
    local pg_host="${DB_HOST:-localhost}"
    local pg_port="${DB_PORT:-5432}"

    local start_time
    start_time=$(date +%s%N)

    local status="healthy"
    if command -v pg_isready &>/dev/null; then
        if pg_isready -h "$pg_host" -p "$pg_port" -t "$TIMEOUT" >/dev/null 2>&1; then
            status="healthy"
        else
            status="unreachable"
            EXIT_CODE=1
        fi
    else
        # Fallback: try TCP connection
        if timeout "$TIMEOUT" bash -c "echo > /dev/tcp/${pg_host}/${pg_port}" 2>/dev/null; then
            status="healthy"
        else
            status="unreachable"
            EXIT_CODE=1
        fi
    fi

    local end_time
    end_time=$(date +%s%N)
    local duration_ms=$(( (end_time - start_time) / 1000000 ))

    if [[ "$OUTPUT_JSON" == "true" ]]; then
        RESULTS+=("{\"name\":\"postgres\",\"url\":\"${pg_host}:${pg_port}\",\"status\":\"${status}\",\"http_code\":0,\"response_ms\":${duration_ms}}")
    else
        local icon
        case "$status" in
            healthy)     icon="[OK]"   ;;
            unreachable) icon="[FAIL]" ;;
        esac
        printf "  %-8s %-12s  TCP      %4dms  %s:%s\n" "$icon" "postgres" "$duration_ms" "$pg_host" "$pg_port"
    fi
}

# Check Valkey connectivity
check_valkey() {
    local valkey_host="${VALKEY_HOST:-localhost}"
    local valkey_port="${VALKEY_PORT:-6379}"

    local start_time
    start_time=$(date +%s%N)

    local status="healthy"
    if command -v valkey-cli &>/dev/null; then
        if valkey-cli -h "$valkey_host" -p "$valkey_port" ping >/dev/null 2>&1; then
            status="healthy"
        else
            status="unreachable"
            EXIT_CODE=1
        fi
    elif command -v redis-cli &>/dev/null; then
        if redis-cli -h "$valkey_host" -p "$valkey_port" ping >/dev/null 2>&1; then
            status="healthy"
        else
            status="unreachable"
            EXIT_CODE=1
        fi
    else
        if timeout "$TIMEOUT" bash -c "echo > /dev/tcp/${valkey_host}/${valkey_port}" 2>/dev/null; then
            status="healthy"
        else
            status="unreachable"
            EXIT_CODE=1
        fi
    fi

    local end_time
    end_time=$(date +%s%N)
    local duration_ms=$(( (end_time - start_time) / 1000000 ))

    if [[ "$OUTPUT_JSON" == "true" ]]; then
        RESULTS+=("{\"name\":\"valkey\",\"url\":\"${valkey_host}:${valkey_port}\",\"status\":\"${status}\",\"http_code\":0,\"response_ms\":${duration_ms}}")
    else
        local icon
        case "$status" in
            healthy)     icon="[OK]"   ;;
            unreachable) icon="[FAIL]" ;;
        esac
        printf "  %-8s %-12s  TCP      %4dms  %s:%s\n" "$icon" "valkey" "$duration_ms" "$valkey_host" "$valkey_port"
    fi
}

# Run all checks
if [[ "$OUTPUT_JSON" != "true" ]]; then
    echo "RelayForge Health Check"
    echo "======================"
    echo ""
    echo "Services:"
fi

for service_def in "${SERVICES[@]}"; do
    IFS='|' read -r name url expected_status <<< "$service_def"
    check_service "$name" "$url" "$expected_status"
done

if [[ "$OUTPUT_JSON" != "true" ]]; then
    echo ""
    echo "Infrastructure:"
fi

check_postgres
check_valkey

# Output JSON if requested
if [[ "$OUTPUT_JSON" == "true" ]]; then
    echo "{\"timestamp\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\",\"services\":[$(IFS=,; echo "${RESULTS[*]}")]}"
else
    echo ""
    if [[ $EXIT_CODE -eq 0 ]]; then
        echo "All services healthy."
    else
        echo "Some services are unhealthy or unreachable."
    fi
fi

exit $EXIT_CODE
