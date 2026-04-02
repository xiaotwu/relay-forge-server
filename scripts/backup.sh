#!/usr/bin/env bash
set -euo pipefail

# =============================================================================
# RelayForge PostgreSQL Backup Script
#
# Usage:
#   ./scripts/backup.sh
#
# Environment variables (or set in .env):
#   DB_HOST          - PostgreSQL host (default: localhost)
#   DB_PORT          - PostgreSQL port (default: 5432)
#   DB_USER          - PostgreSQL user (default: relayforge)
#   DB_NAME          - Database name (default: relayforge)
#   PGPASSWORD       - PostgreSQL password (required)
#   BACKUP_DIR       - Local backup directory (default: /tmp/relayforge-backups)
#   BACKUP_RETENTION - Number of local backups to keep (default: 7)
#   S3_BUCKET        - S3 bucket for remote backup storage (optional)
#   S3_PREFIX        - S3 key prefix (default: backups/postgres)
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
BACKUP_DIR="${BACKUP_DIR:-/tmp/relayforge-backups}"
BACKUP_RETENTION="${BACKUP_RETENTION:-7}"
S3_BUCKET="${S3_BUCKET:-}"
S3_PREFIX="${S3_PREFIX:-backups/postgres}"
S3_ENDPOINT="${S3_ENDPOINT:-}"

TIMESTAMP="$(date -u +%Y%m%d_%H%M%S)"
BACKUP_FILE="${BACKUP_DIR}/${DB_NAME}_${TIMESTAMP}.sql.gz"

log() {
    echo "[$(date -u +%Y-%m-%dT%H:%M:%SZ)] $*"
}

error() {
    log "ERROR: $*" >&2
}

# Verify pg_dump is available
if ! command -v pg_dump &>/dev/null; then
    error "pg_dump is not installed or not in PATH"
    exit 1
fi

# Verify password is set
if [[ -z "${PGPASSWORD:-}" ]]; then
    # Try DB_PASSWORD as fallback
    if [[ -n "${DB_PASSWORD:-}" ]]; then
        export PGPASSWORD="$DB_PASSWORD"
    else
        error "PGPASSWORD or DB_PASSWORD must be set"
        exit 1
    fi
fi

# Create backup directory
mkdir -p "$BACKUP_DIR"

# Perform the backup
log "Starting backup of ${DB_NAME}@${DB_HOST}:${DB_PORT}"
if pg_dump \
    --host="$DB_HOST" \
    --port="$DB_PORT" \
    --username="$DB_USER" \
    --dbname="$DB_NAME" \
    --format=custom \
    --compress=6 \
    --verbose \
    --file="${BACKUP_FILE%.gz}" \
    2>&1 | while IFS= read -r line; do log "  pg_dump: $line"; done; then
    log "Backup created: ${BACKUP_FILE%.gz}"
else
    error "pg_dump failed"
    exit 1
fi

# Compress if using plain format (custom format already compressed)
FINAL_FILE="${BACKUP_FILE%.gz}"
BACKUP_SIZE=$(du -h "$FINAL_FILE" | cut -f1)
log "Backup size: ${BACKUP_SIZE}"

# Upload to S3 if configured
if [[ -n "$S3_BUCKET" ]]; then
    S3_KEY="${S3_PREFIX}/$(basename "$FINAL_FILE")"

    S3_CMD_ARGS=()
    if [[ -n "$S3_ENDPOINT" ]]; then
        S3_CMD_ARGS+=(--endpoint-url "$S3_ENDPOINT")
    fi

    log "Uploading to s3://${S3_BUCKET}/${S3_KEY}"
    if command -v aws &>/dev/null; then
        aws s3 cp "${S3_CMD_ARGS[@]}" "$FINAL_FILE" "s3://${S3_BUCKET}/${S3_KEY}"
        log "Upload complete"
    elif command -v mc &>/dev/null; then
        mc cp "$FINAL_FILE" "s3/${S3_BUCKET}/${S3_KEY}"
        log "Upload complete (via mc)"
    else
        error "Neither aws CLI nor mc (MinIO Client) found; skipping S3 upload"
    fi
fi

# Clean up old local backups
if [[ "$BACKUP_RETENTION" -gt 0 ]]; then
    log "Cleaning up backups older than ${BACKUP_RETENTION} days"
    find "$BACKUP_DIR" -name "${DB_NAME}_*.sql*" -type f -mtime +"$BACKUP_RETENTION" -delete 2>/dev/null || true
    REMAINING=$(find "$BACKUP_DIR" -name "${DB_NAME}_*.sql*" -type f | wc -l)
    log "Local backups remaining: ${REMAINING}"
fi

log "Backup complete: ${FINAL_FILE}"
