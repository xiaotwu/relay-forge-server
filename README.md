<p align="center">
  <img src="./assets/relay-forge-wordmark.png" alt="RelayForge" width="460" />
</p>

RelayForge Server is the backend repository for the RelayForge platform. It contains the Go
services, deployment assets, and release automation for the runtime stack.

If you are looking for the full architecture, operations, security, and release handbook, go to
[`xiaotwu.github.io/relay-forge`](https://xiaotwu.github.io/relay-forge/#/server). The public docs
are intentionally published from the main [`relay-forge`](https://github.com/xiaotwu/relay-forge)
repository.

## Product Role

RelayForge is a split client/server communications platform. The client provides an
iMessage-inspired communication shell with Discord-like guilds, channels, roles, DMs, media,
presence, voice/video entry points, and an admin console. This repository owns the server-side
runtime for those features.

## Components

- `services/api` - REST API, auth, RBAC, guild and message persistence
- `services/realtime` - JWT-protected WebSocket delivery, presence, and Valkey Pub/Sub fan-out
- `services/media` - uploads, recipient-level media ACLs, storage coordination, and LiveKit
  integration
- `services/worker` - schema-aligned cleanup, upload scan, and retention tasks
- `openapi/relayforge.yaml` - backend-owned API contract consumed by the TypeScript SDK
- `infra/docker` - self-hosted deployment assets

## Quickstart

```bash
cp .env.example .env
make test
make build
make deploy-up
make deploy-migrate
```

`make deploy-up` uses `docker compose` with `infra/docker/docker-compose.yml`. If you run Podman,
use equivalent `podman compose` commands against the same compose file or configure your Docker CLI
compatibility layer.

## Runtime Contract

The client/server runtime contract is endpoint based:

| Client variable | Local target |
| --- | --- |
| `API_BASE_URL` | `http://localhost:8080/api/v1` |
| `WS_URL` | `ws://localhost:8081/ws` |
| `MEDIA_BASE_URL` | `http://localhost:8082/api/v1` |
| `LIVEKIT_URL` | `ws://localhost:7880` |

The server `.env.example` contains service-specific variables for API, realtime, media, worker,
PostgreSQL, Valkey, S3/MinIO, LiveKit, SMTP, rate limits, metrics, and OpenTelemetry. Note that the
server-side `API_BASE_URL` is the API service public origin; client builds use the `/api/v1` API
root.

## Development Commands

```bash
# Start the compose stack and run migrations
make deploy-up
make deploy-migrate

# Run migrations against a locally reachable database
make migrate
make migrate-down

# Optional seed data
make seed

# Verification
make test
make build

# Optional checks
make lint
make package-binaries
```

The build target writes service binaries to tracked `bin/*` paths in this repository. Avoid
committing binary churn unless the binary artifacts are intentionally part of the release/change.

## API Contract

`openapi/relayforge.yaml` is the source of truth for API and media-service routes. The client repo
generates TypeScript path types from this file and requires SDK methods to use typed path builders.
When changing routes:

1. Change the Go route and handler.
2. Update `openapi/relayforge.yaml`.
3. Add or update route conformance tests.
4. From the client repo, run `npm run generate:api` and `npm run check:api-contract`.

Current route families under `/api/v1` include auth, users, guilds, nested guild channels, nested
guild roles, channel messages, DMs, and admin. The media service exposes `/api/v1/media/*` and
`/api/v1/voice/*`.

## Realtime

The API publishes successful message mutations to Valkey channel `relayforge.events`. The realtime
service subscribes to that channel and fans out uppercase external websocket envelopes such as
`MESSAGE_CREATE`, `MESSAGE_UPDATE`, `MESSAGE_DELETE`, `DM_MESSAGE_CREATE`, and
`DM_MESSAGE_DELETE`.

WebSocket connections require `?token=<access-token>`. Requested `guilds` query parameters are
validated against database membership before the socket is subscribed. Active connections recheck
user enabled status periodically and close when a user is disabled.

## Media

Uploads are coordinated by the media service:

1. `POST /api/v1/media/upload/presign` creates a pending upload owned by the uploader.
2. The client uploads bytes to the returned storage URL.
3. `POST /api/v1/media/upload/complete` verifies the pending uploader, owner context, key, size,
   and content type before marking the upload clean.
4. `GET /api/v1/media/files/{fileID}` checks recipient-level ACLs before redirecting to a
   short-lived storage URL.

Supported owner contexts are `pending`, `dm_channel`, `channel`, `guild`, and `user_profile`.
DM media is limited to DM participants. Channel media is limited to guild members with channel
access. Guild media is limited to guild members. Pending uploads are mutable only by the uploader.
Profile media is public by design.

Browser media elements may pass the access token through the `token` query parameter because they
cannot send authorization headers. Short-lived scoped media read tokens are the preferred future
hardening.

## Database And Worker

PostgreSQL is the primary system of record for users, guilds, roles, channels, messages, DMs, E2EE
keys, invites, moderation, uploads, polls, settings, sessions, and audit logs.

Important migrations:

- `000001_initial_schema` - base schema
- `000002_retention_and_upload_status` - upload `skipped` status plus `messages.deleted_at` and
  `dm_messages.deleted_at`
- `000003_media_acl` - upload `owner_type`, `owner_id`, and `completed_at`

The worker expects those migrations. Retention jobs delete only soft-deleted message rows with
`is_deleted = true` and an old `deleted_at`, and file scan fallback uses `file_uploads.status`.

## Admin

Admin routes require a valid JWT plus the `admin` role. Current backend support includes dashboard
stats/activity, users, user enable/disable, guild listing/deletion, audit logs, reports
resolve/dismiss, and settings. Disabling a user marks the account disabled, deletes sessions,
revokes refresh tokens, and causes existing access tokens to be rejected by API middleware and
realtime validation before their natural TTL.

## Security

See [SECURITY.md](./SECURITY.md) for the runtime security model. Highlights:

- server-side auth and disabled-user validation on protected API routes
- admin-only route protection
- realtime JWT auth, guild subscription checks, and periodic disabled-user rechecks
- media ACLs on upload completion and file reads
- upload size/type/object metadata validation
- production rejection of wildcard API/media/realtime CORS origins
- optional antivirus scan flow with `skipped` status when scanning is disabled

## Verification

Known passing checks after the security, realtime, media ACL, worker/schema, and contract passes:

```bash
make test
make build
```

Cross-repo client checks:

```bash
cd ../relay-forge
npm test
npm run typecheck
npm run check:api-contract
```

## Releases

- Git tags publish backend binary archives to GitHub Releases.
- Container images publish to GitHub Container Registry on tagged releases.
- Detailed release flow and deployment notes live in the shared handbook.

This repository is licensed under the [Apache-2.0 License](./LICENSE).
