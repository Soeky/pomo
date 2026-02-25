#!/usr/bin/env bash
set -euo pipefail

threshold="${1:-70}"
shift || true

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

failed=0

if [[ "$#" -gt 0 ]]; then
  mapfile -t packages < <(printf '%s\n' "$@")
else
  mapfile -t packages < <(go list ./...)
fi

for pkg in "${packages[@]}"; do
  out="$tmp_dir/$(echo "$pkg" | tr '/.' '__').out"
  go test "$pkg" -coverprofile="$out" >/dev/null
  percent="$(go tool cover -func="$out" | awk '/^total:/ {gsub("%","",$3); print $3}')"
  if awk -v p="$percent" -v t="$threshold" 'BEGIN {exit !(p+0 < t+0)}'; then
    echo "FAIL $pkg coverage ${percent}% < ${threshold}%"
    failed=1
  else
    echo "PASS $pkg coverage ${percent}% >= ${threshold}%"
  fi
done

if [[ "$failed" -ne 0 ]]; then
  exit 1
fi
