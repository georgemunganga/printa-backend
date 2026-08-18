# Outbox Worker Operations

The Printa API and the durable outbox worker are separate processes. The API records events in `outbox_events`; the worker leases ready events, invokes an idempotent handler, and records delivery or a retry/dead-letter outcome. Running them separately prevents slow or failed external delivery from blocking HTTP requests.

## Configuration

The worker requires the same `DATABASE_URL` used by the API. The following optional settings are read from `/etc/printa/printa.env` in a managed environment or `.env` for local development.

| Variable | Default | Purpose |
|---|---:|---|
| `OUTBOX_POLL_INTERVAL` | `2s` | Delay between polls after each processing pass. |
| `OUTBOX_LEASE_DURATION` | `5m` | How long a claimed event is leased before another worker may recover it. |
| `OUTBOX_BATCH_SIZE` | `25` | Maximum events claimed in one pass; valid range is 1–100. |
| `OUTBOX_MAX_ATTEMPTS` | `5` | Maximum delivery attempts before an event is placed in `DEAD_LETTER`. |

> The worker is deliberately safe by default. An event type without a registered handler is not discarded; it is retried and eventually dead-lettered for operator investigation.

## Registered event contracts

| Event type | Producer | Worker action | Delivery behavior |
|---|---|---|---|
| `notification.dispatch.v1` | Notification service `Dispatch` | Decode the notification event and call the communications service. | Uses existing per-channel idempotent delivery logging. Events without configured contact metadata are completed without an external send, preserving the in-app notification. |

The API no longer starts a fire-and-forget communications goroutine for notification dispatch. It stores the customer-visible notification and records durable delivery work for the separately supervised worker.

## Build and local verification

From the repository root, run:

```bash
make check
go build -o bin/printa-outbox-worker ./cmd/worker
DATABASE_URL="$DATABASE_URL" OUTBOX_POLL_INTERVAL=2s ./bin/printa-outbox-worker
```

Stop the process with `SIGTERM` or `Ctrl+C`. The worker completes its current database operation and exits cleanly. It does not need an HTTP port.

## Managed service installation

The template at `deploy/systemd/printa-outbox-worker.service` assumes the application source and binaries live in `/opt/printa-backend`, an unprivileged `api` user owns the service, and secrets are stored only in `/etc/printa/printa.env`.

```bash
sudo install -d -o api -g api /opt/printa-backend/bin /var/log/printa /etc/printa
sudo install -m 600 -o root -g api /path/to/printa.env /etc/printa/printa.env
sudo install -m 644 deploy/systemd/printa-outbox-worker.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now printa-outbox-worker
sudo systemctl status printa-outbox-worker
sudo journalctl -u printa-outbox-worker -f
```

## Release and rollback

Build and replace both API and worker binaries from the same Git commit. Restart the worker only after the binary replacement completes:

```bash
sudo systemctl restart printa-outbox-worker
sudo systemctl status printa-outbox-worker
```

To roll back, deploy the previously known-good worker binary, then restart the service. Existing `PENDING`, `FAILED`, and expired `PROCESSING` events remain durable in PostgreSQL. Do not delete rows merely to clear an alert; inspect `last_error`, handler configuration, and the downstream provider first.

## Operational response

| Symptom | Operator action |
|---|---|
| Events remain `PENDING` | Confirm the service is active, inspect worker logs, and confirm `DATABASE_URL` can reach PostgreSQL. |
| Events remain `PROCESSING` | Wait for the lease to expire or restart a healthy worker; expired leases are reclaimed automatically. |
| Events reach `DEAD_LETTER` | Fix the handler or downstream provider, then explicitly requeue the event through an approved operator procedure. |
| Repeated delivery attempts | Verify the handler’s idempotency key and downstream delivery log before reprocessing. |

This runbook documents operation of the worker executable. It does not deploy the service to a server automatically and must be used with a production environment review, secrets configuration, and downstream handler registration.
