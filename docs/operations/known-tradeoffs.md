# Known Tradeoffs

This document describes intentional design tradeoffs in RelayForge and the rationale behind them.

## E2EE Scope

**Decision:** Only direct messages are end-to-end encrypted. Guild and channel messages are not.

**Rationale:** Guild messages require server-side capabilities that are incompatible with E2EE:

- Full-text search across message history
- Moderation (word filtering, content review, message deletion by admins)
- Audit logging of message actions
- Bot and integration access to messages

Guild messages are protected by TLS in transit and server-side access control. This matches the model used by Signal (1:1 E2EE) vs. Slack/Discord (server-mediated group chat).

## Modular Monolith vs. Microservices

**Decision:** The API service is a modular monolith rather than separate microservices per domain.

**Rationale:** For a self-hostable project targeting small to medium teams:

- Simpler deployment (fewer services to manage)
- No distributed transaction complexity
- Shared database reduces consistency challenges
- Clean internal package boundaries still allow future extraction
- The realtime and media services ARE separated because they have fundamentally different runtime characteristics

## WebSocket vs. SSE

**Decision:** WebSocket for realtime delivery rather than Server-Sent Events.

**Rationale:** WebSocket supports bidirectional communication needed for typing indicators, presence updates, and client-to-server events without additional HTTP requests.

## PostgreSQL as Single Database

**Decision:** All services share one PostgreSQL database.

**Rationale:** Simplifies deployment and ensures transactional consistency. For most self-hosted deployments (up to tens of thousands of users), a single well-indexed PostgreSQL instance is sufficient. Read replicas can be added for scale.

## Argon2id over bcrypt

**Decision:** Argon2id for password hashing instead of bcrypt.

**Rationale:** Argon2id won the Password Hashing Competition and provides better resistance against GPU and ASIC attacks due to its memory-hard design. The tradeoff is slightly higher server memory usage during hashing.

## UUIDv7 Primary Keys

**Decision:** UUIDv7 (time-ordered) for most primary keys instead of auto-increment integers.

**Rationale:** Time-ordered UUIDs enable cursor-based pagination without a separate timestamp column, avoid integer exhaustion concerns, are safe for distributed ID generation, and don't leak information about entity counts. The tradeoff is larger storage per key (16 bytes vs. 4-8 bytes).

## AGPL License

**Decision:** AGPL-3.0-or-later rather than MIT or Apache-2.0.

**Rationale:** The AGPL ensures that modifications to RelayForge remain open source even when deployed as a network service. This protects community contributions while still allowing free use and self-hosting. The tradeoff is that some organizations have policies against AGPL dependencies.

## No Built-in Email Service

**Decision:** SMTP integration for email rather than a built-in email delivery system.

**Rationale:** Email delivery is a solved problem with many good services (Mailgun, SES, SendGrid, self-hosted Postfix). Building a reliable email delivery system is out of scope. The SMTP abstraction allows any provider.
