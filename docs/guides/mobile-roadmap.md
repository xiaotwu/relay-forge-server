# Mobile Roadmap

## Current State

RelayForge does not include native iOS or Android apps in the initial release. The architecture supports mobile through:

- **Mobile-compatible REST API** — all operations available via HTTP
- **WebSocket realtime** — standard WebSocket protocol works on mobile
- **Responsive web layout** — basic usability on mobile browsers
- **PWA support** — installable from the browser with offline capability
- **Typed SDK** — `@relayforge/sdk` can be used in React Native

## Recommended Path: React Native

### Why React Native

1. **Code sharing** — reuse the existing `@relayforge/sdk`, `@relayforge/types`, and `@relayforge/crypto` packages
2. **Design system** — the existing Tailwind-based design tokens can inform NativeWind styling
3. **Single codebase** — one React Native app for iOS and Android
4. **LiveKit support** — LiveKit provides official React Native SDKs
5. **E2EE compatibility** — Web Crypto API polyfills exist for React Native
6. **Team efficiency** — TypeScript and React knowledge transfers directly

### Implementation Plan

#### Phase 1: Core Messaging

- Authentication (login, register, 2FA)
- Guild list and navigation
- Channel list and navigation
- Message timeline with pull-to-refresh
- Message composer
- Push notifications (FCM / APNs)

#### Phase 2: Rich Features

- File uploads with camera integration
- Image/video previews
- Emoji reactions and picker
- Reply and thread support
- Search
- User profile and settings

#### Phase 3: Voice and Video

- LiveKit React Native SDK integration
- Voice channel joining
- P2P and group calls
- Screen sharing (platform-permitting)

#### Phase 4: Polish

- Offline message queue
- Background sync
- Deep linking
- E2EE DM integration
- Biometric authentication

### Alternative Approaches Considered

| Approach                       | Pros                                 | Cons                                                  |
| ------------------------------ | ------------------------------------ | ----------------------------------------------------- |
| **React Native** (recommended) | Code reuse, single team, LiveKit SDK | Bridge overhead for heavy media                       |
| **Flutter**                    | Native performance, single codebase  | No code sharing with existing TS packages             |
| **Native (Swift + Kotlin)**    | Best performance, platform APIs      | Double the development effort, separate teams         |
| **PWA only**                   | Zero additional code                 | Limited push notifications, no background sync on iOS |

## Architecture Compatibility

The API is designed for mobile from the start:

- JWT auth works with mobile token storage (secure enclave / keychain)
- Cursor-based pagination for efficient scrolling
- Presigned URLs for direct S3 upload (avoids proxying large files)
- WebSocket reconnection with state recovery
- Typed SDK works in React Native with minimal adaptation
