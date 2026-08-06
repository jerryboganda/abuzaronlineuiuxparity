# VPS deployment

The central deployment packages the Go API behind HTTPS with PostgreSQL kept private. The branch-edge installer is separate from the central deployment and may operate on the branch LAN during a WAN outage.

Apply the ordered database migrations with `ops/postgres/apply-migrations.sh` (or the PowerShell equivalent on Windows) using a protected `ABUZAR_ADMIN_DATABASE_URL`. Never put that URL in source control or a client bundle.

After migrations, provision the least-privileged API role with `ops/postgres/provision-app-role.sh`, set `ABUZAR_APP_DATABASE_URL` for the API, and then run the API bootstrap command described in `docs/OPERATIONS.md`. The bootstrap command must use the admin DSN only from its protected deployment shell if it needs to write initial identity data.
