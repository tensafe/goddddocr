# goddddocr TODO

## P0: tsplay Integration Readiness

- [x] Provide a standalone HTTP OCR service.
  - Acceptance: `GET /ready` returns 200 after model load, and `POST /ocr` returns OCR text.
- [x] Add service request limits.
  - Acceptance: oversized JSON bodies, multipart uploads, and decoded images return structured errors.
- [x] Add graceful shutdown.
  - Acceptance: `SIGINT` / container stop shuts down the HTTP server with a timeout.
- [x] Add environment-based service config.
  - Acceptance: `GODDDDOCR_ADDR`, `GODDDDOCR_MODEL`, `GODDDDOCR_MAX_IMAGE_BYTES`, and `ONNXRUNTIME_SHARED_LIBRARY_PATH` work.
- [x] Add reusable Go HTTP client for tsplay.
  - Acceptance: caller can use `NewOCRClient(baseURL).ClassifyBytes(ctx, data, nil)`.
- [x] Add a small tsplay-side adapter/example.
  - Acceptance: tsplay can call goddddocr through a configurable base URL and timeout.

## P1: OCR Quality Alignment

- [x] Build initial golden-test dataset against Python ddddocr.
  - Acceptance: Python output fixtures exist for representative image samples.
- [x] Add batch comparison tests.
  - Acceptance: Go OCR output is compared against Python fixtures in CI/local tests.
- [ ] Investigate preprocessing mismatches.
  - Acceptance: differences from PIL resize/grayscale/alpha handling are documented or fixed.
- [x] Add beta model golden tests.
  - Acceptance: `ModelBeta` has at least one fixture and regression test.
- [ ] Add real tsplay captcha samples.
  - Acceptance: project has a private or ignored sample set for local accuracy checks.

## P2: OCR Feature Parity

- [x] Support old and beta OCR models.
  - Acceptance: model can be selected with config or `-model`.
- [x] Support PNG alpha background fix.
  - Acceptance: `PNGFix` and `png_fix` request field are respected.
- [x] Add charset range filtering.
  - Acceptance: supports int, string, and `[]string` semantics compatible with ddddocr.
- [x] Add probability/confidence output.
  - Acceptance: API can return text and confidence without returning a huge probability matrix by default.
- [x] Add optional full probability matrix output.
  - Acceptance: caller can opt in for debugging parity with Python.
- [x] Add HSV color filtering.
  - Acceptance: supports common presets such as red, blue, green, yellow, black, white, gray.
- [x] Add custom ONNX + charset config.
  - Acceptance: caller can load external model and charset metadata.

## P3: Deployment And Packaging

- [x] Add ONNX Runtime resolver.
  - Acceptance: explicit path, environment variables, local `third_party`, embedded runtime, and system loader are tried in order.
- [x] Add ONNX Runtime fetch helper.
  - Acceptance: `go run ./cmd/ortfetch` installs the current platform runtime.
- [x] Add Linux Dockerfile.
  - Acceptance: image builds the service and includes Linux ONNX Runtime.
- [x] Add docker compose skeleton.
  - Acceptance: `docker compose up --build` starts a service with healthcheck.
- [x] Add Docker smoke test script.
  - Acceptance: script builds the image, starts a temporary container, waits for `/ready`, and checks sample OCR.
- [x] Add upstream asset notice.
  - Acceptance: `NOTICE` identifies ddddocr as the model/charset source.
- [x] Add local deployment doctor.
  - Acceptance: `go run ./cmd/ocrdoctor` validates ONNX Runtime, model config, charset, and optional sample OCR without starting HTTP.
- [x] Add release smoke scripts around `ocrdoctor`.
  - Acceptance: macOS/Linux shell and Windows PowerShell scripts can run the same sample OCR check from source or release packages.
- [ ] Decide asset strategy before publishing.
  - Acceptance: choose Git LFS, release assets, or embedded models for distribution.
- [ ] Verify native builds on Windows, Linux, and macOS.
  - Acceptance: each target can start service and classify a sample image.
- [x] Add CI.
  - Acceptance: Linux test/build runs automatically.
- [x] Add optional Docker smoke CI.
  - Acceptance: manual workflow builds the Docker image and verifies `/ready` plus sample OCR.

## P4: Service Performance And Operations

- [x] Add request metrics.
  - Acceptance: expose request count, latency, and error count.
- [x] Add configurable concurrency/session pool.
  - Acceptance: `-workers=N` creates N OCR sessions for parallel inference.
- [x] Add baseline load-test command.
  - Acceptance: `go run ./cmd/ocrbench` reports QPS, p50/p95/p99 latency, errors, and mismatches.
- [x] Add repeatable worker benchmark script.
  - Acceptance: one command runs `workers=1/2/4/8`, records per-run JSON, metrics, logs, and a summary.
- [ ] Record baseline load-test results.
  - Acceptance: document QPS, p50/p95 latency, and memory on a standard machine.
- [ ] Reduce per-request allocations.
  - Acceptance: benchmark shows lower allocations in preprocessing/inference.
- [x] Add structured logging option.
  - Acceptance: logs can be consumed cleanly by Docker/systemd/tsplay.

## P5: Full ddddocr Migration

- [ ] Port object detection.
  - Acceptance: `common_det.onnx` inference and NMS return bounding boxes compatible with Python.
- [ ] Port slide comparison.
  - Acceptance: diff-based gap location works without Python.
- [ ] Port slide match.
  - Acceptance: template/edge matching returns target coordinates compatible with Python.
- [ ] Decide GoCV vs pure-Go image ops.
  - Acceptance: choose based on deployment weight and accuracy.
- [ ] Add HTTP endpoints for detection and slide features.
  - Acceptance: API shape is documented and tested.

## Recommended Next Batch

1. Expand Python-vs-Go golden fixtures with more real captcha samples.
2. Add Python-generated expected output for `ModelBeta` fixtures.
3. Verify Linux Docker build end to end on a clean host.
4. Verify native builds on Windows, Linux, and macOS with `scripts/smoke.*`.
5. Record baseline load-test results for `workers=1/2/4`.
