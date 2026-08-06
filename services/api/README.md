# Central API

The API exposes HTTP-only session authentication, tenant/branch/counter assignment checks, transaction-scoped PostgreSQL settings, RLS-backed resource reads, immutable sale writes, deterministic pricing previews, branch stock-balance reads, central sync push/pull, and conflict listing.

`POST /v1/transactions/preview` is a read-only exact-decimal calculation for price tiers, supplier schemes, discounts, Misc, and GST/PCT/advance-tax ordering. When a sale event includes its preview request, the same calculation is replayed inside the posting transaction and the supplied total must match. `GET /v1/inventory/balance?itemLegacyId=...` derives available quantity from scoped immutable movement rows.

`POST /v1/auth/login` requires `username`, `password`, and `tenantCode`; optional `branchId` and `counterId` are validated against assignments. Every immutable mutation event is required to carry tenant, branch, counter, and operator scope. Passwords are verified by PostgreSQL `pgcrypto` and only a SHA-256 hash of the random session token is stored. The browser receives an HTTP-only cookie; no database credentials are shipped to clients.

The API still requires a configured `DATABASE_URL` and the ordered migrations. It does not accept tenant or branch headers as authorization; scope is derived from the authenticated session.

Run `go run ./services/api/cmd/bootstrap` after migrations to create the first tenant, branch, counter, and tenant-admin operator from protected `ABUZAR_BOOTSTRAP_*` environment variables.
