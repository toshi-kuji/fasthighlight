#!/usr/bin/env bash
# Build the distribution archives of fhl. Used by the release workflow (.github/workflows/release.yml).
# CI sets the version to dev and only checks that the archives can be built.
#
#   scripts/release.sh <version> <output dir>     e.g.: scripts/release.sh v0.3.0 dist
#
# Build fhl for Linux, macOS and Windows (amd64 and arm64) and package it together with LICENSE and
# THIRD_PARTY_NOTICES.md (zip for Windows, tar.gz for the others). Finally write SHA256SUMS.
set -euo pipefail

version=${1:?usage: scripts/release.sh VERSION OUTDIR}
out=${2:?usage: scripts/release.sh VERSION OUTDIR}
root=$(cd "$(dirname "$0")/.." && pwd)
mkdir -p "$out"
out=$(cd "$out" && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

# Timestamp of the files inside the archive (the time of the last commit, so that the same version gives the same archive)
stamp=$(git -C "$root" log -1 --format=%ct 2>/dev/null || echo 0)

for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64; do
  os=${target%/*}
  arch=${target#*/}
  name="fhl_${version}_${os}_${arch}"
  dir="$work/$name"
  mkdir -p "$dir"
  exe=fhl
  [ "$os" = windows ] && exe=fhl.exe
  (cd "$root" && CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -trimpath \
    -ldflags "-s -w -X main.version=$version" -o "$dir/$exe" ./cmd/fhl)
  cp "$root/LICENSE" "$root/THIRD_PARTY_NOTICES.md" "$dir/"
  find "$dir" -exec touch -d "@$stamp" {} +
  if [ "$os" = windows ]; then
    (cd "$work" && rm -f "$out/$name.zip" && zip -qX -r "$out/$name.zip" "$name")
  else
    tar -C "$work" --sort=name --owner=0 --group=0 --numeric-owner --mtime="@$stamp" \
      -czf "$out/$name.tar.gz" "$name"
  fi
done

(cd "$out" && sha256sum fhl_"${version}"_* > SHA256SUMS)
ls -l "$out"
