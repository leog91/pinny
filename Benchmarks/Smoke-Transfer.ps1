$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'Smoke-Persistence.ps1') -Variants @()

# Check each native picker opens and cancellation leaves the collection intact.
# Data transfer, folder switching, and failure recovery are covered by Go tests.
$source = Join-Path $dataRoot ('transfer-dialogs-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $source -Force | Out-Null
$seed = @([ordered]@{ Id = 'dialog-note'; Text = 'Keep this note'; Left = 100; Top = 100; Width = 400; Height = 300; IsPinned = $false; Theme = 'Paper' })
ConvertTo-Json -InputObject $seed -Depth 5 | Set-Content (Join-Path $source 'notes.json') -Encoding utf8
'[]' | Set-Content (Join-Path $source 'trash.json') -Encoding utf8
$exe = Join-Path $root 'dist\Pinny.exe'
$originalLocalAppData = $env:LOCALAPPDATA
$originalPinnyDir = $env:PINNY_DATA_DIR
$app = Start-Process -FilePath $exe -ArgumentList @('--data-dir', $source) -WindowStyle Hidden -PassThru
try {
    $limit = [DateTime]::UtcNow.AddSeconds(10)
    do {
        Start-Sleep -Milliseconds 25
        $app.Refresh()
        if ($app.HasExited) { throw 'Pinny exited before opening notes' }
        if ([DateTime]::UtcNow -gt $limit) { throw 'Pinny startup timeout' }
    } until ($app.MainWindowHandle -ne 0)
    $window = $app.MainWindowHandle
    Start-Sleep -Milliseconds 600
    $notesBefore = Get-Content (Join-Path $source 'notes.json') -Raw
    $trashBefore = Get-Content (Join-Path $source 'trash.json') -Raw
    foreach ($command in @(11, 12, 13)) {
        [void][PinnySmokeNative]::PostMessageW($window, 0x0111, [IntPtr]$command, [IntPtr]::Zero)
        $limit = [DateTime]::UtcNow.AddSeconds(10)
        do {
            $dialog = Find-ProcessDialog ([uint32]$app.Id)
            if ($dialog -ne [IntPtr]::Zero) { break }
            Start-Sleep -Milliseconds 25
        } while ([DateTime]::UtcNow -lt $limit)
        if ($dialog -eq [IntPtr]::Zero) { throw "Native picker did not open for command $command" }
        Start-Sleep -Milliseconds 300
        [void][PinnySmokeNative]::PostMessageW($dialog, 0x0010, [IntPtr]::Zero, [IntPtr]::Zero)
        $limit = [DateTime]::UtcNow.AddSeconds(5)
        do {
            Start-Sleep -Milliseconds 25
            $remaining = Find-ProcessDialog ([uint32]$app.Id)
        } while ($remaining -ne [IntPtr]::Zero -and [DateTime]::UtcNow -lt $limit)
        if ($remaining -ne [IntPtr]::Zero) { throw "Native picker did not cancel for command $command" }
        if ((Get-Content (Join-Path $source 'notes.json') -Raw) -ne $notesBefore -or
            (Get-Content (Join-Path $source 'trash.json') -Raw) -ne $trashBefore) { throw 'Cancellation changed existing data' }
    }
    [void][PinnySmokeNative]::SendMessageW($window, 0x0010, [IntPtr]::Zero, [IntPtr]::Zero)
    if (-not $app.WaitForExit(5000)) { throw 'Pinny did not quit after testing the pickers' }
    Write-Host 'Export, import, and folder pickers opened and cancelled while preserving existing data.'
    $app.Dispose()
    $configRoot = Join-Path $source 'localappdata'
    $settingsDir = Join-Path $configRoot 'Pinny'
    New-Item -ItemType Directory -Path $settingsDir -Force | Out-Null
    @{ DataDir = $source } | ConvertTo-Json | Set-Content (Join-Path $settingsDir 'settings.json') -Encoding utf8
    $env:LOCALAPPDATA = $configRoot
    $env:PINNY_DATA_DIR = $null
    $app = Start-Process -FilePath $exe -WindowStyle Hidden -PassThru
    $limit = [DateTime]::UtcNow.AddSeconds(10)
    do {
        Start-Sleep -Milliseconds 25
        $app.Refresh()
        if ($app.HasExited) { throw 'Pinny exited before loading the remembered notes folder' }
        if ([DateTime]::UtcNow -gt $limit) { throw 'Remembered folder startup timeout' }
    } until ($app.MainWindowHandle -ne 0)
    $window = $app.MainWindowHandle
    $edit = Find-Child $window 'EDIT'
    $text = [System.Text.StringBuilder]::new(256)
    [void][PinnySmokeNative]::SendMessageW($edit, 0x000D, [IntPtr]$text.Capacity, $text)
    if ($text.ToString() -ne 'Keep this note') { throw 'Normal launch did not load the remembered notes folder' }
    [void][PinnySmokeNative]::SendMessageW($edit, 0x000C, [IntPtr]::Zero, 'Saved to the remembered folder')
    [void][PinnySmokeNative]::SendMessageW($window, 0x0010, [IntPtr]::Zero, [IntPtr]::Zero)
    if (-not $app.WaitForExit(5000)) { throw 'Pinny did not quit after loading the remembered folder' }
    $saved = @(Get-Content (Join-Path $source 'notes.json') -Raw | ConvertFrom-Json)
    if ($saved.Count -ne 1 -or $saved[0].Text -ne 'Saved to the remembered folder') { throw 'Normal launch did not save edits in the remembered folder' }
    Write-Host 'Normal launch loaded the remembered folder and saved subsequent edits there.'
}
finally {
    $app.Refresh()
    if (-not $app.HasExited) { Stop-Process -Id $app.Id -Force }
    $app.Dispose()
    $env:LOCALAPPDATA = $originalLocalAppData
    $env:PINNY_DATA_DIR = $originalPinnyDir
}
