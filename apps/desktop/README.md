# Tauri desktop wrapper

This wrapper loads the same SvelteKit build used by Chrome. Native capabilities such as local branch-edge coordination, printing, cash drawers, biometric devices, and other hardware are added through explicit Tauri commands or the branch-edge service; no device fingerprinting or licensing checks are permitted.

Run `pnpm --filter @abuzar/desktop dev` for the development wrapper. Run `pnpm --filter @abuzar/desktop build` to produce the Windows NSIS and MSI bundles under `src-tauri/target/release/bundle/`.

The wrapper exposes explicit `set_api_session`, `get_api_session`, and `clear_api_session` commands backed by the Windows Credential Manager through the `keyring` crate. It never stores database credentials or hardware fingerprints.
