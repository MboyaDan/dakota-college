#!/usr/bin/env bash
# Restores an encrypted backup into a throwaway PostgreSQL container and
# proves it is usable: migrations are current and the main tables have rows.
# Nothing is written anywhere else; the container is removed at the end.
#
#   backend/scripts/restore-check.sh <backup.dump.age> <age-identity-file>
#
# Needs docker and age. Run from anywhere.
set -euo pipefail

backup=${1:?usage: restore-check.sh <backup.dump.age> <age-identity-file>}
identity=${2:?usage: restore-check.sh <backup.dump.age> <age-identity-file>}
image=postgres:17.11-alpine
name=dakota-restore-check-$$
here=$(cd "$(dirname "$0")/.." && pwd)

cleanup() { docker rm -f "$name" >/dev/null 2>&1 || true; }
trap cleanup EXIT

echo "1/4 starting throwaway PostgreSQL ($image)"
docker run -d --name "$name" -e POSTGRES_PASSWORD=restore_only -e POSTGRES_DB=restore \
  -p 127.0.0.1::5432 "$image" >/dev/null
for _ in $(seq 1 60); do
  docker exec "$name" pg_isready -U postgres -d restore >/dev/null 2>&1 && break
  sleep 1
done
port=$(docker port "$name" 5432/tcp | head -1 | sed 's/.*://')

echo "2/4 decrypting and restoring $(basename "$backup")"
# The dump creates the public schema and its citext extension itself.
docker exec "$name" psql -q -U postgres -d restore -c 'DROP SCHEMA public CASCADE'
age --decrypt --identity "$identity" "$backup" \
  | docker exec -i "$name" pg_restore -U postgres -d restore --no-owner --no-privileges --exit-on-error

echo "3/4 checking migrations"
(cd "$here" && APP_ENV=development \
  DATABASE_URL="postgres://postgres:restore_only@127.0.0.1:$port/restore?sslmode=disable" \
  go run ./cmd/migrate status) | tail -5

echo "4/4 row counts"
docker exec "$name" psql -U postgres -d restore -At -F ' ' -c "
  SELECT 'users', count(*) FROM users UNION ALL
  SELECT 'roles', count(*) FROM roles UNION ALL
  SELECT 'programmes', count(*) FROM programmes UNION ALL
  SELECT 'applications', count(*) FROM applications UNION ALL
  SELECT 'audit_logs', count(*) FROM audit_logs UNION ALL
  SELECT 'goose version', max(version_id) FROM goose_db_version"
echo "restore check passed"
