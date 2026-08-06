# Phase R/E security-data wave — evidence

Date: 2026-08-06. This artifact covers only the reviewed security-data wave
against the `AbuzarLegacyReference` source and the
`Legacy Reference Sandbox` target tenant
(`eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee`). The canonical
`FazalDinPP19DataBaseV2` source was not used for import or reconciliation.

## Evidence

- Read-only source schema inspection: `parity/catalog/phase-r-security-source-schema.json`
  (10,890 column records).
- GroupCashAccount inspection: `parity/catalog/phase-r-group-cash-account-inspection.json`.
- Import report: `parity/catalog/phase-r-security-import.json`.
- Reconciliation and reviewed metrics: `parity/catalog/phase-r-security-reconciliation.json`.
- Reviewed source-bound map: `migration/maps/phase-r-security-data.json`.
- Reviewed read-only metric queries: `migration/maps/phase-r-security-metrics.json`.

The final import report read/imported these rows with zero current row
exceptions:

| Source table | Target | Read | Imported |
|---|---|---:|---:|
| Groups | roles | 4 | 4 |
| Groups | legacy_groups | 4 | 4 |
| Users | users | 9 | 9 |
| UserGroups | user_memberships | 9 | 9 |
| GroupRights | group_rights | 726 | 726 |
| GroupAllowedGodown | group_allowed_scopes | 33 | 33 |
| GroupAllowedGroups | group_allowed_scopes | 0 | 0 |
| GroupAllowedHeader | group_allowed_scopes | 35 | 35 |
| GroupAllowedPrice | group_allowed_scopes | 54 | 54 |
| GroupAllowedRecipient | group_allowed_scopes | 8 | 8 |
| GroupAllowedServiceCategory | group_allowed_scopes | 0 | 0 |
| GroupAllowedStartupRight | group_allowed_scopes | 0 | 0 |
| GroupCashAccount | group_allowed_scopes (`cash_account`) | 43 | 43 |

The first sandbox attempts exposed 44 target-side bookkeeping exceptions
(partial legacy-key uniqueness, then text lookup encoding). They were
superseded by the corrected 019 schema/tooling and marked `resolved`; the
final import report has zero row exceptions and the target has no open
security exceptions.

The target contains 173 allow-scope rows, including 43 `cash_account` rows.
`GroupRights` had 726 source rows
with status `1`; all 726 target rows retain `allowed = true` and the original
status in `legacy_status`. If a future source snapshot contains an explicit
deny, the same mapping preserves it as `allowed = false`; no deny row was
invented.

Composite legacy identifiers are retained in `legacy_scope_id` and
`legacy_payload`; role and user UUID joins are resolved from numeric legacy
IDs. Legacy source password values were not selected or imported. Imported
users carry a reset-required password marker.

## Remaining unmapped security data

The read-only source count query recorded:

| Source table | Rows | Decision |
|---|---:|---|
| UserRights | 0 | Unmapped; empty table |
| GroupCashAccount | 0 | Mapped as `cash_account`; composite IDs and payload retained |
| GroupPurExpTemplate | 0 | Unmapped; empty table |

All seven unambiguous `GroupAllowed*` tables and the reviewed
`GroupCashAccount` table are mapped, including the three empty tables. Menu
enforcement and rights-editor behavior remain outside this migration-only
wave.
