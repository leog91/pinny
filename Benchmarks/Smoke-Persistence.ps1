$ErrorActionPreference = 'Stop'
$root = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$dataRoot = Join-Path $PSScriptRoot '.data'
New-Item -ItemType Directory -Path $dataRoot -Force | Out-Null

Add-Type -TypeDefinition @'
using System;
using System.Text;
using System.Runtime.InteropServices;
public static class PinnySmokeNative {
    [StructLayout(LayoutKind.Sequential)] public struct Rect { public int Left, Top, Right, Bottom; }
    public delegate bool ChildCallback(IntPtr window, IntPtr parameter);
    [DllImport("user32.dll")] public static extern bool EnumChildWindows(IntPtr parent, ChildCallback callback, IntPtr parameter);
    [DllImport("user32.dll", CharSet = CharSet.Unicode)] public static extern int GetClassNameW(IntPtr window, StringBuilder name, int length);
    [DllImport("user32.dll", CharSet = CharSet.Unicode)] public static extern int GetWindowTextW(IntPtr window, StringBuilder text, int length);
    [DllImport("user32.dll", CharSet = CharSet.Unicode)] public static extern IntPtr SendMessageW(IntPtr window, uint message, IntPtr wParam, string text);
    [DllImport("user32.dll", CharSet = CharSet.Unicode)] public static extern IntPtr SendMessageW(IntPtr window, uint message, IntPtr wParam, StringBuilder text);
    [DllImport("user32.dll")] public static extern IntPtr SendMessageW(IntPtr window, uint message, IntPtr wParam, IntPtr lParam);
    [DllImport("user32.dll")] public static extern bool MoveWindow(IntPtr window, int x, int y, int width, int height, bool repaint);
    [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr window, out Rect rect);
    [DllImport("user32.dll")] public static extern int GetWindowLongW(IntPtr window, int index);
}
'@

function Find-Child([IntPtr]$parent, [string]$classPart, [string]$caption = '') {
    $found = [IntPtr]::Zero
    $callback = [PinnySmokeNative+ChildCallback]{
        param($window, $parameter)
        $name = [System.Text.StringBuilder]::new(256)
        $text = [System.Text.StringBuilder]::new(256)
        [void][PinnySmokeNative]::GetClassNameW($window, $name, $name.Capacity)
        [void][PinnySmokeNative]::GetWindowTextW($window, $text, $text.Capacity)
        if ($name.ToString().ToUpperInvariant().Contains($classPart) -and
            ($caption -eq '' -or $text.ToString() -eq $caption)) {
            $script:foundChild = $window
            return $false
        }
        return $true
    }
    $script:foundChild = [IntPtr]::Zero
    [void][PinnySmokeNative]::EnumChildWindows($parent, $callback, [IntPtr]::Zero)
    return $script:foundChild
}

function Click-NativeHeader([IntPtr]$window, [int]$x) {
    $position = [IntPtr]($x -bor (16 -shl 16))
    [void][PinnySmokeNative]::SendMessageW($window, 0x0201, [IntPtr]1, $position)
    [void][PinnySmokeNative]::SendMessageW($window, 0x0202, [IntPtr]::Zero, $position)
}

