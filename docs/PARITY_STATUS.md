# Parity status - 2026-08-06

## Implemented in this wave

- The browser legacy shell now builds its menu tree from `parity/catalog/legacy-menu-tree-2026-08-05.json` (275 captured entries, 9 top-level menus), including recursive submenus, command IDs, keyboard shortcuts, and deterministic route metadata.
- Purchase workflows are available at `/app/purchase/pack`, `/return`, `/opening`, `/loose`, and `/order` with editable legacy-style grids, tenant/branch/counter-scoped events, idempotency, online posting, and offline queue fallback.
- Cash/credit sales, sale returns, quotations, and refused-sales documents use concrete transaction endpoints. Sale rows also project into the scoped inventory ledger.
- Customer, supplier, item, manufacturer, category/template master forms persist through `/v1/master/*`; the Users workflow lists, creates, selects, and updates tenant operators with password hashing and validated group, branch, and counter assignments through `/v1/operators`.
- The remaining captured Basic Data master leaves (customer/supplier categories, promotions/sectors, item class/category/generic data, price/tax/PCT settings, lock reasons, segments, types, templates, and related lists) now retain their own master kind and route-specific title instead of being collapsed into one category bucket.
- Master List rows are now selectable: the captured Detail/List workflow reloads legacy payload fields into the Detail form, and Save updates the tenant record through `PATCH /v1/master/{kind}/{id}` instead of always creating a duplicate.
- Item Detail now exposes the legacy supplier sub-grid (Priority, Rate, Disc%, Qty, Bonus, Days) and replaces the tenant-scoped links through `PUT /v1/master/item/{id}/suppliers`; a focused browser regression covers edit and persistence.
- Manage → Groups now lists, creates, and updates tenant-scoped roles and their validated permission sets through `/v1/roles`, with administrator authorization, RLS-backed storage, and audit events.
- Every captured report leaf reaches the report argument/retrieve/export surface. Four primary report projections are implemented; other catalogued report kinds use the scoped immutable-event projection until their exact legacy columns are captured.
- Report workflows now include the legacy retrieval-arguments dialog (areas, date range, cash/credit selection), format selection, print preview, CSV export, and a captured daily-sales-detail loading state.
- Detail/List tabs are live on purchase, sales, and master-data surfaces; sales and purchase list views query persisted tenant/branch-scoped transaction history through `/v1/transactions/{kind}` rather than rendering draft placeholders.
- Sales and purchase List rows and toolbar Previous/Next actions now load persisted documents back into the Detail form, preserving the legacy navigation workflow instead of only changing a status message.
- Manage → Cashier Activity Window now reads the scoped shift ledger through `GET /v1/shifts` and renders operator, open/close, status, and cash totals with a live refresh action.
- The non-legacy workspace dashboard now reads sales history, shifts, and branches from the authenticated API; hardcoded demo metrics/activity were removed and empty-state labels are explicit.
- Dashboard navigation/export/notifications/context actions and the report toolbar (save layout, sort, paging, refresh, print, CSV export, and format settings) now execute concrete client actions.
- The main-window Minimize/Restore controls now change the rendered shell state; Close still returns to the application entry point.
- Captured menu shortcuts are now actionable in the live shell; the recorded Ctrl+Alt+M Session Monitor shortcut (and Ctrl+X Exit) route through the same command metadata as menu clicks.
- Sales and purchase windows now dispatch captured contextual File/Item verbs for New, Save/Post/Save And Post, Print, Previous/Next, and New Item into their live handlers; unsupported contextual verbs remain visibly phase-labelled. The Save And Post accelerator has a focused browser regression.
- Purchase Order now emits the correct `purchase_order` event aggregate when saved, preserving the legacy workflow boundary and avoiding accidental receiving projection.
- Purchase pack/loose/opening/return/order forms now bind their selected supplier, item, godown, batch, and source-batch identities to the canonical document command contract; Save creates a versioned draft and Post/Save And Post uses the existing draft version, while incomplete compatibility entries retain the legacy event fallback.
- The captured purchase Lookup action and Ctrl+B automatic batch workflow are live (`AUTO-YYYYMMDD-NNN`), with focused coverage for canonical rejection, draft/post revision state, purchase orders, returns, free-text fail-closed validation, and batch generation.
- High-frequency contextual purchase/sales commands now have live handlers: list/history navigation, auto batch generation, item sorting, row delete/restore, item/customer/supplier information routes, sale-slip/purchase-label output with branch-edge fallback, change-user/exit, and local offline queue synchronization.
- Preferences persist by tenant/category through `/v1/preferences` and retain the captured tab/grid structure.
- Preferences now load and save tab-specific captured labels/defaults for all 17 categories; Cancel restores the last loaded values and each ellipsis editor focuses its matching field.
- Schedule and Email preference tabs now switch from the untouched screenshot baseline to their captured form layouts on interaction, with editable controls wired to the same tenant/category persistence path.
- Maintenance and manage leaves have concrete tenant-audited workflow surfaces. Inventory adjustments, password changes, cashier shift open/close, integrity checks, and backup requests have API paths; backup artifact generation remains deployment-policy-owned. Captured backup dialogs and the change-user confirmation now have live Svelte controls over an exact untouched-state raster.
- Maintenance/manage form values now persist through `GET/POST /v1/maintenance/{kind}` in tenant-scoped preference storage, reload on direct navigation, and remain represented in the audit ledger instead of being acknowledgement-only actions.
- Generic catalog leaves now load/search the last saved tenant-scoped record; master-data Cancel restores the selected record/new state and maintenance/manage Cancel clears the active form.
- Maintenance and Manage leaves that previously collapsed to reference/notes now expose route-specific persisted fields for pricing, discounts, reorder quantities, supplier links, batch locks, item updates, import validation, initialization scope, group settings, interfaces, and job schedules.
- Report leaves now select the matching immutable aggregate (sale, sale return, purchase, purchase return, quotation, order, inventory, or refused sale), reject inverted date ranges, and provide sortable, paged result grids. Sale and purchase item lookup lists use synchronized item master records when available; when normalized `master_items` is empty, the item list exposes the tenant-scoped Phase E `master_records(kind=item)` catalog as an explicit read-only fallback instead of silently using the 32-row demo list.
- Transaction and workflow toolbar glyphs are normalized through CSS Unicode fallbacks so the live interactive surface does not expose the prior mojibake symbols; daily sales detail also restores customer/item/quantity values from the source event payload.
- The authored web source has no known double-encoded UTF-8 markers; the old text-node MutationObserver band-aid is no longer part of the runtime.
- Users list selection now reopens the captured Detail form; Save updates name, active state, group, password (when supplied), and branch/counter assignment instead of creating a duplicate operator.
- Groups permission checkboxes now reload with the selected role and persist the supported permission set in the same legacy Save workflow.
- Effective role permissions are loaded into the HTTP-only session context and enforced for master data, reports, transaction history/posting, shifts, maintenance actions, operator management, and synchronization conflict review; tenant administrators retain the legacy full-access path.
- Multi-branch/counter operators can select and persist an operational context through `/context` and `POST /v1/session/context`.
- The live sales lookup normalizes legacy master payload casing and price-tier names before filling the transaction grid; the focused canonical lifecycle test still passes after this projection change.

