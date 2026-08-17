# Reliability Foundation

This document describes the local reliability foundation delivered without deploying a permanent worker.

## Runtime checks

| Endpoint | Purpose | Expected successful response |
|---|---|---|
| `/livez` | Confirms the API process can accept HTTP traffic. It does not require PostgreSQL. | HTTP 200 with `status: live` |
| `/readyz` | Confirms the API process can reach PostgreSQL and is ready to serve dependency-backed traffic. | HTTP 200 with `status: ready` and `database: connected` |
| `/healthz` | Backwards-compatible alias for readiness. | Same result as `/readyz` |

Every response includes `X-Request-ID`, which callers should capture when reporting an API issue.

## Configuration

`DATABASE_URL` is mandatory at startup. In `APP_ENV=production`, `JWT_SECRET` is also mandatory. CORS defaults cover the deployed Printa domains and the two local frontend development ports (`5173` and `5174`); additional origins must be explicitly listed through `CORS_ALLOWED_ORIGINS`.

## Durable outbox

Migration `000024_create_outbox_events` adds `outbox_events`. A domain event is recorded with its aggregate, payload, status, attempt count, availability time, and last error before it is eligible for delivery. The `internal/outbox` repository supports durable enqueue, safe concurrent claims using PostgreSQL row locks and `SKIP LOCKED`, completion acknowledgement, retry scheduling, expired-lease recovery, and a bounded dead-letter state. No permanent worker is started in this milestone: delivery execution must be enabled only with a deployed worker process and external provider configuration. A worker must call `ClaimPending`, invoke an idempotent downstream delivery, then call either `MarkDelivered` or `MarkFailed` with a bounded retry policy.

## Local verification

Run `make check` from the backend repository. Start the API using the local `.env`, then check:

```bash
curl -i http://127.0.0.1:18080/livez
curl -i http://127.0.0.1:18080/readyz
```

The reliability foundation is not a deployment guide. Deployment, backup automation, and continuous outbox processing remain separate operational decisions.
