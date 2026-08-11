USE master;
GO
IF OBJECT_ID('dbo.usp_Bismil_PreWarmCache') IS NOT NULL
    DROP PROCEDURE dbo.usp_Bismil_PreWarmCache;
GO
CREATE PROCEDURE dbo.usp_Bismil_PreWarmCache
AS
BEGIN
    SET NOCOUNT ON;
    -- Wait for the app DB to come online after startup
    DECLARE @tries int = 0;
    WHILE DATABASEPROPERTYEX('FazalDinPP19DataBaseV2', 'Status') <> 'ONLINE' AND @tries < 60
    BEGIN
        WAITFOR DELAY '00:00:05';
        SET @tries = @tries + 1;
    END
    IF DATABASEPROPERTYEX('FazalDinPP19DataBaseV2', 'Status') <> 'ONLINE' RETURN;

    -- Touch the hot tables so the buffer pool is warm before the first user logs in.
    -- COUNT_BIG(*) with index hint 0 forces full scans into cache; DB is ~2.5 GB and fits in RAM.
    DECLARE @dummy bigint;
    SELECT @dummy = COUNT_BIG(*) FROM FazalDinPP19DataBaseV2.dbo.Item WITH (NOLOCK);
    SELECT @dummy = COUNT_BIG(*) FROM FazalDinPP19DataBaseV2.dbo.Saledetail WITH (NOLOCK);
    SELECT @dummy = COUNT_BIG(*) FROM FazalDinPP19DataBaseV2.dbo.SaleLedger WITH (NOLOCK);
    SELECT @dummy = COUNT_BIG(*) FROM FazalDinPP19DataBaseV2.dbo.VirtualGl WITH (NOLOCK);
    SELECT @dummy = COUNT_BIG(*) FROM FazalDinPP19DataBaseV2.dbo.StockReport WITH (NOLOCK);
    SELECT @dummy = COUNT_BIG(*) FROM FazalDinPP19DataBaseV2.dbo.Purdetail WITH (NOLOCK);
    SELECT @dummy = COUNT_BIG(*) FROM FazalDinPP19DataBaseV2.dbo.ItemLog WITH (NOLOCK);
    SELECT @dummy = COUNT_BIG(*) FROM FazalDinPP19DataBaseV2.dbo.PurOrderDetail WITH (NOLOCK);
    SELECT @dummy = COUNT_BIG(*) FROM FazalDinPP19DataBaseV2.dbo.SRLedger WITH (NOLOCK);
    SELECT @dummy = COUNT_BIG(*) FROM FazalDinPP19DataBaseV2.dbo.SRdetail WITH (NOLOCK);
    SELECT @dummy = COUNT_BIG(*) FROM FazalDinPP19DataBaseV2.dbo.Purledger WITH (NOLOCK);
    SELECT @dummy = COUNT_BIG(*) FROM FazalDinPP19DataBaseV2.dbo.DeletedSaleItem WITH (NOLOCK);
END
GO
EXEC sp_configure 'show advanced options', 1;
RECONFIGURE;
EXEC sp_configure 'scan for startup procs', 1;
RECONFIGURE;
GO
EXEC sp_procoption @ProcName = 'dbo.usp_Bismil_PreWarmCache', @OptionName = 'startup', @OptionValue = 'on';
GO
PRINT 'PREWARM PROC INSTALLED';
