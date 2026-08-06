$ErrorActionPreference = 'Stop'

if ([string]::IsNullOrWhiteSpace($env:ABUZAR_ADMIN_DATABASE_URL)) {
    throw 'ABUZAR_ADMIN_DATABASE_URL must be set to the protected schema-owner DSN.'
}
if ([string]::IsNullOrWhiteSpace($env:ABUZAR_APP_ROLE)) {
    throw 'ABUZAR_APP_ROLE must be set to the already-created application role.'
}

$script = Join-Path $PSScriptRoot 'grant-app-role.sql'
& psql $env:ABUZAR_ADMIN_DATABASE_URL --set ON_ERROR_STOP=1 --variable "app_role=$($env:ABUZAR_APP_ROLE)" --file $script
if ($LASTEXITCODE -ne 0) {
    throw 'Application role grant failed.'
}
