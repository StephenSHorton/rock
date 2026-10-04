# Install the latest Rock release for Windows amd64.
# Usage: irm https://stephenshorton.github.io/rock/install.ps1 | iex
$ErrorActionPreference = "Stop"
$Repo = "StephenSHorton/rock"
$Module = "github.com/StephenSHorton/rock/cmd/rock"
$Api = if ($env:ROCK_UPDATE_API) { $env:ROCK_UPDATE_API.TrimEnd("/") } else { "https://api.github.com" }
$Download = if ($env:ROCK_UPDATE_DOWNLOAD) { $env:ROCK_UPDATE_DOWNLOAD.TrimEnd("/") } else { "https://github.com/$Repo/releases/download" }

function Install-FromGo {
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        Write-Error "No matching Rock release and Go is not installed. Install Go 1.27+: https://go.dev/dl/"
        exit 1
    }
    Write-Host "Falling back to go install..."
    try {
        go install "$Module@latest"
    } catch {
        go install "$Module@main"
    }
    $gopath = (go env GOPATH)
    $bin = Join-Path $gopath "bin"
    Write-Host "Installed $bin\rock.exe"
    Write-Host "This tree is a go install. Later: go install $Module@latest"
}

function Go-Fallback([string]$reason) {
    Write-Host $reason
    Install-FromGo
    exit 0
}

$arch = "amd64"
if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") {
    Go-Fallback "No Windows arm64 release asset."
}

$headers = @{
    Accept = "application/vnd.github+json"
    "User-Agent" = "rock-install"
}
try {
    $rel = Invoke-RestMethod -Headers $headers -Uri "$Api/repos/$Repo/releases/latest"
} catch {
    Go-Fallback "Could not read the latest GitHub release."
}

$tag = [string]$rel.tag_name
$ver = $tag.TrimStart("v")
if (-not $tag -or -not $ver) {
    Go-Fallback "GitHub release has no tag."
}

$name = "rock_${ver}_windows_${arch}.zip"
$dir = Join-Path ([System.IO.Path]::GetTempPath()) ("rock-install-" + [guid]::NewGuid().ToString())
New-Item -ItemType Directory -Path $dir | Out-Null
try {
    $zip = Join-Path $dir $name
    $sums = Join-Path $dir "checksums.txt"
    try {
        Invoke-WebRequest -Uri "$Download/$tag/$name" -OutFile $zip -UseBasicParsing
        Invoke-WebRequest -Uri "$Download/$tag/checksums.txt" -OutFile $sums -UseBasicParsing
    } catch {
        Go-Fallback "No asset $name on $tag."
    }
    $want = $null
    Get-Content $sums | ForEach-Object {
        $parts = $_ -split "\s+"
        if ($parts.Count -ge 2 -and $parts[-1].TrimStart("*") -eq $name) {
            $want = $parts[0]
        }
    }
    if (-not $want) {
        Go-Fallback "$name is missing from checksums.txt."
    }
    $got = (Get-FileHash -Algorithm SHA256 $zip).Hash
    if ($got.ToLower() -ne $want.ToLower()) {
        Write-Error "checksum mismatch for $name"
        exit 1
    }
    Expand-Archive -Path $zip -DestinationPath $dir -Force
    $bin = Join-Path $dir "rock.exe"
    if (-not (Test-Path $bin)) {
        Write-Error "archive has no rock.exe"
        exit 1
    }
    if ($env:ROCK_INSTALL_DIR) {
        $dest = $env:ROCK_INSTALL_DIR
    } else {
        $dest = Join-Path $env:LOCALAPPDATA "Programs\rock"
    }
    New-Item -ItemType Directory -Path $dest -Force | Out-Null
    Copy-Item -Force $bin (Join-Path $dest "rock.exe")
    Write-Host "Installed $dest\rock.exe ($tag)"
    if ($env:PATH -notlike "*${dest}*") {
        Write-Host "Add $dest to PATH, then run: rock"
    } else {
        Write-Host "Run: rock   later: rock update"
    }
} finally {
    Remove-Item -Recurse -Force $dir -ErrorAction SilentlyContinue
}
