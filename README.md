# goddddocr

Go OCR service/module based on ddddocr ONNX models, without a Python runtime.
The OCR models and charsets are derived from
https://github.com/sml2h3/ddddocr.

## Quick Start

```bash
go test ./...
go run ./cmd/goddddocr-server -addr :8088
```

```bash
curl -s http://127.0.0.1:8088/health
curl -s -X POST http://127.0.0.1:8088/ocr \
  -H 'content-type: application/json' \
  -d "{\"image\":\"$(base64 -i samples/yzm1.png)\",\"confidence\":true}"
```

Before wiring the service into tsplay, run the local doctor command on the
target machine. It checks ONNX Runtime loading, model/charset configuration, and
optionally one sample OCR result without starting HTTP:

```bash
go run ./cmd/ocrdoctor -image samples/yzm1.png -expect 3n3d
go run ./cmd/ocrdoctor -json
scripts/smoke.sh
```

On Windows, use PowerShell:

```powershell
.\scripts\smoke.ps1
```

## Go Client

tsplay should call the service over HTTP first, so cgo and ONNX Runtime stay
out of the tsplay process:

```go
client := goddddocr.NewOCRClient("http://127.0.0.1:8088")
if err := client.Ready(ctx); err != nil {
    return err
}

result, err := client.ClassifyBytes(ctx, imageBytes, &goddddocr.RemoteClassifyOptions{
    CharsetRange: "0123456789abcdefghijklmnopqrstuvwxyz",
    Confidence: true,
})
if err != nil {
    return err
}
fmt.Println(result.Result)
```

## Service Config

CLI flags can be supplied directly or through environment variables:

| Flag | Env | Default |
|---|---|---|
| `-addr` | `GODDDDOCR_ADDR` | `:8088` |
| `-model` | `GODDDDOCR_MODEL` | `old` |
| `-model-path` | `GODDDDOCR_MODEL_PATH` | empty |
| `-charset-path` | `GODDDDOCR_CHARSET_PATH` | empty |
| `-input-name` | `GODDDDOCR_INPUT_NAME` | `input1` |
| `-output-name` | `GODDDDOCR_OUTPUT_NAME` | `387` |
| `-png-fix` | `GODDDDOCR_PNG_FIX` | `false` |
| `-workers` | `GODDDDOCR_WORKERS` | `1` |
| `-log-format` | `GODDDDOCR_LOG_FORMAT` | `text` |
| `-max-image-bytes` | `GODDDDOCR_MAX_IMAGE_BYTES` | `8388608` |
| `-shutdown-timeout` | `GODDDDOCR_SHUTDOWN_TIMEOUT` | `10s` |
| `-onnxruntime-lib` | `ONNXRUNTIME_SHARED_LIBRARY_PATH` | empty |

`-workers=N` creates N independent OCR sessions behind the HTTP service. Start
with `1`, then increase gradually after checking `/metrics` latency and memory.
Use `-log-format json` for one-JSON-object-per-line service and access logs.

Use `-model old` or `-model beta` for the embedded ddddocr OCR models. To load
an external OCR model, provide both `-model-path` and `-charset-path`; the
service reports the active model as `custom`. Custom charset files are JSON
arrays whose first entry must be the CTC blank token, usually an empty string:

```json
["", "0", "1", "2", "a", "b"]
```

Most ddddocr-compatible ONNX OCR models use input `input1` and output `387`.
If your exported model uses different tensor names, pass `-input-name` and
`-output-name`.

The same model flags are accepted by `cmd/ocrdoctor`, so deployment scripts can
validate a custom model before starting the long-running service:

```bash
go run ./cmd/ocrdoctor \
  -model-path /opt/models/custom.onnx \
  -charset-path /opt/models/charset.json \
  -image /opt/models/smoke.png \
  -expect abcd \
  -json
```

The release smoke scripts wrap the same doctor command. They first try
`GODDDDOCR_DOCTOR_BIN`, then a local `ocrdoctor` binary, then `ocrdoctor` from
`PATH`, and finally `go run ./cmd/ocrdoctor` when running from a source checkout.
Use `GODDDDOCR_SMOKE_IMAGE` and `GODDDDOCR_SMOKE_EXPECT` to point them at a
deployment-specific captcha sample:

```bash
GODDDDOCR_SMOKE_IMAGE=/opt/models/smoke.png \
GODDDDOCR_SMOKE_EXPECT=abcd \
scripts/smoke.sh
```

Endpoints:

