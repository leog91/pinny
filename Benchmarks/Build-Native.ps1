$ErrorActionPreference = 'Stop'
$repo = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$output = Join-Path $repo 'dist\native'
New-Item -ItemType Directory -Path $output -Force | Out-Null

if (-not (Get-Command cargo -ErrorAction SilentlyContinue)) { throw 'Rust cargo is not installed or not on PATH.' }
if (-not (Get-Command go -ErrorAction SilentlyContinue)) { throw 'Go is not installed or not on PATH.' }

$rustManifest = Join-Path $repo 'Experiments\Pinny.Rust\Cargo.toml'
& cargo build --manifest-path $rustManifest --release
if ($LASTEXITCODE -ne 0) { throw 'Rust release build failed.' }
$rustExe = Join-Path $repo 'Experiments\Pinny.Rust\target\release\pinny-rust.exe'
Copy-Item -LiteralPath $rustExe -Destination (Join-Path $output 'Pinny.Rust.exe') -Force
& (Join-Path $PSScriptRoot 'Set-ExeIcon.ps1') -Exe (Join-Path $output 'Pinny.Rust.exe') -Icon (Join-Path $repo 'Experiments\Pinny.Rust\Pinny.ico')

Push-Location (Join-Path $repo 'Experiments\Pinny.Go')
try {
    & go build -trimpath -ldflags '-s -w -H windowsgui' -o (Join-Path $output 'Pinny.Go.exe') .
    if ($LASTEXITCODE -ne 0) { throw 'Go release build failed.' }
}
finally { Pop-Location }
& (Join-Path $PSScriptRoot 'Set-ExeIcon.ps1') -Exe (Join-Path $output 'Pinny.Go.exe') -Icon (Join-Path $repo 'Experiments\Pinny.Go\Pinny.ico')

Write-Host "Rust: $(Join-Path $output 'Pinny.Rust.exe')"
Write-Host "Go:   $(Join-Path $output 'Pinny.Go.exe')"
