# Tauri desktop wrapper

This wrapper loads the same SvelteKit build used by Chrome. Native capabilities such as local branch-edge coordination, printing, cash drawers, biometric devices, and other hardware are added through explicit Tauri commands or the branch-edge service; no device fingerprinting or licensing checks are permitted.

Run `pnpm --filter @abuzar/desktop dev` for the development wrapper. Run `pnpm --filter @abuzar/desktop build` to produce the Windows NSIS and MSI bundles under `src-tauri/target/release/bundle/`.

The wrapper exposes explicit `set_api_session`, `get_api_session`, and `clear_api_session` commands backed by the Windows Credential Manager through the `keyring` crate. It never stores database credentials or hardware fingerprints.

## Branch-edge hardware commands

The native bridge exposes `set_edge_config`, `get_edge_config`, and
`clear_edge_config`, plus `get_hardware_capabilities`, `print_sale_slip`,
`get_hardware_readiness`, `print_purchase_labels`, `lookup_barcode`, and
`kick_cash_drawer`. The edge URL and optional shared secret are stored together
in Windows Credential Manager; the read command reports only the URL and
whether a secret is configured.
Hardware commands call the configured branch edge and return its status/error
status and problem code so the caller can distinguish `503
hardware_adapter_unavailable` from a successful operation. They never open a
device directly and never report financial success.

Configure a deployment-provided URL explicitly (for example,
`http://127.0.0.1:8091` in local development). An empty shared secret is
permitted only to match the edge service's local-development mode; deployed
branches should configure `ABUZAR_EDGE_SHARED_SECRET` on the edge and the
matching secret through the native command. Secrets must not be put in source,
logs, or installer arguments.
