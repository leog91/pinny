param([string]$BinutilsDirectory)

$ErrorActionPreference = 'Stop'
$repo = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$experiment = Join-Path $repo 'Experiments\Pinny.Rune'
$revision = (Get-Content -LiteralPath (Join-Path $experiment 'rune-revision.txt') -Raw).Trim()
$source = Join-Path $repo 'dist\rune-source'
if (-not (Test-Path -LiteralPath (Join-Path $source '.git'))) {
    New-Item -ItemType Directory -Path $source -Force | Out-Null
    & git -C $source init
    if ($LASTEXITCODE -ne 0) { throw 'Could not initialize Rune source checkout' }
    & git -C $source remote add origin https://github.com/anomalyco/rune
}
if ((& git -C $source rev-parse HEAD 2>$null) -ne $revision) {
    & git -C $source fetch --depth 1 origin $revision
    if ($LASTEXITCODE -ne 0) { throw 'Could not fetch pinned Rune revision' }
    & git -C $source checkout --detach $revision
    if ($LASTEXITCODE -ne 0) { throw 'Could not check out pinned Rune revision' }
}
if (@(& git -C $source status --porcelain).Count) { throw 'Rune source has local changes; use a clean pinned checkout' }
if ($BinutilsDirectory) { $env:PATH = (Resolve-Path -LiteralPath $BinutilsDirectory).Path + ';' + $env:PATH }
if (-not (Get-Command as.exe -ErrorAction SilentlyContinue)) {
    throw 'GNU Windows builds need MinGW-w64 binutils (as.exe). Supply -BinutilsDirectory or add it to PATH. See Experiments/Pinny.Rune/README.md.'
}
$output = Join-Path $repo 'dist\native'
New-Item -ItemType Directory -Path $output -Force | Out-Null
& cargo +1.99.1-x86_64-pc-windows-gnu build --manifest-path (Join-Path $experiment 'Cargo.toml') --release --locked
if ($LASTEXITCODE -ne 0) { throw 'Rune release build failed' }
$exe = Join-Path $output 'Pinny.Rune.exe'
Copy-Item -LiteralPath (Join-Path $experiment 'target\release\pinny-rune.exe') -Destination $exe -Force
& (Join-Path $PSScriptRoot 'Set-ExeIcon.ps1') -Exe $exe -Icon (Join-Path $repo 'Experiments\Pinny.Rust\Pinny.ico')
Write-Host "Rune ($revision): $exe"
