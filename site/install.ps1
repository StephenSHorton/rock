# Install rock from the main branch.
# Usage: irm https://stephenshorton.github.io/rock/install.ps1 | iex
$ErrorActionPreference = "Stop"

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
  Write-Error "Rock needs Go 1.27 or newer: https://go.dev/dl/"
  exit 1
}

Write-Host "Installing rock from main..."
go install github.com/StephenSHorton/rock/cmd/rock@main

$gopath = (go env GOPATH)
$bin = Join-Path $gopath "bin"
Write-Host "Installed $bin\rock.exe"
if ($env:PATH -notlike "*${bin}*") {
  Write-Host "Add $bin to PATH, then run: rock"
} else {
  Write-Host "Run: rock"
}
