# Release Checklist

## Pre-Release

- [ ] All tests pass on main branch
- [ ] CHANGELOG is updated with release notes
- [ ] Version numbers are bumped in package.json files
- [ ] Database migrations are forward-compatible
- [ ] Docker images build successfully for amd64 and arm64
- [ ] Documentation is up to date
- [ ] Breaking changes are documented with migration guides

## Release Process

1. Create a release branch: `release/vX.Y.Z`
2. Update version numbers:
   - Root `package.json`
   - All workspace `package.json` files
   - Go service version constants
   - `infra/docker/docker-compose.yml`
3. Update CHANGELOG with release date
4. Create a git tag: `git tag -a vX.Y.Z -m "Release vX.Y.Z"`
5. Push tag: `git push origin vX.Y.Z`
6. GitHub Actions will automatically:
   - Build and push Docker images to ghcr.io
   - Build and deploy documentation to GitHub Pages
7. Create GitHub Release with release notes
8. Merge release branch back to main

## Post-Release

- [ ] Verify Docker images are published
- [ ] Verify documentation site is updated
- [ ] Announce release in appropriate channels
- [ ] Monitor for issues in the first 24 hours
- [ ] Update any deployment guides if needed
