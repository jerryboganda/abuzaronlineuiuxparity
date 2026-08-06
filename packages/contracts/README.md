# Shared contracts

This package contains the stable TypeScript shapes shared by the SvelteKit client and generated OpenAPI clients. The Go API mirrors these shapes in its transport layer; business logic never trusts tenant or branch identifiers supplied by an untrusted browser.

The authenticated server session is the source of tenant/branch/counter scope. Clients may display or request a selected scope, but the API validates it against the session assignments and PostgreSQL RLS context.