foreach ($variant in @('winforms', 'win32')) {
    $exe = Join-Path $root ("dist\baseline-2026-09-27\$variant\" +
        @{ winforms = 'Pinny.WinForms.exe'; win32 = 'Pinny.Win32.exe' }[$variant])
    $data = Join-Path $dataRoot "functional-$variant"
    New-Item -ItemType Directory -Path $data -Force | Out-Null
    $initial = @([ordered]@{ Id = [guid]::NewGuid().ToString(); Text = 'Before';
        Left = 100; Top = 100; Width = 320; Height = 320; IsPinned = $false; Theme = 'Light' })
    ConvertTo-Json -InputObject $initial -Depth 5 | Set-Content (Join-Path $data 'notes.json') -Encoding utf8
    $process = Start-Process -FilePath $exe -ArgumentList @('--data-dir', $data) -PassThru
    try {
        $limit = [DateTime]::UtcNow.AddSeconds(15)
        do {
            Start-Sleep -Milliseconds 25
            $process.Refresh()
            if ($process.HasExited) { throw "$variant exited before opening" }
            if ([DateTime]::UtcNow -gt $limit) { throw "$variant window timeout" }
        } until ($process.MainWindowHandle -ne 0)
        Start-Sleep -Milliseconds 300
        $window = $process.MainWindowHandle
        $edit = Find-Child $window 'EDIT'
        if ($edit -eq [IntPtr]::Zero) { throw "$variant editor not found" }
        [void][PinnySmokeNative]::SendMessageW($edit, 0x000C, [IntPtr]::Zero, "Edited $variant")
        [void][PinnySmokeNative]::MoveWindow($window, 200, 200, 400, 300, $true)
        if ($variant -eq 'win32') {
            Click-NativeHeader $window 346 # Pin button in a 400-pixel window.
        } else {
            $button = Find-Child $window 'BUTTON' '◇'
            if ($button -eq [IntPtr]::Zero) { throw 'WinForms pin button not found' }
            [void][PinnySmokeNative]::SendMessageW($button, 0x00F5, [IntPtr]::Zero, [IntPtr]::Zero)
        }
        Start-Sleep -Milliseconds 1100
        $saved = @(Get-Content (Join-Path $data 'notes.json') -Raw | ConvertFrom-Json)
        if ($saved.Count -ne 1 -or $saved[0].Text -ne "Edited $variant" -or
            $saved[0].Left -ne 200 -or $saved[0].Top -ne 200 -or
            $saved[0].Width -ne 400 -or $saved[0].Height -ne 300 -or
            $saved[0].IsPinned -ne $true) {
            throw "$variant did not persist text, geometry, and pin state as expected"
        }
        Write-Host "$variant saved text, geometry, and pin state."
    }
    finally {
        $process.Refresh()
        if (-not $process.HasExited) { Stop-Process -Id $process.Id -Force }
        $process.Dispose()
    }

    $process = Start-Process -FilePath $exe -ArgumentList @('--data-dir', $data) -PassThru
    try {
        $limit = [DateTime]::UtcNow.AddSeconds(15)
        do {
            Start-Sleep -Milliseconds 25
            $process.Refresh()
            if ($process.HasExited) { throw "$variant exited before restoring" }
            if ([DateTime]::UtcNow -gt $limit) { throw "$variant restore timeout" }
        } until ($process.MainWindowHandle -ne 0)
        Start-Sleep -Milliseconds 300
        $window = $process.MainWindowHandle
        $edit = Find-Child $window 'EDIT'
        if ($edit -eq [IntPtr]::Zero) { throw "$variant restored editor not found" }
        $text = [System.Text.StringBuilder]::new(256)
        [void][PinnySmokeNative]::SendMessageW($edit, 0x000D, [IntPtr]$text.Capacity, $text)
        $rect = [PinnySmokeNative+Rect]::new()
        [void][PinnySmokeNative]::GetWindowRect($window, [ref]$rect)
        $style = [PinnySmokeNative]::GetWindowLongW($window, -20)
        if ($text.ToString() -ne "Edited $variant" -or $rect.Left -ne 200 -or $rect.Top -ne 200 -or
            $rect.Right - $rect.Left -ne 400 -or $rect.Bottom - $rect.Top -ne 300 -or
            ($style -band 8) -eq 0) {
            throw "$variant did not restore text, geometry, and pin state as expected"
        }
        Write-Host "$variant restored text, geometry, and pin state."

        if ($variant -eq 'win32') {
            Click-NativeHeader $window 266 # New button.
        } else {
            $button = Find-Child $window 'BUTTON' '+'
            if ($button -eq [IntPtr]::Zero) { throw 'WinForms new-note button not found' }
            [void][PinnySmokeNative]::SendMessageW($button, 0x00F5, [IntPtr]::Zero, [IntPtr]::Zero)
        }
        Start-Sleep -Milliseconds 700
        $saved = @(Get-Content (Join-Path $data 'notes.json') -Raw | ConvertFrom-Json)
        if ($saved.Count -ne 2) { throw "$variant did not create and save a second note" }
        Write-Host "$variant created and saved a second note."
        if ($variant -eq 'win32') {
            Click-NativeHeader $window 382 # Close button.
        } else {
            [void][PinnySmokeNative]::SendMessageW($window, 0x0010, [IntPtr]::Zero, [IntPtr]::Zero)
        }
        Start-Sleep -Milliseconds 200
        $process.Refresh()
        $saved = @(Get-Content (Join-Path $data 'notes.json') -Raw | ConvertFrom-Json)
        if ($process.HasExited -or $saved.Count -ne 1 -or $saved[0].Text -ne '') {
            throw "$variant did not delete only the selected note"
        }
        Write-Host "$variant deleted one note and kept the other open."
    }
    finally {
        $process.Refresh()
        if (-not $process.HasExited) { Stop-Process -Id $process.Id -Force }
        $process.Dispose()
    }
}
