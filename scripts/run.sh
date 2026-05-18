#!/usr/bin/env sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
root_dir=$(CDPATH= cd -- "$script_dir/.." && pwd)

addr=${GODDDDOCR_ADDR:-:8088}
server_bin=${GODDDDOCR_SERVER_BIN:-}

if [ -z "$server_bin" ]; then
	if [ -x "$root_dir/goddddocr-server" ]; then
		server_bin="$root_dir/goddddocr-server"
	elif [ -x "$root_dir/goddddocr-server.exe" ]; then
		server_bin="$root_dir/goddddocr-server.exe"
	else
		server_bin=""
	fi
fi

if [ "${GODDDDOCR_SKIP_SMOKE:-}" != "1" ]; then
	"$script_dir/smoke.sh"
fi

if [ "$#" -eq 0 ]; then
	set -- -addr "$addr"
fi

if [ -n "$server_bin" ]; then
	exec "$server_bin" "$@"
fi

if command -v go >/dev/null 2>&1 && [ -d "$root_dir/cmd/goddddocr-server" ]; then
	cd "$root_dir"
	exec go run ./cmd/goddddocr-server "$@"
fi

echo "goddddocr-server not found. Run from a release package or source checkout with Go installed." >&2
exit 127
