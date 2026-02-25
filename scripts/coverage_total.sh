#!/usr/bin/env bash
set -euo pipefail

threshold="${1:-80}"
shift || true

if [[ "$#" -eq 0 ]]; then
  pkgs=(./internal/...)
else
  pkgs=("$@")
fi

tmp_file="$(mktemp)"
trap 'rm -f "$tmp_file"' EXIT

go test "${pkgs[@]}" -coverprofile="$tmp_file" >/dev/null
pct="$(go tool cover -func="$tmp_file" | awk '/^total:/ {gsub("%","",$3); print $3}')"

echo "Total coverage: ${pct}% (threshold: ${threshold}%)"
if awk -v p="$pct" -v t="$threshold" 'BEGIN {exit !(p+0 < t+0)}'; then
  echo "Coverage gate failed"
  exit 1
fi
