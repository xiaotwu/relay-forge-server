#!/usr/bin/env bash
set -euo pipefail

# =============================================================================
# RelayForge PostgreSQL Restore Script
#
# Usage:
#   ./scripts/restore.sh <backup-file>
#   ./scripts/restore.sh s3://bucket/path/to/backup.sql
#
# Environment variables (or set in .env):
#   DB_HOST          - PostgreSQL host (default: localhost)
#   DB_PORT          - PostgreSQL port (default: 5432)
#   DB_USER          - PostgreSQL user (default: relayforge)
#   DB_NAME          - Database name (default: relayforge)
#   PGPASSWORD       - PostgreSQL password (required)
#   S3_ENDPOINT      - S3 endpoint for non-AWS providers (optional)
# =============================================================================

# Load .env if present
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
if [[ -f "$PROJECT_ROOT/.env" ]]; then
    set -a
    # shellcheck source=/dev/null
    source "$PROJECT_ROOT/.env"
    set +a
fi

# Configuration
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_USER="${DB_USER:-relayforge}"
DB_NAME="${DB_NAME:-relayforge}"
S3_ENDPOINT="${S3_ENDPOINT:-}"

log() {
    echo "[$(date -u +%Y-%m-%dT%H:%M:%SZ)] $*"
}

error() {
    log "ERROR: $*" >&2
}

# Verify arguments
if [[ $# -lt 1 ]]; then
    echo "Usage: $0 <backup-file-or-s3-uri>"
    echo ""
    echo "Examples:"
    echo "  $0 /tmp/relayforge-backups/relayforge_20260101_120000.sql"
    echo "  $0 s3://my-bucket/backups/postgres/relayforge_20260101_120000.sql"
    exit 1
fi

BACKUP_SOURCE="$1"

# Verify pg_restore is available
if ! command -v pg_restore &>/dev/null; then
    error "pg_restore is not installed or not in PATH"
    exit 1
fi

# Verify password is set
if [[ -z "${PGPASSWORD:-}" ]]; then
    if [[ -n "${DB_PASSWORD:-}" ]]; then
        export PGPASSWORD="$DB_PASSWORD"
    else
        error "PGPASSWORD or DB_PASSWORD must be set"
        exit 1
    fi
fi

# Download from S3 if needed
RESTORE_FILE="$BACKUP_SOURCE"
TEMP_FILE=""

if [[ "$BACKUP_SOURCE" == s3://* ]]; then
    TEMP_FILE="$(mktemp /tmp/relayforge-restore-XXXXXX.sql)"
    RESTORE_FILE="$TEMP_FILE"

    S3_CMD_ARGS=()
    if [[ -n "$S3_ENDPOINT" ]]; then
        S3_CMD_ARGS+=(--endpoint-url "$S3_ENDPOINT")
    fi

    log "Downloading backup from ${BACKUP_SOURCE}"
    if command -v aws &>/dev/null; then
        aws s3 cp "${S3_CMD_ARGS[@]}" "$BACKUP_SOURCE" "$TEMP_FILE"
    elif command -v mc &>/dev/null; then
        mc cp "${BACKUP_SOURCE/s3:\/\//s3/}" "$TEMP_FILE"
    else
        error "Neither aws CLI nor mc (MinIO Client) found"
        exit 1
    fi
    log "Download complete"
fi

# Verify the backup file exists
if [[ ! -f "$RESTORE_FILE" ]]; then
    error "Backup file not found: ${RESTORE_FILE}"
    exit 1
fi

FILE_SIZE=$(du -h "$RESTORE_FILE" | cut -f1)
log "Backup file: ${RESTORE_FILE} (${FILE_SIZE})"

# Confirmation prompt
echo ""
echo "WARNING: This will restore the database '${DB_NAME}' on ${DB_HOST}:${DB_PORT}"
echo "         All current data in '${DB_NAME}' will be replaced."
echo ""
read -rp "Type 'yes' to continue: " CONFIRM
if [[ "$CONFIRM" != "yes" ]]; then
    log "Restore cancelled"
    [[ -n "$TEMP_FILE" ]] && rm -f "$TEMP_FILE"
    exit 0
fi

# Terminate existing connections
log "Terminating existing connections to ${DB_NAME}"
psql \
    --host="$DB_HOST" \
    --port="$DB_PORT" \
    --username="$DB_USER" \
    --dbname="postgres" \
    --command="SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '${DB_NAME}' AND pid <> pg_backend_pid();" \
    >/dev/null 2>&1 || true

# Drop and recreate database
log "Dropping and recreating database ${DB_NAME}"
psql \
    --host="$DB_HOST" \
    --port="$DB_PORT" \
    --username="$DB_USER" \
    --dbname="postgres" \
    --command="DROP DATABASE IF EXISTS \"${DB_NAME}\";"

psql \
    --host="$DB_HOST" \
    --port="$DB_PORT" \
    --username="$DB_USER" \
    --dbname="postgres" \
    --command="CREATE DATABASE \"${DB_NAME}\" OWNER \"${DB_USER}\";"

# Restore the backup
log "Restoring from backup..."
if pg_restore \
    --host="$DB_HOST" \
    --port="$DB_PORT" \
    --username="$DB_USER" \
    --dbname="$DB_NAME" \
    --no-owner \
    --no-privileges \
    --verbose \
    "$RESTORE_FILE" \
    2>&1 | while IFS= read -r line; do log "  pg_restore: $line"; done; then
    log "Restore completed successfully"
else
    RESTORE_EXIT=$?
    # pg_restore returns non-zero for warnings too, check if database is usable
    if psql --host="$DB_HOST" --port="$DB_PORT" --username="$DB_USER" --dbname="$DB_NAME" --command="SELECT 1;" >/dev/null 2>&1; then
        log "Restore completed with warnings (exit code ${RESTORE_EXIT})"
    else
        error "Restore failed"
        [[ -n "$TEMP_FILE" ]] && rm -f "$TEMP_FILE"
        exit 1
    fi
fi

# Clean up temp file
if [[ -n "$TEMP_FILE" ]]; then
    rm -f "$TEMP_FILE"
fi

log "Database '${DB_NAME}' has been restored successfully"
