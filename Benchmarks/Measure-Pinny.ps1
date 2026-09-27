param(
    [int]$Repetitions = 3,
    [int[]]$NoteCounts = @(1, 5),
    [int]$SettleSeconds = 2,
    [int]$CpuSeconds = 3
)

$ErrorActionPreference = 'Stop'
$repo = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$runs = Join-Path $PSScriptRoot '.data'
New-Item -ItemType Directory -Path $runs -Force | Out-Null

Add-Type -TypeDefinition @'
using System;
using System.Text;
using System.Runtime.InteropServices;
public static class PinnyBenchmarkWindow {
    private delegate bool WindowCallback(IntPtr window, IntPtr parameter);
    [DllImport("user32.dll")] private static extern bool EnumWindows(WindowCallback callback, IntPtr parameter);
    [DllImport("user32.dll")] private static extern uint GetWindowThreadProcessId(IntPtr window, out uint processId);
    [DllImport("user32.dll")] private static extern bool IsWindowVisible(IntPtr window);
    [DllImport("user32.dll", CharSet = CharSet.Unicode)] private static extern int GetWindowTextW(IntPtr window, StringBuilder text, int capacity);
    public static bool HasNoteWindow(int processId) {
        bool found = false;
        EnumWindows((window, parameter) => {
            GetWindowThreadProcessId(window, out uint owner);
            if (owner != (uint)processId || !IsWindowVisible(window)) return true;
            var title = new StringBuilder(128);
            GetWindowTextW(window, title, title.Capacity);
            if (title.ToString() == "Pinny" || title.ToString() == "Pinny WinForms") {
                found = true;
                return false;
            }
            return true;
        }, IntPtr.Zero);
        return found;
    }
}
'@

$variants = @(
    [pscustomobject]@{ Name = 'WPF'; Exe = 'dist\baseline-2026-09-27\wpf\Pinny.exe' },
    [pscustomobject]@{ Name = 'WinForms'; Exe = 'dist\baseline-2026-09-27\winforms\Pinny.WinForms.exe' },
    [pscustomobject]@{ Name = 'Win32'; Exe = 'dist\baseline-2026-09-27\win32\Pinny.Win32.exe' },
    [pscustomobject]@{ Name = 'Rust Win32'; Exe = 'dist\native\Pinny.Rust.exe'; Synthetic = $true },
    [pscustomobject]@{ Name = 'Go Win32'; Exe = 'dist\native\Pinny.Go.exe' }
)

function Get-PrivateWorkingSetBytes([int]$ProcessId) {
    $samples = $null
    for ($attempt = 0; $attempt -lt 8; $attempt++) {
        try {
            $samples = (Get-Counter -Counter '\Process(*)\ID Process', '\Process(*)\Working Set - Private' -ErrorAction Stop).CounterSamples
            break
        }
        catch { Start-Sleep -Milliseconds 250 }
    }
    if ($null -eq $samples) { throw "Performance counters remained unavailable for PID $ProcessId" }
    $instance = $null
    foreach ($sample in $samples) {
        if ($sample.Path -match '\\process\(([^)]+)\)\\id process$' -and [int]$sample.CookedValue -eq $ProcessId) {
            $instance = $Matches[1]
            break
        }
    }
    if ($null -eq $instance) { throw "Could not find performance counters for PID $ProcessId" }
    foreach ($sample in $samples) {
        if ($sample.Path -match '\\process\(([^)]+)\)\\working set - private$' -and $Matches[1] -eq $instance) {
            return [long]$sample.CookedValue
        }
    }
    throw "Could not find private working set for PID $ProcessId"
}

