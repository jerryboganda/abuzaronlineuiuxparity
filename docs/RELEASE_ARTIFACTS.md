# Windows release artifacts

The current Tauri release was rebuilt from the checked-in SvelteKit frontend and Windows Credential Manager integration on 2026-08-05.

| Format | Artifact | SHA-256 |
|---|---|---|
| NSIS per-user installer | `apps/desktop/src-tauri/target/release/bundle/nsis/Abuzar Next_0.1.0_x64-setup.exe` | `800BD41964CBC5639CE15065B216FAB0823FA1FB6310CC6F8A9FE3402D8FBFED` |
| MSI installer | `apps/desktop/src-tauri/target/release/bundle/msi/Abuzar Next_0.1.0_x64_en-US.msi` | `D57BDB8023B31E294597D65D0CBCB65216408A28E19DC8D1A28C3F81DF467512` |

These are x64 Windows artifacts. They contain the shared web client and do not contain PostgreSQL credentials, SQL Server drivers, license keys, device fingerprints, or duplicate-instance restrictions. A deployment still needs a reachable central API and PostgreSQL service, or a configured branch edge for offline operation.
