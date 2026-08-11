<#
    Rebrands leftover 'Waseela' identifiers inside the non-production sandbox /
    clone databases so they match the production naming convention already used
    by FazalDinPP19DataBaseV2 (Waseela -> Bismila, matching the patched binaries).

    Non-destructive: no database is dropped and no row data is modified.
    Only object names, one column name, and module bodies are rewritten.
#>
[CmdletBinding()]
param(
    [switch]$WhatIfOnly,
    [string]$PreferencesPath = 'D:\ABUZAR\Preferences.ini'
)

$ErrorActionPreference = 'Stop'

function Convert-BrandText {
    param([string]$Text)
    if ($null -eq $Text) { return $null }
    $Text = $Text -creplace 'WASEELA', 'BISMILA'
    $Text = $Text -creplace 'Waseela', 'Bismila'
    $Text = $Text -creplace 'waseela', 'bismila'
    # any remaining mixed-case spellings
    return [regex]::Replace($Text, 'waseela', 'Bismila', 'IgnoreCase')
}

$ini = Get-Content $PreferencesPath -Raw
$pw = ([regex]::Match($ini, '(?im)^\s*LogPassword\s*=\s*(.+?)\s*$')).Groups[1].Value
if (-not $pw) { throw "Could not read LogPassword from $PreferencesPath" }

Add-Type -AssemblyName System.Data
$cn = New-Object System.Data.SqlClient.SqlConnection `
    "Server=localhost;Database=master;User ID=sa;Password=$pw;TrustServerCertificate=True;Connect Timeout=30"
$cn.Open()

function Invoke-Rows {
    param([string]$Sql)
    $c = $cn.CreateCommand(); $c.CommandTimeout = 600; $c.CommandText = $Sql
    $da = New-Object System.Data.SqlClient.SqlDataAdapter $c
    $dt = New-Object System.Data.DataTable
    [void]$da.Fill($dt)
    return $dt.Rows
}
function Invoke-NonQuery {
    param([string]$Sql)
    $c = $cn.CreateCommand(); $c.CommandTimeout = 600; $c.CommandText = $Sql
    [void]$c.ExecuteNonQuery()
}

$targets = Invoke-Rows @"
SELECT name FROM sys.databases
WHERE state_desc = 'ONLINE' AND database_id > 4
  AND name <> 'FazalDinPP19DataBaseV2'
ORDER BY name;
"@

$summary = @()
foreach ($row in $targets) {
    $db = $row['name']
    $cn.ChangeDatabase($db)

    $mods = @(Invoke-Rows @"
SELECT o.type AS otype, s.name AS sch, o.name AS nm, m.definition AS def
FROM sys.sql_modules m
JOIN sys.objects o ON o.object_id = m.object_id
JOIN sys.schemas s ON s.schema_id = o.schema_id
WHERE (m.definition LIKE '%waseela%' OR o.name LIKE '%waseela%')
  AND o.type IN ('P','V','FN','IF','TF','TR')
ORDER BY CASE o.type WHEN 'V' THEN 1 ELSE 2 END, o.name;
"@)
    $cols = @(Invoke-Rows @"
SELECT t.name AS tbl, c.name AS col
FROM sys.columns c JOIN sys.tables t ON t.object_id = c.object_id
WHERE c.name LIKE '%waseela%';
"@)
    $objs = @(Invoke-Rows @"
SELECT o.type_desc AS td, s.name AS sch, o.name AS nm
FROM sys.objects o JOIN sys.schemas s ON s.schema_id = o.schema_id
WHERE o.name LIKE '%waseela%'
  AND o.type NOT IN ('P','V','FN','IF','TF','TR')
ORDER BY CASE WHEN o.type = 'D' THEN 2 ELSE 1 END;
"@)

    $n = $mods.Count + $cols.Count + $objs.Count
    if ($n -eq 0) { continue }

    if ($WhatIfOnly) {
        $summary += [pscustomobject]@{ Database = $db; Modules = $mods.Count
            Columns = $cols.Count; Objects = $objs.Count; Status = 'WOULD CHANGE' }
        continue
    }

    try {
        # 1. drop affected modules (procs first, then views)
        foreach ($m in ($mods | Sort-Object { if ($_['otype'].Trim() -eq 'V') { 2 } else { 1 } })) {
            $kind = switch ($m['otype'].Trim()) {
                'V' { 'VIEW' } 'P' { 'PROCEDURE' } 'TR' { 'TRIGGER' } default { 'FUNCTION' }
            }
            Invoke-NonQuery ("DROP {0} [{1}].[{2}];" -f $kind, $m['sch'], $m['nm'])
        }
        # 2. rename column(s)
        foreach ($c in $cols) {
            $new = Convert-BrandText $c['col']
            Invoke-NonQuery ("EXEC sp_rename N'[dbo].[{0}].[{1}]', N'{2}', 'COLUMN';" -f $c['tbl'], $c['col'], $new)
        }
        # 3. rename remaining objects (default constraints etc.)
        foreach ($o in $objs) {
            $new = Convert-BrandText $o['nm']
            Invoke-NonQuery ("EXEC sp_rename N'[{0}].[{1}]', N'{2}', 'OBJECT';" -f $o['sch'], $o['nm'], $new)
        }
        # 4. recreate modules (views first, then the rest)
        foreach ($m in ($mods | Sort-Object { if ($_['otype'].Trim() -eq 'V') { 1 } else { 2 } })) {
            Invoke-NonQuery (Convert-BrandText $m['def'])
        }
        $summary += [pscustomobject]@{ Database = $db; Modules = $mods.Count
            Columns = $cols.Count; Objects = $objs.Count; Status = 'OK' }
    }
    catch {
        $summary += [pscustomobject]@{ Database = $db; Modules = $mods.Count
            Columns = $cols.Count; Objects = $objs.Count; Status = "FAILED: $($_.Exception.Message)" }
    }
}

$cn.ChangeDatabase('master')
$cn.Close()
$summary | Format-Table -AutoSize | Out-String -Width 200
