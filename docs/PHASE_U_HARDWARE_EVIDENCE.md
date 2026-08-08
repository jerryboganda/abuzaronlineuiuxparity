# Phase U hardware foundation evidence

Date: 2026-08-06

## Implemented

- `services/edge/internal/hardware` now owns injected adapter interfaces and a
  registry configuration for printer, barcode lookup, cash drawer, biometric,
  SMS, and email integrations.
- Sale-slip and purchase-label ESC/POS renderers are deterministic and do not
  open devices or calculate business totals.
- Barcode input trims HID-wedge whitespace/CRLF, rejects control characters,
  and exposes an injected lookup hook.
- Cash-drawer kick is an injected, device-neutral pulse command; no kick is
  reported without an adapter.
- Authenticated edge routes cover printing, barcode normalization/lookup, and
  drawer kick. Missing adapters return `503 hardware_adapter_unavailable`.
- The Tauri desktop bridge now stores explicit edge URL/shared-secret
  configuration in Windows Credential Manager and exposes native commands for
  capabilities, sale-slip printing, purchase-label printing, barcode lookup,
  and cash-drawer kick. The shared secret is never returned by the read
  command.

## Automated evidence

The byte goldens are hexadecimal representations of the complete rendered
byte streams:

| Renderer | Golden | Result |
|---|---:|---|
| Sale slip | 315 bytes | exact byte comparison passed |
| Purchase labels | 84 bytes | exact byte comparison passed |

Commands run from `D:\ABUZAR\AbuzarNext`:

```text
gofmt -w .\services\edge\internal\hardware\registry.go .\services\edge\internal\hardware\escpos.go .\services\edge\internal\hardware\registry_test.go .\services\edge\internal\hardware\escpos_test.go .\services\edge\internal\syncapi\server.go .\services\edge\internal\syncapi\server_test.go
go test ./services/edge/...
go test -v ./services/edge/internal/hardware ./services/edge/internal/syncapi
go vet ./services/edge/...
python -c "import yaml; yaml.safe_load(open('docs/edge-openapi.yaml', encoding='utf-8')); print('edge-openapi.yaml: valid YAML')"
pnpm --filter @abuzar/desktop build
```

Observed result: all edge packages passed; verbose hardware and sync API tests
passed, including shared-secret protection and no-adapter `503` behavior; Go
vet and the edge OpenAPI YAML validation passed. The desktop Rust tests passed
(3 URL/IPC configuration tests), and the Tauri production build produced the NSIS
and MSI bundles.

## Acceptance still open

No physical printer, label printer, barcode scanner, cash drawer, biometric
reader, SMS gateway, or SMTP service was connected or represented as
connected for this change. The Phase U physical acceptance remains open:
pharmacy-device print comparison against legacy output and scanner-to-line-add
at POS speed require a real-device run and operator sign-off. These golden
tests prove deterministic software bytes only; they do not claim legacy byte
parity or physical success.

The desktop build and command tests prove IPC/configuration and edge error
plumbing only. They do not claim that a printer, scanner, drawer, or any other
physical adapter is present.

## Hardening addendum — 2026-08-07

- Adapter configuration validation rejects orphan providers, provider names
  with surrounding whitespace/control characters, and typed-nil adapters.
- `/v1/hardware/readiness` reports configuration validity, aggregate status,
  available/unavailable counts, and per-category diagnostics. The default
  registry is explicitly `ready: false` / `status: unavailable`.
- The unavailable-device acceptance fixture covers sale slips, purchase
  labels, barcode lookup, and cash-drawer kick. It asserts `503
  hardware_adapter_unavailable` and rejects success-shaped response fields.
- The Tauri IPC test proves an edge `503 hardware_adapter_unavailable` remains
  an IPC error even when the response body contains a misleading `printed`
  field.

The checklist is recorded in
[`PHASE_U_DEVICE_ACCEPTANCE_CHECKLIST.md`](PHASE_U_DEVICE_ACCEPTANCE_CHECKLIST.md).
Its physical pilot section remains unchecked.

Hardening commands observed on 2026-08-07:

```text
go test ./services/edge/...
go vet ./services/edge/...
cargo fmt -- --check
cargo test
python -c "import yaml; yaml.safe_load(open('docs/edge-openapi.yaml', encoding='utf-8')); print('edge-openapi.yaml: valid YAML')"
```

All passed; the desktop test suite reported 3 passing tests, including IPC
problem/status propagation.

## Purchase-label workflow follow-up - 2026-08-07

The Purchase transaction surface now sends populated item, batch, expiry, MRP,
and quantity rows to the authenticated edge
`/v1/hardware/print/purchase-labels` route. When the branch adapter is absent or
unavailable, the workflow preserves the browser print-preview fallback and
does not claim a physical print succeeded.

