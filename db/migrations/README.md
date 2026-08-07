# PostgreSQL migrations

Apply migrations in filename order with a dedicated application role. The application role must not own the protected tables; the API sets `app.tenant_id` and `app.branch_id` inside each transaction after membership checks.

The first migration is intentionally a tenancy/synchronization foundation. Existing SQL Server business tables are added in reviewed migration waves after the source inventory and reconciliation reports are approved.

Current order:

1. `001_tenancy.sql` — shared-schema tenancy, operational context, immutable sync events, representative sales/inventory tables, and RLS.
2. `002_branch_scope.sql` — branch-level RLS policies.
3. `003_auth_sessions.sql` — hashed HTTP-only session records and scoped foreign keys.
4. `004_migration_support.sql` — legacy ID mappings, migration exceptions, and reconciliation results.
5. `005_migration_bookkeeping_rls.sql` — tenant RLS policies for migration bookkeeping records.
6. `006_master_records.sql` — tenant-scoped master-data records and extensions.
7. `007_preferences.sql` — tenant-scoped Preferences and maintenance state.
8. `008_role_permissions.sql` — tenant-scoped role permission assignments with RLS.
9. `009_legacy_security_rights.sql` — legacy Groups/GroupRights/GroupAllowed*
   compatibility tables, four representational group roles, and fail-closed
   scope data.
10. `009_phase_e_master_wave.sql` — reviewed legacy IDs on master records and
    tenant-scoped ItemSuppliers links used by the isolated Phase E sandbox.
11. `010_master_normalized.sql` — canonical tenant-scoped master tables,
    compatibility adapters, and normalized item-supplier fields for Phase F.
12. `011_business_documents.sql` — tenant/branch-scoped cash-sale and
    credit-sale drafts, revisions, command receipts, lifecycle status, and
    deterministic pricing snapshots. It does not post stock, batch, GL, or
    party-ledger projections.
13. `012_stock_ledger.sql` — tenant/branch/godown/batch stock batches,
    immutable stock movements, sale allocations, rebuildable balances, and
    reconciliation metadata. It does not claim legacy StockReport valuation
    parity.
14. `013_finance_ledgers.sql` — tenant-scoped chart/account categories,
    immutable balanced GL journals and lines, party ledgers/balances, and
    voucher placeholders. It seeds only explicit posting accounts/categories;
    it does not import historical balances or implement purchases/payables.
15. `014_purchase_documents.sql` — purchase document kinds, supplier/source
    references, batch/tax line metadata, supplier payable/input-tax account
    configuration, and purchase-compatible party-ledger constraints. Existing
    sale rows remain valid; purchase posting is service-owned and rejects
    missing account or source/batch configuration.
16. `015_sync_event_final_payload.sql` — canonical pending-to-final business
    document event payload transition, finalization timestamp, compatibility
    for older state-less payloads, and database enforcement against later
    mutation.
17. `016_branch_rls_hardening.sql` — restrictive tenant-and-branch policies
    layered over the earlier permissive policies for operational,
    business-document, stock, finance, and purchase-sensitive tables. It
    preserves authentication bootstrap and intentional tenant-admin scope.
18. `017_sync_event_delete_guard.sql` — Phase R coordination guard preventing
    deletion of finalized business-document events while preserving pending
    rollback and legacy state-less event compatibility.
19. `018_tax_configuration.sql` — tenant-scoped GST/PCT/advance-tax
    configuration used by the pricing and tax waves.
20. `019_security_data_import_adaptation.sql` — reviewed legacy security
    identifiers/payload columns and the composite-key scope kinds required by
    the isolated Groups/Users/GroupRights/GroupAllowed* import wave.
21. `020_historical_migration_wave.sql` — historical document, stock, finance,
    and pricing import targets with retained source identity.
22. `021_scale_read_indexes.sql` — measured, tenant/branch-scoped read indexes
    for canonical stock, report, GL, and party-ledger paths. The Phase W
    disposable harness captures plans and timings; this does not claim full
    volume acceptance.
23. `022_sale_return_lifecycle.sql` — sale-return entry-kind support and
    source-scoped indexes for stock/finance reversal.
24. `023_open_sale_return_lifecycle.sql` — distinct open-cash/open-credit return
    kinds and scoped query index.
25. `024_preferences_branch_scope.sql` — branch-scoped preference writes and
    tenant-default fallback.
26. `025_sale_return_reversal_contract.sql` — canonical return-kind checks,
    source sale/line identity, reversal uniqueness, and restrictive RLS.
27. `026_historical_line_precision.sql` — preserves up to eight decimal places
    for imported business-document quantities whose loose-unit divisor is not a
    power of ten.
