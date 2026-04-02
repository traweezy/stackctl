#!/usr/bin/env bash

set -euo pipefail

pull_retries="${STACKCTL_IMAGE_PULL_RETRIES:-3}"
retry_delay_seconds="${STACKCTL_IMAGE_PULL_RETRY_DELAY_SECONDS:-5}"

images=(
  "docker.io/library/postgres:16"
  "docker.io/library/redis:7"
  "docker.io/library/nats:2.12.5"
  "docker.io/dpage/pgadmin4:latest"
)

pull_image() {
  local image="$1"
  local attempt=1

  while true; do
    echo "Pulling ${image} (attempt ${attempt}/${pull_retries})"
    if podman pull "${image}"; then
      return 0
    fi

    if [ "${attempt}" -ge "${pull_retries}" ]; then
      echo "Failed to pull ${image} after ${pull_retries} attempts" >&2
      return 1
    fi

    attempt=$((attempt + 1))
    sleep "${retry_delay_seconds}"
  done
}

for image in "${images[@]}"; do
  pull_image "${image}"
done
