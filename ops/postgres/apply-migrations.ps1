$ErrorActionPreference = 'Stop'

if ([string]::IsNullOrWhiteSpace($env:ABUZAR_ADMIN_DATABASE_URL)) {
    throw 'ABUZAR_ADMIN_DATABASE_URL must be set to the protected schema-owner DSN.'
}

$migrationRoot = Join-Path $PSScriptRoot '..\..\db\migrations'
$files = Get-ChildItem -LiteralPath $migrationRoot -Filter '*.sql' -File | Sort-Object Name
if ($files.Count -eq 0) {
    throw "No migrations found under $migrationRoot"
}

foreach ($file in $files) {
    Write-Host "Applying $($file.Name)"
    & psql $env:ABUZAR_ADMIN_DATABASE_URL --set ON_ERROR_STOP=1 --file $file.FullName
    if ($LASTEXITCODE -ne 0) {
        throw "Migration failed: $($file.Name)"
    }
}

Write-Host "Applied $($files.Count) migrations."
