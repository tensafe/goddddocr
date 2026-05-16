#!/usr/bin/env sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
root_dir=$(CDPATH= cd -- "$script_dir/.." && pwd)

docker_image=${GODDDDOCR_DOCKER_IMAGE:-goddddocr:smoke}
container_name=${GODDDDOCR_DOCKER_CONTAINER:-goddddocr-smoke-$$}
port=${GODDDDOCR_DOCKER_PORT:-18088}
platform=${GODDDDOCR_DOCKER_PLATFORM:-}
sample_image=${GODDDDOCR_SMOKE_IMAGE:-"$root_dir/samples/yzm1.png"}
expect=${GODDDDOCR_SMOKE_EXPECT:-"3n3d"}

if [ -z "$platform" ]; then
	case "$(uname -m)" in
		arm64 | aarch64) platform=linux/arm64 ;;
		x86_64 | amd64) platform=linux/amd64 ;;
		*) platform=linux/amd64 ;;
	esac
fi

cleanup() {
	docker rm -f "$container_name" >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

if ! command -v docker >/dev/null 2>&1; then
	echo "docker is required for docker smoke" >&2
	exit 127
fi
if ! command -v curl >/dev/null 2>&1; then
	echo "curl is required for docker smoke" >&2
	exit 127
fi
if [ ! -f "$sample_image" ]; then
	echo "smoke image not found: $sample_image" >&2
	exit 1
fi

cleanup
docker build --platform "$platform" -t "$docker_image" "$root_dir"
docker run --platform "$platform" -d --name "$container_name" -p "127.0.0.1:$port:8088" "$docker_image" >/dev/null

ready_url="http://127.0.0.1:$port/ready"
ready=false
i=0
while [ "$i" -lt 60 ]; do
	if curl -fsS "$ready_url" >/dev/null 2>&1; then
		ready=true
		break
	fi
	i=$((i + 1))
	sleep 1
done
if [ "$ready" != "true" ]; then
	echo "container did not become ready: $ready_url" >&2
	docker logs "$container_name" >&2 || true
	exit 1
fi

encoded=$(base64 < "$sample_image" | tr -d '\n')
payload=$(printf '{"image":"%s","confidence":true}' "$encoded")
response=$(curl -fsS -H "content-type: application/json" -d "$payload" "http://127.0.0.1:$port/ocr")

if [ -n "$expect" ]; then
	case "$response" in
		*\"result\":\"$expect\"*) ;;
		*)
			echo "unexpected OCR response, expected result $expect" >&2
			echo "$response" >&2
			docker logs "$container_name" >&2 || true
			exit 1
			;;
	esac
fi

printf '%s\n' "$response"
