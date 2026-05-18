# 发布与多平台构建

本文档说明 goddddocr 的版本规则、GitHub 自动发布流程、本地打包方式，以及
不同操作系统/CPU 架构的构建支持。

## 版本规则

当前稳定发布线使用 `v1.x.x` tag，例如：

```bash
git tag v1.0.1
git push origin v1.0.1
```

`release.yml` 会拒绝不符合 `v1.x.x` 的版本号。这样可以避免误把临时 tag 或
实验版本发布成正式包。

## 自动发布流程

推送 `v1.x.x` tag 后，GitHub Actions 会执行：

1. 校验版本号。
2. 为每个目标平台构建服务端和辅助命令。
3. 下载并放入对应平台的 ONNX Runtime 动态库。
4. 打包 `README.md`、`README.zh-CN.md`、`docs/zh-CN`、`LICENSE`、`NOTICE`、
   一键运行脚本、smoke 脚本和样例图片。
5. 额外生成一个包含所有支持平台 ONNX Runtime 的汇总包。
6. 在可运行的目标平台上执行 smoke 检查。
7. 上传发行包并生成 `SHA256SUMS`。
8. 创建或更新 GitHub Release。

也可以手动触发：

```bash
gh workflow run release.yml -f version=v1.0.1
```

## 支持的平台

| 目标平台 | 运行器 | 包格式 | 说明 |
|---|---|---|---|
| `linux/amd64` | `ubuntu-24.04` | `.tar.gz` | 常见 x86_64 Linux 服务器 |
| `linux/arm64` | `ubuntu-24.04-arm` | `.tar.gz` | ARM64 Linux 服务器 |
| `darwin/amd64` | `macos-15-intel` | `.tar.gz` | Intel Mac，使用 ONNX Runtime 1.23.2 |
| `darwin/arm64` | `macos-15` | `.tar.gz` | Apple Silicon Mac |
| `windows/amd64` | `windows-2025` | `.zip` | 常见 64 位 Windows |
| `windows/arm64` | `windows-11-arm` | `.zip` | ARM64 Windows |

ONNX Runtime 动态库随发行包放在
`third_party/onnxruntime/<GOOS>_<GOARCH>/`，启动时会被自动发现。
Release 页面还会额外提供 `goddddocr-onnxruntime-v1.x.x.tar.gz`。这个包包含
全部支持平台的 ONNX Runtime 动态库、Windows Visual C++ Redistributable
安装器和文档；runtime 目录结构同样是
`third_party/onnxruntime/<GOOS>_<GOARCH>/`，适合离线环境统一下发。

默认 OCR 模型、检测模型和字符集已经通过 `go:embed` 内置到服务端和辅助命令的
Go 二进制中，发行包不需要额外携带 `assets/models/*.onnx`。只有使用自定义
模型时，才需要通过 `-model-path`、`-charset-path` 或 `-det-model-path`
提供外部文件。

版本策略：

- `darwin/amd64` 使用 ONNX Runtime `1.23.2`，因为官方 `1.25.0` 没有发布
  macOS amd64 CPU 归档。
- 其他随包目标使用 ONNX Runtime `1.25.0`。

## Windows 运行时依赖

Windows 发行包会包含 `onnxruntime.dll`。不过 Microsoft 官方 ONNX Runtime
DLL 本身还依赖 Visual C++ Runtime / UCRT。实测 `onnxruntime-win-x64-1.25.0`
会导入：

- `MSVCP140.dll`
- `MSVCP140_1.dll`
- `VCRUNTIME140.dll`
- `VCRUNTIME140_1.dll`
- 多个 `api-ms-win-crt-*` UCRT API-set DLL

因此 Windows 发行包会把对应的 Microsoft Visual C++ Redistributable 安装器
放在 `redist/windows/` 下。目标主机如果没有对应运行时，再自行安装即可，
架构要和 goddddocr 包一致：

- `windows/amd64`：运行 `redist/windows/vc_redist.x64.exe`。
- `windows/arm64`：运行 `redist/windows/vc_redist.arm64.exe`。

Go 二进制本身在 Release workflow 中使用 MSYS2/mingw-w64 编译，并通过
`-extldflags=-static` 尽量静态链接 MinGW 运行时，避免再额外依赖
`libgcc_s_*` 或 `libwinpthread-1.dll`。但这不能消除 `onnxruntime.dll` 对
Microsoft Visual C++ Redistributable 的依赖。

Microsoft 官方说明页：
https://learn.microsoft.com/cpp/windows/latest-supported-vc-redist

## 本地打包

为当前平台构建发行包：

```bash
GODDDDOCR_VERSION=v1.0.1 make package-release
```

直接调用脚本也可以：

```bash
scripts/package_release.sh v1.0.1
```

Windows PowerShell：

```powershell
.\scripts\package_release.ps1 -Version v1.0.1
```

只打包全部支持平台的 ONNX Runtime：

```bash
GODDDDOCR_VERSION=v1.0.1 make package-onnxruntime
```

默认输出目录是 `dist/`。可以通过 `GODDDDOCR_RELEASE_OUT` 修改：

```bash
GODDDDOCR_VERSION=v1.0.1 \
GODDDDOCR_RELEASE_OUT=/tmp/goddddocr-release \
make package-release
```

## 使用发行包

Linux/macOS 一键安装并启动最新版：

```bash
curl -fsSL https://raw.githubusercontent.com/tensafe/goddddocr/main/scripts/install_run.sh | sh
```

指定版本或追加服务参数：

```bash
curl -fsSL https://raw.githubusercontent.com/tensafe/goddddocr/main/scripts/install_run.sh \
  | GODDDDOCR_VERSION=v1.0.1 sh -s -- -addr :8088 -workers 2
```

Linux/macOS：

```bash
tar -xzf goddddocr-v1.0.1-linux-amd64.tar.gz
cd goddddocr-v1.0.1-linux-amd64
scripts/smoke.sh
./goddddocr-server -addr :8088
# 或：
scripts/run.sh -addr :8088
```

Windows：

```powershell
Expand-Archive .\goddddocr-v1.0.1-windows-amd64.zip
cd .\goddddocr-v1.0.1-windows-amd64\goddddocr-v1.0.1-windows-amd64
.\scripts\smoke.ps1
.\goddddocr-server.exe -addr :8088
```

## 自定义模型发布

如果部署时使用自定义 OCR 模型，不建议直接把私有模型提交到仓库。推荐把模型
作为独立部署资产管理，然后启动服务时指定：

```bash
./goddddocr-server \
  -model-path /opt/models/custom.onnx \
  -charset-path /opt/models/charset.json
```

发布前可先用 `ocrdoctor` 验证：

```bash
./ocrdoctor \
  -model-path /opt/models/custom.onnx \
  -charset-path /opt/models/charset.json \
  -image /opt/models/smoke.png \
  -expect abcd \
  -json
```

## 许可证与声明

项目代码使用 MIT License。发行包内会带上 `LICENSE` 和 `NOTICE`：

- `LICENSE`：goddddocr 的 MIT 许可证。
- `NOTICE`：说明内置模型和字符集资产来自 ddddocr，并保留上游项目信息。

发布二进制包或二次分发时，请保留这两个文件。
