$ErrorActionPreference = 'Stop'

if ([string]::IsNullOrWhiteSpace($env:ABUZAR_ADMIN_DATABASE_URL)) {
    throw 'ABUZAR_ADMIN_DATABASE_URL must be set to the protected schema-owner DSN.'
}
if ([string]::IsNullOrWhiteSpace($env:ABUZAR_APP_ROLE)) {
    throw 'ABUZAR_APP_ROLE must be set to the dedicated application role name.'
}

$script = Join-Path $PSScriptRoot 'provision-app-role.sql'
& psql $env:ABUZAR_ADMIN_DATABASE_URL --set ON_ERROR_STOP=1 `
    --variable "app_role=$($env:ABUZAR_APP_ROLE)" --file $script
if ($LASTEXITCODE -ne 0) {
    throw 'Application role provisioning failed.'
}

# The optional ABUZAR_APP_ROLE_PASSWORD is consumed by psql from the
# protected process environment. It is never placed in arguments or output.
