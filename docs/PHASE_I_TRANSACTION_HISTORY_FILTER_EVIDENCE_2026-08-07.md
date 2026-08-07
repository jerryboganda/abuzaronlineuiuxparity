# Transaction-history filter follow-up - 2026-08-07

This is a bounded operator-workflow increment. It does not claim exact
PowerBuilder filter matching or complete list-screen parity.

## Implemented

- Sales List exposes a legacy-style `Filter:` input and `Filter / Retrieve`
  action.
- Purchase List exposes the same control for the active purchase kind.
- Enter and button activation use the existing authenticated
  `/v1/transactions/{kind}` contract and forward the trimmed filter value;
  no duplicate client-side filtering or unscoped query was introduced.
- The existing server query scopes the filter to document, party, and item
  text within the authenticated tenant and branch.

## Verification evidence

| Check | Result |
|---|---|
| `cmd /c pnpm --filter @abuzar/web check` | Passed: `svelte-check` reported 0 errors and 0 warnings. |
| `git diff --check` on the three touched web files | Passed; Git reported only the expected Windows line-ending warnings. |

## Remaining boundary

The exact PowerBuilder wildcard/operator behavior, focus order, list sorting,
and screenshot comparison remain open. The short check did not claim a live
authenticated API result or operator UAT.
