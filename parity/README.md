# UI/UX parity evidence

The current PowerBuilder executable is the canonical reference. Each screen is catalogued before implementation with:

- route/window identifier;
- capture viewport and Windows display scale;
- normal, empty, error, loading, and permission states;
- keyboard and focus behavior;
- input validation and navigation;
- report/print output;
- source screenshot and approved new-client screenshot.

No screen is marked complete until its screenshot and workflow comparison passes.

Run `parity/tools/inventory-legacy.ps1` to create a metadata-only inventory of the reference installation before capture. It reads filenames and sizes only; it does not open the legacy database or modify the fallback application.

## First runtime baseline

The first controlled runtime capture is recorded in
`parity/catalog/legacy-runtime-session-2026-08-05.json`. At the canonical
1920x1080, 96-DPI Windows desktop, the legacy executable opened its 350x263
login window and its validation/database-problem dialogs were captured with
native window bounds and SHA-256 hashes. A read-only SQL connectivity probe
confirmed that the local SQL Server endpoint was reachable, but the legacy
startup stopped at its own database-problem dialog before the main shell could
be displayed.

The captured login and dialog states are evidence, not an approval of parity.
The complete menu, screen, workflow, report, printer, and hardware catalogue
still requires a working legacy main window and an approved operator walkthrough.

## Controlled sandbox run

On 2026-08-05 the legacy executable was run against a copy-only restored
database in `D:\ABUZAR\LegacyReferenceSandbox`. The cloned startup procedure
and cloned encrypted runtime setting were adjusted only inside that sandbox so
the real shell could be reached. The shell yielded nine top-level menus and
275 menu entries, and representative master-data, transaction, report,
preference, backup, and integrity screens were captured. The detailed result,
including the SQL Agent compatibility finding and two PowerBuilder R0002
workflow crashes, is in
`parity/LEGACY_RUNTIME_AUDIT_2026-08-05.md` and the machine-readable
`parity/catalog/legacy-workflow-capture-2026-08-05.json`.

This controlled run makes the reference accessible; it does not approve any
module or claim 100% pixel/functional parity. The canonical
`V2_AbuzarSoftware` installation remains the untouched fallback.

The empty login state is the first completed comparison: the locked reference
asset and the Chrome element capture match at 350x263 with zero differing
pixels. The comparison evidence is in
`parity/catalog/login-baseline-comparison.json`.
