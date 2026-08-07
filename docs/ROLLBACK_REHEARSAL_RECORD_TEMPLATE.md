# Rollback rehearsal record

Use this record for a disposable/sandbox rehearsal before cutover. It is not a
production rollback authorization and must not be completed by writing to
`FazalDinPP19DataBaseV2` or by posting through the legacy application.

**Required result:** `PASS` before production go/no-go.  
**Allowed outcomes:** `PASS`, `FAIL`, `BLOCKED`.  
**Legacy write rule:** `false` is mandatory.

## 1. Rehearsal metadata

| Field | Value |
|---|---|
| Record ID | `<change-id>-rollback-<attempt>` |
| Environment | `sandbox` / `disposable` / `pilot` |
| Rehearsal date | `<YYYY-MM-DD>` |
| Start UTC | `<timestamp>` |
| End UTC | `<timestamp>` |
| Release artifact and SHA-256 | `<artifact>` / `<hash>` |
| Tenant / branch / counters | `<approved UUIDs>` |
| Incident lead | `<role/name>` |
| Release manager | `<role/name>` |
| DBA | `<role/name>` |
| Branch operator | `<role/name>` |
| Legacy write performed | `false` |
| Outcome | `PASS` / `FAIL` / `BLOCKED` |

## 2. Preconditions

- [ ] Target backup completed before the rehearsal.
- [ ] Backup SHA-256 recorded: `<hash>`.
- [ ] Restore test target identified: `<database/host>`.
- [ ] API and edge health baseline captured.
- [ ] `/v1/metrics` baseline captured without secrets or customer payloads.
- [ ] Edge `pendingEvents` before switch: `<count>`.
- [ ] Terminal inventory attached: `<secure-evidence-path>`.
- [ ] Legacy executable and database were not modified.
- [ ] Test data and expected business totals recorded: `<secure-evidence-path>`.

## 3. Scenario

Select at least one. Record the exact injected failure and why it is
representative.

- [ ] **Pre-first-write switch failure:** terminals are repointed, pilot health
  fails, and terminals are returned to the preserved legacy read-only client.
- [ ] **Post-write containment:** a disposable AbuzarNext transaction is
  created, posting is stopped, events are preserved/exported, and terminals are
  held rather than silently resuming legacy writes.
- [ ] **Target recovery:** API/edge writers stop, the approved PostgreSQL backup
  is restored to the disposable target, migrations/health/reconciliation run,
  and the client is repointed only after approval.
- [ ] Other approved scenario: `<description>`.

## 4. Step record

Record UTC timestamps, operator, expected result, observed result, and evidence
for every step. A checked box without evidence is not a pass.

| # | UTC | Operator | Action | Expected | Observed | Evidence | Result |
|---:|---|---|---|---|---|---|---|
| 1 | `<time>` | `<role>` | Announce rollback and stop new AbuzarNext posts | No new posts accepted | `<observation>` | `<log/request IDs>` | `pass/fail` |
| 2 | `<time>` | `<role>` | Preserve API/edge logs, metrics, queue, screenshots, and request IDs | Evidence is immutable and redacted | `<observation>` | `<path>` | `pass/fail` |
| 3 | `<time>` | `<role>` | Record edge queue and outstanding events | Queue disposition is known | `<count/status>` | `<path>` | `pass/fail` |
| 4 | `<time>` | `<role>` | Repoint pilot terminal to the intact legacy executable | Client starts from the approved working directory | `<observation>` | `<screenshot/log>` | `pass/fail` |
| 5 | `<time>` | `<role>` | Confirm legacy database remains read-only | No legacy DML/DDL or posted transaction | `<observation>` | `<DBA evidence>` | `pass/fail` |
| 6 | `<time>` | `<role>` | Preserve/export new AbuzarNext events and audit records | No new event is lost or deleted | `<observation>` | `<secure path/hash>` | `pass/fail` |
| 7 | `<time>` | `<role>` | Restore disposable target, if scenario requires | Restore and hash verification succeed | `<observation>` | `<backup/restore report>` | `pass/fail` |
| 8 | `<time>` | `<role>` | Re-run health, RLS, reconciliation, and pilot checks | All approved checks pass before repoint | `<observation>` | `<checklist/report>` | `pass/fail` |
| 9 | `<time>` | `<role>` | Release rollback hold or continue containment | Decision is signed | `<decision>` | `<change record>` | `pass/fail` |

## 5. Data safety and reconciliation

| Check | Result / evidence |
|---|---|
| New event count before failure | `<count>` |
| New event count preserved after rollback | `<count>` |
| Unsent edge events before failure | `<count>` |
| Unsent edge events preserved | `<count>` |
| Target backup restored | `yes/no/not applicable` |
| Source/target business totals compared | `yes/no/not applicable` |
| Invoice ranges checked | `yes/no/not applicable` |
| Stock/GL/party-ledger impact documented | `<path>` |
| Data loss or duplicate rows | `none` / `<incident ID>` |
| Legacy write performed | `false` |

If any new transaction was posted before rollback, do not mark this record
`PASS` until the event preservation and controlled recovery decision are
documented. Mechanical repointing to the legacy executable does not make those
transactions available in SQL Server.

## 6. Defects and follow-up

| ID | Severity | Description | Owner | Containment | Due | Evidence |
|---|---|---|---|---|---|---|
| `<id>` | `S0/S1/S2/S3` | `<description>` | `<role>` | `<action>` | `<UTC>` | `<path>` |

## 7. Approvals

| Role | Name | Decision | UTC timestamp | Signature/evidence |
|---|---|---|---|---|
| Release manager | `<name>` | `PASS/FAIL/BLOCKED` | `<timestamp>` | `<reference>` |
| DBA | `<name>` | `PASS/FAIL/BLOCKED` | `<timestamp>` | `<reference>` |
| Business approver | `<name>` | `PASS/FAIL/BLOCKED` | `<timestamp>` | `<reference>` |
| Branch operator | `<name>` | `PASS/FAIL/BLOCKED` | `<timestamp>` | `<reference>` |

The release manager may copy the result into
[`CUTOVER_GO_NO_GO_TEMPLATE.json`](CUTOVER_GO_NO_GO_TEMPLATE.json) only when
the evidence paths exist and all required approvals are present.
