# Milestone M4 (Edge Hardware Integration & Desktop Tauri) Analysis & Evidence Report

## 1. Observation

### 1.1 Core Hardware Integration Subsystem (`services/edge/internal/hardware/`)
- **ESC/POS Sale Slip Renderer** (`services/edge/internal/hardware/escpos.go:57-101`):
  `RenderSaleSlip(slip SaleSlip)` generates deterministic byte streams using standard ESC/POS commands:
  - `escInit` (`\x1b\x40`): Printer initialization
  - `escAlignCenter` (`\x1b\x61\x01`) & `escAlignLeft` (`\x1b\x61\x00`): Text alignment
  - `escBoldOn` (`\x1b\x45\x01`) & `escBoldOff` (`\x1b\x45\x00`): Emphasized text
  - `escCutPartial` (`\x1d\x56\x01`): Partial paper cut
  - Verified by golden hex byte fixture `services/edge/internal/hardware/testdata/sale-slip.hex` in `TestRenderSaleSlipMatchesByteGolden` (`escpos_test.go:11-32`).

- **ESC/POS Purchase Label Renderer** (`services/edge/internal/hardware/escpos.go:106-134`):
  `RenderPurchaseLabels(batch PurchaseLabelBatch)` formats label items with batch, expiry, MRP, and quantity fields, with optional partial paper cut (`batch.CutAfter`).
  - Verified by golden hex byte fixture `services/edge/internal/hardware/testdata/purchase-label.hex` in `TestRenderPurchaseLabelsMatchesByteGolden` (`escpos_test.go:34-49`).

- **Cash Drawer Pulse Command (`0x1b 0x70`)** (`services/edge/internal/hardware/registry.go:48-60`):
  `CashDrawerKickCommand` defines device-neutral pulse parameters (`Pin`, `OnTime`, `OffTime`). Default parameters:
  - `Pin: 0`, `OnTime: 25`, `OffTime: 250`
  - `ESCPosBytes()` returns byte sequence: `[]byte{0x1b, 0x70, 0x00, 0x19, 0xfa}`
  - Verified in `TestRegistryUsesInjectedAdaptersWithoutClaimingUnavailableHardware` (`registry_test.go:79-82`).

- **Barcode Lookup & Input Normalization** (`services/edge/internal/hardware/registry.go:326-356`):
  `NormalizeBarcode(raw)` strips scanner leading/trailing whitespace (`\r\n\t `), validates against control characters (`< 0x20` or `0x7f`), and resolves items via `LookupBarcode(ctx, code)`.
  - Verified in `TestNormalizeBarcodeTrimsScannerWhitespace` (`registry_test.go:85-101`).

- **Hardware Capability Registry & Readiness Reporting** (`services/edge/internal/hardware/registry.go:12-20, 100-110, 249-279`):
  `Registry` manages 6 capabilities: `thermal_printer`, `barcode_scanner`, `cash_drawer`, `biometric_reader`, `sms`, `email`.
  - Default `New()` constructs unconfigured registry where `Readiness()` returns `status: unavailable`, `availableCount: 0`, `totalCount: 6`.
  - Attempts to invoke missing hardware operations return `ErrAdapterUnavailable` without throwing panics or reporting fake success.

### 1.2 Edge REST API Endpoints (`services/edge/internal/syncapi/server.go`)
- Hardware endpoints registered in `NewWithHardware`:
  - `GET /v1/hardware/capabilities` -> `server.hardwareCapabilities` (lines 109-111)
  - `GET /v1/hardware/readiness` -> `server.hardwareReadiness` (lines 113-115)
  - `POST /v1/hardware/print/sale-slip` -> `server.printSaleSlip` (lines 117-128)
  - `POST /v1/hardware/print/purchase-labels` -> `server.printPurchaseLabels` (lines 130-141)
  - `POST /v1/hardware/barcode/normalize` -> `server.normalizeBarcode` (lines 143-156)
  - `POST /v1/hardware/barcode/lookup` -> `server.lookupBarcode` (lines 158-171)
  - `POST /v1/hardware/cash-drawer/kick` -> `server.kickCashDrawer` (lines 173-179)