## Verification evidence

- `pnpm --filter @abuzar/web check` - 0 errors, 0 warnings.
- `pnpm exec playwright test --workers=1` - 20 browser smoke tests passed, including persisted sales-history List rendering, recursive report-menu navigation, keyboard shortcuts, route reachability for transaction, master, report, preferences, maintenance, and manage surfaces, direct-SSR and interactive Schedule/Email/maintenance regressions, operator update, and Groups permission persistence.
- A post-restart targeted run of the legacy-shell shortcut, Users update, and Groups permission tests passed (`3 passed`); the exact `127.0.0.1` webServer URL prevents Playwright from starting a competing Vite process.
- `go test ./migration/...` - passed after adding fail-closed validation for empty tenant/branch injections and mapped identifiers.
- The Users browser regression passes with a mocked tenant-scoped PATCH request and confirms the selected operator is edited through the Detail form.
- The focused follow-on browser checks for route-specific Maintenance fields and interactive Schedule/Email preference layouts pass (`2 passed`); the API package regression suite also passes after expanding captured Basic Data master kinds.
- `pnpm --filter @abuzar/web build` - production static build passed.
- `go test ./services/api/... ./services/edge/... ./migration/...` - passed.
- A catalog walk exercised all 221 captured leaf routes generated from the 275-entry menu catalog; every route rendered a Svelte `main` surface with no navigation failure.
- The same catalog walk also passes direct SSR/deep-link requests (`220` unique generated URLs; the remaining leaf is the intentional File Exit route), including generic module pages after a cold start; generic module slugs and legacy path values are normalized before title rendering so direct `/app/module/*`, report, master, maintenance, and manage requests cannot dereference an undefined value.
- Local API `/v1/health`, edge `/v1/health`, and all sampled web deep links returned HTTP 200 after restart.
- The Windows Tauri production build completed and produced both the NSIS setup executable and MSI under `apps/desktop/src-tauri/target/release/bundle`.
- `ops/local/start-local.ps1`, `stop-local.ps1`, and `status-local.ps1` provide a repeatable reboot-safe local stack lifecycle with detached supervisors, restart-on-crash behavior, bounded logs, and health probes without touching the legacy installation.
- Local PostgreSQL migrations applied through `008_role_permissions.sql`; login, context selection, a scoped sale POST, report retrieval, integrity checks, backup-audit request, preference reads, and role permission storage were exercised.
- Live operator regression coverage exercised login, group listing, operator listing, operator creation, tenant/branch/counter assignment, and cleanup; the PostgreSQL array-to-JSON boundary was corrected so operator lists no longer return a 503.
- Authenticated disposable sale and receiving events were posted through the API and retrieved through the new history endpoint, including item/amount projections and cleanup verification.
- Current local health: API and edge both report `status=ok` and `database=ok` on ports 8080/8091; web is served at `http://127.0.0.1:5173/login`.
- The maximized shell screenshot gate passes exactly (`differentPixels=0` at 1936x1048).
- Exact captured-default comparisons now pass (`differentPixels=0`) for Pack Purchase, Cash Sale, Customer, Supplier, Item, User, Preferences, Daily Sales Detail loading, report retrieval arguments, report format selection, Backup Database, Backup Device, Backup Information, Change User, and the Database Integrity Monitor. The reports are recorded in `tmp/*-comparison.json` and can be regenerated with `parity/tools/compare-png.ps1`.

