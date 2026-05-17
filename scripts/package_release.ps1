[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$Version,
    [string]$Goos = $(go env GOOS),
    [string]$Goarch = $(go env GOARCH),
    [string]$OutDir = $env:GODDDDOCR_RELEASE_OUT
)

$ErrorActionPreference = "Stop"

if ($Version -notmatch '^v1\.\d+\.\d+$') {
    throw "release version must match v1.x.x, got: $Version"
}

$target = "$Goos/$Goarch"
$supportedTargets = @(
    "windows/amd64",
    "windows/arm64"
)
if ($supportedTargets -notcontains $target) {
    throw "unsupported release target: $target"
}

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$rootDir = (Resolve-Path (Join-Path $scriptDir "..")).Path
if ([string]::IsNullOrWhiteSpace($OutDir)) {
    $OutDir = Join-Path $rootDir "dist"
}

$packageName = "goddddocr-$Version-$Goos-$Goarch"
$packageDir = Join-Path $OutDir $packageName
$archivePath = Join-Path $OutDir "$packageName.zip"

Remove-Item -Recurse -Force $packageDir -ErrorAction SilentlyContinue
Remove-Item -Force $archivePath -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path $packageDir | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $packageDir "scripts") | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $packageDir "samples") | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $packageDir "docs") | Out-Null

$previousCgoEnabled = $env:CGO_ENABLED
$previousGoos = $env:GOOS
$previousGoarch = $env:GOARCH

Push-Location $rootDir
try {
    $env:CGO_ENABLED = $(if ($env:CGO_ENABLED) { $env:CGO_ENABLED } else { "1" })
    $env:GOOS = $Goos
    $env:GOARCH = $Goarch

    $commands = @("goddddocr-server", "ocrdoctor", "ocrbench", "ocrprep", "ocreval", "ortfetch")
    $ldflags = "-s -w -extldflags=-static"
    foreach ($command in $commands) {
        $output = Join-Path $packageDir "$command.exe"
        & go build -trimpath -ldflags $ldflags -o $output "./cmd/$command"
    }

    if ($null -eq $previousGoos) {
        Remove-Item Env:GOOS -ErrorAction SilentlyContinue
    } else {
        $env:GOOS = $previousGoos
    }
    if ($null -eq $previousGoarch) {
        Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
    } else {
        $env:GOARCH = $previousGoarch
    }

    & go run ./cmd/ortfetch -goos $Goos -goarch $Goarch -out (Join-Path $packageDir "third_party/onnxruntime")

    $redistDir = Join-Path $packageDir "redist/windows"
    New-Item -ItemType Directory -Force -Path $redistDir | Out-Null
    if ($Goarch -eq "amd64") {
        Invoke-WebRequest -Uri "https://aka.ms/vc14/vc_redist.x64.exe" -OutFile (Join-Path $redistDir "vc_redist.x64.exe")
    } elseif ($Goarch -eq "arm64") {
        Invoke-WebRequest -Uri "https://aka.ms/vc14/vc_redist.arm64.exe" -OutFile (Join-Path $redistDir "vc_redist.arm64.exe")
    }

    Copy-Item -Path README.md, README.zh-CN.md, LICENSE, NOTICE -Destination $packageDir
    Copy-Item -Recurse -Path docs/zh-CN -Destination (Join-Path $packageDir "docs")
    Copy-Item -Path scripts/smoke.sh, scripts/smoke.ps1 -Destination (Join-Path $packageDir "scripts")
    Copy-Item -Path samples/yzm1.png, samples/yzm2.jpeg -Destination (Join-Path $packageDir "samples")

    if ($env:GODDDDOCR_SKIP_PACKAGE_SMOKE -ne "1") {
        & (Join-Path $packageDir "scripts/smoke.ps1")
    }

    Compress-Archive -Path $packageDir -DestinationPath $archivePath -Force
    Write-Output $archivePath
} finally {
    Pop-Location
    if ($null -eq $previousCgoEnabled) {
        Remove-Item Env:CGO_ENABLED -ErrorAction SilentlyContinue
    } else {
        $env:CGO_ENABLED = $previousCgoEnabled
    }
    if ($null -eq $previousGoos) {
        Remove-Item Env:GOOS -ErrorAction SilentlyContinue
    } else {
        $env:GOOS = $previousGoos
    }
    if ($null -eq $previousGoarch) {
        Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
    } else {
        $env:GOARCH = $previousGoarch
    }
}
