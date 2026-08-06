# Web/PWA client

The SvelteKit application is the canonical frontend for Chrome and Tauri. The current shell establishes custom typography, dense PowerBuilder-inspired controls, navigation, tenant/branch/counter scope presentation, status indicators, and parity test hooks. Existing screens are added only after their baseline evidence is catalogued under `parity/`.

For a branch-edge connection, set `localStorage.abuzar.edgeUrl` to the branch service URL and `localStorage.abuzar.edgeSecret` to the provisioned branch secret. The secret is only a deployment credential; it is never compiled into the client. Production deployments should prefer a reverse proxy or the Tauri credential store rather than persistent browser storage for that value.
