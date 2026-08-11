-- Bismil weekly maintenance: smart index rebuild/reorg + stats update
-- Safe to run any time; brief locks during rebuilds. Run when app is idle.
SET NOCOUNT ON;

DECLARE @sql nvarchar(max) = N'';

-- Rebuild indexes >30% fragmented, reorganize 5-30%
SELECT @sql = @sql +
    CASE WHEN ps.avg_fragmentation_in_percent > 30
         THEN N'ALTER INDEX ' + QUOTENAME(i.name) + N' ON ' + QUOTENAME(s.name) + N'.' + QUOTENAME(t.name) + N' REBUILD WITH (FILLFACTOR = 90);' + CHAR(10)
         ELSE N'ALTER INDEX ' + QUOTENAME(i.name) + N' ON ' + QUOTENAME(s.name) + N'.' + QUOTENAME(t.name) + N' REORGANIZE;' + CHAR(10)
    END
FROM sys.dm_db_index_physical_stats(DB_ID(), NULL, NULL, NULL, 'LIMITED') ps
JOIN sys.indexes i ON ps.object_id = i.object_id AND ps.index_id = i.index_id
JOIN sys.tables t ON t.object_id = ps.object_id
JOIN sys.schemas s ON s.schema_id = t.schema_id
WHERE ps.avg_fragmentation_in_percent > 5
  AND ps.page_count > 100
  AND i.index_id > 0
  AND t.is_ms_shipped = 0;

EXEC sp_executesql @sql;

-- Refresh statistics on tables with meaningful churn
DECLARE @stats nvarchar(max) = N'';
SELECT @stats = @stats + N'UPDATE STATISTICS ' + QUOTENAME(s.name) + N'.' + QUOTENAME(t.name) + N' WITH FULLSCAN;' + CHAR(10)
FROM sys.tables t
JOIN sys.schemas s ON s.schema_id = t.schema_id
WHERE t.is_ms_shipped = 0
  AND EXISTS (
      SELECT 1 FROM sys.stats st
      CROSS APPLY sys.dm_db_stats_properties(st.object_id, st.stats_id) sp
      WHERE st.object_id = t.object_id AND sp.modification_counter > 500
  );

EXEC sp_executesql @stats;

PRINT 'Maintenance completed: ' + CONVERT(varchar(19), GETDATE(), 120);
