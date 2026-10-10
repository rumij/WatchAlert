#!/usr/bin/env bash
set -euo pipefail
release_tag="${1:-zabbix-silence-20261010}"
if [[ ! "$release_tag" =~ ^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$ ]]; then
  echo "Invalid image tag" >&2
  exit 1
fi
repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
output_dir="$repo_root/dist/$release_tag"
mkdir -p "$output_dir"
docker build --platform "${WATCHALERT_PLATFORM:-linux/amd64}" --build-arg "VERSION=$release_tag" -t "watchalert-backend:$release_tag" "$repo_root"
docker build --platform "${WATCHALERT_PLATFORM:-linux/amd64}" --build-arg "NPM_REGISTRY=${WATCHALERT_NPM_REGISTRY:-https://registry.npmjs.org}" -t "watchalert-frontend:$release_tag" "$repo_root/web"
docker image save -o "$output_dir/watchalert-app-images.tar" "watchalert-backend:$release_tag" "watchalert-frontend:$release_tag"
cp "$repo_root/deploy/docker-compose/upgrade-app.yaml" "$repo_root/deploy/docker-compose/upgrade-validation.yaml" "$output_dir/"
cp "$repo_root/docs/container-upgrade-20261010.md" "$output_dir/UPGRADE.md"
printf '%s\n' "$release_tag" > "$output_dir/RELEASE_TAG"
(cd "$output_dir" && sha256sum watchalert-app-images.tar > SHA256SUMS)
echo "Offline application package: $output_dir"
