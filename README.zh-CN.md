# goddddocr

[English README](README.md)

goddddocr 是一个基于 ddddocr ONNX 模型的 Go OCR 模块和 HTTP 服务，不依赖
Python 运行时。OCR、检测模型和字符集资产来自
https://github.com/sml2h3/ddddocr，相关声明见 [NOTICE](NOTICE)。

## 快速开始

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

部署前建议先在目标机器运行自检。`ocrdoctor` 会检查 ONNX Runtime、模型、
字符集，并可选地验证一张样例图：

```bash
go run ./cmd/ocrdoctor -image samples/yzm1.png -expect 3n3d
scripts/smoke.sh
```

Windows 使用 PowerShell：

```powershell
.\scripts\smoke.ps1
```

## 下载发行包

GitHub Releases 使用 `v1.x.x` 版本标签。每个发行包包含服务端二进制、
辅助命令、对应平台的 ONNX Runtime 动态库、自检脚本、样例图片、英文/中文
文档、`LICENSE` 和 `NOTICE`。Release 页面还会额外发布
`goddddocr-onnxruntime-v1.x.x.tar.gz`，里面包含全部支持平台的 ONNX Runtime
动态库和 Windows Visual C++ Redistributable 安装器，适合离线分发或统一归档。

| 平台 | GitHub Runner | 文件格式 | ONNX Runtime | 动态库 |
|---|---|---|---|---|
| `linux/amd64` | `ubuntu-24.04` | `.tar.gz` | `1.25.0` | `libonnxruntime.so` |
| `linux/arm64` | `ubuntu-24.04-arm` | `.tar.gz` | `1.25.0` | `libonnxruntime.so` |
| `darwin/amd64` | `macos-15-intel` | `.tar.gz` | `1.23.2` | `onnxruntime.dylib` |
| `darwin/arm64` | `macos-15` | `.tar.gz` | `1.25.0` | `onnxruntime.dylib` |
| `windows/amd64` | `windows-2025` | `.zip` | `1.25.0` | `onnxruntime.dll` |
| `windows/arm64` | `windows-11-arm` | `.zip` | `1.25.0` | `onnxruntime.dll` |

macOS Intel (`darwin/amd64`) 使用 ONNX Runtime `1.23.2`，因为官方 `1.25.0`
没有提供 macOS amd64 CPU 归档。

Linux 和 macOS：

```bash
tar -xzf goddddocr-v1.0.0-linux-amd64.tar.gz
cd goddddocr-v1.0.0-linux-amd64
scripts/smoke.sh
./goddddocr-server -addr :8088
```

Windows PowerShell：

```powershell
Expand-Archive .\goddddocr-v1.0.0-windows-amd64.zip
cd .\goddddocr-v1.0.0-windows-amd64\goddddocr-v1.0.0-windows-amd64
.\scripts\smoke.ps1
.\goddddocr-server.exe -addr :8088
```

Windows 包会带上 `onnxruntime.dll`，但这个 DLL 仍依赖 Microsoft Visual C++
运行时，例如 `MSVCP140.dll`、`VCRUNTIME140.dll`、`VCRUNTIME140_1.dll` 和 UCRT
API-set DLL。Windows 发行包会把对应的 Microsoft Visual C++ Redistributable
安装器放在 `redist/windows/` 下；如果目标机器没有安装过对应运行时，再按需安装：

- `windows/amd64`：`redist/windows/vc_redist.x64.exe`
- `windows/arm64`：`redist/windows/vc_redist.arm64.exe`

更多发布流程见 [中文发布文档](docs/zh-CN/release.md)。

## HTTP 服务配置

常用参数既可以用命令行传入，也可以用环境变量配置：

| 参数 | 环境变量 | 默认值 |
|---|---|---|
| `-addr` | `GODDDDOCR_ADDR` | `:8088` |
| `-model` | `GODDDDOCR_MODEL` | `old` |
| `-model-path` | `GODDDDOCR_MODEL_PATH` | 空 |
| `-charset-path` | `GODDDDOCR_CHARSET_PATH` | 空 |
| `-workers` | `GODDDDOCR_WORKERS` | `1` |
| `-log-format` | `GODDDDOCR_LOG_FORMAT` | `text` |
| `-onnxruntime-lib` | `ONNXRUNTIME_SHARED_LIBRARY_PATH` | 空 |

