# Phase E beginning — evidence status (2026-08-06)

## Scope

This closes only the safest executable beginning of Phase E: canonical source
inspection, reviewed enterprise/config and core-master maps, isolated sandbox
import, and read-only reconciliation. It does **not** claim that the legacy
database is fully migrated.

The inspector ran against `FazalDinPP19DataBaseV2` using metadata queries only.
The refreshed manifest contains **763 base tables and 10,890 column records**:
`parity/catalog/sqlserver-schema.json`. No source DML or DDL was issued.

The import source was `AbuzarLegacyReference`, not the canonical database. The
target was the newly provisioned `Legacy Reference Sandbox` tenant
(`eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee`) and its `SANDBOX` branch. Existing
tenants and application data were not selected by the maps.

## Evidence

| Artifact | Result |
|---|---|
| `parity/catalog/phase-e-enterprise-import.json` | 11 mapped tables; 22 source rows read/imported; 0 current row exceptions |
| `parity/catalog/phase-e-core-import.json` | 7 mapped tables; 83,425 source rows read/imported; 0 current row exceptions |
| `parity/catalog/phase-e-enterprise-reconciliation.json` | 11/11 table counts matched; 20/20 reviewed metrics matched |
| `parity/catalog/phase-e-core-reconciliation.json` | 7/7 table counts matched; 20/20 reviewed metrics matched |
| `migration/maps/phase-e-*.json` | 18 reviewed source-to-target mappings and 20 read-only metrics |

Core sandbox counts were: Item 30,052; Customer 2; Supplier 235;
Manufacturer 838; Godown 1; PricePolicy 30,052; ItemSuppliers 22,245.
The Item count is the sandbox observation and is not substituted with the
earlier canonical-database estimate.

The importer was also run a second time for the enterprise map. A deliberate
canonical-source invocation was refused before connecting. Thirteen
superseded exceptions from the first pre-fix sandbox attempt are retained as
resolved bookkeeping records; the final import reports and reconciliations
contain no current exceptions.

## Remaining unmapped work

Only 18 of the 763 inspected base tables are in these maps (745 remain
unmapped). In particular, no document tables (`Sale*`, `Pur*`, orders/returns),
ledgers (`SaleLedger`, `Purledger`, `VirtualGl`), stock (`StockReport`), users/
groups/rights, batches, report/configuration families beyond the reviewed
subset, or other legacy tables are claimed migrated. The selected Item payload
is a reviewed subset of the 135 source columns; the manifest remains the
authoritative inventory for the rest.

Next safe step is a separately reviewed document/ledger/stock mapping wave,
with business metrics and source-specific field transformations approved
before any import.
