#Requires -Version 5.1
<#
  Cria uma VM Ubuntu na Azure (brazilsouth, B1s), copia o projeto e sobe docker compose.
  Requer: Docker Desktop (usa a imagem azure-cli) e OpenSSH (scp/ssh).

  Uso:
    .\deploy\azure-vm.ps1
#>
$ErrorActionPreference = "Stop"
Set-Location (Split-Path $PSScriptRoot -Parent)

$AzImage = "mcr.microsoft.com/azure-cli:2.67.0"
$AzDir = Join-Path $env:USERPROFILE ".azure"
New-Item -ItemType Directory -Force -Path $AzDir | Out-Null

function Invoke-Az {
    param([Parameter(ValueFromRemainingArguments = $true)][string[]]$AzArgs)
    $volWork = "${PWD}:/work"
    $volAz = "${AzDir}:/root/.azure"
    docker run --rm -v $volWork -v $volAz -w /work $AzImage az @AzArgs
    if ($LASTEXITCODE -ne 0) { throw "az $($AzArgs -join ' ') falhou (exit $LASTEXITCODE)" }
}

Write-Host "Login Azure (abra o URL e cole o codigo)..."
Invoke-Az login --use-device-code

$rg = "rg-urlshortener"
$loc = "brazilsouth"
$vm = "vm-urlshortener"
$dns = "urlshort-" + (-join ((1..6) | ForEach-Object { Get-Random -InputObject ([char[]]"abcdefghijklmnopqrstuvwxyz0123456789") }))

Write-Host "Resource group $rg ($loc)"
Invoke-Az group create --name $rg --location $loc -o table

Write-Host "VM $vm (Standard_B1s) — isso leva alguns minutos"
Invoke-Az vm create `
    --resource-group $rg `
    --name $vm `
    --image Ubuntu2204 `
    --size Standard_B1s `
    --admin-username azureuser `
    --generate-ssh-keys `
    --public-ip-sku Standard `
    --nsg-rule SSH `
    --custom-data deploy/cloud-init.yml `
    --public-ip-address-dns-name $dns `
    -o json | Out-File -Encoding utf8 (Join-Path $PSScriptRoot "last-vm.json")

Invoke-Az vm open-port --resource-group $rg --name $vm --port 80 --priority 1001

$ip = (Invoke-Az vm show -d --resource-group $rg --name $vm --query publicIps -o tsv).Trim()
if (-not $ip) { throw "Nao foi possivel obter o IP publico" }
Write-Host "IP publico: $ip"

$pw = -join ((1..32) | ForEach-Object { Get-Random -InputObject ([char[]]"abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789") })
$jwt = -join ((1..48) | ForEach-Object { Get-Random -InputObject ([char[]]"abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789") })
$salt = -join ((1..32) | ForEach-Object { Get-Random -InputObject ([char[]]"abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789") })

$envBody = @"
POSTGRES_PASSWORD=$pw
JWT_SECRET=$jwt
IP_HASH_SALT=$salt
PUBLIC_BASE_URL=http://$ip
"@
Set-Content -Path ".env.prod" -Value $envBody -NoNewline
Write-Host "Segredos gravados em .env.prod (nao commitar)"

$key = Join-Path $env:USERPROFILE ".ssh\id_rsa"
if (-not (Test-Path $key)) { $key = Join-Path $env:USERPROFILE ".ssh\id_ed25519" }
$ssh = @("-o", "StrictHostKeyChecking=accept-new", "azureuser@$ip")

Write-Host "Aguardando cloud-init/docker na VM..."
$ready = $false
for ($i = 0; $i -lt 36; $i++) {
    ssh @ssh "command -v docker >/dev/null && docker compose version >/dev/null && echo OK"
    if ($LASTEXITCODE -eq 0) { $ready = $true; break }
    Start-Sleep -Seconds 10
}
if (-not $ready) { Write-Warning "Docker ainda nao respondeu; tente ssh azureuser@$ip e rode o compose manualmente." }

Write-Host "Enviando arquivos..."
ssh @ssh "mkdir -p /opt/urlshortener"
# tar via ssh evita scp arquivo-a-arquivo
$exclude = "--exclude=.git --exclude=.env --exclude=.env.prod --exclude=bin"
cmd /c "tar $exclude -cf - . | ssh -o StrictHostKeyChecking=accept-new azureuser@$ip `"tar -xf - -C /opt/urlshortener`""
scp -o StrictHostKeyChecking=accept-new .env.prod "azureuser@${ip}:/opt/urlshortener/.env"

Write-Host "Subindo compose..."
ssh @ssh "cd /opt/urlshortener && sudo docker compose -f docker-compose.prod.yml --env-file .env up --build -d"

Write-Host ""
Write-Host "Pronto: http://$ip"
Write-Host "Health:  curl http://$ip/health"
Write-Host "Quando tiver dominio na Cloudflare: registro A -> $ip (proxy laranja), SSL Flexible, e atualize PUBLIC_BASE_URL para https://seu.dominio"
Write-Host "Destruir: docker run --rm -v ${AzDir}:/root/.azure $AzImage az group delete --name $rg --yes"