$results = [System.Collections.Generic.List[object]]::new()
foreach ($count in $NoteCounts) {
    foreach ($variant in $variants) {
        $exe = Join-Path $repo $variant.Exe
        if (-not (Test-Path $exe)) { throw "Build missing: $exe" }
        for ($round = 1; $round -le $Repetitions; $round++) {
            $data = Join-Path $runs ('{0}-{1}-{2}' -f ($variant.Name -replace '\s+', '-'), $count, $round)
            New-Item -ItemType Directory -Path $data -Force | Out-Null
            $notes = @(
                for ($i = 0; $i -lt $count; $i++) {
                    [ordered]@{
                        Id = [guid]::NewGuid().ToString()
                        Text = "Benchmark note $i`r`nA short plain-text note for comparison."
                        Left = 80 + (($i % 5) * 32)
                        Top = 80 + (($i % 5) * 32)
                        Width = 320
                        Height = 320
                        IsPinned = $false
                        Theme = 'Light'
                    }
                }
            )
            ConvertTo-Json -InputObject $notes -Depth 5 | Set-Content -Path (Join-Path $data 'notes.json') -Encoding utf8

            Write-Host "Measuring $($variant.Name), $count note(s), run $round/$Repetitions..."
            $watch = [System.Diagnostics.Stopwatch]::StartNew()
            $arguments = if ($variant.Synthetic) { @('--notes', $count) } else { @('--data-dir', ('"{0}"' -f $data)) }
            $process = Start-Process -FilePath $exe -ArgumentList $arguments -PassThru
            try {
                do {
                    Start-Sleep -Milliseconds 20
                    $process.Refresh()
                    if ($process.HasExited) { throw "$($variant.Name) exited before opening a window (code $($process.ExitCode))" }
                    if ($watch.Elapsed.TotalSeconds -gt 20) { throw "$($variant.Name) did not open a window within 20 seconds" }
                } until ([PinnyBenchmarkWindow]::HasNoteWindow($process.Id))
                $startupMs = [math]::Round($watch.Elapsed.TotalMilliseconds)

                Start-Sleep -Seconds $SettleSeconds
                $process.Refresh()
                $privateWorkingSet = Get-PrivateWorkingSetBytes $process.Id
                $privateBytes = $process.PrivateMemorySize64
                $workingSet = $process.WorkingSet64
                $cpuStart = $process.TotalProcessorTime
                Start-Sleep -Seconds $CpuSeconds
                $process.Refresh()
                $idleCpuMs = [math]::Round(($process.TotalProcessorTime - $cpuStart).TotalMilliseconds)

                $results.Add([pscustomobject]@{
                    Variant = $variant.Name
                    Notes = $count
                    Run = $round
                    StartupMs = $startupMs
                    PrivateWorkingSetMB = [math]::Round($privateWorkingSet / 1MB, 1)
                    PrivateBytesMB = [math]::Round($privateBytes / 1MB, 1)
                    WorkingSetMB = [math]::Round($workingSet / 1MB, 1)
                    IdleCpuMsOverWindow = $idleCpuMs
                    CpuWindowSeconds = $CpuSeconds
                })
                $results | Export-Csv -Path (Join-Path $PSScriptRoot 'results.csv') -NoTypeInformation -Encoding utf8
            }
            finally {
                $process.Refresh()
                if (-not $process.HasExited) { Stop-Process -Id $process.Id -Force }
                $process.Dispose()
            }
        }
    }
}

$output = Join-Path $PSScriptRoot 'results.csv'
$results | Export-Csv -Path $output -NoTypeInformation -Encoding utf8
& (Join-Path $PSScriptRoot 'Update-Chart.ps1') -ResultsPath $output
$results | Group-Object Variant, Notes | ForEach-Object {
    $items = @($_.Group)
    [pscustomobject]@{
        Variant = $items[0].Variant
        Notes = $items[0].Notes
        MedianPrivateWorkingSetMB = ($items.PrivateWorkingSetMB | Sort-Object)[[int][math]::Floor($items.Count / 2)]
        MedianPrivateBytesMB = ($items.PrivateBytesMB | Sort-Object)[[int][math]::Floor($items.Count / 2)]
        MedianStartupMs = ($items.StartupMs | Sort-Object)[[int][math]::Floor($items.Count / 2)]
        MedianIdleCpuMs = ($items.IdleCpuMsOverWindow | Sort-Object)[[int][math]::Floor($items.Count / 2)]
    }
} | Format-Table -AutoSize
Write-Host "Raw results: $output"
