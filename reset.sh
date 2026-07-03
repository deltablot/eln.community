#!/bin/bash

set -e
./cli db reset
docker compose down
sudo rm -rf data/postgres
docker compose up -d
echo "waiting"
sleep 5
./cli db migrate up && ./cli db seed && ./cli admin add $1 && ./cli admin remove 0000-0000-0000-0000
