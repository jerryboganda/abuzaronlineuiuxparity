# Branch edge service

The branch edge is the local coordinator for offline counter operations. It stores validated immutable, idempotent synchronization events in SQLite and exposes local sale, return, receiving, inventory, shift, push, pull, status, and health endpoints. Central authorization and server-authoritative reconciliation happen at the central API when connectivity returns; SQLite files are never shared directly across workstations.

Set `ABUZAR_EDGE_SHARED_SECRET` for a deployed branch edge. Health remains public for monitoring; transaction and synchronization endpoints require `Authorization: Bearer <secret>` when the secret is configured. Leaving it empty is intended only for local development.

To enable automatic central synchronization, configure `ABUZAR_EDGE_CENTRAL_URL` and a branch-scoped tenant-admin `ABUZAR_EDGE_CENTRAL_SESSION` on the edge host, plus optional `ABUZAR_EDGE_SYNC_INTERVAL` (default `30s`). A branch-scoped tenant-admin session may forward events from the branch's assigned operators/counters; it cannot cross the tenant or branch boundary. The synchronizer pushes the local immutable queue, advances its local acknowledgement cursor only after a successful central response, then pulls central events into SQLite. If either setting is missing, the edge remains a local-only service and can still be synchronized through the HTTP contract.

`GET /v1/hardware/capabilities` reports the explicit printer, barcode, cash-drawer, biometric, SMS, and email adapter states. An unavailable adapter is reported as unavailable; the edge never fakes a financial or identity device response.

Hardware adapters are injected into `internal/hardware.Registry` through
`hardware.Config`; the default registry has no adapters. The package exposes
interfaces for ESC/POS printing, barcode lookup, cash-drawer kick, biometric
verification, SMS, and email. No OS device is opened implicitly.

The renderer produces deterministic ESC/POS bytes through
`RenderSaleSlip` and `RenderPurchaseLabels`. The authenticated hardware
endpoints return `503 hardware_adapter_unavailable` when the corresponding
adapter is absent, rather than reporting a successful print or drawer kick.
`POST /v1/hardware/barcode/normalize` is safe to use with an HID wedge: it
trims scanner whitespace/CRLF and rejects remaining control characters.
Cash-drawer adapters receive an explicit device-neutral pulse command; an
ESC/POS adapter can translate its default `ESC p` bytes to the drawer.
