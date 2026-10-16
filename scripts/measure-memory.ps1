param(
    [string]$Exe = 'dist\Pinny.exe',
    [string]$Output = 'dist\memory.csv',
    [int[]]$NoteCounts = @(1, 5, 20),
    [ValidateRange(1, 20)][int]$Repetitions = 3,
    [ValidateRange(1, 60)][int]$SettleSeconds = 3,
    [ValidateRange(0, 1000000)][int]$TextCharacters = 64
)

# Active desktop client only. All input and settings are isolated from real notes.
$ErrorActionPreference = 'Stop'
$repo = Split-Path $PSScriptRoot -Parent
$exePath = (Resolve-Path (Join-Path $repo $Exe)).Path
$outputPath = [IO.Path]::GetFullPath((Join-Path $repo $Output))
$runRoot = Join-Path $repo ('dist\memory-data-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $runRoot -Force | Out-Null
New-Item -ItemType Directory -Path (Split-Path $outputPath -Parent) -Force | Out-Null

if (-not ('PinnyMemoryWindows' -as [type])) {
    Add-Type -TypeDefinition @'
using System;
using System.Runtime.InteropServices;
public static class PinnyMemoryWindows {
    private delegate bool Callback(IntPtr window, IntPtr parameter);
    [DllImport("user32.dll")] private static extern bool EnumWindows(Callback callback, IntPtr parameter);
    [DllImport("user32.dll")] private static extern uint GetWindowThreadProcessId(IntPtr window, out uint pid);
    [DllImport("user32.dll")] private static extern bool IsWindowVisible(IntPtr window);
    [DllImport("user32.dll")] public static extern uint GetGuiResources(IntPtr process, uint flags);
    public static int Count(int pid) {
        int count = 0;
        EnumWindows((window, parameter) => {
            GetWindowThreadProcessId(window, out uint owner);
            if (owner == (uint)pid && IsWindowVisible(window)) count++;
            return true;
        }, IntPtr.Zero);
        return count;
    }
}
'@
}

$results = foreach ($count in $NoteCounts) {
    if ($count -lt 1 -or $count -gt 100) { throw 'Note counts must be between 1 and 100.' }
    for ($round = 1; $round -le $Repetitions; $round++) {
        $data = Join-Path $runRoot "$count-$round"
        New-Item -ItemType Directory -Path $data -Force | Out-Null
        $notes = @(for ($i = 0; $i -lt $count; $i++) {
            [ordered]@{
                Id = "memory-$i"; Text = ('x' * $TextCharacters); Title = "Memory test $i"
                Left = 80 + ($i % 5) * 32; Top = 80 + ($i % 5) * 32
                Width = 320; Height = 320; IsPinned = $false; Theme = 'Light'
            }
        })
        # Write BOM-free UTF-8, including under Windows PowerShell.
        $json = ConvertTo-Json -InputObject $notes -Depth 4
        [IO.File]::WriteAllText((Join-Path $data 'notes.json'), $json, [Text.UTF8Encoding]::new($false))
        $app = $null
        $originalLocal = $env:LOCALAPPDATA
        try {
            $env:LOCALAPPDATA = $data
            $stderr = Join-Path $data 'stderr.log'
            $app = Start-Process -FilePath $exePath -ArgumentList @('--data-dir', ('"{0}"' -f $data)) -WindowStyle Hidden -RedirectStandardError $stderr -PassThru
            $env:LOCALAPPDATA = $originalLocal
            $ready = [Diagnostics.Stopwatch]::StartNew()
            while ([PinnyMemoryWindows]::Count($app.Id) -lt $count) {
                if ($app.HasExited) { throw "App exited ($($app.ExitCode)) before all note windows appeared. See $stderr" }
                if ($ready.Elapsed.TotalSeconds -gt 30) { throw 'Timed out waiting for note windows.' }
                Start-Sleep -Milliseconds 50
            }
            Start-Sleep -Seconds $SettleSeconds
            $app.Refresh()
            $cpu = $app.TotalProcessorTime.TotalMilliseconds
            Start-Sleep -Seconds 1
            $app.Refresh()
            [pscustomobject]@{
                Exe = [IO.Path]::GetFileName($exePath); Notes = $count; CharactersPerNote = $TextCharacters; Run = $round
                WorkingSetMiB = [math]::Round($app.WorkingSet64 / 1MB, 3)
                PrivateBytesMiB = [math]::Round($app.PrivateMemorySize64 / 1MB, 3)
                PeakWorkingSetMiB = [math]::Round($app.PeakWorkingSet64 / 1MB, 3)
                Handles = $app.HandleCount
                GdiObjects = [PinnyMemoryWindows]::GetGuiResources($app.Handle, 0)
                UserObjects = [PinnyMemoryWindows]::GetGuiResources($app.Handle, 1)
                IdleCpuMsOver1Second = [math]::Round($app.TotalProcessorTime.TotalMilliseconds - $cpu, 3)
            }
        }
        finally {
            $env:LOCALAPPDATA = $originalLocal
            if ($null -ne $app) {
                if (-not $app.HasExited) { Stop-Process -Id $app.Id -Force }
                $app.Dispose()
            }
        }
    }
}
$results | Export-Csv -NoTypeInformation -Path $outputPath
$results | Format-Table -AutoSize
Write-Host "Measurements: $outputPath"
