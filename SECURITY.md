# Security Policy

## Reporting Vulnerabilities

Do not open public issues for RelayForge vulnerabilities. Report privately with the affected
component, reproduction steps, impact, logs or traces when available, and whether the issue affects
the client repo, server repo, or both.

## Runtime Security Model

The server is authoritative for authentication, authorization, guild/channel access, realtime
subscriptions, media ownership, and admin operations.

Current controls:

- JWT access tokens are short-lived and protected API routes recheck that the user still exists and
  is not disabled.
- Admin disable deletes sessions, revokes refresh tokens, and causes existing access tokens to fail
  API middleware and realtime validation before their natural TTL.
- Admin routes require the `admin` role in addition to a valid token.
- Realtime sockets require `?token=<access-token>`, validate requested guild subscriptions against
  membership, and periodically recheck disabled-user status.
- API message mutations publish uppercase events to Valkey channel `relayforge.events`; realtime
  fans out only to validated guild or user recipients.
- Media upload completion requires the pending uploader, a valid owner context, matching object key,
  matching size, and matching content type when storage reports it.
- Media reads use recipient-level ACLs for `dm_channel`, `channel`, `guild`, `pending`, and
  `user_profile` owner contexts before redirecting to storage.
- API, media, and realtime production config reject wildcard CORS/origin settings.
- Worker retention jobs target migrated schema fields (`file_uploads.status`, `messages.deleted_at`,
  and `dm_messages.deleted_at`) instead of stale columns.

## Media ACL Rules

- Pending uploads can be completed or mutated only by the uploader.
- DM media can be read only by DM participants.
- Channel media can be read only by users with guild membership and channel access.
- Guild media can be read only by guild members.
- Profile media is public by design.
- Browser media tags may authenticate through the `token` query parameter; short-lived scoped media
  read tokens are the recommended future improvement.

## Operational Requirements

- Use a strong `AUTH_JWT_SECRET` in every non-local environment.
- Set explicit `API_CORS_ORIGINS`, `MEDIA_CORS_ORIGINS`, and `REALTIME_ALLOWED_ORIGINS` in
  production.
- Keep PostgreSQL, Valkey, S3/MinIO, LiveKit, SMTP, and release credentials out of source control.
- Run migrations before starting workers or media services that depend on upload ACL columns.
- Treat generated storage presigned URLs as short-lived secrets.

## Known Limitations

- Browser media rendering currently uses token query authentication for image/video elements.
  Replace this with short-lived scoped media read tokens when the media token issuer is added.
- Antivirus scanning is optional. When disabled, stale pending uploads can be marked `skipped` by
  the worker instead of scanned.
