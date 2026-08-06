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
(2 URL/configuration tests), and the Tauri production build produced the NSIS
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
