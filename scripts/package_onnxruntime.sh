#!/usr/bin/env sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
root_dir=$(CDPATH= cd -- "$script_dir/.." && pwd)

version=${1:-${GODDDDOCR_VERSION:-}}
out_dir=${GODDDDOCR_RELEASE_OUT:-"$root_dir/dist"}
package_name="goddddocr-onnxruntime-$version"
package_dir="$out_dir/$package_name"
archive_path="$out_dir/$package_name.tar.gz"

if [ -z "$version" ]; then
	echo "usage: scripts/package_onnxruntime.sh v1.x.x" >&2
	exit 2
fi

if ! printf '%s' "$version" | grep -Eq '^v1\.[0-9]+\.[0-9]+$'; then
	echo "release version must match v1.x.x, got: $version" >&2
	exit 2
fi

rm -rf "$package_dir" "$archive_path"
mkdir -p "$package_dir/docs" "$out_dir"

cd "$root_dir"

targets="
linux amd64
linux arm64
darwin amd64
darwin arm64
windows amd64
windows arm64
"

printf '%s\n' "ONNX Runtime bundle for goddddocr $version" > "$package_dir/README.txt"
printf '%s\n' "" >> "$package_dir/README.txt"
printf '%s\n' "Copy third_party/onnxruntime into a goddddocr source checkout or release package." >> "$package_dir/README.txt"
printf '%s\n' "goddddocr discovers third_party/onnxruntime/<GOOS>_<GOARCH>/ automatically." >> "$package_dir/README.txt"
printf '%s\n' "" >> "$package_dir/README.txt"
printf '%s\n' "Windows note: onnxruntime.dll depends on Microsoft Visual C++ Redistributable." >> "$package_dir/README.txt"
printf '%s\n' "This bundle includes the official installers under redist/windows/." >> "$package_dir/README.txt"
printf '%s\n' "Install the matching one only when the target host is missing the runtime." >> "$package_dir/README.txt"

printf '%s\n' "$targets" | while read -r goos goarch; do
	if [ -z "${goos:-}" ]; then
		continue
	fi
	go run ./cmd/ortfetch -goos "$goos" -goarch "$goarch" -out "$package_dir/third_party/onnxruntime"
done

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

mkdir -p "$package_dir/redist/windows"
download_file "https://aka.ms/vc14/vc_redist.x64.exe" "$package_dir/redist/windows/vc_redist.x64.exe"
download_file "https://aka.ms/vc14/vc_redist.arm64.exe" "$package_dir/redist/windows/vc_redist.arm64.exe"

cp README.md README.zh-CN.md LICENSE NOTICE "$package_dir/"
cp -R docs/zh-CN "$package_dir/docs/"

(cd "$out_dir" && tar -czf "$archive_path" "$package_name")

printf '%s\n' "$archive_path"
