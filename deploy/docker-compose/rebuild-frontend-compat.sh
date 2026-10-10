#!/usr/bin/env bash
set -euo pipefail
asset_image="${1:-watchalert-frontend:zabbix-silence-20261010}"
runtime_image="${2:-watchalert-frontend:before-silence}"
output_image="${3:-watchalert-frontend:zabbix-silence-20261010-compat}"
if [[ "$output_image" == "$asset_image" || "$output_image" == "$runtime_image" ]]; then
  echo "Output image must use a separate tag" >&2
  exit 1
fi
repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
docker image inspect "$asset_image" >/dev/null
docker image inspect "$runtime_image" >/dev/null
docker build --pull=false --network=none \
  --build-arg "ASSET_IMAGE=$asset_image" \
  --build-arg "RUNTIME_IMAGE=$runtime_image" \
  -f "$repo_root/web/Dockerfile.compat" -t "$output_image" "$repo_root/web"
echo "Built frontend compatibility image: $output_image"
