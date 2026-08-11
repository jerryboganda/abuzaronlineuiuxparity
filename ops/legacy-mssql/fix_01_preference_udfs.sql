/* =====================================================================
   Bismil / BISMILA  --  DB performance fix pack
   Fix 1: Preference lookup UDFs were non-SARGable.

   BEFORE:  WHERE LOWER(Name) = LOWER(@prefname)
            -> LOWER() on the column blocks index usage, so every single
               call did a FULL SCAN of SoftwarePreferences (1363 rows).
               These UDFs are called per-row inside report DataWindows,
               producing 23,598 table scans for ONE report retrieve.

   AFTER:   WHERE Name = @PrefName
            -> seeks PK_SoftwarePreferences_Name (clustered, unique).

   SAFETY:  SoftwarePreferences.Name collation = SQL_Latin1_General_CP1_CI_AS
            (case-INsensitive), and COUNT(*) = COUNT(DISTINCT LOWER(Name)) = 1363,
            so "Name = @PrefName" matches exactly the same row(s) as before.
   ===================================================================== */
SET ANSI_NULLS ON;
SET QUOTED_IDENTIFIER ON;
GO

ALTER FUNCTION DBO.Fn_GetPreference( @PrefName VARCHAR(100) )
RETURNS VARCHAR(200)
WITH SCHEMABINDING
AS
BEGIN
    /* Author : Muhammad Azhar Inayat
       Purpose: returns the string value of the named preference.
       2026-08-12 perf fix: removed LOWER() (non-SARGable); collation is CI. */
    DECLARE @PrefValue VARCHAR(200);

    SET @PrefValue = ISNULL((SELECT PrefValue
                             FROM DBO.SoftwarePreferences
                             WHERE Name = @PrefName), '');

    RETURN LTRIM(RTRIM(@PrefValue));
END
GO

ALTER FUNCTION DBO.Fn_Get_Int_Preference( @PrefName VARCHAR(100) )
RETURNS INT
WITH SCHEMABINDING
AS
BEGIN
    /* 2026-08-12 perf fix: removed LOWER() (non-SARGable); collation is CI. */
    DECLARE @PrefValue INT;

    SET @PrefValue = CAST((SELECT LTRIM(RTRIM(PrefValue))
                           FROM DBO.SoftwarePreferences
                           WHERE Name = @PrefName) AS INT);

    RETURN @PrefValue;
END
GO

ALTER FUNCTION DBO.Fn_Get_DateTime_Preference( @PrefName VARCHAR(100) )
RETURNS DATETIME
WITH SCHEMABINDING
AS
BEGIN
    /* 2026-08-12 perf fix: removed LOWER() (non-SARGable); collation is CI. */
    DECLARE @PrefValue DATETIME;

    SET @PrefValue = CAST((SELECT LTRIM(RTRIM(PrefValue))
                           FROM DBO.SoftwarePreferences
                           WHERE Name = @PrefName) AS DATETIME);

    RETURN @PrefValue;
END
GO
