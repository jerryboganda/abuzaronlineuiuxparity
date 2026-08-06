#!/usr/bin/env sh
set -eu

: "${ABUZAR_ADMIN_DATABASE_URL:?ABUZAR_ADMIN_DATABASE_URL must be set to the protected schema-owner DSN}"
: "${ABUZAR_APP_ROLE:?ABUZAR_APP_ROLE must name an already-created application role}"

script=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)/grant-app-role.sql
psql "$ABUZAR_ADMIN_DATABASE_URL" --set ON_ERROR_STOP=1 --variable="app_role=$ABUZAR_APP_ROLE" --file "$script"
