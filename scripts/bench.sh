#!/usr/bin/env bash
# Benchmark for fasthighlight. For the procedure, see docs/benchmark.md.
#
#   scripts/bench.sh [input Markdown]        measures only our own implementation (auto, sequential, parallel)
#   BASELINES=1 scripts/bench.sh [input]      also measures the comparison implementations (Chroma, gotreesitter, Node)
#                                            (implementations that only have the languages in use run on the default input only)
#
# Cold: 7 runs, each from startup through one conversion to exit.
# Warm: after initialization each binary converts 300 times and reports the median and p95 (3 trials).
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
input=$(realpath "${1:-$root/testdata/input.md}")
bin="$root/bench-bin"
mkdir -p "$bin"
cd "$root"

cold() { # cold <name> <command...>
  local name=$1; shift
  local t
  printf '%s cold(s):' "$name"
  for _ in $(seq 7); do
    t=$( { TIMEFORMAT='%R'; time ONCE=1 "$@" >/dev/null; } 2>&1 )
    printf ' %s' "$t"
  done
  echo
}

warm() { # warm <command...>
  for _ in 1 2 3; do "$@"; done
}

go build -o "$bin/fasthighlight" ./cmd/bench
for m in auto seq par; do
  cold "fasthighlight-$m" "$bin/fasthighlight" "$m" "$input" "$bin/out-$m.html"
  warm "$bin/fasthighlight" "$m" "$input" "$bin/out-$m.html"
done
cmp "$bin/out-seq.html" "$bin/out-par.html"
cmp "$bin/out-seq.html" "$bin/out-auto.html"
echo "spans: $(grep -o '<span' "$bin/out-seq.html" | wc -l)"

[ "${BASELINES:-}" = 1 ] || exit 0

(
  cd bench
  go build -o "$bin/chroma-all" ./chroma-all
  go build -o "$bin/chroma-subset" ./chroma-subset
  go build -tags 'grammar_subset grammar_subset_typescript grammar_subset_javascript' -o "$bin/gotreesitter" ./gotreesitter
)
# chroma-subset and gotreesitter only have TypeScript and JavaScript, so they are measured only on the default input (ts and js only).
subset="chroma-subset gotreesitter"
[ "$input" = "$root/testdata/input.md" ] || subset=
for b in chroma-all $subset; do
  cold "$b" "$bin/$b" "$input" "$bin/out-$b.html"
  warm "$bin/$b" "$input" "$bin/out-$b.html"
done

if [ ! -d bench/node/node_modules ]; then
  (cd bench/node && npm install --no-audit --no-fund)
fi
for v in hljs shiki; do
  cold "node-$v" node bench/node/bench.mjs "$v" "$input" "$bin/out-node-$v.html"
  warm node bench/node/bench.mjs "$v" "$input" "$bin/out-node-$v.html"
done
