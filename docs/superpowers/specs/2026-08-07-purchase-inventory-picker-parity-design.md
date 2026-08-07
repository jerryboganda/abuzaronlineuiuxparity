# Purchase Workflows Real-Time Inventory Picker & Item Search Parity Design

## Overview
This design document details the technical specification for adding full real-time **Inventory Lookup & Item Search Parity** across all 5 Purchase routes in AbuzarNext (`pack`, `loose`, `opening`, `return`, `order`).

## Intent & Requirements
1. **100% Functionality & Visual Parity**:
   - Matches legacy PowerBuilder behavior (`abuzar.exe`), where opening any Purchase document window presents a real-time inventory picker showing the complete item catalog.
2. **Dual-Mode Inventory Surface**:
   - **Embedded Inventory Panel**: Integrated real-time inventory table situated between header fields and the transaction grid across all purchase routes.
   - **Floating / Popup Inventory Window**: Dockable and toggleable floating modal window (`F2` shortcut or toolbar button) displaying real-time stock levels, purchase prices, 10 sale price tiers, pack units, packing, location, and manufacturer.
3. **Real-time Live Filtering**:
   - Search across Item Name, Code, Legacy ID, Alias Name, Alternate Alias, Barcode, Location, and Manufacturer.
4. **Instant Item Selection**:
   - Single click or `Enter` key on any item populates the current active line row in the purchase document, auto-assigning purchase price, sale price, manufacturer, location, and pack units.
5. **Universal Coverage Across All Purchase Routes**:
   - Pack Purchase (`/app/purchase/pack`)
   - Purchases (Loose) (`/app/purchase/loose`)
   - Opening Purchase (`/app/purchase/opening`)
   - Purchase Return (`/app/purchase/return`)
   - Purchase Order (`/app/purchase/order`)

## Data & API Integration
- **`GET /v1/items/lookup?q={query}`**: Returns canonical item lookup records with stock, purchase price, sale prices (1-10), location, and manufacturer.
- **State Management**: Reactive inventory lookup results filtered by current godown context and live user input query.

## Component Architecture
- `apps/web/src/routes/app/purchase/[kind]/+page.svelte`: Updated with dual-mode inventory surface state, reactive lookup streams, selection handlers, keyboard shortcuts (`F2`), and floating modal window overlay.
- `apps/web/src/lib/styles.css`: CSS styling for `.legacy-purchase-lookup` panel and floating `.legacy-purchase-inventory-window`.

## Verification Plan
1. Typecheck: `pnpm --filter @abuzar/web check` (0 errors, 0 warnings).
2. Static Build: `pnpm --filter @abuzar/web build`.
3. Automated Playwright E2E Spec: Verify item selection, real-time filtering, and row population across all 5 purchase routes.
4. Production VPS Deployment: Upload updated static bundle to `/opt/docker/abuzarnext/build/` on VPS (`185.252.233.186`).
