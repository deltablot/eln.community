#!/usr/bin/env bash
set -e

CYPRESS_VERSION="15.1.0"
X11_USER="$(id -un)"
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd -- "${SCRIPT_DIR}/../.." && pwd)"

cleanup() {
    xhost "-SI:localuser:${X11_USER}" >/dev/null
}

trap cleanup EXIT
xhost "+SI:localuser:${X11_USER}" >/dev/null

echo "Project directory: ${PROJECT_DIR}"
test -f "${PROJECT_DIR}/cypress.config.js" && echo "Cypress config found"

docker run --rm -it \
    --user node \
    --add-host host.docker.internal:host-gateway \
    --env DISPLAY="$DISPLAY" \
    --env CYPRESS_BASE_URL=http://host.docker.internal:8080 \
    --volume /tmp/.X11-unix:/tmp/.X11-unix:ro \
    --volume "${PROJECT_DIR}":/e2e \
    --workdir /e2e \
    --entrypoint cypress \
    cypress/included:"${CYPRESS_VERSION}" \
    open --project /e2e