- Bearer secret protection: `requireSecret` middleware enforces `Authorization: Bearer <sharedSecret>` (lines 60-73).
- Error translation: `writeHardwareProblem` converts `ErrAdapterUnavailable` to HTTP 503 (`hardware_adapter_unavailable`) and `ErrInvalidConfiguration` to HTTP 503 (`hardware_configuration_invalid`).

### 1.3 Desktop Tauri IPC Bridge & Windows Credential Manager (`apps/desktop/src-tauri/src/lib.rs`)
- **Windows Credential Manager Integration**: Uses Rust `keyring` crate targeting service `com.abuzar.next`:
  - `set_edge_config`, `get_edge_config`, `clear_edge_config` (account `edge-config`) (lines 153-264)
  - `set_api_session`, `get_api_session`, `clear_api_session` (account `api-session`) (lines 388-418)
- **Tauri IPC Command Dispatch**:
  - `get_hardware_capabilities`, `get_hardware_readiness`, `print_sale_slip`, `print_purchase_labels`, `lookup_barcode`, `kick_cash_drawer` (lines 340-385)
- **HTTP Client & Error Bridge**: `edge_request_with_config` (lines 266-325) validates HTTP/HTTPS Edge URL, attaches `Authorization: Bearer <shared_secret>`, and translates HTTP problem details into `HardwareCommandError`.
- **IPC Error Preservation**: Rust test `ipc_preserves_unavailable_edge_error_and_rejects_success_payload` (lines 470-507) verifies that HTTP 503 `hardware_adapter_unavailable` is strictly preserved as an error in Rust and never converted into a success payload.

### 1.4 SvelteKit Frontend Integration (`apps/web/`)
- `apps/web/src/lib/api.ts:48-76`: `edgeBaseUrl()` resolves configured Edge URL or defaults to `http://127.0.0.1:8091`. `edgeRequest<T>(path, body)` executes POST requests with `Bearer abuzar.edgeSecret`.
- `apps/web/src/routes/app/sales/+page.svelte`: `printSaleSlip()` (lines 115-136) calls `edgeRequest('/v1/hardware/print/sale-slip', slip)`, falling back to `window.print()`. `submitSale` (line 623) triggers cash drawer pulse `edgeRequest('/v1/hardware/cash-drawer/kick', {})` on cash sales.
- `apps/web/src/routes/app/purchase/[kind]/+page.svelte`: `printPurchaseLabels()` (lines 195-203) handles purchase label preview / print triggers.

### 1.5 Test Verification Results
- Executed `go test ./services/edge/... -v`:
  - `github.com/abuzar/abuzar-next/services/edge/internal/hardware` PASS (0.651s)
  - `github.com/abuzar/abuzar-next/services/edge/internal/store` PASS (1.329s)
  - `github.com/abuzar/abuzar-next/services/edge/internal/syncapi` PASS (1.385s)
  - `github.com/abuzar/abuzar-next/services/edge/internal/syncer` PASS (2.012s)
- Executed `cargo test --manifest-path apps/desktop/src-tauri/Cargo.toml`:
  - `test tests::accepts_explicit_http_edge_url ... ok`
  - `test tests::rejects_edge_url_credentials_and_ambiguous_suffixes ... ok`
  - `test tests::ipc_preserves_unavailable_edge_error_and_rejects_success_payload ... ok`
  - Result: 3 passed, 0 failed.

---

## 2. Logic Chain

1. **ESC/POS Stream Integrity**:
   - `RenderSaleSlip` and `RenderPurchaseLabels` construct pure byte streams conforming to ESC/POS standards (`0x1b 0x40`, `0x1b 0x61 0x01`, `0x1b 0x45 0x01`, `0x1d 0x56 0x01`).
   - The renderers format data supplied by caller workflows without embedding business rules or communicating directly with hardware drivers.
   - Byte-exact verification against `sale-slip.hex` and `purchase-label.hex` guarantees deterministic output formatting.

2. **Cash Drawer Pulse Protocol**:
   - `CashDrawerKickCommand.ESCPosBytes()` produces `0x1b 0x70 0x00 0x19 0xfa`, matching ESC/P pulse specification `ESC p m t1 t2` (pin 0, onTime 25ms, offTime 250ms).
   - This provides standard drawer open capability for POS receipt printers with RJ11/RJ12 drawer kick ports.

