#!/usr/bin/env sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
root_dir=$(CDPATH= cd -- "$script_dir/.." && pwd)

cd "$root_dir"

go test ./...
go build ./cmd/...
if [ "${GODDDDOCR_SKIP_ORTFETCH:-}" != "1" ]; then
	go run ./cmd/ortfetch
fi
scripts/smoke.sh
