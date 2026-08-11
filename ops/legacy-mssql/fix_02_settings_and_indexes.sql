/* =====================================================================
   Bismil / BISMILA  --  DB fix pack 02
   Fix 2: PAGE_VERIFY was TORN_PAGE_DETECTION (deprecated, weak corruption
          detection). Switch to CHECKSUM (SQL Server default since 2005).
   Fix 3: Indexes recommended by sys.dm_db_missing_index_* for the actual
          report workload captured from the running application.
   ===================================================================== */
SET NOCOUNT ON;
SET QUOTED_IDENTIFIER ON;
GO

/* ---- Fix 2: page checksums ---- */
IF EXISTS (SELECT 1 FROM sys.databases
           WHERE name = 'FazalDinPP19DataBaseV2' AND page_verify_option_desc <> 'CHECKSUM')
BEGIN
    ALTER DATABASE FazalDinPP19DataBaseV2 SET PAGE_VERIFY CHECKSUM;
    PRINT 'PAGE_VERIFY set to CHECKSUM';
END
ELSE
    PRINT 'PAGE_VERIFY already CHECKSUM';
GO

/* ---- Fix 3a: SaleLedger covering index for date-range sales reports ----
   Query shape (captured live from the app):
       WHERE saleledger.date BETWEEN @from AND @to
         AND saleledger.salecatcode IN (...)
   Without this the engine scans SaleLedger (192 MB) and then key-looks-up
   every qualifying invoice.                                              */
IF NOT EXISTS (SELECT 1 FROM sys.indexes
               WHERE object_id = OBJECT_ID('dbo.SaleLedger')
                 AND name = 'IX_SaleLedger_SaleCat_Date_Cover')
BEGIN
    CREATE NONCLUSTERED INDEX IX_SaleLedger_SaleCat_Date_Cover
        ON dbo.SaleLedger (SaleCatCode, [date])
        INCLUDE (CustCode, SalesmanCode, DiscPerc, FlatDisc, MiscCharges,
                 SalesTax, Posted, Remarks, CustRefNo, InvGSTPerc1,
                 MachineName, SaleTypeCode, ImpactInventory)
        WITH (FILLFACTOR = 90, SORT_IN_TEMPDB = ON, ONLINE = OFF);
    PRINT 'Created IX_SaleLedger_SaleCat_Date_Cover';
END
ELSE
    PRINT 'IX_SaleLedger_SaleCat_Date_Cover already exists';
GO

/* ---- Fix 3b: Item active-list index (92% estimated gain, tiny index) ---- */
IF NOT EXISTS (SELECT 1 FROM sys.indexes
               WHERE object_id = OBJECT_ID('dbo.Item')
                 AND name = 'IX_Item_Active_ICatCode')
BEGIN
    CREATE NONCLUSTERED INDEX IX_Item_Active_ICatCode
        ON dbo.Item (Active)
        INCLUDE (ICatCode)
        WITH (FILLFACTOR = 90, SORT_IN_TEMPDB = ON);
    PRINT 'Created IX_Item_Active_ICatCode';
END
ELSE
    PRINT 'IX_Item_Active_ICatCode already exists';
GO

/* ---- Fix 3c: PurOrderHeader supplier/posted lookup ---- */
IF NOT EXISTS (SELECT 1 FROM sys.indexes
               WHERE object_id = OBJECT_ID('dbo.PurOrderHeader')
                 AND name = 'IX_PurOrderHeader_Supp_Posted')
BEGIN
    CREATE NONCLUSTERED INDEX IX_PurOrderHeader_Supp_Posted
        ON dbo.PurOrderHeader (SuppCode, Posted)
        INCLUDE ([Date], SupplierReference, ListOfPurInvoices, TotalOfPurchases)
        WITH (FILLFACTOR = 90, SORT_IN_TEMPDB = ON);
    PRINT 'Created IX_PurOrderHeader_Supp_Posted';
END
ELSE
    PRINT 'IX_PurOrderHeader_Supp_Posted already exists';
GO

UPDATE STATISTICS dbo.SaleLedger WITH FULLSCAN;
UPDATE STATISTICS dbo.Item WITH FULLSCAN;
UPDATE STATISTICS dbo.PurOrderHeader WITH FULLSCAN;
PRINT 'Statistics refreshed';
GO