Focused web evidence: `cmd /c pnpm --filter @abuzar/web check` passed with 0
errors and 0 warnings. Edge renderer and unavailable-adapter tests remain the
authoritative software boundary; physical label layout, printer connection,
and legacy byte/raster comparison remain open.

## Barcode scanner UI wiring, biometric/SMS/SMTP adapters, and cutover
## tooling — 2026-08-08

Code-complete and unit/integration-tested this session; no physical device or
live external credential was exercised for any item below. Everything here
follows the same rule as the rest of this document: passing software tests
prove software behavior, not device presence or a successful live send.

- **Barcode scanner UI wiring.** `apps/web/src/lib/barcode-scanner.ts` (new)
  implements HID-wedge keystroke-timing detection (`createBarcodeScanListener`)
  and a client-side mirror of the edge normalization rule
  (`normalizeBarcodeClientSide`). It is wired into the item-lookup input on
  both `apps/web/src/routes/app/sales/+page.svelte` and
  `apps/web/src/routes/app/purchase/[kind]/+page.svelte`, calling the existing
  authenticated `/v1/hardware/barcode/lookup` edge route on a detected scan.
  `apps/web/tests/sales-canonical.spec.ts` and
  `apps/web/tests/purchase-canonical.spec.ts` add Playwright coverage that
  dispatches synthetic fast-keystroke `keydown` events, mocks the edge lookup
  route, and asserts a resolved scan adds a line while an unresolved scan
  surfaces a non-blocking error. **Still needs:** a real physical HID scanner
  exercised at a POS terminal — the synthetic keystroke-timing simulation in
  the tests is not evidence of real scanner hardware behavior.
- **Biometric, SMTP, and SMS adapters.** `services/edge/internal/hardware/smtp.go`
  and `sms.go` (new) implement injected `EmailAdapter`/`SMSAdapter`
  interfaces; `registry.go` adds `VerifyBiometric`, `SendEmail`, and `SendSMS`
  registry methods that pass through to configured adapters and return the
  existing `hardware_adapter_unavailable` behavior when none is configured.
  New authenticated edge routes: `POST /v1/hardware/biometric/verify`,
  `POST /v1/hardware/email/send`, `POST /v1/hardware/sms/send`
  (`services/edge/internal/syncapi/server.go`), documented in
  `docs/edge-openapi.yaml`. The Tauri desktop bridge adds a `verify_biometric`
  command (`apps/desktop/src-tauri/src/lib.rs`). `services/api/internal/httpapi/channel_send.go`
  (new) gives the central API an explicit, env-configured (`ABUZAR_EDGE_CHANNEL_URL`,
  `ABUZAR_EDGE_CHANNEL_SECRET`) path to reach a branch edge's real SMS/email
  adapters for Maintenance test-email/test-sms actions, wired into
  `maintenance.go`; without that configuration those actions are honestly
  reported `not_configured`, the same pattern already used for
  pg_dump/pg_restore. `.env.example` documents the new `SMTP_*`,
  `SMS_GATEWAY_*`, and `ABUZAR_EDGE_CHANNEL_URL` variables.
  **By design, no biometric matching algorithm exists anywhere in this
  change** — `registry.go` documents that no biometric matching happens in
  this service; verification is entirely delegated to whatever adapter is
  injected, and no vendor SDK or physical reader was available in this
  environment to build or test one against. **Still needs:** real SMTP/SMS
  gateway credentials configured and a live test send; a real biometric
  vendor SDK implementing the adapter interface, injected and tested against
  a physical reader.
- **Cutover and parallel-day tooling** (not hardware, but built alongside
  this wave and relevant to the same "code exists, physical/live run does
  not" distinction): `ops/cutover/validate-go-no-go.ps1`,
  `ops/cutover/rollback.ps1`, and `migration/cmd/livecompare` are recorded in
  `docs/CUTOVER_GO_NO_GO_TEMPLATE.json`'s `_validatorScript` field,
  `docs/RUNBOOK_CUTOVER.md` §9.3/§4.1, and `docs/PARALLEL_DAY_WATCHER.md`
  respectively.

## Acceptance still open (updated 2026-08-08)

The physical acceptance boundary described above under "Acceptance still
open" is unchanged in kind and now also covers the items added this session:
barcode scanner UI wiring proves synthetic-event and mocked-network behavior
only, not a real HID scanner; the biometric/SMTP/SMS adapter plumbing proves
routing, configuration, and error-shape behavior only, not a live send or a
real biometric match (which remains unimplemented by design). The checklist
in [`PHASE_U_DEVICE_ACCEPTANCE_CHECKLIST.md`](PHASE_U_DEVICE_ACCEPTANCE_CHECKLIST.md)
records these as distinct, still-unchecked rows separate from the
already-passing adapter/plumbing unit-test gate.
