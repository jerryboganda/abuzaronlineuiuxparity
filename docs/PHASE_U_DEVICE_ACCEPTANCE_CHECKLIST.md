# Phase U device acceptance checklist

Date: 2026-08-07

This checklist separates deterministic software safety from physical-device
acceptance. No item below is a claim that a physical device is connected.

## Automated unavailable-device gate

- [x] Run `go test ./services/edge/internal/hardware ./services/edge/internal/syncapi`.
- [x] Confirm `testdata/unavailable-hardware-acceptance.json` passes.
- [x] Confirm every absent printer, barcode lookup, and cash-drawer request
      returns `503 hardware_adapter_unavailable`.
- [x] Confirm those responses contain no `printed`, `bytes`, `itemId`, `name`,
      or `kicked` success fields.
- [x] Confirm `GET /v1/hardware/capabilities` reports every absent adapter as
      `available: false`.
- [x] Confirm `GET /v1/hardware/readiness` reports `ready: false` and
      `status: unavailable` when the default registry is used.
- [x] Confirm invalid provider/adapter combinations return
      `503 hardware_configuration_invalid` without invoking an adapter.
- [x] Confirm the desktop IPC test preserves the edge status and problem code,
      even when an error body contains a misleading success-shaped field.

## Automated adapter/plumbing gate — 2026-08-08 (barcode UI, biometric, SMTP, SMS)

Software-only, same as the section above: proves code/routing/error-shape
behavior, not physical device presence or a successful live send.

- [x] Barcode scanner UI wiring (`apps/web/src/lib/barcode-scanner.ts`) is
      live on the sales and purchase item-lookup inputs; Playwright coverage
      in `apps/web/tests/sales-canonical.spec.ts` and
      `apps/web/tests/purchase-canonical.spec.ts` proves a simulated
      fast-keystroke scan calls the edge lookup route and adds a line, and
      that an unresolved lookup surfaces a non-blocking error.
- [x] `Registry.VerifyBiometric`, `SendEmail`, and `SendSMS` exist with
      injected adapter interfaces (`services/edge/internal/hardware/smtp.go`,
      `sms.go`, `registry.go`) and authenticated edge routes
      (`POST /v1/hardware/biometric/verify`, `/email/send`, `/sms/send`).
- [x] Central-API `services/api/internal/httpapi/channel_send.go` reaches a
      configured branch edge for Maintenance test-email/test-sms and reports
      `not_configured` honestly when `ABUZAR_EDGE_CHANNEL_URL` is unset.
- [x] Desktop Tauri `verify_biometric` IPC command exists
      (`apps/desktop/src-tauri/src/lib.rs`).
- [ ] **Not covered by the above:** any biometric matching algorithm. None
      exists in this codebase by design — no vendor SDK or physical reader
      was available to build or test one against.

## Branch configuration review

- [ ] Configure the branch-edge URL explicitly through the desktop command.
- [ ] Store the shared secret only through deployment secret handling and the
      native credential store; do not put it in source, logs, or installer
      arguments.
- [ ] Review the readiness response before enabling a hardware workflow.
- [ ] Record the provider name and adapter implementation for each category.
- [ ] Keep any category that is not physically verified unavailable.

## Physical pilot acceptance — not run

- [ ] Thermal sale slip compared with the approved legacy sample.
- [ ] Purchase labels compared with the approved legacy sample.
- [ ] HID scanner normalization and item lookup measured at POS speed with a
      **real physical scanner** (the automated gate above uses synthetic
      keystroke-timing events, not real scanner hardware).
- [ ] Cash drawer pulse tested with the approved drawer and pin/timing.
- [ ] Biometric reader tested against a real vendor SDK/device (no such SDK
      is integrated yet; this row cannot be started until one is chosen).
- [ ] SMTP live test send to a real mailbox using configured `SMTP_*`
      credentials.
- [ ] SMS live test send through a configured `SMS_GATEWAY_*` provider.
- [ ] Operator sign-off recorded for the branch and device serials.

The physical section remains intentionally unchecked. Deterministic render
goldens, adapter fakes, readiness diagnostics, and unavailable-device fixtures
— now including the barcode-UI, biometric, SMTP, and SMS adapter/plumbing
gate above — prove software behavior only; they do not prove byte parity,
device presence, live credential validity, or successful physical/live
output.
