/* Attempt to re-validate every untrusted / disabled FK.
   Trusted FKs let the optimizer eliminate redundant joins and simplify plans.
   Any FK that FAILS indicates real orphaned data -> reported, never auto-deleted. */
SET NOCOUNT ON;

IF OBJECT_ID('tempdb..#res') IS NOT NULL DROP TABLE #res;
CREATE TABLE #res (fk sysname, tbl sysname, status varchar(20), err nvarchar(2000));

DECLARE @fk sysname, @tbl sysname, @sql nvarchar(1000);
DECLARE c CURSOR LOCAL FAST_FORWARD FOR
    SELECT fk.name, OBJECT_NAME(fk.parent_object_id)
    FROM sys.foreign_keys fk
    WHERE fk.is_not_trusted = 1 OR fk.is_disabled = 1;

OPEN c;
FETCH NEXT FROM c INTO @fk, @tbl;
WHILE @@FETCH_STATUS = 0
BEGIN
    BEGIN TRY
        SET @sql = N'ALTER TABLE dbo.' + QUOTENAME(@tbl) +
                   N' WITH CHECK CHECK CONSTRAINT ' + QUOTENAME(@fk) + N';';
        EXEC sp_executesql @sql;
        INSERT #res VALUES (@fk, @tbl, 'TRUSTED', NULL);
    END TRY
    BEGIN CATCH
        INSERT #res VALUES (@fk, @tbl, 'ORPHANS', ERROR_MESSAGE());
    END CATCH
    FETCH NEXT FROM c INTO @fk, @tbl;
END
CLOSE c; DEALLOCATE c;

PRINT '===== SUMMARY =====';
SELECT status, COUNT(*) AS cnt FROM #res GROUP BY status;

PRINT '===== FKs THAT FAILED (orphaned rows exist) =====';
SELECT tbl, fk, LEFT(err, 300) AS err FROM #res WHERE status = 'ORPHANS' ORDER BY tbl, fk;

PRINT '===== REMAINING UNTRUSTED AFTER PASS =====';
SELECT COUNT(*) AS still_untrusted FROM sys.foreign_keys WHERE is_not_trusted = 1 OR is_disabled = 1;
