param(
    [ValidateRange(1, 100)][int]$Repetitions = 5,
    [ValidateSet(1, 5)][int[]]$NoteCounts = @(1, 5),
    [ValidateRange(1, 60)][int]$SettleSeconds = 5,
    [ValidateRange(2, 120)][int]$CpuSeconds = 15,
    [int]$Seed = 20261116,
    [string[]]$Variants = @('WPF', 'WinForms', 'Win32', 'Rust Win32', 'Go Win32', 'Rust Rune'),
    [string]$OutputDirectory,
    [switch]$SkipWarmup
)

# A new suite: the September harness and its results deliberately stay unchanged.
$ErrorActionPreference = 'Stop'
$repo = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
if (-not $OutputDirectory) {
    $OutputDirectory = Join-Path $PSScriptRoot ('runs\' + (Get-Date -Format 'yyyy-MM-dd-HHmmss'))
}
if (Test-Path -LiteralPath $OutputDirectory) { throw "Use a new output directory: $OutputDirectory" }
$catalog = @(
    @{ Name = 'WPF'; Exe = 'dist\baseline-2026-09-27\wpf\Pinny.exe'; Input = 'JSON' },
    @{ Name = 'WinForms'; Exe = 'dist\baseline-2026-09-27\winforms\Pinny.WinForms.exe'; Input = 'JSON' },
    @{ Name = 'Win32'; Exe = 'dist\baseline-2026-09-27\win32\Pinny.Win32.exe'; Input = 'JSON' },
    @{ Name = 'Rust Win32'; Exe = 'dist\native\Pinny.Rust.exe'; Input = 'synthetic' },
    @{ Name = 'Go Win32'; Exe = 'dist\native\Pinny.Go.exe'; Input = 'JSON' },
    @{ Name = 'Rust Rune'; Exe = 'dist\native\Pinny.Rune.exe'; Input = 'JSON (read only)' }
)
foreach ($name in $Variants) { if ($name -notin $catalog.Name) { throw "Unknown variant: $name" } }
$selected = @($catalog | Where-Object { $_.Name -in $Variants })
foreach ($variant in $selected) {
    if (-not (Test-Path -LiteralPath (Join-Path $repo $variant.Exe))) { throw "Build missing: $($variant.Exe)" }
}
New-Item -ItemType Directory -Path $OutputDirectory | Out-Null
$OutputDirectory = (Resolve-Path -LiteralPath $OutputDirectory).Path
$dataRoot = Join-Path $PSScriptRoot ('.data\comparison-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $dataRoot | Out-Null

if (-not ('PinnyComparisonWindow' -as [type])) {
Add-Type -TypeDefinition @'
using System;
using System.Collections.Generic;
using System.Runtime.InteropServices;
using System.Text;
public static class PinnyComparisonWindow {
    private delegate bool Callback(IntPtr hwnd, IntPtr data);
    [StructLayout(LayoutKind.Sequential)] public struct Rect { public int Left, Top, Right, Bottom; }
    public class Note { public long Handle; public int Width, Height; public uint Dpi; }
    [DllImport("user32.dll")] private static extern bool EnumWindows(Callback cb, IntPtr data);
    [DllImport("user32.dll")] private static extern uint GetWindowThreadProcessId(IntPtr hwnd, out uint pid);
    [DllImport("user32.dll")] private static extern bool IsWindowVisible(IntPtr hwnd);
    [DllImport("user32.dll", CharSet=CharSet.Unicode)] private static extern int GetWindowTextW(IntPtr hwnd, StringBuilder text, int length);
    [DllImport("user32.dll")] private static extern bool GetWindowRect(IntPtr hwnd, out Rect rect);
    [DllImport("user32.dll")] private static extern uint GetDpiForWindow(IntPtr hwnd);
    [DllImport("user32.dll")] private static extern IntPtr SendMessageTimeoutW(IntPtr hwnd, uint msg, UIntPtr wp, IntPtr lp, uint flags, uint timeout, out UIntPtr result);
    [DllImport("dwmapi.dll")] public static extern int DwmFlush();
    public static Note[] Windows(int processId, bool responsive) {
        var notes = new List<Note>();
        EnumWindows((hwnd, data) => {
            uint pid; GetWindowThreadProcessId(hwnd, out pid);
            if (pid != processId || !IsWindowVisible(hwnd)) return true;
            var title = new StringBuilder(128); GetWindowTextW(hwnd, title, title.Capacity);
            if (title.ToString() != "Pinny" && title.ToString() != "Pinny WinForms") return true;
            UIntPtr result;
            if (responsive && SendMessageTimeoutW(hwnd, 0, UIntPtr.Zero, IntPtr.Zero, 2, 100, out result) == IntPtr.Zero) return true;
            Rect rect; GetWindowRect(hwnd, out rect);
            notes.Add(new Note { Handle=hwnd.ToInt64(), Width=rect.Right-rect.Left, Height=rect.Bottom-rect.Top, Dpi=GetDpiForWindow(hwnd) });
            return true;
        }, IntPtr.Zero);
        return notes.ToArray();
    }
}
'@
}

function Get-ProcessCounters([int]$ProcessId) {
    # Wildcard queries can include a process that exits during sampling. PDH then
    # emits an error for that instance while returning valid samples for others.
    # Only the target's validated ID + working-set pair determines success.
    for ($attempt = 0; $attempt -lt 8; $attempt++) {
        $samples = @((Get-Counter '\Process(*)\ID Process', '\Process(*)\Working Set - Private' -ErrorAction SilentlyContinue).CounterSamples | Where-Object Status -eq 0)
        $id = @($samples | Where-Object { $_.Path -match '\\id process$' -and [int]$_.CookedValue -eq $ProcessId })
        if ($id.Count -eq 1) {
            $working = @($samples | Where-Object { $_.InstanceName -eq $id[0].InstanceName -and $_.Path -match '\\working set - private$' })
            if ($working.Count -eq 1) { return $working[0].CookedValue }
        }
        Start-Sleep -Milliseconds 250
    }
    throw "Valid private working set unavailable for PID $ProcessId after eight attempts"
}

# Missing GPU counters are recorded as unavailable, never as a fabricated zero.
function Get-GpuSample([int]$ProcessId) {
    $memory = $null; $engines = $null
    try { $memory = @((Get-Counter '\GPU Process Memory(*)\Dedicated Usage', '\GPU Process Memory(*)\Shared Usage' -ErrorAction SilentlyContinue).CounterSamples | Where-Object Status -eq 0) } catch { }
    try { $engines = @((Get-Counter '\GPU Engine(*)\Utilization Percentage' -ErrorAction SilentlyContinue).CounterSamples | Where-Object Status -eq 0) } catch { }
    $prefix = '^pid_' + $ProcessId + '_'
    $ownedMemory = @($memory | Where-Object { $_.InstanceName -match $prefix })
    $ownedEngines = @($engines | Where-Object { $_.InstanceName -match $prefix })
    [pscustomobject]@{
        DedicatedMiB = if ($ownedMemory.Count) { [math]::Round((($ownedMemory | Where-Object { $_.Path -match '\\dedicated usage$' } | Measure-Object CookedValue -Sum).Sum / 1MB), 3) } else { $null }
        SharedMiB = if ($ownedMemory.Count) { [math]::Round((($ownedMemory | Where-Object { $_.Path -match '\\shared usage$' } | Measure-Object CookedValue -Sum).Sum / 1MB), 3) } else { $null }
        EngineSumPercent = if ($ownedEngines.Count) { [math]::Round(($ownedEngines | Measure-Object CookedValue -Sum).Sum, 3) } else { $null }
    }
}

$metadata = [ordered]@{
    StartedUtc = [datetime]::UtcNow.ToString('o'); OS = [Environment]::OSVersion.VersionString
    Cpu = @(Get-CimInstance Win32_Processor | Select-Object Name, NumberOfCores, NumberOfLogicalProcessors)
    Gpu = @(Get-CimInstance Win32_VideoController | Select-Object Name, DriverVersion)
    LogicalProcessors = [Environment]::ProcessorCount; Repetitions = $Repetitions; NoteCounts = $NoteCounts
    SettleSeconds = $SettleSeconds; CpuSeconds = $CpuSeconds; Seed = $Seed; Warmup = -not $SkipWarmup
    GitRevision = (& git -C $repo rev-parse HEAD); GitStatus = @(& git -C $repo status --short)
    Tools = @{ Rust = (& rustc --version); RuneRust = (& rustc +1.99.1-x86_64-pc-windows-gnu --version); Go = (& go version); Dotnet = (& dotnet --version) }
    Binaries = @($selected | ForEach-Object {
        $file = Get-Item -LiteralPath (Join-Path $repo $_.Exe)
        @{ Variant = $_.Name; Path = $_.Exe; Input = $_.Input; Bytes = $file.Length; Sha256 = (Get-FileHash -LiteralPath $file.FullName -Algorithm SHA256).Hash }
    })
    SourceSizes = @($selected | ForEach-Object {
        $paths = switch ($_.Name) {
            'WPF' { 'Experiments/Pinny.WpfBaseline' }
            'WinForms' { 'Experiments/Pinny.WinForms'; 'Experiments/Baseline' }
            'Win32' { 'Experiments/Pinny.Win32'; 'Experiments/Baseline' }
            'Rust Win32' { 'Experiments/Pinny.Rust/src' }
            'Go Win32' { 'Experiments/Pinny.Go' }
            'Rust Rune' { 'Experiments/Pinny.Rune/src' }
        }
        $files = @($paths | ForEach-Object { Get-ChildItem -LiteralPath (Join-Path $repo $_) -Recurse -File } |
            Where-Object { $_.Extension -in @('.cs','.xaml','.go','.rs') -and $_.FullName -notmatch '[\\/](bin|obj|target)[\\/]' })
        $lines = 0; $bytes = 0
        foreach ($file in $files) { $lines += @(Get-Content -LiteralPath $file.FullName).Count; $bytes += $file.Length }
        @{ Variant=$_.Name; Files=$files.Count; Lines=$lines; Bytes=$bytes; Includes=($paths -join ', ') }
    })
    StartupDefinition = 'First visible note; all requested notes visible and WM_NULL-responsive, followed by DwmFlush. Neither proves first text paint.'
    Workload = 'One/five Light notes, 320x320 requested units, same two-line text; positions 80+32*i. Rust Win32 constructs content in memory; others parse isolated JSON.'
}
$metadata | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath (Join-Path $OutputDirectory 'environment.json') -Encoding utf8
$results = [System.Collections.Generic.List[object]]::new()
$gpuSamples = [System.Collections.Generic.List[object]]::new()
$random = [Random]::new($Seed)

foreach ($count in $NoteCounts) {
    $firstRound = if ($SkipWarmup) { 1 } else { 0 }
    for ($round = $firstRound; $round -le $Repetitions; $round++) {
        # Shuffle a copy for each round; recorded seed and order make this auditable.
        $order = @($selected)
        for ($i = $order.Count - 1; $i -gt 0; $i--) {
            $j = $random.Next($i + 1); $temp = $order[$i]; $order[$i] = $order[$j]; $order[$j] = $temp
        }
        foreach ($variant in $order) {
            $data = Join-Path $dataRoot ($variant.Name.Replace(' ', '-') + "-$count-$round")
            New-Item -ItemType Directory -Path $data | Out-Null
            $notes = @(for ($n = 0; $n -lt $count; $n++) {
                [ordered]@{ Id = ('00000000-0000-0000-0000-{0:d12}' -f ($n + 1)); Text = "Benchmark note $n`r`nA short plain-text note for comparison."; Left = 80 + ($n % 5)*32; Top = 80 + ($n % 5)*32; Width = 320; Height = 320; IsPinned = $false; Theme = 'Light' }
            })
            ConvertTo-Json -InputObject $notes -Depth 4 | Set-Content -LiteralPath (Join-Path $data 'notes.json') -Encoding utf8
            $arguments = if ($variant.Input -eq 'synthetic') { @('--notes', $count) } else { @('--data-dir', ('"{0}"' -f $data)) }
            Write-Host "$($variant.Name), $count notes, round $round/$Repetitions (0 = warmup)"
            $watch = [Diagnostics.Stopwatch]::StartNew()
            # These are the visible GUI subjects of the experiment, not background helpers.
            $process = Start-Process -FilePath (Join-Path $repo $variant.Exe) -ArgumentList $arguments -PassThru
            try {
                $firstMs = $null
                do {
                    $process.Refresh()
                    if ($process.HasExited) { throw "$($variant.Name) exited: $($process.ExitCode)" }
                    $visible = [PinnyComparisonWindow]::Windows($process.Id, $false)
                    if ($null -eq $firstMs -and $visible.Count -gt 0) { $firstMs = $watch.Elapsed.TotalMilliseconds }
                    if ($visible.Count -eq $count) {
                        $ready = [PinnyComparisonWindow]::Windows($process.Id, $true)
                        if ($ready.Count -eq $count) { break }
                    }
                    if ($watch.Elapsed.TotalSeconds -gt 60) { throw "Timed out waiting for $count responsive windows from $($variant.Name)" }
                    Start-Sleep -Milliseconds 10
                } while ($true)
                $dwmResult = [PinnyComparisonWindow]::DwmFlush()
                $allMs = $watch.Elapsed.TotalMilliseconds
                Start-Sleep -Seconds $SettleSeconds
                $ready = [PinnyComparisonWindow]::Windows($process.Id, $true)
                if ($ready.Count -ne $count) { throw 'Note count changed during settling' }
                if ($round -eq 0) { continue }
                $privateWorking = Get-ProcessCounters $process.Id
                $process.Refresh()
                $privateBytes = $process.PrivateMemorySize64; $working = $process.WorkingSet64
                $cpuStart = $process.TotalProcessorTime
                $idleWatch = [Diagnostics.Stopwatch]::StartNew()
                $samples = @()
                do {
                    $gpu = Get-GpuSample $process.Id
                    $samples += $gpu
                    $gpuSamples.Add([pscustomobject]@{ Variant=$variant.Name; Notes=$count; Run=$round; ElapsedSeconds=[math]::Round($idleWatch.Elapsed.TotalSeconds, 3); DedicatedMiB=$gpu.DedicatedMiB; SharedMiB=$gpu.SharedMiB; EngineSumPercent=$gpu.EngineSumPercent })
                    if ($idleWatch.Elapsed.TotalSeconds -lt $CpuSeconds) { Start-Sleep -Milliseconds 500 }
                } while ($idleWatch.Elapsed.TotalSeconds -lt $CpuSeconds)
                $process.Refresh()
                $cpuMs = ($process.TotalProcessorTime - $cpuStart).TotalMilliseconds
                $elapsed = $idleWatch.Elapsed.TotalSeconds
                $results.Add([pscustomobject]@{
                    Variant=$variant.Name; Notes=$count; Run=$round; Order=$results.Count+1
                    StartupMs=[math]::Round($firstMs, 1); AllWindowsReadyMs=[math]::Round($allMs, 1); DwmFlushResult=$dwmResult
                    PrivateWorkingSetMB=[math]::Round($privateWorking/1MB, 2); PrivateBytesMB=[math]::Round($privateBytes/1MB, 2); WorkingSetMB=[math]::Round($working/1MB, 2)
                    IdleCpuMsOverWindow=[math]::Round($cpuMs, 2); CpuWindowSeconds=[math]::Round($elapsed, 3)
                    IdleCpuOneCorePercent=[math]::Round($cpuMs/($elapsed*10), 3)
                    GpuDedicatedMiB=($samples | Where-Object { $null -ne $_.DedicatedMiB } | Measure-Object DedicatedMiB -Maximum).Maximum
                    GpuSharedMiB=($samples | Where-Object { $null -ne $_.SharedMiB } | Measure-Object SharedMiB -Maximum).Maximum
                    GpuEngineSumPercent=($samples | Where-Object { $null -ne $_.EngineSumPercent } | Measure-Object EngineSumPercent -Average).Average
                    WindowGeometry=($ready | ForEach-Object { "$($_.Width)x$($_.Height)@$($_.Dpi)dpi" }) -join ';'
                    InputMode=$variant.Input
                })
                $results | Export-Csv -LiteralPath (Join-Path $OutputDirectory 'results.csv') -NoTypeInformation -Encoding utf8
                $gpuSamples | Export-Csv -LiteralPath (Join-Path $OutputDirectory 'gpu-samples.csv') -NoTypeInformation -Encoding utf8
            } finally {
                $process.Refresh()
                if (-not $process.HasExited) { Stop-Process -Id $process.Id -Force }
                $process.Dispose()
            }
        }
    }
}
& (Join-Path $PSScriptRoot 'Write-ComparisonReport.ps1') -Directory $OutputDirectory
Write-Host "Comparison saved: $OutputDirectory"
