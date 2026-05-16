#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REPORT_DIR="${GODDDDOCR_PREP_REPORT_DIR:-reports/preprocess}"
PYTHON_BIN="${GODDDDOCR_PREP_PYTHON:-python3}"
PNG_FIX="${GODDDDOCR_PREP_PNG_FIX:-false}"
FAIL_ON_DIFF="${GODDDDOCR_PREP_FAIL_ON_DIFF:-false}"
MAX_DIFF_PIXELS="${GODDDDOCR_PREP_MAX_DIFF_PIXELS:-}"
MAX_DIFF_RATE="${GODDDDOCR_PREP_MAX_DIFF_RATE:-}"
MAX_ABS_DIFF="${GODDDDOCR_PREP_MAX_ABS_DIFF:-}"
MAX_RMSE="${GODDDDOCR_PREP_MAX_RMSE:-}"
export GODDDDOCR_PREP_MAX_DIFF_PIXELS="$MAX_DIFF_PIXELS"
export GODDDDOCR_PREP_MAX_DIFF_RATE="$MAX_DIFF_RATE"
export GODDDDOCR_PREP_MAX_ABS_DIFF="$MAX_ABS_DIFF"
export GODDDDOCR_PREP_MAX_RMSE="$MAX_RMSE"

if [[ "$#" -gt 0 ]]; then
  IMAGES=("$@")
elif [[ -n "${GODDDDOCR_PREP_IMAGES:-}" ]]; then
  # shellcheck disable=SC2206
  IMAGES=(${GODDDDOCR_PREP_IMAGES})
else
  IMAGES=("samples/yzm1.png")
fi

mkdir -p "${ROOT_DIR}/${REPORT_DIR}"

status=0
for image in "${IMAGES[@]}"; do
  if [[ "$image" = /* ]]; then
    image_path="$image"
    image_label="${image#/}"
  else
    image_path="${ROOT_DIR}/${image}"
    image_label="$image"
  fi
  if [[ ! -f "$image_path" ]]; then
    echo "preprocess_compare: image not found: $image" >&2
    status=1
    continue
  fi

  safe_name="${image_label//[^A-Za-z0-9._-]/_}"
  sample_dir="${ROOT_DIR}/${REPORT_DIR}/${safe_name}"
  mkdir -p "$sample_dir"

  python_png="${sample_dir}/python.png"
  python_csv="${sample_dir}/python.csv"
  python_json="${sample_dir}/python.json"
  go_png="${sample_dir}/go.png"
  go_csv="${sample_dir}/go.csv"
  go_json="${sample_dir}/go.json"
  diff_png="${sample_dir}/diff.png"

  python_args=(
    "${ROOT_DIR}/scripts/python_preprocess_reference.py"
    -image "$image_path"
    -out "$python_png"
    -matrix-csv "$python_csv"
    -json "$python_json"
  )
  go_args=(
    run ./cmd/ocrprep
    -image "$image_path"
    -out "$go_png"
    -matrix-csv "$go_csv"
    -json "$go_json"
    -compare-csv "$python_csv"
    -diff-png "$diff_png"
  )
  if [[ "$PNG_FIX" == "1" || "$PNG_FIX" == "true" ]]; then
    python_args+=(-png-fix)
    go_args+=(-png-fix)
  fi

  echo "==> $image"
  "$PYTHON_BIN" "${python_args[@]}" >/dev/null
  (cd "$ROOT_DIR" && go "${go_args[@]}") >/dev/null

  summary="$("$PYTHON_BIN" - "$go_json" <<'PY'
import json
import os
import sys


def env_float(name):
    value = os.environ.get(name, "").strip()
    if not value:
        return None
    try:
        return float(value)
    except ValueError:
        print(f"invalid {name}={value!r}", file=sys.stderr)
        sys.exit(4)


with open(sys.argv[1], "r", encoding="utf-8") as fp:
    report = json.load(fp)
diff = report.get("diff") or {}
print(
    "exact={exact} different_pixels={different} different_pixel_rate={rate} max_abs_diff={maxdiff} mean_abs_diff={mean} rmse={rmse}".format(
        exact=str(diff.get("exact_match")).lower(),
        different=diff.get("different_pixels"),
        rate=diff.get("different_pixel_rate"),
        maxdiff=diff.get("max_abs_diff"),
        mean=diff.get("mean_abs_diff"),
        rmse=diff.get("rmse"),
    )
)
thresholds = {
    "different_pixels": env_float("GODDDDOCR_PREP_MAX_DIFF_PIXELS"),
    "different_pixel_rate": env_float("GODDDDOCR_PREP_MAX_DIFF_RATE"),
    "max_abs_diff": env_float("GODDDDOCR_PREP_MAX_ABS_DIFF"),
    "rmse": env_float("GODDDDOCR_PREP_MAX_RMSE"),
}
violations = []
for key, limit in thresholds.items():
    if limit is None:
        continue
    value = diff.get(key)
    if value is None:
        violations.append(f"{key}=missing > {limit:g}")
    elif float(value) > limit:
        violations.append(f"{key}={value} > {limit:g}")
first = (diff.get("first_differences") or [])[:1]
if first:
    point = first[0]
    print(
        "first_difference=x:{x} y:{y} actual:{actual} reference:{reference} delta:{delta}".format(
            x=point.get("x"),
            y=point.get("y"),
            actual=point.get("actual"),
            reference=point.get("reference"),
            delta=point.get("delta"),
        )
    )
if violations:
    for violation in violations:
        print(f"threshold_violation={violation}")
    sys.exit(3)
if diff.get("exact_match") is not True:
    sys.exit(2)
PY
  )" || compare_status=$?
  compare_status="${compare_status:-0}"
  while IFS= read -r line; do
    echo "    $line"
  done <<<"$summary"
  echo "    report: ${REPORT_DIR}/${safe_name}/go.json"
  echo "    diff:   ${REPORT_DIR}/${safe_name}/diff.png"

  if [[ "$compare_status" -eq 2 ]]; then
    if [[ "$FAIL_ON_DIFF" == "1" || "$FAIL_ON_DIFF" == "true" ]]; then
      status=1
      break
    fi
  elif [[ "$compare_status" -ne 0 ]]; then
    status=1
    break
  fi
  unset compare_status
done

exit "$status"
