#!/bin/bash

set -e

if [[ $# -ne 2 ]]
then
  echo 'Usage: ./custom_db.sh <dump.custom> <db_name>'
  exit 1
fi

DUMP_FILE="$1"
DB_NAME="$2"
DB_USER="eln"

if [[ ! -f "$DUMP_FILE" ]]
then
  echo "File does not exists: $DUMP_FILE"
  exit 1
fi

echo 'Stopping containers and removing local database'
docker compose down
sudo rm -rf data/postgres
echo 'Starting postgreSQL container only'
docker compose up -d --wait postgres

docker compose ps
docker compose exec -T postgres sh -lc 'echo "POSTGRES_USER=$POSTGRES_USER"; echo "POSTGRES_DB=$POSTGRES_DB"; cat "$PGDATA/PG_VERSION"'

echo 'Copying dump into PostgreSQL container'
docker compose exec -T postgres dropdb -U "$DB_USER" --if-exists --maintenance-db=postgres "$DB_NAME"
docker compose cp "$DUMP_FILE" postgres:/tmp/db.custom

echo "Recreating database: $DB_NAME"
docker compose exec -T postgres createdb -U "$DB_USER" --maintenance-db=postgres "$DB_NAME"

echo "Restoring dump into database: $DB_NAME"
docker compose exec -T postgres pg_restore -U "$DB_USER" --verbose --exit-on-error --no-owner --no-acl --dbname "$DB_NAME" /tmp/db.custom

echo 'Starting all containers'
docker compose up -d