## Addendum — independent live audit, 2026-08-06

A full side-by-side runtime audit (legacy exe + new stack both running, screens walked visually, menus enumerated per window, databases counted, code reviewed) found that the wave claims above describe *surface* coverage, not functional parity. Authoritative documents:

- `docs/GAP_ANALYSIS_2026-08-06.md` — 15 verified gap areas (G1–G15) with evidence in `tmp/gap-audit/`.
- `docs/PARITY_FIX_PLAN_A-Z.md` — 26-phase plan to reach 100% visual + functional parity.

Corrections to earlier claims:

- **Mojibake was not fixed** by the CSS "Unicode fallbacks" wave: 13 rules in `apps/web/src/lib/styles.css` used double-escaped `content: "\\XXXX"` and rendered literal `\2190`/`\25A6 Detail` text on live surfaces. Actually fixed and browser-verified on 2026-08-06.
- **`Sales > CashSale` routed to the generic `/app/module/cashsale` stub**, not the real sales surface (legacy caption `Cash&Sale` cleans to `CashSale`, which the route map missed). Fixed in `lib/legacy-menu.ts` and browser-verified on 2026-08-06.
- The 275-entry menu catalog is **incomplete**: with a document window open the legacy shell exposes 325+ entries (contextual Item menu + ~35 File verbs + Window refresh variants). See `tmp/gap-audit/live-menu-tree-pack-purchase.json`.
- The 221-route catalog walk proves routes render, not that they are functional: most business logic (tax, discounts, stock checks, batch/expiry, GL, ledgers, rights) is absent and the local database contains no migrated data (0 `master_records`, 0 `legacy_id_mappings` vs 763 legacy tables).
- Correction after the Phase E sandbox wave: the local disposable database now contains 61,202 `master_records` and 83,447 `legacy_id_mappings` from a reviewed 18-table sandbox/reference import. This is not the canonical SQL Server migration; historical documents, ledgers, stock/batch data, and reconciliation remain open.
- The local stack supervisor now detaches web/API/edge processes and restarts failed children; the Playwright webServer probe is pinned to `127.0.0.1` so test teardown does not compete with the supervised web process. A 24-hour soak is still open.
- All 17 captured Preferences tabs are now individually mapped to their own canonical raster baseline and each tab comparison passes at `1536x972`.
- These exact raster gates cover the untouched canonical state only. Controls remain mounted and switch to the live semantic Svelte surface on pointer, focus, or keyboard input; functional tests, not the raster, validate posting and persistence.

