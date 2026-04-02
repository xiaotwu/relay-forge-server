# Backend Infrastructure Layout

`infra/` now contains only backend deployment assets.

## Recommended Path

For most self-hosted users, start with `infra/docker/`.

```bash
make deploy-up
make deploy-migrate
```

The Docker path remains available here for the extracted backend project, but binary packaging is
the primary release target.

## Included Assets

- `infra/docker/` — backend Dockerfiles and compose stacks
- reverse-proxy and observability helper configs used by the backend stack
