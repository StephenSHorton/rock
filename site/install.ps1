# Install the current Rock 1.0 build.
# Usage: irm https://stephenshorton.github.io/rock/install.ps1 | iex
$ErrorActionPreference = "Stop"

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
  Write-Error "Rock needs Go 1.27 or newer: https://go.dev/dl/"
  exit 1
}

$ref = "cursor/rock-1-0-a11b"
# Used only when the GitHub API does not answer. Move this when 1.0 moves.
$fallback = "09b1e7a060600951c2daf7b745ca61a0488ce877"
Write-Host "Finding the current 1.0 commit..."
$sha = $null
foreach ($try in 1..3) {
  try {
    $commit = Invoke-RestMethod -Headers @{ Accept = "application/vnd.github+json" } `
      -Uri "https://api.github.com/repos/StephenSHorton/rock/commits/$ref"
    $sha = $commit.sha
    if ($sha) { break }
  } catch {
    Start-Sleep -Seconds 1
  }
}
if (-not $sha) {
  Write-Host "GitHub did not answer. Installing the last known 1.0 commit."
  $sha = $fallback
}

Write-Host "Installing rock $sha..."
go install "github.com/StephenSHorton/rock/cmd/rock@$sha"

$gopath = (go env GOPATH)
$bin = Join-Path $gopath "bin"
Write-Host "Installed $bin\rock.exe"
if ($env:PATH -notlike "*${bin}*") {
  Write-Host "Add $bin to PATH, then run: rock"
} else {
  Write-Host "Run: rock"
}
