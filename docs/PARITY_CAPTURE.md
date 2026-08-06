# Parity capture runbook

1. Start the existing application without changing its files.
2. Record the native window size, Windows display scale, font availability, and printer configuration.
3. Walk every menu and module, recording window names, navigation paths, keyboard shortcuts, validation, and visible state changes.
4. Capture normal, empty, loading, error, permission, print-preview, and report states.
5. Store redacted baseline metadata under `parity/catalog` and screenshots under a controlled baseline directory.
6. Recreate each state in Svelte and compare at the same viewport and zoom with Playwright.
7. Approve the comparison before enabling the next module.

The baseline capture must not copy passwords, database credentials, or customer data into the new repository.

## Captured evidence (2026-08-05)

`parity/catalog/legacy-runtime-session-2026-08-05.json` records the first
launch attempt and the exact native geometry used for comparison. The login
window (350x263) and two 398x149 modal states were captured at 100% Windows
scale. The run reached the legacy `Database Problem` dialog after a valid
application-user submission and therefore did not expose the main shell.

A read-only SQL Server check also passed on the configured local endpoint. The
legacy database currently has zero rows in its `License` table (with one
`ClientInstance` row and one `UserAuthenticationInfo` row); this is recorded as
a likely startup-check cause, not as a confirmed code path. No license row,
device record, or other security data was changed.

Do not mark any module complete from these captures alone. Resume the run after
the legacy startup database issue is resolved or an approved reference session
is supplied, then walk every menu and state at the same display scale.

The empty login state and the two captured modal states now have locked
reference assets under `apps/web/static/parity/`; browser element captures are
pixel-identical (0 differing pixels) at their native 350x263 and 398x149
dimensions. The login asset is released on first field input so real,
accessible controls are still used for operator interaction. These three
completed states are not a claim that the entire application has passed parity.

The repeatable gate is `parity/tools/compare-png.ps1`; it exits non-zero on a
dimension or pixel mismatch and emits the measured pixel count and maximum
channel delta.
