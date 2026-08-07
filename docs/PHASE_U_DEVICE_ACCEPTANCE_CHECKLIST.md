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
- [ ] HID scanner normalization and item lookup measured at POS speed.
- [ ] Cash drawer pulse tested with the approved drawer and pin/timing.
- [ ] Operator sign-off recorded for the branch and device serials.

The physical section remains intentionally unchecked. Deterministic render
goldens, adapter fakes, readiness diagnostics, and unavailable-device fixtures
prove software behavior only; they do not prove byte parity, device presence,
or successful physical output.
