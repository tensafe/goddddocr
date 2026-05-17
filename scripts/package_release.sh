#!/usr/bin/env sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
root_dir=$(CDPATH= cd -- "$script_dir/.." && pwd)

version=${1:-${GODDDDOCR_VERSION:-}}
goos=${GOOS:-$(go env GOOS)}
goarch=${GOARCH:-$(go env GOARCH)}
out_dir=${GODDDDOCR_RELEASE_OUT:-"$root_dir/dist"}

if [ -z "$version" ]; then
	echo "usage: scripts/package_release.sh v1.x.x" >&2
	exit 2
fi

if ! printf '%s' "$version" | grep -Eq '^v1\.[0-9]+\.[0-9]+$'; then
	echo "release version must match v1.x.x, got: $version" >&2
	exit 2
fi

case "$goos/$goarch" in
	linux/amd64 | linux/arm64 | darwin/amd64 | darwin/arm64 | windows/amd64 | windows/arm64) ;;
	*)
		echo "unsupported release target: $goos/$goarch" >&2
		exit 2
		;;
esac

package_name="goddddocr-$version-$goos-$goarch"
package_dir="$out_dir/$package_name"
archive_path="$out_dir/$package_name.tar.gz"
exe_suffix=""
if [ "$goos" = "windows" ]; then
	exe_suffix=".exe"
	archive_path="$out_dir/$package_name.zip"
fi

rm -rf "$package_dir" "$archive_path"
mkdir -p "$package_dir/scripts" "$package_dir/samples" "$package_dir/docs" "$out_dir"

cd "$root_dir"

commands="goddddocr-server ocrdoctor ocrbench ocrprep ocreval ortfetch"
ldflags="-s -w"
if [ "$goos" = "windows" ]; then
	ldflags="$ldflags -extldflags=-static"
fi
for command in $commands; do
	CGO_ENABLED=${CGO_ENABLED:-1} GOOS="$goos" GOARCH="$goarch" \
		go build -trimpath -ldflags "$ldflags" -o "$package_dir/$command$exe_suffix" "./cmd/$command"
done

go run ./cmd/ortfetch -goos "$goos" -goarch "$goarch" -out "$package_dir/third_party/onnxruntime"

download_file() {
	url=$1
	out=$2
	if command -v curl >/dev/null 2>&1; then
		curl -fsSL --retry 3 --retry-delay 2 -o "$out" "$url"
		return
	fi
	echo "curl is required to download $url" >&2
	exit 127
}

if [ "$goos" = "windows" ]; then
	mkdir -p "$package_dir/redist/windows"
	case "$goarch" in
		amd64)
			download_file "https://aka.ms/vc14/vc_redist.x64.exe" "$package_dir/redist/windows/vc_redist.x64.exe"
			;;
		arm64)
			download_file "https://aka.ms/vc14/vc_redist.arm64.exe" "$package_dir/redist/windows/vc_redist.arm64.exe"
			;;
	esac
fi

cp README.md README.zh-CN.md LICENSE NOTICE "$package_dir/"
cp -R docs/zh-CN "$package_dir/docs/"
cp scripts/smoke.sh scripts/smoke.ps1 "$package_dir/scripts/"
chmod +x "$package_dir/scripts/smoke.sh"
cp samples/yzm1.png samples/yzm2.jpeg "$package_dir/samples/"

if [ "${GODDDDOCR_SKIP_PACKAGE_SMOKE:-}" != "1" ] && [ "$goos" != "windows" ]; then
	(cd "$package_dir" && scripts/smoke.sh)
fi

if [ "$goos" = "windows" ]; then
	(cd "$out_dir" && zip -qr "$archive_path" "$package_name")
else
	(cd "$out_dir" && tar -czf "$archive_path" "$package_name")
fi

printf '%s\n' "$archive_path"
