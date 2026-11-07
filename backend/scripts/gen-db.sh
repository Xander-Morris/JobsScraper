#!/usr/bin/env bash
# Regenerates internal/gen/{table,model} from the migrations: applies them to a
# scratch database, then runs go-jet against it.
set -euo pipefail

cd "$(dirname "$0")/.."

ADMIN_DSN="${DATABASE_GEN_ADMIN_DSN:-postgresql://postgres:postgres@localhost:55432/postgres?sslmode=disable}"
GEN_DB="jetgen"
GEN_DSN="${ADMIN_DSN%/*}/${GEN_DB}?sslmode=disable"

psql_admin() {
  go run -tags tools ./scripts/sqlexec "$ADMIN_DSN" "$1"
}

psql_admin "DROP DATABASE IF EXISTS ${GEN_DB} WITH (FORCE)"
psql_admin "CREATE DATABASE ${GEN_DB}"

DATABASE_CONNECTION="$GEN_DSN" go run ./cmd/migrate

out="$(mktemp -d)"
trap 'rm -rf "$out"' EXIT

go tool jet -dsn="$GEN_DSN" -schema=public -path="$out" -ignore-tables=schema_migrations

rm -rf internal/gen
mkdir -p internal/gen
mv "$out/$GEN_DB/public/"* internal/gen/
gofmt -w internal/gen

psql_admin "DROP DATABASE ${GEN_DB} WITH (FORCE)"
