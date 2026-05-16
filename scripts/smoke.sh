#!/usr/bin/env sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
root_dir=$(CDPATH= cd -- "$script_dir/.." && pwd)

image=${GODDDDOCR_SMOKE_IMAGE:-"$root_dir/samples/yzm1.png"}
expect=${GODDDDOCR_SMOKE_EXPECT:-"3n3d"}

run_doctor() {
	if [ -n "${GODDDDOCR_DOCTOR_BIN:-}" ]; then
		"$GODDDDOCR_DOCTOR_BIN" "$@"
		return
	fi
	if [ -x "$root_dir/ocrdoctor" ]; then
		"$root_dir/ocrdoctor" "$@"
		return
	fi
	if command -v ocrdoctor >/dev/null 2>&1; then
		ocrdoctor "$@"
		return
	fi
	if command -v go >/dev/null 2>&1 && [ -d "$root_dir/cmd/ocrdoctor" ]; then
		(cd "$root_dir" && go run ./cmd/ocrdoctor "$@")
		return
	fi

	echo "ocrdoctor not found. Set GODDDDOCR_DOCTOR_BIN or run from the source tree with Go installed." >&2
	return 127
}

if [ -n "$expect" ]; then
	set -- -image "$image" -expect "$expect" -json "$@"
else
	set -- -image "$image" -json "$@"
fi

run_doctor "$@"
