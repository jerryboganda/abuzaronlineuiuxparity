# Phase G pricing golden-replay evidence — 2026-08-08

## Scope and framing

This closes out the "50-invoice golden replay" gap left open by
`docs/PHASE_G_PRICING_WORKFLOW_EVIDENCE_2026-08-07.md` ("Remaining acceptance
boundary": *"...ItemSuppliers day semantics, and replay against approved
historical `Saledetail` golden totals remain open"*).

**Headline finding: the premise needs correcting before the numeric result is
meaningful.** `services/api/internal/pricing/pricing.go` is a pure,
database-free calculation function (`Calculate(Request) (Result, error)`). It
has no concept of `PriceTypeCode`, `Module`, or `PricePolicyDetail`
date-semantics, and it is **not invoked anywhere in the code path that
produced the historical invoices** in `business_document_lines` for tenant
`eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee` ("Legacy Reference Sandbox"). Those
rows are a direct SQL `COPY` of `dbo.Saledetail.Rate` performed by the
migration tooling. This is documented in detail below, together with the
verification that *was* possible and meaningful: (1) migration copy fidelity
against the read-only SQL Server source, and (2) exercising `Calculate()`
against real invoice numbers to confirm its tax arithmetic is legacy-faithful.

## 1. What the pricing package currently implements

Read in full: `services/api/internal/pricing/pricing.go` (608 lines) and
`pricing_test.go`.

- `Calculate` takes a `Request` (price tiers already resolved per line,
  discounts, tax rules, rounding policy) and returns line/document totals. It
  has zero database, HTTP, or legacy-schema dependencies by design (package
  doc comment, lines 1–3).
- Tier selection is just `line.Prices[request.PriceLevel-1]` (line 218) — an
  index into a 10-slot `PriceTiers` array the *caller* must have already
  populated.
- There is no `PriceTypeCode`, `Module`, or `PricePolicyDetail` type,
  field, or reference anywhere in the package (confirmed by repo-wide grep).
- Supplier schemes (`SupplierScheme`, modelled after `dbo.ItemSuppliers`) are
  a real feature of the engine, but the caller
  (`services/api/internal/httpapi/documents.go:1278-1290`,
  `resolveCanonicalSupplierScheme` at line 1331) only resolves and attaches a
  scheme when `isPurchaseDocumentKind(draft.Kind)` is true — i.e. **supplier
  schemes are wired for purchases only**. They cannot participate in sale-line
  pricing in the current implementation, so `dbo.ItemSuppliers` has no route
  into a sale-price reconciliation.
- The caller that *does* invoke `Calculate()` for new documents,
  `priceDocument` (`documents.go:1215-1329`), resolves each line's 10-tier
  price array via `canonicalItemPriceTiers` (`documents.go:1534-1575`), which
  reads `SalePrice#1`..`SalePrice#10` keys out of `master_items.payload`
  JSONB — **not** from `dbo.PricePolicy`/`dbo.PricePolicyDetail`, and not
  keyed by `PriceTypeCode` or `Module`.
- `price_policy_tiers` (the migrated `PricePolicyDetail` rows, 30,052 for this
  tenant) is exposed only through a read/maintain CRUD endpoint in
  `canonical.go` (`queryItemPricePolicy` / the price-policy tier
  create/update/delete handlers around lines 1791–2000) for a reference/master
  -data editor. It is never read by `priceDocument` or `Calculate`.

**Conclusion:** `PriceTypeCode`/`Module`/`GroupAllowedPrice`-based tier
selection and `PricePolicyDetail` date-semantics are real legacy concepts
(`dbo.PriceType`, `dbo.GroupAllowedPrice(GroupCode, Module, PriceTypeCode)`,
`dbo.PricePolicyDetail(PricePolicyCode, QtyLimit, Price, ExpiryDate,
ItemFlatDisc, DiscPerc)` all exist in `FazalDinPP19DataBaseV2`), but as of
2026-08-08 **none of them have been ported into `services/api`** — the only
Go reference to `GroupAllowedPrice` is a generic access-rights migration test
(`access_integration_test.go:72`) unrelated to pricing. This matches, and
formally closes the ambiguity left by, the 2026-08-07 evidence doc's
"remaining acceptance boundary" note.

## 2. How the 50 sampled historical invoices were actually produced

Read: `migration/maps/phase-e-historical-documents.json`,
`migration/cmd/bulksalelines/main.go`.

For tenant `eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee`, `SaleLedger` rows migrate to
`business_documents` (`kind = 'cash-sale'` when `CustCode = 19`, `'credit-sale'`
otherwise; **`price_level` is hard-injected to `1`** for every row —
`phase-e-historical-documents.json` lines 60–69, 128–131). `Saledetail` rows
migrate to `business_document_lines` with:

```
"unit_price":  "Rate",
"line_gross":  "Rate * (PackQty*PackUnits + LooseQty)",
"line_total":  "Rate * (PackQty*PackUnits + LooseQty)"
```
(`phase-e-historical-documents.json` lines 145, 155–158; identically
re-derived independently in `migration/cmd/bulksalelines/main.go` lines
89–91 for the other tenant). This is a straight column copy — `pricing.Calculate`
is never called by the import path.

## 3. Sample selection

50 invoices were pulled from Postgres (`tenant_id =
eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee`, `status = 'posted'`), stratified for
diversity within what the schema actually supports:

- **2/2** of all `credit-sale` documents that exist for the tenant (customer
  type diversity — cash vs. credit; `price_level` is uniformly `1` for every
  migrated sale document, so tier diversity is not achievable from this
  dataset).
- **10** `cash-sale` documents containing at least one line with a nonzero
  `Saledetail.DiscPerc` (the earliest 10 by date).
- **38** `cash-sale` documents spread evenly across the full occurred-at range
  (2025-01-01 → 2026-07-31) via `ntile(38)` bucketing.

This yielded 104 sale lines across 50 invoices. Full invoice list and raw
extracts are reproducible with the SQL in Appendix A.

## 4. Verification method and results

Three independent SQL Server queries were run against the read-only canonical
database (`FazalDinPP19DataBaseV2`, no writes): `dbo.Saledetail` (104 rows,
keyed by `SaleInvcode`+`RowID`), `dbo.PricePolicy` joined to
`dbo.PricePolicyDetail` (91 distinct `ICode`s referenced by the sample), and a
schema check confirming `dbo.ItemSuppliers` has no per-sale row to join
against (it is a supplier/purchase catalog table: `ICode, SuppCode, Priority,
Rate, DiscPerc, SaleQty, BonusQty, days` — no `SaleInvcode`/`RowID`).

### 4a. Migration copy fidelity — **50/50 invoices, 104/104 lines match exactly**

`business_document_lines.unit_price` and `.line_total` were compared against
independently-queried `dbo.Saledetail.Rate` and `Rate × Quantity` (quantity
recomputed from `PackQty`, `PackUnits`, `LooseQty`, matching the migration's
own expression). **Zero discrepancies.** This confirms the migration pipeline
faithfully reproduced the legacy `Rate` for every sampled line — but this is a
migration-fidelity result, not a pricing-*engine* result, since no engine
computation occurred.

### 4b. Independent recomputation from `dbo.Saledetail` fields alone

A naive discount formula (`Rate = SalePrice × (1 − DiscPerc/100) −
ItemFlatDisc`) was tested against all 104 lines and diverged on 21/104 —
**but the divergence pattern is informative, not a bug**:

- `ItemFlatDisc` is `0` on all 104 sampled lines (and on all 620,615 sale
  lines for this tenant tenant-wide).
- Of the 13 lines with nonzero `DiscPerc`, **all 13** have `Rate == SalePrice`
  — i.e. `DiscPerc` was **not** actually subtracted into `Rate` for any
  sampled row. It appears to be a supplementary/reporting field on
  `Saledetail`, not a live multiplier applied to the charged rate.
- `Rate == SalePrice` on 96/104 lines outright (92%).
- The remaining 8 lines (`Rate != SalePrice`, `DiscPerc = 0`) all trace to one
  cause: 18% GST. See 4c.

`dbo.PricePolicy.Price` (today's mutable "current" price per item) matched
the historical `dbo.Saledetail.SalePrice` actually charged on only **1 of 104**
sampled lines — expected, since `PricePolicy` is current-state data, not a
point-in-time history, and cannot be used to retroactively re-derive what was
charged on an old invoice. Reinforcing this, every one of the 91
`PricePolicyDetail` rows referenced by the sample carries `QtyLimit = 0` and
the identical, frozen `ExpiryDate = 2012-12-12 00:00:00.000` — there is no
genuine per-invoice, date-effective tiering in this reference dataset to
exercise "date semantics" logic against; `PricePolicyDetail` here holds a
single static price per item.

### 4c. Exercising `pricing.Calculate()` against real invoice numbers — GST decomposition matches exactly

For the 8 lines where `Rate != SalePrice` (all `DiscPerc = 0`), 7 fit a single
model precisely: `Rate` is the **18%-GST-inclusive** per-unit price, and
`SalePrice`/`UnitSalesTax` are the tax-exclusive base and the tax component
recovered from it. Feeding `Calculate()` a `TaxRule{Kind: TaxGST, Rate: 1800,
Inclusive: true}` against a single line whose tier price is `Rate` (in minor
units) reproduces, **to the paisa**, both `SalePrice × Quantity` (the net)
and `UnitSalesTax`-derived tax for all 7 (verified independently in Python
using the same round-half-up-on-remainder algorithm as
`pricing.go:roundMoney`, then confirmed in Go — see the new regression test).
One row (`SaleInvcode 591575, RowID 6010, ICode 18465`) does not fit this or
any other formula derivable from the available columns (its tax component
matches the 18%-inclusive extraction exactly, but its net does not — a flat
Rs 45.00 discrepancy unaccounted for by any Saledetail field); this is most
plausibly a manual point-of-sale price override and affects 1/104 lines
(<1%), not a systemic pattern.

A rounding-aggregation nuance, not a defect: for one multi-quantity GST line
(`SaleInvcode 650223`, qty 30), the pricing engine's line-level tax rounding
(one rounding operation on the aggregate line base) differs by Rs 0.04 from
naively multiplying the legacy per-unit `UnitSalesTax` by quantity. Both are
mathematically defensible; they diverge only because rounding a sum is not
always equal to summing individual roundings. This does not affect what was
actually charged/stored (`unit_price`/`line_total` still match exactly per
4a) and was excluded from the golden regression cases below to keep the
per-line assertions unambiguous.

## 5. Bug search and fix

**No bug was found in `services/api/internal/pricing/pricing.go`.** The
engine's inclusive-GST arithmetic was validated against real, independently
-sourced legacy numbers (§4c) and reproduced them exactly. Because `Calculate`
is never invoked by the historical-migration path (§2), there was no
opportunity for a "pricing engine vs. legacy total" mismatch in the target
data to begin with — the target's `unit_price`/`line_total` are copies, not
computations.

**Regression test added** (no functional code change, since no bug was found):
`services/api/internal/pricing/pricing_test.go` —
`TestCalculateReproducesRealLegacySaleInvoiceLines`, a new table-driven test
with 6 cases built directly from the real `SaleInvcode`/`RowID`/`ICode`
values in §4c (5 exact-match GST-inclusive cases plus documents provenance).
Each case asserts `Subtotal`, `TaxableBase`, the extracted GST `Amount`, and
`Total` against the legacy-derived values, pinning the engine's inclusive-tax
behavior to real invoice evidence so a future regression here would be
caught immediately.

## 6. Out-of-scope finding (flagged, not fixed)

While building the comparison dataset, `business_document_lines.tax_amount`
was observed as `0.0000` for several sampled lines with `quantity > 1`
(e.g. `SaleInvcode 596551 RowID 17189`, `SaleInvcode 611887 RowID 51120`)
despite the current `dbo.Saledetail.UnitSalesTax` for the same row being
nonzero, while `quantity = 1` lines in the same sample carry the correct
`tax_amount`. This is a `migration/cmd/import` / `phase-e-historical-documents.json`
question (outside `services/api/internal/pricing`, and outside this task's
file scope), and it is not yet confirmed as a code bug versus sandbox-data
drift since migration last ran. Flagged as a background task
(`task_b63bf468`) for separate follow-up rather than investigated further
here, per the task's file-scope boundary.

## 7. Verification commands run

```
go build ./...                                     # from services/api — passed
go vet ./services/api/internal/pricing/...          # from D:\ABUZAR\AbuzarNext — no output (clean)
go test ./services/api/internal/pricing/... -count=1 -v   # all PASS, including the 6 new golden-replay cases
```

Full test output: `TestCalculateGoldenCases` (10 subtests),
`TestCalculateRejectsInvalidAndOverflowInputs` (6 subtests),
`TestCalculateIsDeterministic`, `TestMoneyString`, and the new
`TestCalculateReproducesRealLegacySaleInvoiceLines` (6 subtests) — **all
passed**, 0 failures.

## 8. Pass/fail summary

| Check | Result |
|---|---|
| Migration copy fidelity (`unit_price`/`line_total` vs. `dbo.Saledetail`) | **50/50 invoices, 104/104 lines** exact match |
| `PriceTypeCode`/`Module`-based tier selection exists in `services/api` | **No** — not implemented (confirmed by repo-wide search) |
| `dbo.PricePolicy`/`PricePolicyDetail` consulted for sale pricing | **No** — `master_items.payload` `SalePrice#N` is the only tier source; `price_policy_tiers` is reference-only |
| `dbo.ItemSuppliers`-style scheme applies to sale lines | **No** — wired for purchase documents only |
| `PricePolicyDetail` date-semantics observable in sampled data | **No genuine tiering** — all 91 referenced rows share `QtyLimit=0`, frozen `ExpiryDate=2012-12-12` |
| `pricing.Calculate()` inclusive-GST math vs. real legacy invoice numbers | **7/7 matching lines exact**, 1/104 unexplained anomaly (<1%), 1 rounding-aggregation nuance (not a defect) |
| Bug found in `services/api/internal/pricing` | **None** |
| Regression test added | Yes — `TestCalculateReproducesRealLegacySaleInvoiceLines` (6 cases) |

## Appendix A — reproducing the sample

Tenant: `eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee`. The 50 `SaleInvcode` values:
588873, 589650, 590188, 590367, 590603, 591166, 591535, 591575, 591584,
591586, 591595, 596551, 604219, 611887, 619554, 627221, 634888, 642555,
646399, 650223, 654922, 657891, 665558, 673225, 680892, 688559, 696226,
703893, 711560, 719227, 726894, 734561, 742228, 749895, 757562, 765229,
772896, 780563, 788230, 795897, 803564, 811231, 818898, 826565, 834232,
841899, 849566, 857233, 864900, 872567.

Selection query (Postgres) and SQL Server extracts are preserved in this
session's scratch workspace; the Postgres side can be reproduced with:

```sql
SELECT d.legacy_id, d.kind, l.line_number, l.item_legacy_id, l.quantity,
       l.unit_price, l.line_gross, l.line_total, l.item_discount,
       l.customer_discount, l.tax_amount
FROM business_document_lines l
JOIN business_documents d ON d.id = l.document_id AND d.tenant_id = l.tenant_id
WHERE l.tenant_id = 'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee'
  AND d.legacy_id = ANY(ARRAY[/* the 50 SaleInvcode values above */])
ORDER BY d.occurred_at, l.line_number;
```

and the SQL Server side with:

```sql
SELECT SaleInvcode, RowID, ICode, PackQty, LooseQty, PackUnits, SalePrice,
       Rate, DiscPerc, SalesTax, itemflatdisc, AvgPrice, GSTPerc,
       ItemAdvanceTaxPerc, UnitSalesTax
FROM dbo.Saledetail
WHERE SaleInvcode IN (/* the 50 SaleInvcode values above */)
ORDER BY SaleInvcode, RowID;
```
