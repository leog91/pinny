param(
    [Parameter(Mandatory)][string]$Exe,
    [Parameter(Mandatory)][string]$Icon
)

$ErrorActionPreference = 'Stop'

# RT_ICON entries plus one RT_GROUP_ICON make the .ico visible to Explorer and
# available to LoadImageW from the executable module.
if (-not ('PinnyIconResource' -as [type])) {
Add-Type -TypeDefinition @'
using System;
using System.Runtime.InteropServices;
public static class PinnyIconResource
{
    [DllImport("kernel32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
    public static extern IntPtr BeginUpdateResourceW(string fileName, bool deleteExisting);
    [DllImport("kernel32.dll", SetLastError = true)]
    [return: MarshalAs(UnmanagedType.Bool)]
    public static extern bool UpdateResourceW(IntPtr update, IntPtr type, IntPtr name,
        ushort language, byte[] data, uint size);
    [DllImport("kernel32.dll", SetLastError = true)]
    [return: MarshalAs(UnmanagedType.Bool)]
    public static extern bool EndUpdateResourceW(IntPtr update, bool discard);
}
'@
}

$exePath = (Resolve-Path -LiteralPath $Exe).Path
$ico = [IO.File]::ReadAllBytes((Resolve-Path -LiteralPath $Icon).Path)
if ($ico.Length -lt 6 -or [BitConverter]::ToUInt16($ico, 0) -ne 0 -or
    [BitConverter]::ToUInt16($ico, 2) -ne 1) {
    throw "Not a valid ICO file: $Icon"
}
$count = [BitConverter]::ToUInt16($ico, 4)
if ($count -lt 1 -or 6 + 16 * $count -gt $ico.Length) {
    throw "Invalid ICO directory: $Icon"
}

$groupStream = [IO.MemoryStream]::new()
$group = [IO.BinaryWriter]::new($groupStream)
$group.Write([uint16]0)
$group.Write([uint16]1)
$group.Write([uint16]$count)

$update = [PinnyIconResource]::BeginUpdateResourceW($exePath, $false)
if ($update -eq [IntPtr]::Zero) {
    throw "Could not open executable resources: $([Runtime.InteropServices.Marshal]::GetLastWin32Error())"
}
$committed = $false
try {
    for ($i = 0; $i -lt $count; $i++) {
        $entryOffset = 6 + 16 * $i
        $size = [BitConverter]::ToUInt32($ico, $entryOffset + 8)
        $dataOffset = [BitConverter]::ToUInt32($ico, $entryOffset + 12)
        if ([uint64]$dataOffset + [uint64]$size -gt [uint64]$ico.Length) {
            throw "Invalid ICO image $i"
        }
        $image = [byte[]]::new([int]$size)
        [Array]::Copy($ico, [int]$dataOffset, $image, 0, [int]$size)
        $id = [int](100 + $i)
        if (-not [PinnyIconResource]::UpdateResourceW($update, [IntPtr]3, [IntPtr]$id,
                [uint16]0, $image, $size)) {
            throw "Could not add ICO image $i : $([Runtime.InteropServices.Marshal]::GetLastWin32Error())"
        }
        $group.Write($ico, $entryOffset, 12)
        $group.Write([uint16]$id)
    }
    $group.Flush()
    $groupBytes = $groupStream.ToArray()
    if (-not [PinnyIconResource]::UpdateResourceW($update, [IntPtr]14, [IntPtr]1,
            [uint16]0, $groupBytes, [uint32]$groupBytes.Length)) {
        throw "Could not add icon group: $([Runtime.InteropServices.Marshal]::GetLastWin32Error())"
    }
    if (-not [PinnyIconResource]::EndUpdateResourceW($update, $false)) {
        throw "Could not write icon resources: $([Runtime.InteropServices.Marshal]::GetLastWin32Error())"
    }
    $committed = $true
}
finally {
    if (-not $committed) {
        [PinnyIconResource]::EndUpdateResourceW($update, $true) | Out-Null
    }
    $group.Dispose()
    $groupStream.Dispose()
}
