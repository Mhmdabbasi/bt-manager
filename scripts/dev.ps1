$ErrorActionPreference = 'Stop'
$Root = Split-Path -Parent $PSScriptRoot
Write-Host 'Starting bt-manager API on 127.0.0.1:8080...'
$Server = Start-Process -PassThru -NoNewWindow -FilePath 'go' -ArgumentList @('run','./cmd/bt-manager','-listen','127.0.0.1:8080') -WorkingDirectory $Root
try { Push-Location (Join-Path $Root 'web'); npm run dev }
finally { Pop-Location; if ($Server -and -not $Server.HasExited) { Stop-Process -Id $Server.Id -Force } }
