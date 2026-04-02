# RelayForge Architecture Overview

## System Overview

RelayForge is a self-hostable chat platform built as a **modular monolith** with separated realtime and media services. The architecture prioritizes deployment simplicity for small teams while maintaining clean boundaries for future scaling.

```
                    ┌─────────────────────────────────────┐
                    │           Load Balancer              │
                    │      (Nginx / Caddy / Traefik)       │
                    └──────┬──────────┬──────────┬────────┘
                           │          │          │
              ┌────────────▼──┐  ┌────▼────┐  ┌──▼──────────┐
              │   Web / SPA   │  │   API   │  │  Realtime   │
              │   (Static)    │  │  :8080  │  │   :8081     │
              └───────────────┘  └────┬────┘  └──────┬──────┘
                                      │              │
                    ┌─────────────────┼──────────────┼──────┐
                    │                 │              │      │
              ┌─────▼────┐     ┌─────▼────┐   ┌────▼───┐  │
              │PostgreSQL │     │  Valkey   │   │LiveKit │  │
              │  :5432    │     │  :6379    │   │ :7880  │  │
              └───────────┘     └──────────┘   └────────┘  │
                                                           │
              ┌───────────┐     ┌──────────┐               │
              │   MinIO   │     │  Worker  │───────────────┘
              │  (S3)     │     │ (Background)
              │  :9000    │     └──────────┘
              └───────────┘
```

## Architectural Principles

1. **Modular monolith over microservices** -- The API service contains all domain logic in well-separated internal packages. This avoids distributed system complexity while maintaining clean boundaries. The realtime and media services are separated because they have fundamentally different runtime characteristics (long-lived connections, media processing).

2. **Server-side authority** -- All permission checks happen on the server. The client never determines access; it only requests and renders.

3. **E2EE only for DMs** -- Guild/channel messages need server-side search, moderation, and audit capabilities that are incompatible with E2EE. DMs use the Double Ratchet protocol for true end-to-end encryption.

4. **Cloud-portable** -- No dependency on any cloud vendor's proprietary services. PostgreSQL, Valkey, S3-compatible storage, and LiveKit can all be self-hosted or run on any cloud.

5. **Observable by default** -- Structured logging, OpenTelemetry traces, Prometheus metrics, and health endpoints are built in.

## Service Boundaries

### API Service (`services/api`)

- HTTP REST API for all CRUD operations
- Authentication (JWT, 2FA, device management)
- Authorization (RBAC with channel-level overrides)
- Guild, channel, message, role management
- Search and filtering
- Admin operations
- Audit logging
- E2EE key bundle distribution for DMs
- Database migrations

### Realtime Service (`services/realtime`)

- WebSocket gateway
- Authenticated connections (JWT verification)
- Real-time message delivery
- Presence tracking
- Typing indicators
- Read receipt distribution
- Guild/channel event broadcasting
- Valkey pub/sub for cross-instance fan-out

### Media Service (`services/media`)

- File upload handling (chunked, presigned URLs)
- MIME validation
- Antivirus scanning integration (ClamAV)
- S3-compatible storage operations
- LiveKit room management and token generation
- Media metadata extraction

### Worker Service (`services/worker`)

- Email delivery (verification, password reset)
- Audit log archival
- Data retention enforcement
- File cleanup
- Antivirus scan queue processing
- Scheduled maintenance tasks

## Data Ownership

| Domain                         | Owner         | Storage            |
| ------------------------------ | ------------- | ------------------ |
| Users, auth, sessions, devices | API           | PostgreSQL         |
| Guilds, channels, categories   | API           | PostgreSQL         |
| Roles, permissions, overrides  | API           | PostgreSQL         |
| Messages, threads, reactions   | API           | PostgreSQL         |
| DM E2EE key bundles            | API           | PostgreSQL         |
| Audit logs                     | API           | PostgreSQL         |
| Presence, typing               | Realtime      | Valkey (ephemeral) |
| File metadata                  | API           | PostgreSQL         |
| File blobs                     | Media         | S3                 |
| Voice/video rooms              | Media/LiveKit | LiveKit + Valkey   |
| Background jobs                | Worker        | Valkey (queues)    |

## WebSocket Authentication

1. Client connects to the realtime service with a JWT access token as a query parameter or in the first message
2. Realtime service validates the JWT signature and expiration
3. On success, the connection is associated with the user ID and subscribed to relevant channels
4. The realtime service checks guild membership and channel permissions before delivering events
5. Token refresh is handled by the client via the API; a new token is sent over the WebSocket to re-authenticate

## Permission Evaluation

Permissions are evaluated server-side using a layered model:

1. **System-level**: Is the user a system admin?
2. **Guild-level**: What roles does the user have in this guild?
3. **Channel-level**: Are there permission overrides for this channel?
4. **Owner protection**: The guild owner cannot be removed or demoted. Admins cannot escalate beyond the owner's authority.

Each permission is a bit flag. Channel overrides can explicitly allow or deny specific permissions, overriding the role defaults.

## DM E2EE Model

- Uses X3DH (Extended Triple Diffie-Hellman) for initial key exchange
- Uses the Double Ratchet algorithm for ongoing message encryption
- Each device has its own identity key pair, signed prekey, and one-time prekeys
- Key bundles are published to the server but the server never sees plaintext DM content
- Device revocation removes key bundles and notifies peers
- Multi-device: each device maintains its own ratchet session; messages are encrypted per-device

## LiveKit Integration

- Voice channels map 1:1 to LiveKit rooms
- Room names follow the pattern: `guild:{guild_id}:voice:{channel_id}`
- P2P calls use rooms named: `dm:{sorted_user_ids}`
- Group calls use rooms named: `group:{call_id}`
- The media service issues LiveKit tokens with appropriate permissions
- Room lifecycle is managed by the media service (create on first join, destroy on last leave)
