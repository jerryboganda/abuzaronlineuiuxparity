-- ROLLBACK for server-level rebrand 2026-08-12T02:53:52
EXEC sp_procoption 'usp_Bismillah_PreWarmCache','startup','off';
CREATE PROCEDURE dbo.usp_Abuzar_PreWarmCache
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
EXEC sp_procoption 'usp_Abuzar_PreWarmCache','startup','on';
DROP PROCEDURE dbo.usp_Bismillah_PreWarmCache;
GO
CREATE EVENT SESSION [abuzar_errors] ON SERVER
ADD EVENT sqlserver.error_reported(
  ACTION(sqlserver.client_app_name, sqlserver.database_name, sqlserver.sql_text)
  WHERE ([severity]>=(11)))
ADD TARGET package0.ring_buffer(SET max_memory=(4096))
WITH (MAX_MEMORY=4096 KB, EVENT_RETENTION_MODE=ALLOW_SINGLE_EVENT_LOSS,
      MAX_DISPATCH_LATENCY=30 SECONDS, STARTUP_STATE=ON);
ALTER EVENT SESSION [abuzar_errors] ON SERVER STATE = START;
DROP EVENT SESSION [bismillah_errors] ON SERVER;
