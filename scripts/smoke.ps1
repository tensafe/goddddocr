[CmdletBinding()]
param(
    [string]$Image = $env:GODDDDOCR_SMOKE_IMAGE,
    [string]$Expect = $(if ($env:GODDDDOCR_SMOKE_EXPECT) { $env:GODDDDOCR_SMOKE_EXPECT } else { "3n3d" }),
    [switch]$NoJson,
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$ExtraArgs
)

$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$rootDir = (Resolve-Path (Join-Path $scriptDir "..")).Path

if ([string]::IsNullOrWhiteSpace($Image)) {
    $Image = Join-Path $rootDir "samples/yzm1.png"
}

$doctorArgs = @("-image", $Image)
if (-not [string]::IsNullOrWhiteSpace($Expect)) {
    $doctorArgs += @("-expect", $Expect)
}
if (-not $NoJson) {
    $doctorArgs += "-json"
}
if ($ExtraArgs) {
    $doctorArgs += $ExtraArgs
}

function Invoke-Doctor {
    param([string]$CommandPath)

    & $CommandPath @doctorArgs
    exit $LASTEXITCODE
}

if (-not [string]::IsNullOrWhiteSpace($env:GODDDDOCR_DOCTOR_BIN)) {
    Invoke-Doctor $env:GODDDDOCR_DOCTOR_BIN
}

$localExe = Join-Path $rootDir "ocrdoctor.exe"
if (Test-Path $localExe) {
    Invoke-Doctor $localExe
}

$localBinary = Join-Path $rootDir "ocrdoctor"
if (Test-Path $localBinary) {
    Invoke-Doctor $localBinary
}

$pathCommand = Get-Command "ocrdoctor" -ErrorAction SilentlyContinue
if ($pathCommand) {
    Invoke-Doctor $pathCommand.Source
}

$goCommand = Get-Command "go" -ErrorAction SilentlyContinue
if ($goCommand -and (Test-Path (Join-Path $rootDir "cmd/ocrdoctor"))) {
    Push-Location $rootDir
    try {
        & $goCommand.Source run ./cmd/ocrdoctor @doctorArgs
        exit $LASTEXITCODE
    } finally {
        Pop-Location
    }
}

Write-Error "ocrdoctor not found. Set GODDDDOCR_DOCTOR_BIN or run from the source tree with Go installed."
exit 127
