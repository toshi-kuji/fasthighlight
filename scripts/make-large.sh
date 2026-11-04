#!/usr/bin/env bash
# Build a large input for benchmarking: concatenate testdata/input.md and testdata/multilang.md alternately N times (default 5).
#
#   scripts/make-large.sh [N] > bench-bin/large.md
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
n=${1:-5}
for i in $(seq "$n"); do
  cat "$root/testdata/input.md"
  printf '\n'
  cat "$root/testdata/multilang.md"
  printf '\n'
done
