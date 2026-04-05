<p align="center">
  <img src="./assets/relay-forge-wordmark.png" alt="RelayForge" width="460" />
</p>

RelayForge Server is the backend repository for the RelayForge platform. It contains the Go
services, deployment assets, and release automation for the runtime stack.

If you are looking for the full architecture, operations, security, and release handbook, go to
[`xiaotwu.github.io/relay-forge`](https://xiaotwu.github.io/relay-forge/#/server). The public docs
are intentionally published from the main [`relay-forge`](https://github.com/xiaotwu/relay-forge)
repository.

## Components

- `services/api` - REST API, auth, RBAC, guild and message persistence
- `services/realtime` - WebSocket delivery, presence, and fan-out
- `services/media` - uploads, storage coordination, and LiveKit integration
- `services/worker` - background jobs and retention tasks
- `infra/docker` - self-hosted deployment assets

## Quickstart

```bash
cp .env.example .env
make test
make build
make deploy-up
make deploy-migrate
```

## Releases

- Git tags publish backend binary archives to GitHub Releases.
- Container images publish to GitHub Container Registry on tagged releases.
- Detailed release flow and deployment notes live in the shared handbook.

This repository is licensed under the [Apache-2.0 License](./LICENSE).
