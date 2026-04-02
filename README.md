# RelayForge Backend

This repository contains the standalone RelayForge backend project.

## Contents

- `services/` - API, realtime, media, and worker services
- `infra/` - backend deployment assets
- `docs/` - backend architecture and operations notes
- `scripts/` - backend maintenance helpers
- `.github/workflows/` - backend CI and release workflows

## Goals

- Build and test independently from the client repository
- Support direct binary deployment as the primary release path
- Retain Docker support inside this backend project only
- Keep the backend repository self-contained

## Quick Start

```bash
cp .env.example .env
make test
make build
```

## Published Docs

The shared RelayForge handbook is published from the client repo's GitHub Pages site:

- `https://xiaotwu.github.io/relay-forge/#/server`

See `infra/README.md` for backend deployment assets and `docs/` for architecture and operations
guides.