## Acceptance still open

## Pricing and transaction-core evidence - 2026-08-06

- The deterministic pricing engine is now reachable through the authenticated `POST /v1/transactions/preview` contract. It validates exact decimal money/percentage/whole-quantity inputs, applies price-tier selection, supplier schemes, item/customer/document discounts, Misc, and GST/PCT/advance-tax ordering without floating-point arithmetic.
- Sale detail fields now debounce a server pricing preview while the legacy-style form is edited. The displayed total and discount values use the preview when online, while the existing local line total remains the offline fallback; the posted event carries the auditable pricing result.
- Focused evidence: `go test ./services/api/internal/httpapi ./services/api/internal/pricing`, `go test ./services/api/... ./services/edge/... ./migration/...`, `pnpm --filter @abuzar/web check`, and three targeted Playwright smoke tests all pass. A live authenticated preview returned `223.02` for a two-line `200.00` basket with a 5% document discount, `1.00` flat discount, and 18% GST.
- Sale projection replays a supplied pricing request inside the posting transaction, rejects forged totals, validates `draft`/`posted`/`voided` status values, detects document-number collisions, and rejects non-positive inventory quantities. This closes an integrity gap but is not yet a full stock/GL/lifecycle implementation.
- `GET /v1/inventory/balance` now requires tenant/branch/godown scope and reads `stock_balances`; selecting an item in the live sale grid refreshes its Stock cell from that normalized cache. A labeled `inventory_movements` fallback is used only when the requested normalized scope has no rows.

The pricing preview is a validated calculation surface, not proof that all legacy tax tables, batch/expiry rules, stock availability, GL/ledger postings, or every document lifecycle are complete. Those remain acceptance work below.

The legacy catalog is fully reachable, but true 100% functional and pixel parity is not yet proven. The captured interior states cover representative transaction, master, preference, report-loading, maintenance-dialog, and change-user workflows; remaining leaves still need their own legacy screen/workflow capture, exact field rules, report columns, printing/hardware behavior, and screenshot/keyboard acceptance before they can be marked parity-complete. The 186 report leaves currently share a safe scoped event projection where their exact legacy report columns have not been captured. Printer, barcode, cash-drawer, biometric, SMS/email, and complete historical SQL Server migration acceptance are also still open.

## Phase S/T maintenance and manage slice - 2026-08-06

- Maintenance operations now create tenant/branch-scoped `maintenance_operation`
  audit records and return an operation/job identifier plus an explicit status.
  Integrity output is application-scope row checking; physical PostgreSQL
  integrity, backup, restore, import/export artifacts, and external sends remain
  `not_configured` unless a deployment adapter is supplied.
- Backup/restore requests never claim that a physical artifact was created or
  restored. Import requests reject server paths and are not opened by the API.
  Stock adjustment/opening forms send canonical item/godown/batch identity to
  the existing guarded inventory projection; invalid quantity, expiry, batch,
  tenant, and branch checks remain server authoritative.
- Session Monitor now reads active HTTP sessions through a permission-gated,
  current-branch query and does not expose token material. Existing real
  password, user/group, and cashier shift APIs remain the execution paths;
  template values are configuration only and SMS/email delivery still requires
  the edge adapters in Phase U.
- Cancel restores the last loaded/saved workflow state instead of presenting a
  false reset, and direct reload includes the last operation status.

Evidence: [`tmp/phase-s-t-maintenance-evidence-2026-08-06.md`](../tmp/phase-s-t-maintenance-evidence-2026-08-06.md).