28. `027_historical_item_history_adjustments.sql` — retains source-backed
    `ItemLog` snapshots and `AdjHeader`/`AdjDetail` rows for the Phase Q item
    history and stock-adjustment report leaves. Current-item/godown joins are
    optional so an absent current master cannot silently drop historical rows;
    exact PowerBuilder columns and print layouts remain an acceptance boundary.

29. `028_business_document_void_reversals.sql` adds append-only compensating
    reversals for posted sales, returns, and purchases. Void status changes are
    safe only when inverse stock movements, a reversing GL journal, and a
    reversing party-ledger entry commit atomically with the command.
30. `029_auxiliary_master_kinds.sql` extends the tenant-scoped
    `master_records` compatibility store with the captured hyphenated Basic
    Data route kinds used by the auxiliary master CRUD wave. It retains
    source-shaped fields in `payload`; it does not claim normalized source
    tables or recovered legacy rule semantics.
31. `030_historical_deleted_sale_items.sql` retains the captured
    `dbo.DeletedSaleItem` audit stream with typed item/godown, quantity/bonus,
    pricing, discount/tax, machine/user, invoice, source-row, and raw-payload
    fields under tenant/branch scope. Exact PowerBuilder deleted-item columns
    and print semantics remain an acceptance boundary.
32. `031_historical_withholding_tax.sql` retains the captured
    `dbo.PurPayment` withholding-deduction fields separately from purchase-line
    advance tax: payment/invoice identity, posted state, supplier identity,
    base/rate/amount, account/check/remarks, user, and raw payload under
    tenant/branch scope. Exact legacy grouping and print semantics remain an
    acceptance boundary.
33. `032_historical_party_payment_allocations.sql` retains the reviewed
    customer and supplier settlement streams: `dbo.PurPayment`, direct
    `dbo.Purledger` payment snapshots, `dbo.InstallmentReceiptDetail`, and
    direct `dbo.SaleLedger` payment snapshots. Canonical party/document links
    are nullable so unresolved legacy IDs remain auditable instead of being
    dropped; exact adjustment-allocation and legacy print semantics remain an
    acceptance boundary.
34. `033_historical_party_ledger_adjustments.sql` retains the captured
    `dbo.SaleReceivableAdj` customer debit/credit adjustment rows separately
    from payment amounts. It preserves unresolved invoice/date/party identity,
    account/check/remarks/user fields, and raw payload under tenant/branch
    scope; exact legacy adjustment posting and print semantics remain an
    acceptance boundary.
35. `034_historical_party_return_allocations.sql` retains distinct customer
    sale-return and supplier purchase-return allocation rows from the reviewed
    `SRAllocationHeader/Detail` and `PRAllocationHeader/Detail` streams. Source
    return/invoice identity, allocation and outstanding amounts, posted state,
    and raw payload are preserved under tenant/branch scope; statement/ledger
    visibility is bounded and these rows do not mutate aging until legacy
    allocation semantics are reconciled.
36. `036_item_alternate_aliases.sql` extends the canonical item-alias kind
    contract so the captured Item Form alternate-alias command can replace
    alternate names without modifying primary alias or barcode lookup rows.
37. `037_item_images.sql` retains the captured ItemImage per-item row,
    description, blob, and type fields in a tenant-scoped canonical collection
    for the Item Form image command.
38. `038_item_notes.sql` retains the captured ItemNotes one-blob-per-item
    value as tenant-scoped `bytea` so UTF-8, RTF, and other legacy rich-text
    encodings can round-trip without lossy conversion. The Item Form notes API
    applies bounded input and tenant RLS.
39. `039_item_associations.sql` retains the captured ItemAssociation
    `ICode`/`AssocICode` pair as a tenant-scoped canonical relation, preserving
    both legacy identities when an associated item is not yet resolved.
40. `040_item_authors.sql` retains the captured ItemAuthor `ICode`,
    `AuthorCode`, `Priority`, and `ROWID` relationship fields in a
    tenant-scoped canonical collection with bounded priority/order checks.
41. `041_item_models.sql` retains the captured ItemInModel `ICode`/`ModelCode`
    membership pair in a tenant-scoped canonical collection while keeping the
    separate Model master migration boundary explicit.
42. `042_item_registration_requests.sql` retains ItemRegRequest request
    metadata and full source-shaped item payload snapshots under tenant RLS;
    it records populate history but does not infer or perform external legacy
    registration-server delivery.
43. `043_item_unposted_transaction_index.sql` adds the tenant/branch/item
    lookup index used by the Item Form's `Show Un-Posted Transaction Report`
    command. The report reads canonical draft business-document lines with an
    explicit bounded response; it does not claim historical SQL Server import
    coverage.

The historical `020` wave contains a counter fixture whose parent tenant and
branch are expected from the reviewed importer/bootstrap environment. The
Phase W disposable harness creates those parent rows only in its throwaway
database before applying `020`; it does not alter the historical migration or
seed those rows in an application database.
