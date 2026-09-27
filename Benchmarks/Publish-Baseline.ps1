param([string]$Dotnet = 'dotnet')

$ErrorActionPreference = 'Stop'
$repo = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$outputRoot = Join-Path $repo 'dist\baseline-2026-09-27'
$builds = @(
    [pscustomobject]@{ Name = 'WPF'; Project = 'Experiments\Pinny.WpfBaseline\Pinny.WpfBaseline.csproj'; Folder = 'wpf'; Exe = 'Pinny.exe' },
    [pscustomobject]@{ Name = 'WinForms'; Project = 'Experiments\Pinny.WinForms\Pinny.WinForms.csproj'; Folder = 'winforms'; Exe = 'Pinny.WinForms.exe' },
    [pscustomobject]@{ Name = 'Win32'; Project = 'Experiments\Pinny.Win32\Pinny.Win32.csproj'; Folder = 'win32'; Exe = 'Pinny.Win32.exe' }
)

foreach ($build in $builds) {
    $project = Join-Path $repo $build.Project
    $output = Join-Path $outputRoot $build.Folder
    Write-Host "Publishing frozen $($build.Name) baseline..."
    & $Dotnet publish $project -c Release -r win-x64 --self-contained true `
        '-p:PublishSingleFile=true' `
        '-p:IncludeNativeLibrariesForSelfExtract=true' `
        '-p:EnableCompressionInSingleFile=true' `
        '-p:DebugType=none' `
        '-p:DebugSymbols=false' `
        -o $output
    if ($LASTEXITCODE -ne 0) { throw "Publish failed: $($build.Name)" }
    $exe = Join-Path $output $build.Exe
    if (-not (Test-Path -LiteralPath $exe)) { throw "Published executable missing: $exe" }
    Write-Host $exe
}
