# VPS Production Deployment Implementation Plan — pms.polytronx.com

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan step-by-step.

**Goal:** Deploy AbuzarNext to production VPS `185.252.233.186` for `pms.polytronx.com` using shared infrastructure (`platform-postgres`, Nginx Proxy Manager).

**Tech Stack:** Docker, Nginx Alpine, Go (Linux binary), PostgreSQL 17, PowerShell, SSH.

## Global Constraints
- Target VPS: `185.252.233.186` via SSH root.
- Domain: `pms.polytronx.com`.
- Shared Postgres: `platform-postgres` container.
- Shared Proxy: Nginx Proxy Manager container.
- Lightweight: RAM usage $\le 50$ MB.

---

### Task 1: Provision Database & Run Migrations on VPS

**Actions:**
1. Create `abuzarnext` database and `abuzarnext` user in `platform-postgres`.
2. Transfer and apply PostgreSQL migration scripts 001–029 to `abuzarnext` database on VPS.

---

### Task 2: Build & Package Lightweight Binaries & Static Web Build

**Actions:**
1. Cross-compile Go API binary for Linux x86_64 (`GOOS=linux GOARCH=amd64 CGO_ENABLED=0`).
2. Build SvelteKit static export (`apps/web/build`).
3. Prepare deployment directory `/opt/docker/abuzarnext` on VPS with `docker-compose.yml`, Go API binary, and static web files.

---

### Task 3: Deploy Docker Containers & Connect to Shared Network

**Actions:**
1. Launch `abuzarnext-api` and `abuzarnext-web` containers via `docker-compose up -d` on VPS.
2. Verify `/healthz` API endpoint returns HTTP 200 `{"status":"ok"}` on port 8095.

---

### Task 4: Configure Nginx Proxy Manager for pms.polytronx.com

**Actions:**
1. Create NPM proxy host configuration for `pms.polytronx.com` pointing to `172.17.0.1:8095`.
2. Reload Nginx Proxy Manager container.
3. Test HTTPS request to `https://pms.polytronx.com` and verify clean loading of AbuzarNext UI and API.