- `GET /health`
- `GET /ready`
- `GET /metrics`
- `POST /ocr`
- `POST /ocr/file`

`POST /ocr` accepts:

```json
{
  "image": "base64-encoded-image",
  "png_fix": false,
  "charset_range": "0123456789abcdefghijklmnopqrstuvwxyz",
  "color_filter_colors": ["red", "blue"],
  "color_filter_custom_ranges": [[[90, 30, 30], [110, 255, 255]]],
  "confidence": true,
  "probability": false
}
```

`charset_range` may be a number, a string, or a string array. The response keeps
`result` as the recognized text and includes `confidence` only when requested.
Set `probability` to `true` to include a Python-compatible full probability
matrix:

```json
{
  "result": "3n3d",
  "probability": {
    "text": "3n3d",
    "charsets": ["", "0", "1"],
    "probability": [[0.01, 0.02, 0.97]],
    "confidence": 0.97
  }
}
```

`color_filter_colors` keeps only matching pixels and turns the rest white before
OCR preprocessing. Presets match ddddocr's HSV ranges: `red`, `blue`, `green`,
`yellow`, `orange`, `purple`, `cyan`, `black`, `white`, and `gray`.
`color_filter_custom_ranges` accepts HSV ranges in OpenCV scale
`[[lower_hsv], [upper_hsv]]`, where H is `0..180` and S/V are `0..255`.

`GET /metrics` returns service counters and latency aggregates as JSON:

```json
{
  "total_requests": 42,
  "completed_requests": 42,
  "error_requests": 1,
  "status_codes": {"200": 41, "400": 1},
  "average_latency_ms": 8.4,
  "max_latency_ms": 31.2
}
```

## Baseline Load Test

Use `ocrbench` while tuning `-workers`:

```bash
go run ./cmd/goddddocr-server -addr :8088 -workers 2
go run ./cmd/ocrbench -url http://127.0.0.1:8088 \
  -image samples/yzm1.png \
  -requests 100 \
  -concurrency 4 \
  -expect 3n3d
```

Run the same image and request count with `-workers 1`, `2`, and `4`, then
compare QPS, p50, p95, p99, errors, and `/metrics` output.

## ONNX Runtime

The code is portable across Windows, macOS, and Linux. The only platform-native
piece is the ONNX Runtime shared library. Loading order:

1. `Config.SharedLibraryPath` or `-onnxruntime-lib`
2. `ONNXRUNTIME_SHARED_LIBRARY_PATH`
3. `ONNXRUNTIME_HOME`
4. `third_party/onnxruntime/<GOOS>_<GOARCH>/`
5. embedded darwin/arm64 runtime, when available
6. system library path

Install the runtime for the current system:

```bash
go run ./cmd/ortfetch
```

Or install for another target:

```bash
go run ./cmd/ortfetch -goos linux -goarch amd64
go run ./cmd/ortfetch -goos windows -goarch amd64
go run ./cmd/ortfetch -goos darwin -goarch arm64
```

Manual setup also works:

```bash
export ONNXRUNTIME_SHARED_LIBRARY_PATH=/path/to/libonnxruntime.so
```

Windows uses `onnxruntime.dll`; macOS uses `libonnxruntime.dylib` or
`onnxruntime.dylib`; Linux uses `libonnxruntime.so`.

Because `github.com/yalue/onnxruntime_go` uses cgo, build on the target system
or install the matching cross C compiler:

- Windows: MSYS2/mingw-w64 or build natively on Windows.
- Linux: build natively or use a Linux cross compiler/container.
- macOS: Xcode command line tools.

## Docker

```bash
docker compose up --build
```

The container exposes `8088` and uses `/ready` for health checks.

## Golden OCR Fixtures

`fixtures/ocr_golden.json` records Python ddddocr outputs for sample images and
keeps the Go port honest as preprocessing and model options evolve. These
fixtures are test data only; the library and service still run without Python.

```bash
go test . -run TestGoldenOCRFixtures
go test ./...
```

Each fixture can set `model`, `charset_range`, `png_fix`, and
`min_confidence`. Add new representative captcha images under `samples/` or an
ignored local sample directory, record the Python ddddocr output in
`python_ddddocr`, and keep `expected` equal to that value unless the fixture is
documenting an intentional compatibility difference. If Python tooling is not
available yet, `expected` may still be used as a Go model regression baseline.

## Status

- OCR classification: implemented.
- HTTP service: `/health`, `/ocr`, `/ocr/file`.
- Detection and slide matching: planned, will require OpenCV/GoCV-compatible
  image processing.
