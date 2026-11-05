$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
$env:GOCACHE = Join-Path $projectRoot '.cache/build'
$env:GOPATH = Join-Path $projectRoot '.cache/go'
$env:GOMODCACHE = Join-Path $projectRoot '.cache/mod'
$previousGOOS = $env:GOOS
$previousGOARCH = $env:GOARCH
$previousCGO = $env:CGO_ENABLED
Push-Location $projectRoot
try {
    New-Item -ItemType Directory -Path dist -Force | Out-Null
    $env:CGO_ENABLED = '0'
    foreach ($targetOS in @('windows', 'linux', 'darwin')) {
        foreach ($targetArch in @('amd64', 'arm64')) {
            $env:GOOS = $targetOS
            $env:GOARCH = $targetArch
            $suffix = if ($targetOS -eq 'windows') { '.exe' } else { '' }
            $outputPath = "dist/sqlterm-$targetOS-$targetArch$suffix"
            go build -trimpath -o $outputPath ./cmd/sqlterm
            if ($LASTEXITCODE -ne 0) { throw "Build failed: $targetOS/$targetArch" }
            Write-Output "Built $outputPath"
        }
    }
} finally {
    $env:GOOS = $previousGOOS
    $env:GOARCH = $previousGOARCH
    $env:CGO_ENABLED = $previousCGO
    Pop-Location
}
