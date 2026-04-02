# RelayForge Roadmap

## Current Status: v0.1.0 (Initial Release)

Core platform with guilds, channels, messaging, voice/video, E2EE DMs, moderation, admin console, web app, and desktop app.

---

## Short-Term (v0.2.x - v0.3.x)

### Performance and Stability

- Database query optimization and connection pool tuning
- WebSocket connection handling hardening (reconnect, backpressure)
- Message list virtualization performance improvements
- Image/media lazy loading and progressive rendering

### Feature Completion

- Voice messages (WhatsApp-style voice notes)
- Advanced message search with filters (author, date range, has:file, has:link)
- Polls with timer and results display
- Draft sync across devices
- Notification preferences per guild/channel
- Read receipt optimization (batch updates)

### Security Hardening

- WebAuthn / passkey support for 2FA
- CSRF token implementation for cookie-based auth flows
- IP-based session anomaly detection
- Configurable password policy rules

---

## Medium-Term (v0.4.x - v0.6.x)

### Mobile Applications

- **Recommended approach:** React Native with shared SDK package
- iOS and Android apps with push notifications
- Offline message queue and sync
- Camera/microphone integration for calls
- PWA improvements as an interim step

### Enhanced Moderation

- AutoMod rules engine (regex patterns, spam detection)
- Slow mode improvements
- Content reporting workflow with review queues
- Temporary bans with auto-expiry
- User reputation scoring

### Rich Media

- Link preview generation (OpenGraph, oEmbed)
- GIF search integration (Tenor/Giphy API)
- Media gallery view per channel
- Video/audio message playback
- Image compression and thumbnail generation

### Push Notifications

- Web Push API for browser notifications
- FCM/APNs for mobile push
- Notification batching and digest mode
- Do not disturb schedules

---

## Long-Term (v1.0+)

### Federation and Bridges

- ActivityPub or Matrix protocol bridge exploration
- Instance-to-instance guild sharing
- Cross-instance direct messaging
- Federation discovery and trust model

### Advanced Search

- Meilisearch or Elasticsearch integration
- Full-text search across all message history
- Faceted search (by author, channel, date, type)
- Search result highlighting and context

### Internationalization (i18n)

- Centralized message catalog system
- UI translation workflow
- RTL language support
- Date, time, and number formatting localization

### Plugin / Extension System

- Bot framework with WebSocket and HTTP APIs
- Custom slash commands
- Webhook integrations (incoming and outgoing)
- Theme/appearance customization plugins

### Multi-Tenancy

- Tenant isolation at the database level
- Per-tenant configuration and branding
- Tenant management admin interface
- Usage metering and quotas

### Scalability

- Read replicas for database queries
- Horizontal WebSocket gateway scaling with Valkey pub/sub
- CDN integration for static assets and media
- Queue-based background job processing (NATS or similar)
- Database sharding strategy for very large deployments

---

## Non-Goals

These features are intentionally out of scope:

- **Paid subscription / Nitro features** — RelayForge is free and open source
- **Gaming integrations** — focus is on communication, not gaming activity
- **AI chatbots** — may be added as a plugin, not core
- **Blockchain / crypto integration** — not relevant to the product
- **Social media features** — no feeds, stories, or public profiles
