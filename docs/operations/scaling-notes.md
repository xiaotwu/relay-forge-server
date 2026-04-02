# Scaling Notes

## Target Scale

RelayForge is designed for small to medium deployments:

- **Single instance:** 1-1,000 concurrent users
- **Small team production:** 1,000-10,000 concurrent users
- **Medium scale:** 10,000-50,000 concurrent users (requires tuning)

## Bottleneck Analysis

### Database (PostgreSQL)

The most likely bottleneck for most deployments.

**Mitigations:**

- Use appropriate indexes (already defined in the schema)
- Cursor-based pagination for message queries (avoids OFFSET performance degradation)
- Connection pooling via pgxpool (configurable `DB_MAX_OPEN_CONNS`)
- Consider read replicas for heavy read workloads (search, member lists)
- Partial indexes on `deleted_at IS NULL` for soft-deleted tables
- GIN index on `messages.content` for full-text search

**At scale (50k+ users):**

- Consider dedicated search via Meilisearch/Elasticsearch
- Partition the messages table by channel_id or created_at
- Archive old messages to cold storage

### WebSocket Connections (Realtime Service)

**Mitigations:**

- Configurable `REALTIME_MAX_CONNECTIONS` limit
- Multiple realtime service instances with Valkey pub/sub for cross-instance fan-out
- Presence and typing data is ephemeral (stored in Valkey, not PostgreSQL)
- Connection ping/pong for stale connection detection

**At scale:**

- Run multiple realtime instances behind a load balancer with sticky sessions
- Use Valkey pub/sub channels per guild for efficient fan-out
- Consider dedicated presence service at very high scale

### Object Storage (S3)

**Mitigations:**

- Presigned URLs allow direct client-to-S3 upload/download (no proxy through the media service)
- CDN can be placed in front of S3 for read-heavy workloads
- Upload size limits prevent abuse

### Voice/Video (LiveKit)

**Mitigations:**

- LiveKit is designed for horizontal scaling
- Room-based isolation means each voice channel is independent
- Multiple LiveKit nodes can be deployed for capacity
- Token-based auth prevents unauthorized room access

## Horizontal Scaling Guide

| Component  | Scaling Strategy                                           |
| ---------- | ---------------------------------------------------------- |
| API        | Stateless — add instances behind load balancer             |
| Realtime   | Add instances with Valkey pub/sub for cross-node messaging |
| Media      | Stateless — add instances behind load balancer             |
| Worker     | Add instances (jobs are distributed via Valkey queues)     |
| PostgreSQL | Read replicas for reads, vertical scaling for writes       |
| Valkey     | Cluster mode for large deployments                         |
| LiveKit    | Add nodes per LiveKit documentation                        |
| Web        | Static files — serve via CDN                               |

## Resource Recommendations

### Small (< 100 users)

- 1 vCPU, 2 GB RAM for all Go services
- PostgreSQL: 1 vCPU, 1 GB RAM
- Valkey: 256 MB RAM
- Total: ~4 GB RAM

### Medium (100-1,000 users)

- 2 vCPU, 4 GB RAM for Go services
- PostgreSQL: 2 vCPU, 4 GB RAM
- Valkey: 1 GB RAM
- Total: ~10 GB RAM

### Large (1,000-10,000 users)

- 4 vCPU, 8 GB RAM for each Go service (2+ replicas)
- PostgreSQL: 4 vCPU, 16 GB RAM (with read replica)
- Valkey: 4 GB RAM (cluster mode)
- Total: ~60 GB RAM across all nodes
