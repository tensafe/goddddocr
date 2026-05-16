#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REPORT_DIR="${GODDDDOCR_PREP_REPORT_DIR:-reports/preprocess}"
PYTHON_BIN="${GODDDDOCR_PREP_PYTHON:-python3}"
PNG_FIX="${GODDDDOCR_PREP_PNG_FIX:-false}"
FAIL_ON_DIFF="${GODDDDOCR_PREP_FAIL_ON_DIFF:-false}"

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
import sys

with open(sys.argv[1], "r", encoding="utf-8") as fp:
    report = json.load(fp)
diff = report.get("diff") or {}
print(
    "exact={exact} different_pixels={different} max_abs_diff={maxdiff} mean_abs_diff={mean} rmse={rmse}".format(
        exact=str(diff.get("exact_match")).lower(),
        different=diff.get("different_pixels"),
        maxdiff=diff.get("max_abs_diff"),
        mean=diff.get("mean_abs_diff"),
        rmse=diff.get("rmse"),
    )
)
if diff.get("exact_match") is not True:
    sys.exit(2)
PY
  )" || compare_status=$?
  compare_status="${compare_status:-0}"
  echo "    $summary"
  echo "    report: ${REPORT_DIR}/${safe_name}/go.json"
  echo "    diff:   ${REPORT_DIR}/${safe_name}/diff.png"

  if [[ "$compare_status" -ne 0 ]]; then
    if [[ "$FAIL_ON_DIFF" == "1" || "$FAIL_ON_DIFF" == "true" ]]; then
      status=1
      break
    fi
  fi
  unset compare_status
done

exit "$status"
