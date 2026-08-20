# Phase G/H document vertical-slice evidence — 2026-08-06

## Scope

This slice adds the canonical business-document lifecycle for
`cash-sale` and `credit-sale`. It is intentionally not a claim of complete
sales parity.

## Implemented

- `011_business_documents.sql` adds tenant/branch/counter-scoped documents and
  lines, immutable revision snapshots, command receipts, per-kind numbering
  support, optimistic versions, and `draft`/`posted`/`void` status.
- `POST /v1/documents/{kind}` accepts `save`, `post`, `save-and-post`, and
  `void` commands. The URL and command kind must agree.
- The command path requires the authenticated sales-write permission and an
  authenticated branch/counter. It validates UUIDs, RFC3339 timestamps,
  positive quantities, decimal inputs, lifecycle transitions, and expected
  versions before any document mutation.
- Lines resolve only active canonical `master_items` in the authenticated
  tenant. Credit sales additionally require an active canonical customer.
- The existing deterministic pricing engine is used for price tiers, supplier
  schemes, line/document discounts, Misc, GST/PCT/advance-tax inputs, and the
  persisted pricing snapshot.
- Idempotent commands use tenant-scoped command and idempotency keys. A
  different payload for an existing key returns a conflict; a same-payload
  retry replays the stored response. The receipt, document, lines, revision,
  and audit row commit atomically.
- Successful commands also emit one schema-versioned immutable
  `sync_events` row with aggregate `business_document`, including drafts.
  `eventId` in the response is that sync-event UUID; the command receipt UUID
  is internal and is not presented as an event. The event is inserted with a
  stable identity inside the same transaction, stock/finance projections run
  against that identity, the document is reread, and only then is the event
  payload updated to `state: final`. The transaction cannot expose the
  provisional payload to another observer.
- Posted sales now include authoritative stock allocations and balanced
  finance/party-ledger summaries in the final event payload. Drafts remain
  projection-free; failed stock/finance projection rolls back the document,
  event, receipt, and projections together.
- `017_sync_event_delete_guard.sql` adds a `BEFORE DELETE` guard for finalized
  `business_document` events. The application-role grant script also revokes
  `DELETE` on `sync_events`; pending events and legacy state-less events remain
  compatible.
- Existing `/v1/transactions/*` routes were not changed and remain the
  compatibility event adapter.

## Verification observed

- Focused document/finance API integration tests passed. The full
  `go test ./services/api/... ./services/edge/... ./migration/...` run remains
  blocked by the unrelated existing
  `TestReadModelsExposeCanonicalSalesWithoutDuplicateCompatibilityRows`
  report-kind failure; no report files were changed.
- Ordered PostgreSQL migrations — initial attempt was blocked while the local
  disposable PostgreSQL instance was shutting down; after restart, all
  migrations applied successfully, including migration 011.
- Re-running the ordered migration set — passed (idempotent second pass).
- PostgreSQL inspection confirmed `business_documents`,
  `business_document_lines`, `business_document_revisions`, and
  `command_receipts`, plus the credit-customer constraint.
- Focused unit tests cover lifecycle validation, idempotency hash
  discrimination, balanced pricing, authentication, and migration contracts.
- `go test ./services/api/internal/httpapi
  -run TestBusinessDocumentLifecycleIntegration -count=1` with the protected
  local `DATABASE_URL` environment — passed against
  the disposable PostgreSQL cluster. The fixture covered draft save, same-key
  replay, different-payload conflict, stale revision rejection, post, void,
    no mutation after a missing canonical item, cross-tenant item rejection, and
    one immutable sync event per successful command.
- `go test ./services/api/internal/httpapi -run
  'TestFinanceSalePostingIntegration|TestBusinessDocumentLifecycleIntegration'
  -count=1` — passed. The finance integration verifies the committed
  `sync_events` payload is final, carries the stable event ID, balanced finance
  data, and non-empty stock allocations.
- `go vet ./services/api/... ./services/edge/... ./migration/...` — passed.
- Migration `017_sync_event_delete_guard.sql` applied twice successfully.
- The finance integration regression passed for rejected finalized-event
  deletion, pending-event deletion compatibility, and legacy state-less event
  deletion.
- `pnpm --filter @abuzar/web check` — passed with 0 errors and 0 warnings.
- `pnpm --filter @abuzar/web build` — passed.

## Explicit remaining gaps

- Sale posting now mutates stock batches/allocations and sale GL/party-ledger
  projections when the required canonical godown, batch stock, and finance
  accounts exist. It does not claim complete legacy valuation or policy parity.
- Cash drawer, tax-register, and full historical accounting reconciliation
  remain open.
- Customer pricing/group policy lookup and complete migrated PricePolicy
  promotion are not yet wired to canonical database policy tables; callers
  must provide the supported pricing inputs.
- Only cash-sale and credit-sale are implemented by this service. Returns,
  open returns, quotations, refused sales, purchases, and purchase orders
  remain on existing compatibility/event paths.
- Full authenticated database integration coverage for every lifecycle branch
  remains follow-up work; the focused Go suite is unit/contract coverage.
