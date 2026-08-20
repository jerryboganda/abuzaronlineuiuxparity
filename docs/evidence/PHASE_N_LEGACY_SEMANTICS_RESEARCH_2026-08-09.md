# Legacy semantics research — Category/Manufacturer/User Wise Sales Reports — 2026-08-09

Pure-research, read-only pass. No code, tests, or docs other than this file were
touched. Scope: the ~21 still-open leaves under `Reports > Sales Reports >
Category Wise`, `> Manufacturer Wise`, `> User Wise` (per
`docs/evidence/PHASE_N_REPORT_PROMOTION_2026-08-09.md`'s declined-candidates table and
`tmp/gap-audit/legacy-cash-sale-menu.json`'s captured menu paths), plus the
cross-cutting Gross Profit/COGS blocker that also affects `Customer Sales`
and `Purchase Reports` siblings.

**Headline finding: the recurring "no legacy capture to resolve
column-shape/grouping ambiguity" framing conflates two different blockers.**
For Category Wise and Manufacturer Wise, the underlying **name-resolution
data problem is already solved** — `master_categories` and
`master_manufacturers` are fully populated in the sandbox tenant with real
code→name mappings and join to 100% of `master_items` rows (evidence below).
What is still genuinely missing for those two families is the **screenshot/
column-layout evidence** (no capture exists anywhere in this repo). For User
Wise, and for Gross Profit/COGS, the blocker is a **real, confirmed data
gap** — not a missing join, not a sandbox-specific artifact fixable by
picking a different tenant.

---

## 1. `parity/captures/legacy/` — no matching captures

All 70 filenames were listed and the plausibly-relevant ones were opened
(`Read` renders PNGs). None of the 70 corresponds to Category Wise,
Manufacturer Wise, or User Wise Sales Reports, or their parent submenu
flyouts:

- `desktop-reports-menu-open-2026-08-05.png` — shows the **top-level**
  Reports menu only (`Daily Reports`, `Stock Reports`, `Sales Reports`,
  `Purchase Reports`, `Accounts Reports`, `Listing`, `RePrinting`, `Item
  Reports`, `Purchase Return Reports`). The `Sales Reports` flyout itself was
  never expanded/captured, so none of its children (including Category
  Wise/Manufacturer Wise/User Wise) are visible.
- `desktop-sales-menu-open-2026-08-05.png` — the **Sales** top-level menu
  (Cash Sale, Credit Sale, Sale Return, Open Sale Return, Quotation, Refused
  Sales) — a transaction-entry menu, not Reports.
- `sandbox-report-daily-sales-detail-2026-08-05.png` — a "Preparing Your
  Report to Display, Please Wait..." loading placeholder for **Daily Sales
  Detail** (a different, already-golden-verified report family). No content
  rendered.
- `sandbox-report-retrieval-arguments-2026-08-05.png` — a generic "Specify
  Retrieval Arguments" dialog (area selector + date range + Cash/Credit
  checkboxes) captured while opening Daily Sales Detail. Confirms the shared
  retrieval-arguments UI pattern (reusable across all Sales Reports leaves)
  but carries no report-specific column information.
- `sandbox-report-select-format-2026-08-05.png` — a "Select Format" dialog
  captured for Daily Sales Detail, listing 10 named format variants
  (Standard, Standard Format2/3, "Daily Sales Detail with Pack Qty",
  "Standard (with Patient Column)", etc.). Again specific to Daily Sales
  Detail, not Category/Manufacturer/User Wise.
