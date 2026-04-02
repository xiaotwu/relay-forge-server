# RelayForge Backend

This directory contains the extracted RelayForge backend project. It is intended to be moved into
its own repository.

## Contents

- `services/` — API, realtime, media, and worker services
- `infra/` — backend deployment assets
- `docs/` — backend architecture and operations notes
- `scripts/` — backend maintenance helpers
- `.github/workflows/` — backend CI and release workflows

## Goals

- Build and test independently from the client repository
- Support direct binary deployment as the primary release path
- Retain Docker support inside this backend project only
- Stay relocatable without depending on files from the parent repository

## Quick Start

```bash
cp .env.example .env
make test
make build
```

See `infra/README.md` for backend deployment assets.
