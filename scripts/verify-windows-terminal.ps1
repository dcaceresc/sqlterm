$ErrorActionPreference = 'Stop'

if (-not $env:WT_SESSION) {
    throw 'Run from a PowerShell tab in Windows Terminal, not from CI or a redirected session.'
}

Write-Host "In another app, manually copy this exact sample (without a trailing newline):"
Write-Host 'i:quit ggVG SELECT 1;'
Write-Host 'The diagnostic does not access or change the clipboard directly.'
Write-Host 'Return to this pane; press Enter to start, then follow the on-screen instructions.'
$null = Read-Host

# Los cambios de entorno son solo para este proceso y se restauran al salir.
$oldDiagnostic = $env:SQLTERM_WINDOWS_TERMINAL_DIAGNOSTIC
$oldProxy = $env:GOPROXY
$oldToolchain = $env:GOTOOLCHAIN
Push-Location (Join-Path $PSScriptRoot '..')
try {
    $env:SQLTERM_WINDOWS_TERMINAL_DIAGNOSTIC = '1'
    $env:GOPROXY = 'off'
    $env:GOTOOLCHAIN = 'local'
    & go test -mod=readonly ./internal/terminal -run '^TestWindowsConsoleDiagnostic$' -count=1 -short=false -timeout=0 -v
    $result = $LASTEXITCODE
} finally {
    $env:SQLTERM_WINDOWS_TERMINAL_DIAGNOSTIC = $oldDiagnostic
    $env:GOPROXY = $oldProxy
    $env:GOTOOLCHAIN = $oldToolchain
    Pop-Location
}
exit $result
