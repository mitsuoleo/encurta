# Smoke contra o Compose local (API em :8080).
# Uso: .\make.ps1 up ; .\scripts\smoke.ps1
$ErrorActionPreference = "Stop"
$base = if ($env:BASE_URL) { $env:BASE_URL } else { "http://localhost:8080" }

$health = curl.exe -sS "$base/health"
if ($health -notmatch '"app":"ok"') { throw "health falhou: $health" }

$email = "smoke+$([guid]::NewGuid().ToString('N').Substring(0,8))@example.com"
$regFile = Join-Path $env:TEMP "smoke-reg.json"
Set-Content -Path $regFile -Value (@{ email = $email; password = "password1" } | ConvertTo-Json -Compress) -NoNewline
$reg = curl.exe -sS -X POST "$base/auth/register" -H "Content-Type: application/json" --data-binary "@$regFile"
Remove-Item $regFile
$token = ($reg | ConvertFrom-Json).access_token
if (-not $token) { throw "register falhou: $reg" }

$alias = "meu-link-" + (Get-Random -Maximum 9999)
$linkFile = Join-Path $env:TEMP "smoke-link.json"
Set-Content -Path $linkFile -Value (@{ url = "https://example.com/smoke"; alias = $alias } | ConvertTo-Json -Compress) -NoNewline
$created = curl.exe -sS -X POST "$base/links" -H "Content-Type: application/json" -H "Authorization: Bearer $token" --data-binary "@$linkFile"
Remove-Item $linkFile
$code = ($created | ConvertFrom-Json).short_code
if ($code -ne $alias) { throw "alias inesperado: $created" }

$code = [uri]::EscapeDataString($code)
$redir = curl.exe -sS -D - -o NUL "$base/$code" | Out-String
if ($redir -notmatch "302 Found") { throw "redirect deveria ser 302:`n$redir" }

Start-Sleep -Seconds 1
$stats = curl.exe -sS "$base/links/$code/analytics" -H "Authorization: Bearer $token"
if ($stats -notmatch "total_clicks") { throw "analytics falhou: $stats" }

curl.exe -sS -o NUL -D - -X POST "$base/auth/logout" -H "Authorization: Bearer $token" | Out-Null

Write-Host "smoke ok ($base alias=$alias)"
