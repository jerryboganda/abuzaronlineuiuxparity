# Scope: Milestone M4 — Report Engine & Hardware Integration Standard

## Architecture
- SvelteKit frontend (`apps/web`): Report preview surface (ruler, zoom, loaded-row paging, letterhead), report catalog resolution, export handlers, hardware preview/trigger UI.
- Go REST API backend (`services/api`): 151 Catalog Report definitions, parameter parsing, data aggregations, CSV/workbook export formatting.
- Go Edge service (`services/edge`): Hardware integration (ESC/POS receipt/label rendering, cash drawer pulse `0x1b 0x70`, barcode lookup service), Tauri IPC bridge, Windows Credential Manager integration.

## Feature Inventory
| # | Feature | Description | Status |
|---|---------|-------------|--------|
| 1 | 151 Catalog Report Definitions | 151 non-blank report catalog leaves mapped to explicit API & web definitions | IN_PROGRESS |
| 2 | Report Preview & Formatting Surface | Print preview surface (ruler, zoom, loaded-row paging, letterhead) | IN_PROGRESS |
| 3 | Report Export Capabilities | CSV, workbook, and multi-format export hooks | IN_PROGRESS |
| 4 | Edge Hardware Integration Subsystem | ESC/POS receipt/label renderers, cash drawer pulse (0x1b 0x70), barcode lookup | IN_PROGRESS |
| 5 | Desktop Tauri IPC & Windows Credentials | Tauri IPC bridge, Windows Credential Manager integration for edge secrets | IN_PROGRESS |

## Interface Contracts
### Web (`apps/web`) ↔ API (`services/api`)
- Reports: `GET /v1/reports/[kind]` resolving to 151 catalog definitions
- Export: `GET /v1/reports/[kind]/export?format=csv|xlsx`

### Web (`apps/web`) ↔ Edge Hardware (`services/edge`)
- Hardware Readiness: `GET http://127.0.0.1:8091/v1/hardware/readiness`
- ESC/POS Print Slip: `POST http://127.0.0.1:8091/v1/hardware/print/sale-slip`
- Cash Drawer Kick: `POST http://127.0.0.1:8091/v1/hardware/drawer/kick`
- Barcode Lookup: `GET http://127.0.0.1:8091/v1/hardware/barcode/[code]`

## Code Layout
- `apps/web/src/routes/reports/...` & `apps/web/src/lib/components/reports/...`: Svelte report catalog & preview UI
- `services/api/internal/reports/...` & `services/api/cmd/server/...`: Go API report engine & 151 report definitions
- `services/edge/internal/hardware/...` & `services/edge/internal/tauri/...`: ESC/POS, cash drawer pulse, barcode lookup, Tauri IPC, Windows Credential Manager
