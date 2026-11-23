<#
.SYNOPSIS
  Builds Indie Studio and produces a portable zip and (optionally) an NSIS installer.

.DESCRIPTION
  Requires the Wails CLI on PATH and, for -Nsis, NSIS (makensis).
  The NSIS template in build/windows/installer detects and installs WebView2
  when it is missing.

.EXAMPLE
  powershell -ExecutionPolicy Bypass -File scripts/package.ps1 -Version 0.9.0 -Nsis
#>
param(
  [string]$Version = "",
  [switch]$Nsis,
  [switch]$SkipFrontend
)

$ErrorActionPreference = "Stop"
$root = Resolve-Path (Join-Path $PSScriptRoot "..")
Push-Location $root

try {
  $buildArgs = @("build", "-clean")
  if ($Version) {
    $buildArgs += @("-ldflags", "-X main.Version=$Version")
  }
  if ($SkipFrontend) {
    $buildArgs += "-s"
  }
  Write-Host "> wails $($buildArgs -join ' ')" -ForegroundColor Cyan
  & wails @buildArgs
  if ($LASTEXITCODE -ne 0) { throw "wails build failed" }

  $exe = Join-Path $root "build\bin\indie-studio.exe"
  if (-not (Test-Path $exe)) { throw "missing $exe" }

  $label = if ($Version) { $Version } else { "dev" }
  $zip = Join-Path $root "build\bin\indie-studio-$label-portable.zip"
  if (Test-Path $zip) { Remove-Item $zip -Force }
  Compress-Archive -Path @($exe, (Join-Path $root "README.md")) -DestinationPath $zip -CompressionLevel Optimal
  Write-Host "portable: $zip" -ForegroundColor Green

  if ($Nsis) {
    $nsisExe = Join-Path $root "build\bin\indie-studio-amd64-installer.exe"
    if (-not (Get-Command makensis -ErrorAction SilentlyContinue)) {
      $nsisDir = "C:\Program Files (x86)\NSIS"
      if (Test-Path (Join-Path $nsisDir "makensis.exe")) {
        $env:PATH = "$nsisDir;$env:PATH"
        Write-Host "found NSIS at $nsisDir" -ForegroundColor DarkGray
      }
    }
    Write-Host "> wails build -nsis" -ForegroundColor Cyan
    $nsisArgs = @("build", "-nsis")
    if ($Version) { $nsisArgs += @("-ldflags", "-X main.Version=$Version") }
    if ($SkipFrontend) { $nsisArgs += "-s" }
    & wails @nsisArgs
    if ($LASTEXITCODE -ne 0) { throw "wails -nsis failed (is NSIS installed and on PATH?)" }
    if (-not (Test-Path $nsisExe)) { throw "missing $nsisExe" }
    Write-Host "installer: $nsisExe" -ForegroundColor Green
    Write-Host "sign a manifest with: go run ./cmd/pbmanifest manifest -key ... -file `"$nsisExe`" ..." -ForegroundColor Yellow
  }
} finally {
  Pop-Location
}