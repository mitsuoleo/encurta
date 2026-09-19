param(
    [Parameter(Position = 0)]
    [string]$Target = "up"
)

$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

switch ($Target) {
    "up" {
        docker compose up --build -d
        docker compose ps
        Write-Host "App: http://localhost:8080"
    }
    "down" { docker compose down }
    "logs" { docker compose logs -f api }
    "test" { docker run --rm -v "${PWD}:/src" -w /src golang:1.23-bookworm go test ./... }
    "lint" { docker run --rm -v "${PWD}:/src" -w /src golangci/golangci-lint:v1.64.8 golangci-lint run ./... }
    "smoke" { & "$PSScriptRoot\scripts\smoke.ps1" }
    default {
        Write-Host "Usage: .\make.ps1 [up|down|logs|test|lint|smoke]"
        exit 1
    }
}
