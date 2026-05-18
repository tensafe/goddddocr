#!/usr/bin/env sh
set -eu

repo=${GODDDDOCR_REPO:-tensafe/goddddocr}
version=${GODDDDOCR_VERSION:-latest}
install_root=${GODDDDOCR_INSTALL_DIR:-"$HOME/.local/share/goddddocr"}
addr=${GODDDDOCR_ADDR:-:8088}

if [ "$#" -gt 0 ]; then
	case "$1" in
		latest | v[0-9]*)
			version=$1
			shift
			;;
	esac
fi

need_command() {
	if ! command -v "$1" >/dev/null 2>&1; then
		echo "$1 is required" >&2
		exit 127
	fi
}

need_command curl
need_command tar

detect_goos() {
	case "$(uname -s)" in
		Linux) printf '%s\n' linux ;;
		Darwin) printf '%s\n' darwin ;;
		*)
			echo "unsupported OS: $(uname -s). Download the matching release package manually." >&2
			exit 2
			;;
	esac
}

detect_goarch() {
	case "$(uname -m)" in
		x86_64 | amd64) printf '%s\n' amd64 ;;
		arm64 | aarch64) printf '%s\n' arm64 ;;
		*)
			echo "unsupported architecture: $(uname -m)" >&2
			exit 2
			;;
	esac
}

resolve_latest_version() {
	url=$(curl -fsSL -o /dev/null -w '%{url_effective}' "https://github.com/$repo/releases/latest")
	case "$url" in
		*/tag/v*) printf '%s\n' "${url##*/tag/}" ;;
		*)
			echo "could not resolve latest release from $url" >&2
			exit 1
			;;
	esac
}

goos=$(detect_goos)
goarch=$(detect_goarch)

if [ "$version" = "latest" ]; then
	version=$(resolve_latest_version)
fi

if ! printf '%s' "$version" | grep -Eq '^v1\.[0-9]+\.[0-9]+$'; then
	echo "release version must match v1.x.x, got: $version" >&2
	exit 2
fi

package_name="goddddocr-$version-$goos-$goarch"
archive_name="$package_name.tar.gz"
url="https://github.com/$repo/releases/download/$version/$archive_name"
dest_dir="$install_root/$version/$goos-$goarch"

tmp_dir=$(mktemp -d "${TMPDIR:-/tmp}/goddddocr-install.XXXXXX")
cleanup() {
	rm -rf "$tmp_dir"
}
trap cleanup EXIT INT TERM

echo "Downloading $url"
curl -fL --retry 3 --retry-delay 2 -o "$tmp_dir/$archive_name" "$url"

mkdir -p "$install_root/$version"
rm -rf "$dest_dir.tmp"
mkdir -p "$dest_dir.tmp"
tar -xzf "$tmp_dir/$archive_name" -C "$dest_dir.tmp"
rm -rf "$dest_dir"
mv "$dest_dir.tmp/$package_name" "$dest_dir"
rm -rf "$dest_dir.tmp"

echo "Installed $package_name to $dest_dir"

if [ "${GODDDDOCR_INSTALL_ONLY:-}" = "1" ]; then
	exit 0
fi

if [ "$#" -eq 0 ]; then
	set -- -addr "$addr"
fi

if [ -x "$dest_dir/scripts/run.sh" ]; then
	exec "$dest_dir/scripts/run.sh" "$@"
fi

if [ "${GODDDDOCR_SKIP_SMOKE:-}" != "1" ] && [ -x "$dest_dir/scripts/smoke.sh" ]; then
	"$dest_dir/scripts/smoke.sh"
fi

if [ -x "$dest_dir/goddddocr-server" ]; then
	exec "$dest_dir/goddddocr-server" "$@"
fi

echo "goddddocr-server not found in $dest_dir" >&2
exit 127