- `sandbox-preferences-report-2026-08-05.png` — the Preferences > Report tab
  (refresh interval, default report header, "Apply Default Date for Report
  Arg. Window?", "Show Account in Reports"). General report preferences, not
  a specific report's layout.

**Conclusion: zero of the 70 captures document the column layout of any
Category Wise / Manufacturer Wise / User Wise leaf.** This part of the
blocker is real and unresolved by anything already in the repo.

## 2. `tmp/gap-audit/` — menu enumeration only, no column/field data

Directory contents: `enum-menu.ps1`, `open-and-capture.ps1`, several
`*-menu.json` files, and screenshots of unrelated flows (Cash Sale, Item
Master, Manage Groups, database errors, the shell). The `*-menu.json` files
(`legacy-cash-sale-menu.json`, `legacy-item-master-menu.json`,
`legacy-manage-groups-menu.json`, `legacy-report-sale-detail-menu.json`,
`live-menu-tree-pack-purchase.json`) are **byte-for-byte identical full menu
dumps** (306 items, captured at different points while other windows were
focused) — each one is a live-app menu enumeration, not a report-content
capture. They confirm the **exact, verbatim menu text and hierarchy**:

```
Reports > Sales Reports > Category Wise
Reports > Sales Reports > Category Wise > Sale And Return
Reports > Sales Reports > Category Wise > Sales
Reports > Sales Reports > Category Wise > Deviated Items
Reports > Sales Reports > Category Wise > Monthly Sale
Reports > Sales Reports > Category Wise > Net Sale
Reports > Sales Reports > Category Wise > Gross Profit
Reports > Sales Reports > Category Wise > Item Wise Sale Discounts Detail
Reports > Sales Reports > Category Wise > Item Category Wise Monthly Sales
Reports > Sales Reports > Category Wise > Category Wise Day Net Sale

Reports > Sales Reports > Manufacturer Wise
Reports > Sales Reports > Manufacturer Wise > Sales
Reports > Sales Reports > Manufacturer Wise > Sales Detail And Summary
Reports > Sales Reports > Manufacturer Wise > Net Sales
Reports > Sales Reports > Manufacturer Wise > Item Sales/Discount
Reports > Sales Reports > Manufacturer Wise > Manufacturer Wise Sales And Return Summary
Reports > Sales Reports > Manufacturer Wise > CNIC/NTN Registered Customers Sales

Reports > Sales Reports > User Wise
Reports > Sales Reports > User Wise > Invoice Graph
Reports > Sales Reports > User Wise > Sales
Reports > Sales Reports > User Wise > Category Summary
Reports > Sales Reports > User Wise > Discount Report
Reports > Sales Reports > User Wise > Net Cash
Reports > Sales Reports > User Wise > Sales Commission
Reports > Sales Reports > User Wise > User Wise Sales Summary
```

Every menu item record has exactly 5 fields: `path`, `position`, `commandId`,
`grayed`, `hasSubmenu` (confirmed programmatically). **No tooltip,
status-bar hint, or field-list metadata exists in this schema** — the PowerBuilder
menu-enumeration tool this was captured with does not expose accelerator
help text or anything beyond the item's own label. So beyond confirming
precise titles/nesting (useful for exact-string matching in the registry),
these JSON files add nothing toward resolving column shape.

## 3. `parity/catalog/legacy-menu-tree-2026-08-05.json` /
   `legacy-menu-tree-contextual.json` — same conclusion

`legacy-menu-tree-2026-08-05.json` has the identical top-level schema
(`capturedAt`, `processId`, `windowTitle`, `menuItemCount`, `topLevelCount`,
`items[]`) as the `tmp/gap-audit` dumps — same 5-field-per-item shape, no
extra metadata.

`legacy-menu-tree-contextual.json` is a *collection* of 5 such captures
(`contexts.0`..`contexts.4`, one per distinct `windowType`/`sourceFile`), but
each nested capture uses the same 5-field item schema. Confirmed
programmatically: no `contexts` entry carries tooltip, description, or
field-list data beyond `path`/`position`/`commandId`/`grayed`/`hasSubmenu`.

**Conclusion: the menu catalogs are exhaustively useful for exact leaf
naming and hierarchy (already reflected in the registry's kind names) but
contain zero column/grouping evidence.**

## 4. Docs grep — `GAP_ANALYSIS_2026-08-06.md`, `PARITY_STATUS.md`, `AGENT_HANDOFF.md`

No hits for "Category Wise", "Manufacturer Wise", "User Wise", "gross
profit"/"deviated"/"sales commission"/"discount report" in
`GAP_ANALYSIS_2026-08-06.md` or `AGENT_HANDOFF.md`. `PARITY_STATUS.md` has
hits, but every one is about the **`Customer Sales`** submenu (a sibling
family, `Reports > Sales Reports > Customer Sales > Customer Category Wise
...`) — a *customer's* category, not an *item's* category/manufacturer, and
not the `Category Wise`/`Manufacturer Wise`/`User Wise` submenus themselves.
Specifically:
- `PARITY_STATUS.md:28-30` — Customer Sales customer-category and
  customer-summary projections, six-field/eleven-field contracts, "exact
  PowerBuilder joins... remain open."
- `PARITY_STATUS.md:423-438` (`## Customer Sales summary projections
  follow-up`) — describes the `Customer Wise Gross Profit` leaf under
  `Customer Sales > Customer Category Wise Sales`, which aggregates by
  *customer*, not item category/manufacturer. Blocked on the same
  stock-allocation cost gap documented in §6 below.

**Conclusion: no doc anywhere in the repo already documents the expected
shape of the Category Wise / Manufacturer Wise / User Wise (item-level)
submenus.** The most concrete prior investigation is
`docs/evidence/PHASE_N_REPORT_PROMOTION_2026-08-09.md`'s "Candidates examined and
declined" table (lines 163-174), which independently reached the same "no
capture exists" conclusion for `category-wise-sales`,
`manufacturer-wise-sales`, `category-wise-item-wise-sale-discounts-detail`,
and `hourly-sales-graph`, and separately declined `user-wise-sales` on a
**data**, not a capture, basis (see §6).

## 5. Legacy install / sandbox on disk — present, not launched

Confirmed present (existence check only, nothing launched):
- `D:\ABUZAR\LegacyReferenceSandbox` — exists (`AbuzarLegacyReference.mdf`,
  `_log.ldf`, `Application/`, `legacy-reference-20260805.bak`).
- `D:\ABUZAR\V2_AbuzarSoftware\Application\abuzar.exe` — exists.

Per `docs/archive/AGENT_HANDOFF.md` §2 ("Environment map"), the sandbox copy is the
correct target for any future write/interactive exercise (the canonical
install must never be modified). **A future session with GUI-automation
tools (computer-use / agent-browser against the Electron/PowerBuilder
window) could open Reports > Sales Reports > Category Wise > (each leaf) and
Manufacturer Wise/User Wise directly in this sandbox and read off the real
column headers, grouping, and Select-Format variants** — this is the
single highest-value next step for closing the remaining ambiguity, and is
explicitly out of scope for this dispatch (no GUI-automation tools were
available/used here).

## 6. Database check — the actual news: category/manufacturer name data is NOT missing

Schema (`db/migrations/010_master_normalized.sql`):
- `master_categories(tenant_id, category_kind, legacy_id, code, name, payload, active, ...)`
  — `category_kind` distinguishes `item_category` / `customer_category` /
  `supplier_category` / `manufacturer_category`; unique on
  `(tenant_id, category_kind, code)`.
- `master_manufacturers(tenant_id, legacy_id, code, name, payload, active, ...)`.
- `item_supplier_links` is defined in `db/migrations/009_phase_e_master_wave.sql`
  (`tenant_id, legacy_item_id, legacy_supplier_id, priority, rate,
  discount_percent, sale_quantity, bonus_quantity, scheme_days, payload`).

Queried live against `postgres://postgres@127.0.0.1:5432/abuzar_next` with
the supplied RLS preamble, tenant `eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee`
("Legacy Reference Sandbox", 30,052 items, 291,361 posted cash/credit sale
documents):

```
master_categories: 10 rows total
  item_category: 1=MEDICINES, 2=NARCOTICS, 3=CONSUMER, 4=COUNSELING,
                 5=DIAGNOSTIC, 6=MILK, 7=DR KHALID MUGHAL   (7 real names)
  customer_category: 1=DEFAULT CATEGORY
  manufacturer_category: 1=DEFAULT
  supplier_category: 1=DEFAULT

master_manufacturers: 838 rows, all with real names, e.g.
  code=62 name="BRISTOL-MYERS", code=1 name="GEN",
  code=33 name="AKHAI PHARMACEUTICAL,KARACHI.", code=72 name="CCL PHARMACEUTICALS PVT LTD."
```

`master_items.payload` carries the raw legacy numeric codes as `ICatCode`
(item category) and `ManfCode` (manufacturer) — e.g. one sampled item has
`ManfCode=253, ICatCode=1`. **These join to the normalized tables for 100%
of items**, verified directly:

```sql
-- tenant eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee
total_items = 30052
items_with_manf_join (master_items i JOIN master_manufacturers m
  ON m.code = i.payload->>'ManfCode') = 30052   -- 100%
items_with_cat_join (master_items i JOIN master_categories c
  ON c.category_kind='item_category' AND c.code = i.payload->>'ICatCode') = 30052  -- 100%
```

**This directly refines (does not simply confirm) the "no name resolution"
framing in `docs/evidence/PHASE_N_REPORT_PROMOTION_2026-08-09.md` lines 167-168.**
That doc is correct that `master_items.payload` has 0/30,052 rows with a
literal `Category`/`Manufacturer` *name* key — but it did not check the
separate, already-populated normalized `master_categories`/
`master_manufacturers` tables (added later, in migration 010), which resolve
every item's code to a real name via a one-hop join that no
`category-wise-*`/`manufacturer-wise-*` report query currently performs.
**The name-resolution data is present and complete in the sandbox tenant
today; the gap is a missing join in the report SQL, not a missing-data
blocker.** What remains genuinely unresolved for these two families is
**only** the column-layout/grouping-shape evidence from §1/§2/§4 (no
capture, no doc) — i.e., *how* to group and what else to display alongside
the now-resolvable name, not *whether* the name can be resolved.

### User Wise — genuinely, confirmedly blocked (not a join gap)

```sql
SELECT count(operator_id), count(*) FROM business_documents
WHERE tenant_id='eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee'
  AND kind IN ('cash-sale','credit-sale') AND status='posted';
-- 0 | 291361
```

Checked whether *any* other tenant in the database has real per-user
attribution that the sandbox lacks: every tenant with `operator_id`
populated (408 rows across ~280 tenant codes) is a tiny synthetic
Go-test-fixture tenant (`credit-limit-*`, `finance-*`, `stock-*`,
`sale-return-*`, `read-model-*`, 1-5 documents each) — none has more than 5
posted documents, and none is a bulk migrated-legacy dataset. **No tenant in
this database has real-scale sales data with per-user attribution.**

Went one level deeper and checked the raw legacy import payload itself (not
just the typed `operator_id` column), in case the field exists unmapped:

```sql
SELECT legacy_payload->>'PostedBy', count(*) FROM business_documents
WHERE tenant_id='eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee'
  AND kind IN ('cash-sale','credit-sale') GROUP BY 1;
-- '' (empty/null) | 291361   -- every single row
```

`legacy_payload` does carry a `PostedBy` key (confirming the legacy schema
*has* a cashier/user field), but it is `null` for all 291,361 migrated
documents in the sandbox. **This is a genuine source-data gap** — either the
legacy SQL Server source never populated `PostedBy` for this dataset, or the
migration intentionally/accidentally dropped it before it could be typed
into `operator_id`. Either way, it is not fixable by a join or by looking at
a different tenant; the value simply isn't present anywhere in this
database. `docs/evidence/PHASE_N_REPORT_PROMOTION_2026-08-09.md` line 169's decision
to decline `user-wise-sales` on this basis is independently confirmed
correct and is now confirmed to extend to the raw import payload, not just
the typed column.

### Gross Profit / COGS — genuinely, confirmedly blocked (not tenant-specific)

`salesProfitMarginReadModelQuery` (`services/api/internal/httpapi/reports.go:2388-2419`,
underlying `Category Wise > Gross Profit`, `Customer Sales > ... > Customer
Wise Gross Profit`, and any future `Supplier/Manufacturer Wise G/P`) computes
`cost` via a `LEFT JOIN LATERAL` against `stock_allocations` keyed on
`document_line_id`. Checked population directly:

```sql
SELECT count(*) FROM stock_allocations WHERE tenant_id='eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee';
-- 0
SELECT count(*) FROM business_document_lines WHERE tenant_id='eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee';
-- 895015
```

`stock_allocations` is **completely empty** for the sandbox's 895,015 lines.
Cross-tenant check (same method as operator_id): the only tenants with any
`stock_allocations` rows at all are the same class of tiny synthetic
Go-test-fixture tenants (max 5 rows, e.g. `credit-limit-*`). **No tenant in
this database has meaningful FIFO/cost-allocation coverage over bulk sales
data.** This matches and confirms `docs/PARITY_STATUS.md`'s and
`docs/evidence/PHASE_N_GOLDEN_VERIFICATION_SUMMARY_2026-08-09.md`'s existing
"cost_count = 0" / stock-allocation-gap findings (lines 226-241, 421-438) —
independently re-derived here from the table directly rather than taken on
faith. This is an ops/migration backfill gap (stock-allocation records were
apparently never generated for the bulk-migrated historical sale lines), not
a query-code defect and not something a different tenant's data would route
around.

---

## Per-family verdict (answers the dispatcher's question directly)

| Family | Verdict |
|---|---|
| **Category Wise > Sales** (`category-wise-sales`) | Data blocker resolved (item_category name-join is 100% coverage, evidence §6) — column-shape/grouping still genuinely blocked, no capture/doc anywhere (§1–§4). |
| **Category Wise > Sale And Return** | Same as above: name-join solved, layout/grouping still blocked, no capture. |
| **Category Wise > Deviated Items** | Layout/grouping blocked (no capture) **and** semantically ambiguous — "deviated" threshold/definition not documented anywhere found. Name-join is not the limiting factor here. |
| **Category Wise > Monthly Sale** | Name-join solved; layout/grouping (monthly bucketing granularity, columns) still blocked, no capture. |
| **Category Wise > Net Sale** | Name-join solved; layout/grouping still blocked, no capture. |
| **Category Wise > Gross Profit** | Still genuinely blocked — independent of name-join, blocked on the confirmed-empty `stock_allocations` table (§6), a real ops/migration gap, not a sandbox quirk (no other tenant has this data either). |
| **Category Wise > Item Wise Sale Discounts Detail** | Layout/grouping blocked (title/submenu mismatch noted in `PHASE_N_REPORT_PROMOTION_2026-08-09.md:172`); name-join available if a category-grouped variant is chosen. |
| **Category Wise > Item Category Wise Monthly Sales** | Name-join solved; layout/grouping still blocked, no capture. |
| **Category Wise > Category Wise Day Net Sale** | Name-join solved; layout/grouping still blocked, no capture. |
| **Manufacturer Wise > Sales** (`manufacturer-wise-sales`) | Data blocker resolved (manufacturer name-join is 100% coverage, 838 real names, §6) — column-shape/grouping still genuinely blocked, no capture/doc. |
| **Manufacturer Wise > Sales Detail And Summary** | Name-join solved; layout/grouping (detail vs. summary split) still blocked, no capture. |
| **Manufacturer Wise > Net Sales** | Name-join solved; layout/grouping still blocked, no capture. |
| **Manufacturer Wise > Item Sales/Discount** | Name-join solved; layout/grouping still blocked, no capture. |
| **Manufacturer Wise > Manufacturer Wise Sales And Return Summary** | Name-join solved; layout/grouping still blocked, no capture. |
| **Manufacturer Wise > CNIC/NTN Registered Customers Sales** | Name-join not the limiting factor (this leaf filters by customer CNIC/NTN, not manufacturer/category); layout/grouping/filter-shape still blocked, no capture. |
| **User Wise > Sales** | Still genuinely blocked — `operator_id` 0/291,361 populated and raw `legacy_payload->>'PostedBy'` null for all 291,361 sale documents (§6); no tenant in the DB has real per-user attribution at scale. Not a join gap; a real source-data absence. |
| **User Wise > Invoice Graph** | Same operator-attribution blocker as above. |
| **User Wise > Category Summary** | Same operator-attribution blocker (also would need the item-category join, which is separately solved). |
| **User Wise > Discount Report** | Same operator-attribution blocker. |
| **User Wise > Sales Commission** | Same operator-attribution blocker, plus no commission-rate/scheme table identified in this pass — a second, independent gap even if attribution existed. |
| **User Wise > User Wise Sales Summary** | Same operator-attribution blocker. |
| **User Wise > Net Cash** | NOT blocked — already `projectionStatus = "real"` via `phaseQFinancialOverrides` (`financeMode`), per `PHASE_N_REPORT_PROMOTION_2026-08-09.md:175-180`. Out of scope for this research; flagged here only to avoid double-counting. |
| **Gross Profit / COGS (cross-cutting: Category Wise > Gross Profit, Customer Sales > ... > Customer Wise Gross Profit, prospective Supplier/Manufacturer Wise G/P)** | Still genuinely blocked, confirmed dataset-wide: `stock_allocations` is empty (0 rows) for the sandbox's 895,015 lines and for every other tenant with meaningful bulk sales data (§6). This is an ops/migration backfill gap, not fixable by picking a different tenant or by a code join. |

## What would actually unblock the next promotion wave

1. **Immediately actionable, no new capture needed:** any `category-wise-*`
   or `manufacturer-wise-*` leaf can now have its item grouped by a real
   category/manufacturer *name* (not just the opaque `ICatCode`/`ManfCode`)
   by joining `master_items.payload->>'ICatCode'` to
   `master_categories.code` (`category_kind='item_category'`), or
   `master_items.payload->>'ManfCode'` to `master_manufacturers.code` —
   both verified at 100% coverage against the sandbox tenant. What is still
   missing for these leaves is *only* the exact column set/sort/subtotal
   shape, which requires either (a) a legacy screenshot (not available in
   this repo; would need a future GUI-automation session against
   `D:\ABUZAR\LegacyReferenceSandbox`), or (b) an explicit, labeled
   best-effort guess accepted as such.
2. **Not actionable without new data:** User Wise (all leaves except the
   already-real Net Cash) and Gross Profit/COGS cannot be unblocked by
   research alone — they need either a migration/backfill pass that
   populates `operator_id`/`PostedBy` and `stock_allocations` from the
   legacy source (if the source data exists at all — unconfirmed, would
   need to check the legacy SQL Server `dbo.Cashsale`/`dbo.Saledetail`
   tables directly, which this dispatch could not reach), or an explicit
   accepted decision to ship these as `VACUOUS-NO-DATA` rather than as
   blocked/guessed.
