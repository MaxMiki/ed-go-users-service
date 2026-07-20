#!/bin/bash
set -e

cd "$(dirname "$0")"

if command -v docker-compose >/dev/null 2>&1; then
    docker-compose up -d
elif command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
    docker compose up -d
else
    echo "Error: docker-compose or the docker compose plugin is required." >&2
    exit 1
fi

echo "MongoDB is starting. Use 'docker logs -f users-mongo' to watch the logs."
