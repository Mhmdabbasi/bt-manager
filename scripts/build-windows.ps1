$ErrorActionPreference = 'Stop'
$Root = Split-Path -Parent $PSScriptRoot
Push-Location (Join-Path $Root 'web')
try { npm install; npm run typecheck; npm run build } finally { Pop-Location }
Push-Location $Root
try {
  go mod tidy
  go test ./...
  $env:CGO_ENABLED = '0'
  New-Item -ItemType Directory -Force -Path (Join-Path $Root 'dist') | Out-Null
  go build -trimpath -ldflags '-s -w' -o (Join-Path $Root 'dist\bt-manager.exe') ./cmd/bt-manager
  Write-Host 'Portable Windows executable created at dist\bt-manager.exe'
  Write-Host 'Windows installer / Wails shell is a subsequent milestone.'
} finally { Pop-Location }
