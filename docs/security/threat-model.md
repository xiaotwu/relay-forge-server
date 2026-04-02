# RelayForge Threat Model Summary

## Attack Surface

### External Attack Vectors

| Vector               | Mitigation                                                        |
| -------------------- | ----------------------------------------------------------------- |
| Brute force login    | Rate limiting, Argon2id slow hashing, account lockout             |
| JWT theft            | Short-lived tokens (15 min), refresh rotation, session revocation |
| SQL injection        | Parameterized queries via pgx (no string concatenation)           |
| XSS                  | React's built-in escaping, Content-Security-Policy headers        |
| CSRF                 | SameSite cookies, Origin header validation                        |
| File upload abuse    | MIME allowlist, size limits, optional ClamAV scanning             |
| WebSocket hijacking  | JWT auth on connection, origin checking                           |
| DM eavesdropping     | E2EE with Double Ratchet (server cannot read DM content)          |
| Privilege escalation | Server-side RBAC checks on every operation                        |
| Denial of service    | Rate limiting, connection limits, request size limits             |

### Internal / Operator Risks

| Risk                       | Mitigation                                                     |
| -------------------------- | -------------------------------------------------------------- |
| Database leak              | Passwords hashed with Argon2id, DM content E2EE encrypted      |
| JWT secret compromise      | Configurable rotation, short access token TTL                  |
| S3 bucket misconfiguration | Presigned URLs, no public write access by default              |
| Log data leakage           | Structured logging never logs passwords, tokens, or DM content |
| Audit log tampering        | Append-only audit table, admin-only access                     |

## Trust Boundaries

1. **Client <-> API**: All requests authenticated and authorized. Client is never trusted for access decisions.
2. **API <-> Database**: Application-level access control. Database credentials managed via environment variables.
3. **API <-> Valkey**: Used for ephemeral data (presence, typing, pub/sub). No sensitive data stored long-term.
4. **Media <-> S3**: Presigned URLs for direct client upload/download. Server validates MIME and size.
5. **Client <-> LiveKit**: Tokens issued by media service with room-specific grants. LiveKit handles media transport security (DTLS-SRTP).
6. **E2EE DM boundary**: Server stores ciphertext only. Key material never leaves the client in plaintext.

## Known Tradeoffs

- Guild/channel messages are NOT end-to-end encrypted — this is intentional to support search, moderation, and audit. They are protected by TLS in transit and server-side access control.
- TOTP 2FA secrets are stored encrypted on the server — if the database is compromised, these could theoretically be extracted. Consider hardware key support (WebAuthn) in the future.
- Rate limiting is per-IP, which may not be effective behind shared proxies without additional headers (X-Forwarded-For trust must be configured).
