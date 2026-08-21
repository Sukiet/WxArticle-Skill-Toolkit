$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $MyInvocation.MyCommand.Path
$outputDir = Join-Path $root "debug"
$binaryPath = Join-Path $outputDir "skill-tool.exe"

New-Item -ItemType Directory -Path $outputDir -Force | Out-Null

$env:CGO_ENABLED = "0"

go build -trimpath -ldflags="-s -w" -o $binaryPath .
if ($LASTEXITCODE -ne 0) {
    throw "go build failed with exit code $LASTEXITCODE"
}

Write-Output "Built: $binaryPath"
