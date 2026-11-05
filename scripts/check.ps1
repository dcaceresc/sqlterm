param([switch]$Integration)
$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
$env:GOCACHE = Join-Path $projectRoot '.cache/build'
$env:GOPATH = Join-Path $projectRoot '.cache/go'
$env:GOMODCACHE = Join-Path $projectRoot '.cache/mod'
Push-Location $projectRoot
try {
    go test ./...
    if ($LASTEXITCODE -ne 0) { throw 'Tests failed' }
    go vet ./...
    if ($LASTEXITCODE -ne 0) { throw 'go vet failed' }
    if ($Integration) {
        go test ./internal/db -run TestIntegration -v -count=1
        if ($LASTEXITCODE -ne 0) { throw 'Integration tests failed' }
    }
} finally { Pop-Location }
