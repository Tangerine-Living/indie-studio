# Publishes one signed Indie Studio update to the public updates repository.
#
#   $env:GH_TOKEN = "<token>"      # never committed, never printed
#   .\scripts\publish-update.ps1 -Version 1.0.0 -Channel stable `
#     -Installer build\bin\pixelbench-amd64-installer.exe -Notes "首个稳定版"
#
# The signing key defaults to ..\pixelbench-release-keys\private.b64, outside
# this repository. The matching public key is committed at release\public.b64
# and embedded into the app, so the app can verify manifests without a URL.

param(
  [Parameter(Mandatory = $true)][string]$Version,
  [ValidateSet('stable', 'dev')][string]$Channel = 'stable',
  [Parameter(Mandatory = $true)][string]$Installer,
  [string]$Notes = '',
  [string]$Repo = 'Tangerine-Living/indie-studio',
  [string]$KeyFile = ''
)

$ErrorActionPreference = 'Stop'
$root = Split-Path $PSScriptRoot -Parent
$token = $env:GH_TOKEN
if (-not $token) {
  throw 'Set GH_TOKEN to a GitHub token with repo write access.'
}
$Version = $Version.TrimStart('v')
if (-not (Test-Path -LiteralPath $Installer)) {
  throw "Installer not found: $Installer"
}
$installerPath = (Resolve-Path -LiteralPath $Installer).Path
if (-not $KeyFile) {
  $KeyFile = Join-Path (Split-Path $root -Parent) 'pixelbench-release-keys\private.b64'
}
if (-not (Test-Path -LiteralPath $KeyFile)) {
  throw "Signing key not found: $KeyFile (run: go run .\cmd\pbmanifest keygen -out <dir>)"
}

$headers = @{
  Authorization        = "Bearer $token"
  Accept               = 'application/vnd.github+json'
  'X-GitHub-Api-Version' = '2022-11-28'
}
$api = 'https://api.github.com'

# 1. Create the release that will host the installer asset.
$tag = if ($Channel -eq 'dev') { "dev-v$Version" } else { "v$Version" }
$releaseBody = @{
  tag_name         = $tag
  name             = "Indie Studio $Version ($Channel)"
  body             = $Notes
  draft            = $false
  prerelease       = $Channel -eq 'dev'
  make_latest      = if ($Channel -eq 'stable') { 'true' } else { 'false' }
} | ConvertTo-Json
$release = Invoke-RestMethod -Method Post -Uri "$api/repos/$Repo/releases" -Headers $headers `
  -ContentType 'application/json' -Body $releaseBody
Write-Host "release: $($release.html_url)"

# 2. Upload the Windows installer as a release asset.
$assetName = Split-Path $installerPath -Leaf
$uploadUri = "https://uploads.github.com/repos/$Repo/releases/$($release.id)/assets?name=$([uri]::EscapeDataString($assetName))"
$asset = Invoke-RestMethod -Method Post -Uri $uploadUri -Headers $headers `
  -ContentType 'application/octet-stream' -InFile $installerPath
$downloadUrl = $asset.browser_download_url
Write-Host "asset: $downloadUrl"

# 3. Sign a manifest that points at that asset.
$outDir = Join-Path ([IO.Path]::GetTempPath()) ("pbmanifest-" + [guid]::NewGuid())
New-Item -ItemType Directory -Path $outDir | Out-Null
$manifestPath = Join-Path $outDir 'latest.json'
Push-Location $root
try {
  & go run .\cmd\pbmanifest manifest -key $KeyFile -version $Version -channel $Channel `
    -file $installerPath -url $downloadUrl -page $release.html_url -notes $Notes -out $manifestPath
  if ($LASTEXITCODE -ne 0) { throw 'pbmanifest failed' }
} finally {
  Pop-Location
}

# 4. Commit the manifest and its signature to <channel>/latest.json on main.
function Put-RepoFile([string]$path, [string]$file) {
  $uri = "$api/repos/$Repo/contents/$path"
  $existing = $null
  try {
    $existing = Invoke-RestMethod -Method Get -Uri $uri -Headers $headers
  } catch {
    if ($_.Exception.Response.StatusCode.value__ -ne 404) { throw }
  }
  $body = @{
    message = "publish $Channel $Version"
    content = [Convert]::ToBase64String([IO.File]::ReadAllBytes($file))
  }
  if ($existing -and $existing.sha) { $body.sha = $existing.sha }
  Invoke-RestMethod -Method Put -Uri $uri -Headers $headers -ContentType 'application/json' `
    -Body ($body | ConvertTo-Json) | Out-Null
  Write-Host "committed: $path"
}

Put-RepoFile "$Channel/latest.json" $manifestPath
Put-RepoFile "$Channel/latest.json.sig" "$manifestPath.sig"
Remove-Item -LiteralPath $outDir -Recurse -Force

Write-Host "done: $Channel $Version"