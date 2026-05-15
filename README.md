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
  -d "{\"image\":\"$(base64 -i samples/yzm1.png)\"}"
```

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

## Status

- OCR classification: implemented.
- HTTP service: `/health`, `/ocr`, `/ocr/file`.
- Detection and slide matching: planned, will require OpenCV/GoCV-compatible
  image processing.
