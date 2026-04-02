# Production Readiness Checklist

## Security

- [ ] Change `AUTH_JWT_SECRET` to a strong random string (64+ characters)
- [ ] Set `RELAY_ENV=production`
- [ ] Enable TLS/HTTPS via reverse proxy
- [ ] Set `DB_SSL_MODE=require` (or `verify-full` if possible)
- [ ] Set strong `DB_PASSWORD` and `VALKEY_PASSWORD`
- [ ] Set strong `S3_ACCESS_KEY` and `S3_SECRET_KEY`
- [ ] Set strong `LIVEKIT_API_KEY` and `LIVEKIT_API_SECRET`
- [ ] Review and restrict `API_CORS_ORIGINS`
- [ ] Enable rate limiting (`RATE_LIMIT_ENABLED=true`)
- [ ] Review `UPLOAD_ALLOWED_MIME_TYPES` and `UPLOAD_MAX_FILE_SIZE`
- [ ] Consider enabling antivirus scanning (`ANTIVIRUS_ENABLED=true`)
- [ ] Configure security headers in reverse proxy (HSTS, CSP, X-Frame-Options, etc.)
- [ ] Review firewall rules — only expose ports 80/443 publicly

## Database

- [ ] PostgreSQL is running with appropriate resource limits
- [ ] Connection pooling is configured (`DB_MAX_OPEN_CONNS`, `DB_MAX_IDLE_CONNS`)
- [ ] Automated backups are configured (see `scripts/backup.sh`)
- [ ] Backup restoration has been tested
- [ ] Migrations have been applied (`make deploy-migrate ENV_FILE=.env.production` for the default Docker deployment)

## Deployment

- [ ] All services are using production Docker images (not dev mode)
- [ ] Resource limits are set for containers/pods
- [ ] Health checks are configured and monitored
- [ ] Reverse proxy is configured with SSL certificates
- [ ] LiveKit is deployed and accessible (or the bundled Compose service is healthy)
- [ ] S3-compatible storage is available and buckets are created (or the bundled MinIO service is healthy)
- [ ] DNS records are configured
- [ ] Container restart policies are set (`unless-stopped` or equivalent)

## Observability

- [ ] Structured logging is enabled (`RELAY_LOG_FORMAT=json`)
- [ ] Log level is set appropriately (`RELAY_LOG_LEVEL=info` for production)
- [ ] Prometheus metrics are being scraped
- [ ] Health check endpoints are monitored
- [ ] Alerting is configured for critical failures
- [ ] OpenTelemetry tracing is enabled if needed

## Operations

- [ ] Backup schedule is established (daily recommended)
- [ ] Backup retention policy is defined
- [ ] On-call or monitoring rotation is established
- [ ] Runbook for common issues is documented
- [ ] Scaling strategy is understood for expected load
- [ ] Update/upgrade process is documented

## Testing

- [ ] All tests pass (`make test`)
- [ ] Application has been smoke-tested end-to-end
- [ ] Registration, login, guild creation, messaging all work
- [ ] Voice/video channels connect successfully
- [ ] File upload and download work
- [ ] Admin console is accessible and functional
