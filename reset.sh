#!/bin/bash

set -e
docker compose down
sudo rm -rf data/postgres
docker compose up -d
echo "waiting"
sleep 5
./cli db migrate up
./cli db seed
./cli admin add "$1"
if [ "$2" ]
then
  ./cli admin remove "$2"
fi