内置 OCR 模型可使用 `-model old` 或 `-model beta`。如果要加载自定义 ONNX
模型，需要同时提供 `-model-path` 和 `-charset-path`；字符集文件是 JSON 数组，
第一个元素通常是 CTC blank token：

```json
["", "0", "1", "2", "a", "b"]
```

`-workers=N` 会在服务内创建 N 个独立 OCR session。建议从 `1` 开始，再结合
`/metrics` 的延迟和内存情况逐步调大。

## API 概览

服务端点：

- `GET /health`
- `GET /ready`
- `GET /metrics`
- `POST /ocr`
- `POST /ocr/file`
- `POST /det`
- `POST /det/file`
- `POST /slide_comparison`
- `POST /slide-comparison`
- `POST /slide_match`
- `POST /slide-match`

OCR 请求示例：

```json
{
  "image": "base64-encoded-image",
  "charset_range": "0123456789abcdefghijklmnopqrstuvwxyz",
  "confidence": true,
  "probability": false
}
```

识别响应保留 ddddocr 风格的 `result` 字段：

```json
{
  "result": "3n3d",
  "confidence": 0.97
}
```

检测接口需要用 `-det` 启用。滑块比较和滑块匹配是纯 Go 实现，不需要额外
ONNX 模型。

## Go 客户端

推荐业务系统先通过 HTTP 调用服务，这样 cgo 和 ONNX Runtime 不会进入业务
进程：

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

## ONNX Runtime

Go 代码本身支持 Windows、macOS 和 Linux。真正跟平台绑定的是 ONNX Runtime
动态库。加载顺序如下：

1. `Config.SharedLibraryPath` 或 `-onnxruntime-lib`
2. `ONNXRUNTIME_SHARED_LIBRARY_PATH`
3. `ONNXRUNTIME_HOME`
4. `third_party/onnxruntime/<GOOS>_<GOARCH>/`
5. 可用时使用内嵌的 darwin/arm64 runtime
6. 系统动态库路径

安装当前平台的 runtime：

```bash
go run ./cmd/ortfetch
```

也可以安装指定目标平台的 runtime：

```bash
go run ./cmd/ortfetch -goos linux -goarch amd64
go run ./cmd/ortfetch -goos linux -goarch arm64
go run ./cmd/ortfetch -goos windows -goarch amd64
go run ./cmd/ortfetch -goos windows -goarch arm64
go run ./cmd/ortfetch -goos darwin -goarch amd64
go run ./cmd/ortfetch -goos darwin -goarch arm64
```

`cmd/ortfetch` 会按目标平台选择默认版本：`darwin/amd64` 下载 ONNX Runtime
`1.23.2`，其他随包目标下载 `1.25.0`。

## CI 与自动发布

普通 CI 会运行测试、构建命令、安装当前平台 ONNX Runtime，并执行 smoke：

```bash
scripts/ci_linux.sh
```

本地打包当前平台：

```bash
GODDDDOCR_VERSION=v1.0.0 make package-release
```

本地打包全部平台的 ONNX Runtime：

```bash
GODDDDOCR_VERSION=v1.0.0 make package-onnxruntime
```

推送 `v1.x.x` tag 后，GitHub Actions 会自动构建 Linux `amd64/arm64`、
macOS `amd64/arm64`、Windows `amd64/arm64` 发行包，同时生成全量 ONNX
Runtime 包和 `SHA256SUMS`，并发布到 GitHub Release：

```bash
git tag v1.0.0
git push origin v1.0.0
```

也可以手动触发：

```bash
gh workflow run release.yml -f version=v1.0.0
```

## Docker

```bash
docker compose up --build
```

容器暴露 `8088`，并使用 `/ready` 做健康检查。完整 Docker 自检：

```bash
scripts/docker_smoke.sh
```

## 许可证

goddddocr 使用 MIT License 发布。来自 ddddocr 的模型和字符集资产在
[NOTICE](NOTICE) 中说明；重新分发源码或二进制包时请保留该声明。
