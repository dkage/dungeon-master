[CmdletBinding()]
param([switch]$BuildOnly, [Parameter(ValueFromRemainingArguments = $true)][string[]]$ServeArgs)

$ErrorActionPreference = 'Stop'
$previousLocation = Get-Location
$previousCGO = $env:CGO_ENABLED
try {
    Set-Location $PSScriptRoot
    foreach ($tool in @('go', 'npm')) {
        if (-not (Get-Command $tool -ErrorAction SilentlyContinue)) {
            throw "$tool is missing from PATH. Install the README prerequisites and reopen your terminal."
        }
    }
    Push-Location (Join-Path $PSScriptRoot 'web')
    try {
        & npm ci
        if ($LASTEXITCODE -ne 0) { throw 'npm ci failed' }
        & npm run build
        if ($LASTEXITCODE -ne 0) { throw 'Frontend build failed' }
    } finally { Pop-Location }
    # Keep the tracked placeholder after Vite clears its output directory.
    if (-not (Test-Path 'web/dist/.gitkeep')) { New-Item -ItemType File 'web/dist/.gitkeep' | Out-Null }
    $env:CGO_ENABLED = '0'
    & go build -trimpath -o bin/dungeon.exe ./cmd/dungeon
    if ($LASTEXITCODE -ne 0) { throw 'Go build failed. Stop any running dungeon.exe before rebuilding.' }
    if (-not $BuildOnly) {
        & (Join-Path $PSScriptRoot 'bin/dungeon.exe') serve @ServeArgs
        if ($LASTEXITCODE -ne 0) { throw 'Dungeon Master exited with an error' }
    }
} finally {
    $env:CGO_ENABLED = $previousCGO
    Set-Location $previousLocation
}
