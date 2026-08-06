#!/usr/bin/env sh
set -eu

: "${ABUZAR_ADMIN_DATABASE_URL:?ABUZAR_ADMIN_DATABASE_URL must be set to the protected schema-owner DSN}"

root=$(CDPATH= cd -- "$(dirname -- "$0")/../../db/migrations" && pwd)
count=0
for file in "$root"/*.sql; do
  [ -f "$file" ] || continue
  echo "Applying $(basename "$file")"
  psql "$ABUZAR_ADMIN_DATABASE_URL" --set ON_ERROR_STOP=1 --file "$file"
  count=$((count + 1))
done

[ "$count" -gt 0 ]
echo "Applied $count migrations."
