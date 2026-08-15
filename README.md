# Printa Backend

Printa Backend is the Go REST API for the Printa print-on-demand ecosystem. It is intentionally implemented as a **modular monolith**: each business domain owns its handler, service, repository, models, and database migrations while the application remains deployable as one coherent service. This keeps early delivery simple while preserving clean seams for future service extraction.

## Domains

| Domain | Responsibilities |
|---|---|
| Identity and users | User registration, JWT login, OTP verification, Google OAuth, roles, and active-state controls. |
| Vendors and inventory | Vendor onboarding, stores, store staff, product availability, and stock. |
| Catalogue and orders | Platform products, order creation, lifecycle transitions, POS orders, and VAT-aware totals. |
| Routing and production | Store selection, routing rules, production jobs, assignment, and queue depth. |
| POS, billing, and payments | Cashier transactions, refunds, subscriptions, invoices, MTN MoMo, Airtel Money, and provider webhooks. |
| Administration and communication | Platform administration, audit logs, notifications, delivery logs, and Email/SMS/Push/WhatsApp dispatching. |

## Local development

The API requires Go 1.24 or newer and PostgreSQL. Copy the example environment file, then provide a local database URL and a development-only JWT secret.

```bash
cp .env.example .env
# Set DATABASE_URL and JWT_SECRET in .env.
```

The standard local checks are deliberately exposed through a safe Makefile.

```bash
make check
make run
```

The API uses `APP_PORT`, which defaults to `8080`. A local health check is available at `GET /healthz`.

## Database migrations

Migrations are ordered SQL pairs in [`migrations/`](./migrations). Every migration must have matching `.up.sql` and `.down.sql` files with no version gaps. Verify this before committing changes:

```bash
make verify-migrations
```

With the `migrate` CLI installed and `DATABASE_URL` set, apply migrations using:

```bash
make migrate-up
```

Use `make migrate-down` only in a disposable development database.

## API contract and documentation

The canonical OpenAPI contract is embedded in the binary at [`internal/apidocs/openapi.yaml`](./internal/apidocs/openapi.yaml). When the API is running it is available at:

| Resource | Local URL |
|---|---|
| Interactive API reference | `http://localhost:8080/api/v1/docs` |
| OpenAPI YAML contract | `http://localhost:8080/api/v1/openapi.yaml` |
| Health check | `http://localhost:8080/healthz` |

The interactive reference supports bearer-token authorization for protected endpoints. Public endpoints include health checks, registration, password login, OTP flows, OAuth entry points, payment webhooks, and the documentation itself.

## Quality expectations

Run `make check` before pushing backend changes. It formats source, runs static analysis and tests, validates migration pairs, verifies the OpenAPI asset, and builds the production binary. New endpoint work must update the OpenAPI contract in the same pull request so frontend teams can integrate against a stable documented interface.

## Repository layout

```text
cmd/api/                  API composition root
internal/apidocs/         Embedded OpenAPI contract and documentation handlers
internal/middleware/      JWT authentication, RBAC, and shared HTTP middleware
internal/modules/         Domain modules with handler, service, repository, and model layers
migrations/               Ordered PostgreSQL migration pairs
scripts/                  Deterministic local validation scripts
```
