# Mobile API release — 2026-09-27

Deployed the mobile release configuration endpoint and customer-owned order payment reads from `6f2ee98`, followed by push adapter error handling that no longer fabricates provider acceptance.

Validation: API/payment/comms tests pass; the worker package compiles. Both public API domains return successful readiness, mobile configuration, catalogue and store responses. Payment reads without authentication return 401. Public configured methods include cash, MTN MoMo, Airtel Money and Zamtel Money; settlement was not initiated or verified.

Deployment uses a static Go 1.24 binary, atomic replacement and a readiness check with rollback. Previous binaries are retained in `/opt/printa-api/backups/` as `printa-api-pre-mobile-6f2ee98` and `printa-api-before-push-failure-fix`.

This does not deploy a push worker or provide device registration, Expo/APNs/FCM credentials or receipts. The legacy adapter is not a complete mobile push delivery integration. No customer messages or payments were initiated during these checks.
