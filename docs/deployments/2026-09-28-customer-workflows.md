# Customer workflow fix — 2026-09-28

Functional source: `eabd3ef`. Backend tests pass for customer order ownership across CUSTOMER, VENDOR, ADMIN, STAFF and CASHIER; non-owners and vendor billing through customer endpoints are rejected. Paused vendor financial operations remain HTTP 423 with a readable code/message. Direct comms-provider sending now requires ADMIN/VENDOR.

Account-owned payment methods, notifications and support no longer depend on vendor operating prerequisites. Dedicated customer chat/payment routes preserve ownership checks and authenticated attachment downloads. Mobile clients must update to use `/api/v1/customer/...`; original operational routes remain for vendor compatibility.

Built with Go 1.24.13 and CGO disabled, matching the prior runtime toolchain. Atomic binary replacement, service restart and readiness succeeded. Both public domains return 200 readiness and 401 for unauthenticated customer chat/payment, notifications and payment-method requests. No database migrations, customer messages, orders or payments were performed.

Original rollback binary: `/opt/printa-api/backups/printa-api-before-customer-scope-20260928022616`.

The canonical OpenAPI contract additionally documents all new customer payment/chat endpoints, ownership and server-authoritative payment amounts. The mobile release audit records the device and external-integration work still required.
