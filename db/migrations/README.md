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

The historical `020` wave contains a counter fixture whose parent tenant and
branch are expected from the reviewed importer/bootstrap environment. The
Phase W disposable harness creates those parent rows only in its throwaway
database before applying `020`; it does not alter the historical migration or
seed those rows in an application database.
