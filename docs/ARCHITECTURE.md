# Abuzar Next architecture

## Runtime paths

1. Chrome/PWA uses HTTPS to the central Go API when connected.
2. At a disconnected branch, Chrome or Tauri uses the branch-edge Go service on the local LAN.
3. The branch edge stores immutable transaction events in SQLite and, when configured with a branch-scoped tenant-admin session, pushes local events idempotently and pulls central events through the Go synchronizer when connectivity returns.
4. PostgreSQL is never exposed directly to a browser or counter workstation.

## Tenancy boundary

The authenticated session provides an operator membership and allowed branch/counter assignments. The API sets PostgreSQL `app.tenant_id`, `app.branch_id`, and the appropriate tenant-scope flag inside each transaction. Row-level security is defense in depth; API authorization remains mandatory.

The central sync endpoint has a separate branch-forwarding rule: only a branch-scoped tenant administrator may submit events for other operators, and every forwarded operator/counter must be active and assigned to that branch/counter. Browser transaction endpoints always require the current operator identity.

## Offline boundary

Offline writes are restricted to sale, return, receiving, inventory, and shift events using already-synchronized master data. Master-data changes wait for connectivity. Every event has a globally unique event ID and tenant-scoped idempotency key. Financial records are immutable; conflicting administrative edits enter the review queue. Events pulled from the central service are marked server-originated locally so they are never re-queued as outbound work.
