#!/usr/bin/env bash
# Builds a release binary into bin/linux.
set -euo pipefail

cd "$(dirname "$0")"

mkdir -p bin/linux
go build -trimpath -ldflags "-s -w" -o bin/linux/pdx-portrait-creator ./cmd/pdx-portrait-creator

echo "built bin/linux/pdx-portrait-creator"
