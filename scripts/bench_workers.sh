#!/usr/bin/env sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
root_dir=$(CDPATH= cd -- "$script_dir/.." && pwd)

workers=${GODDDDOCR_BENCH_WORKERS:-"1 2 4 8"}
requests=${GODDDDOCR_BENCH_REQUESTS:-100}
concurrency=${GODDDDOCR_BENCH_CONCURRENCY:-4}
port=${GODDDDOCR_BENCH_PORT:-18089}
image=${GODDDDOCR_BENCH_IMAGE:-"$root_dir/samples/yzm1.png"}
expect=${GODDDDOCR_BENCH_EXPECT:-"3n3d"}
out_dir=${GODDDDOCR_BENCH_OUT:-"/tmp/goddddocr-bench-$(date +%Y%m%d-%H%M%S)"}

server_pid=""

cleanup() {
	if [ -n "$server_pid" ]; then
		kill "$server_pid" >/dev/null 2>&1 || true
		wait "$server_pid" >/dev/null 2>&1 || true
		server_pid=""
	fi
}
trap cleanup EXIT INT TERM

require_command() {
	if ! command -v "$1" >/dev/null 2>&1; then
		echo "$1 is required" >&2
		exit 127
	fi
}

wait_ready() {
	base_url=$1
	log_file=$2
	i=0
	while [ "$i" -lt 60 ]; do
		if curl -fsS "$base_url/ready" >/dev/null 2>&1; then
			return 0
		fi
		if ! kill -0 "$server_pid" >/dev/null 2>&1; then
			echo "server exited before ready; log follows:" >&2
			cat "$log_file" >&2 || true
			return 1
		fi
		i=$((i + 1))
		sleep 1
	done
	echo "server did not become ready: $base_url/ready" >&2
	cat "$log_file" >&2 || true
	return 1
}

append_summary() {
	worker=$1
	json_file=$2
	metrics_file=$3
	rss_kb=$4

	if command -v jq >/dev/null 2>&1; then
		qps=$(jq -r '.qps' "$json_file")
		p50=$(jq -r '.p50_latency_ms' "$json_file")
		p95=$(jq -r '.p95_latency_ms' "$json_file")
		p99=$(jq -r '.p99_latency_ms' "$json_file")
		errors=$(jq -r '.errors' "$json_file")
		mismatches=$(jq -r '.mismatches' "$json_file")
		printf '| %s | %s | %s | %s | %s | %s | %s | %s | %s |\n' "$worker" "$qps" "$p50" "$p95" "$p99" "$errors" "$mismatches" "$rss_kb" "$metrics_file" >> "$summary_file"
	else
		printf '| %s | see %s |  |  |  |  |  | %s | %s |\n' "$worker" "$json_file" "$rss_kb" "$metrics_file" >> "$summary_file"
	fi
}

require_command go
require_command curl

if [ ! -f "$image" ]; then
	echo "benchmark image not found: $image" >&2
	exit 1
fi

mkdir -p "$out_dir"
server_bin="$out_dir/goddddocr-server"
bench_bin="$out_dir/ocrbench"
summary_file="$out_dir/summary.md"

cd "$root_dir"
go build -o "$server_bin" ./cmd/goddddocr-server
go build -o "$bench_bin" ./cmd/ocrbench

cat > "$summary_file" <<EOF
# goddddocr Worker Benchmark

- image: \`$image\`
- expect: \`$expect\`
- requests: \`$requests\`
- concurrency: \`$concurrency\`
- base_url: \`http://127.0.0.1:$port\`
- host: \`$(uname -a)\`

| workers | qps | p50 ms | p95 ms | p99 ms | errors | mismatches | rss kb | metrics |
|---:|---:|---:|---:|---:|---:|---:|---:|---|
EOF

for worker in $workers; do
	base_url="http://127.0.0.1:$port"
	log_file="$out_dir/server-workers-$worker.log"
	json_file="$out_dir/bench-workers-$worker.json"
	metrics_file="$out_dir/metrics-workers-$worker.json"

	"$server_bin" -addr "127.0.0.1:$port" -workers "$worker" > "$log_file" 2>&1 &
	server_pid=$!
	wait_ready "$base_url" "$log_file"

	"$bench_bin" \
		-url "$base_url" \
		-image "$image" \
		-requests "$requests" \
		-concurrency "$concurrency" \
		-expect "$expect" \
		-json > "$json_file"

	curl -fsS "$base_url/metrics" > "$metrics_file" || true
	rss_kb=$(ps -o rss= -p "$server_pid" 2>/dev/null | tr -d ' ' || true)
	append_summary "$worker" "$json_file" "$metrics_file" "${rss_kb:-unknown}"

	cleanup
done

printf 'benchmark output: %s\n' "$out_dir"
printf 'summary: %s\n' "$summary_file"