3. **Barcode Input Processing**:
   - `NormalizeBarcode` strips scanner carriage returns (`\r\n`) and tab characters produced by HID wedge hardware.
   - Control character validation prevents invalid scanner streams from causing SQL or downstream formatting errors.

4. **Fail-Safe Degradation and Error Boundary**:
   - In environments without physical hardware adapters, `Registry` returns `ready: false` and `status: unavailable`.
   - Hardware API requests return HTTP 503 (`hardware_adapter_unavailable`).
   - The system never throws panics, fake success responses, or silent failures.

5. **Desktop Security & IPC Pass-Through**:
   - Windows Credential Manager (`keyring` crate) isolates edge secrets and API session tokens from plaintext config files.
   - The Tauri IPC bridge in `apps/desktop/src-tauri/src/lib.rs` forwards IPC command invocations to the Edge REST service with Bearer token authentication.
   - Error responses (e.g. 503 Service Unavailable) are mapped directly to `HardwareCommandError`, maintaining strict error semantics across the web-desktop boundary.

6. **Web Frontend Integration**:
   - SvelteKit sales and purchase routes seamlessly call Edge hardware REST endpoints when available, with automatic browser fallback for print dialogs when offline or unconfigured.

---

## 3. Caveats

- **Physical Device Drivers**: Physical hardware adapters (e.g., serial/USB thermal printer drivers, physical cash drawer controllers, USB HID scanner hooks) are abstracted by Go interfaces (`PrinterAdapter`, `CashDrawerAdapter`, `BarcodeLookupAdapter`). The edge service operates in headless/simulated mode by default and allows physical adapters to be injected at host startup.
- **Desktop Tauri Runtime Requirement**: Desktop IPC features (`apps/desktop/src-tauri/`) require running inside a Tauri application window for IPC commands. When accessed in standard web browsers, frontend applications fall back to direct HTTP REST calls to `http://127.0.0.1:8091`.

---

## 4. Conclusion

The Milestone M4 Edge Hardware Integration Subsystem and Desktop Tauri Integration are fully designed, implemented, and verified.
- **ESC/POS Renderers**: Receipts and labels render deterministic byte streams verified against golden byte files.
- **Cash Drawer**: Pulse command generates correct `0x1b 0x70` ESC/P byte sequence (`0x1b 0x70 0x00 0x19 0xfa`).
- **Barcode Lookup**: Normalizes HID wedge inputs and queries local item source.
- **Readiness & Degradation**: Returns explicit status and HTTP 503 when adapters are unconfigured.
- **Tauri IPC & Credential Manager**: Stores edge secrets in Windows Credential Manager (`com.abuzar.next`) and bridges IPC commands securely.
- **Tests**: 100% of unit/integration tests in `services/edge/...` pass clean.

---

## 5. Verification Method

To independently verify these findings:

1. **Run Go Edge Package Tests**:
   ```powershell
   go test ./services/edge/... -v
   ```
   *Expected output*: All tests in `internal/hardware`, `internal/store`, `internal/syncapi`, and `internal/syncer` pass with `PASS`.

2. **Run Desktop Tauri Rust Tests**:
   ```powershell
   cargo test --manifest-path apps/desktop/src-tauri/Cargo.toml
   ```
   *Expected output*: Tests `accepts_explicit_http_edge_url`, `rejects_edge_url_credentials_and_ambiguous_suffixes`, and `ipc_preserves_unavailable_edge_error_and_rejects_success_payload` pass with `ok`.

3. **Inspect Source Files**:
   - `services/edge/internal/hardware/escpos.go` (ESC/POS renderer logic)
   - `services/edge/internal/hardware/registry.go` (Cash drawer pulse, barcode lookup, capability registry)
   - `services/edge/internal/syncapi/server.go` (HTTP hardware handlers)
   - `apps/desktop/src-tauri/src/lib.rs` (Tauri IPC commands & Windows Credential Manager integration)
   - `apps/web/src/lib/api.ts` (Web edge client integration)
