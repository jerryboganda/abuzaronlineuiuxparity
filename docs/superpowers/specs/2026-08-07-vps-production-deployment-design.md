# VPS Production Deployment Design — pms.polytronx.com

- **Date**: 2026-08-07
- **Target VPS**: `185.252.233.186`
- **Domain**: `pms.polytronx.com`
- **Goal**: Deploy AbuzarNext to the production VPS using shared infrastructure (PostgreSQL `platform-postgres`, Nginx Proxy Manager), maintaining ultra-lightweight resource usage ($\le 50$ MB RAM footprint total).

---

## 1. Shared Infrastructure Architecture

```
Internet / Cloudflare DNS (pms.polytronx.com)
            │
            ▼
[Nginx Proxy Manager (Port 80/443)]
            │  (SSL termination & reverse proxy to 127.0.0.1:8095)
            ▼
[AbuzarNext Web Container (Nginx Alpine)] (Port 8095)
    ├── Serves static SvelteKit SPA (/app, /login, /report...)
    └── Proxies /v1/* to [AbuzarNext API Container (Go binary)]
            │
            ▼
[Platform PostgreSQL Container (platform-postgres)] (Port 5432 internal)
    └── Database: `abuzarnext`
```

---

## 2. Component Specifications

### 2.1 Database (`abuzarnext`)
- Created within existing `platform-postgres` container on `platform` docker network.
- Migrations 001 through 029 applied sequentially using ordered SQL scripts.

### 2.2 Go API Container (`abuzarnext-api`)
- Compiled as a statically linked CGO-free Linux x86_64 binary (`GOOS=linux GOARCH=amd64`).
- Base image: `alpine:latest` (~10MB image, ~15MB runtime RAM).
- Environment variables: `DATABASE_URL`, `HTTP_LISTEN_ADDR=:8080`, `APP_ENV=production`.

### 2.3 Web & Reverse Proxy Container (`abuzarnext-web`)
- SvelteKit static export (`apps/web/build`) served via lightweight Nginx Alpine container.
- Nginx configuration:
  - Serves static frontend with fallback to `/index.html` for client-side routing.
  - Proxies `/v1/*` and `/healthz` directly to `abuzarnext-api:8080`.

### 2.4 Domain & SSL (`pms.polytronx.com`)
- Nginx Proxy Manager host configuration for `pms.polytronx.com`.
- Proxy target: `http://172.17.0.1:8095` (or host gateway).
- SSL: Self-Signed / Let's Encrypt / Cloudflare SSL compatibility.

---

## 3. Verification & Health Gates
1. PostgreSQL database `abuzarnext` operational with 29 applied schema migrations.
2. Containers `abuzarnext-api` and `abuzarnext-web` running and healthy.
3. `/healthz` endpoint returning HTTP 200 `{"status":"ok"}`.
4. `https://pms.polytronx.com` returning HTTP 200 with AbuzarNext UI loading cleanly.
