# Phase Q — Party statement source-scope evidence (2026-08-07)

The customer and supplier statement report definitions now disclose the same
return-allocation sources already used by the finance read model:

- customer statements: `SRAllocationDetail` return allocations;
- supplier statements: `PRAllocationDetail` return allocations.

This corrects the explicit retrieval-scope contract in the report metadata; it
does not change the source-backed SQL union or infer settlement semantics.

Focused verification passed:

```text
go test ./services/api/internal/httpapi -run 'Test(PhaseQQueriesArePostedAndScopeBound|PartyStatementDefinitionsDiscloseReturnAllocations)' -count=1
```

Exact legacy statement grouping, return/payment settlement semantics, print
output, live reconciliation, and operator acceptance remain open.
