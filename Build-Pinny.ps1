param(
    [string]$Output = 'dist\Pinny.exe'
)

$ErrorActionPreference = 'Stop'
$repo = $PSScriptRoot
$outputPath = [IO.Path]::GetFullPath((Join-Path $repo $Output))
New-Item -ItemType Directory -Path ([IO.Path]::GetDirectoryName($outputPath)) -Force | Out-Null

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    throw 'Go is not installed or not on PATH.'
}

Push-Location (Join-Path $repo 'Pinny.Go')
try {
    & go build -trimpath -ldflags '-s -w -H windowsgui' -o $outputPath .
    if ($LASTEXITCODE -ne 0) { throw 'Pinny Go release build failed.' }
}
finally { Pop-Location }

& (Join-Path $repo 'Benchmarks\Set-ExeIcon.ps1') -Exe $outputPath -Icon (Join-Path $repo 'Pinny.Go\Pinny.ico')
Write-Host "Pinny: $outputPath"
