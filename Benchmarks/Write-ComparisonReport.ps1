param([Parameter(Mandatory)][string]$Directory)
$ErrorActionPreference = 'Stop'
$rows = @(Import-Csv -LiteralPath (Join-Path $Directory 'results.csv'))
$envInfo = Get-Content -LiteralPath (Join-Path $Directory 'environment.json') -Raw | ConvertFrom-Json
$culture = [Globalization.CultureInfo]::InvariantCulture
function Numbers($items, [string]$field) {
    @($items | Where-Object { $null -ne $_.$field -and $_.$field -ne '' } | ForEach-Object { [double]::Parse($_.$field, $culture) } | Sort-Object)
}
function Median($values) {
    if (-not $values.Count) { return $null }
    $mid = [int][math]::Floor($values.Count/2)
    if ($values.Count % 2) { return $values[$mid] }
    return ($values[$mid-1]+$values[$mid])/2
}
function Display($value, [string]$format = '0.0') {
    if ($null -eq $value) { return 'unavailable' }
    return ([double]$value).ToString($format, $culture)
}
$summary = @($rows | Group-Object Variant, Notes | ForEach-Object {
    $items = @($_.Group)
    $s = [ordered]@{ Variant=$items[0].Variant; Notes=[int]$items[0].Notes; Runs=$items.Count }
    foreach ($field in @('PrivateWorkingSetMB','PrivateBytesMB','StartupMs','AllWindowsReadyMs','IdleCpuOneCorePercent','GpuDedicatedMiB','GpuSharedMiB','GpuEngineSumPercent')) {
        $values = @(Numbers $items $field)
        $s[$field] = Median $values
        $s[$field+'Min'] = if ($values.Count) { $values[0] } else { $null }
        $s[$field+'Max'] = if ($values.Count) { $values[-1] } else { $null }
    }
    [pscustomobject]$s
} | Sort-Object Notes, PrivateWorkingSetMB)
$summary | Export-Csv -LiteralPath (Join-Path $Directory 'summary.csv') -NoTypeInformation -Encoding utf8
$md = [Collections.Generic.List[string]]::new()
$md.Add('# Pinny Windows UI comparison')
$md.Add('')
$recordedUtc = ([datetime]$envInfo.StartedUtc).ToUniversalTime().ToString('yyyy-MM-dd HH:mm:ss')
$md.Add("Recorded UTC: $recordedUtc. Medians of $($envInfo.Repetitions) measured runs per implementation and note count; warmup: $($envInfo.Warmup).")
$md.Add('')
$md.Add('This is a new measurement series. The [September 2026 report](../../README.md), CSV, charts, scripts, and five original experiment sources remain unchanged. See [method changes and reasons](../../COMPARISON.md).')
$md.Add('')
$md.Add('[Raw runs](results.csv) · [Summary CSV](summary.csv) · [GPU samples](gpu-samples.csv) · [Environment and binary hashes](environment.json) · [Chart](comparison.svg) · [Size, for curiosity](SIZE.md)')
if (Test-Path -LiteralPath (Join-Path $Directory 'NOTES.md')) {
    $md.Add('')
    $md.Add('[Build, verification and interpretation notes](NOTES.md)')
}
$md.Add('')
$md.Add('## Results')
$md.Add('')
$md.Add('Memory is MiB. CPU is percent of one logical core, so 100% means one core busy. GPU memory columns are median per-run observed peaks; engine utilization is a sum across owned engine instances, not whole-device utilization.')
$md.Add('')
$md.Add('| Implementation | Notes | Private working set | Private bytes | First visible ms | All responsive ms | Idle CPU % | GPU dedicated | GPU shared | GPU engines % |')
$md.Add('| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |')
foreach ($s in $summary) {
    $md.Add("| $($s.Variant) | $($s.Notes) | $(Display $s.PrivateWorkingSetMB) | $(Display $s.PrivateBytesMB) | $(Display $s.StartupMs) | $(Display $s.AllWindowsReadyMs) | $(Display $s.IdleCpuOneCorePercent '0.000') | $(Display $s.GpuDedicatedMiB) | $(Display $s.GpuSharedMiB) | $(Display $s.GpuEngineSumPercent '0.000') |")
}
$md.Add('')
$md.Add('## Variability')
$md.Add('')
$md.Add('Ranges are observed minima–maxima, not confidence intervals. Small startup differences near the polling/scheduling interval should not drive a framework decision.')
$md.Add('')
$md.Add('| Implementation | Notes | Runs | Private working set range MiB | First visible range ms | All responsive range ms |')
$md.Add('| --- | ---: | ---: | ---: | ---: | ---: |')
foreach ($s in $summary) {
    $md.Add("| $($s.Variant) | $($s.Notes) | $($s.Runs) | $(Display $s.PrivateWorkingSetMBMin)–$(Display $s.PrivateWorkingSetMBMax) | $(Display $s.StartupMsMin)–$(Display $s.StartupMsMax) | $(Display $s.AllWindowsReadyMsMin)–$(Display $s.AllWindowsReadyMsMax) |")
}
$md.Add('')
$md.Add('## Build and workload')
$md.Add('')
$md.Add("OS: $($envInfo.OS). CPU: $($envInfo.Cpu.Name -join ', '). GPU: $($envInfo.Gpu.Name -join ', ').")
$md.Add('')
$md.Add("Settling: $($envInfo.SettleSeconds) seconds after all windows respond. Idle sampling: at least $($envInfo.CpuSeconds) seconds; actual duration is recorded per run. Seed: $($envInfo.Seed).")
$md.Add('')
$md.Add('| Implementation | EXE MiB | Input |')
$md.Add('| --- | ---: | --- |')
foreach ($binary in $envInfo.Binaries) { $md.Add("| $($binary.Variant) | $(Display ($binary.Bytes/1MB) '0.00') | $($binary.Input) |") }
$md.Add('')
$md.Add('EXE size is not installed footprint: the C# builds are self-contained compressed single files; framework libraries and native drivers can allocate or extract other data. Rune uses the pinned revision in Experiments/Pinny.Rune/rune-revision.txt; its build notes describe enabled features and compiler settings.')
$md.Add('')
$md.Add('## Geometry and remaining limits')
$md.Add('')
$md.Add('| Implementation | Notes | Observed outer size and window DPI |')
$md.Add('| --- | ---: | --- |')
foreach ($group in ($rows | Group-Object Variant, Notes)) {
    $geometry = @($group.Group.WindowGeometry | Sort-Object -Unique | ForEach-Object {
        $parts = @($_ -split ';')
        $distinct = @($parts | Sort-Object -Unique)
        if ($distinct.Count -eq 1 -and $parts.Count -gt 1) { "$($parts.Count) × $($distinct[0])" } else { $_ }
    }) -join ' / '
    $md.Add("| $($group.Group[0].Variant) | $($group.Group[0].Notes) | $geometry |")
}
$md.Add('')
$md.Add('- Startup ends at visible windows / WM_NULL responsiveness plus a compositor flush. It does **not** prove first rendered text or GPU completion. First-paint instrumentation across all six implementations remains future work.')
$md.Add('- All implementations request the same Light-note workload. Different DPI awareness, layout engines, wrapping and native controls can still produce different physical geometry; inspect the table above before treating the runs as pixel-equivalent.')
$md.Add('- Rust Win32 generates content in memory; the others parse JSON. Rune edits remain in memory and never save the input. The others have different persistence behavior. This is a UI-cost experiment, not full application parity or a storage benchmark.')
$md.Add('- GPU counters are best effort and per process. Unavailable includes no matching instance. Zero is a sampled zero. Driver/compositor allocations outside the process are excluded; GPU and CPU memory can overlap and must not be added into a claimed total.')
$md.Add('- Notes are staggered and overlap, matching the original workload. Initial editor focus/caret behavior is framework-dependent; caret state and occlusion can affect idle activity. Background machine activity and warm OS/driver caches remain uncontrolled.')
$md.Add('- No language-wide performance conclusion follows from these implementations. The active Go application is outside this frozen comparison.')
$md | Set-Content -LiteralPath (Join-Path $Directory 'README.md') -Encoding utf8

$sizeNotes = [Collections.Generic.List[string]]::new()
$sizeNotes.Add('# Size, for curiosity')
$sizeNotes.Add('')
$sizeNotes.Add('A development side note, not a performance ranking. These numbers describe these particular implementations and packaging choices.')
$sizeNotes.Add('')
$sizeNotes.Add('| Implementation | Published EXE MiB | App source files | Source lines | Source KiB |')
$sizeNotes.Add('| --- | ---: | ---: | ---: | ---: |')
foreach ($binary in $envInfo.Binaries) {
    $source = $envInfo.SourceSizes | Where-Object Variant -eq $binary.Variant
    $sizeNotes.Add("| $($binary.Variant) | $(Display ($binary.Bytes/1MB) '0.00') | $($source.Files) | $($source.Lines) | $(Display ($source.Bytes/1KB)) |")
}
$sizeNotes.Add('')
$sizeNotes.Add('- Source lines are physical lines including blanks, comments and any tests in the counted app files. Only .cs, .xaml, .go and .rs files are counted; generated bin/obj/target trees, dependencies, lockfiles, icons and project files are excluded. Shared C# model/storage source is counted in each implementation that compiles it.')
$sizeNotes.Add('- C# executables bundle a compressed .NET runtime. Rust and Go executables include linked runtime/library code but still depend on Windows system components. Rune also uses graphics drivers. EXE size is not installed or in-memory size.')
$sizeNotes.Add('- Rune app source is small partly because the framework supplies layout, text editing, rendering and shell code. Direct Win32 source implements much of that itself. The Rune experiment also has fewer storage features. Lines of code are not a measure of equivalent functionality or maintenance quality.')
$sizeNotes.Add('- SDK installations, compiler caches and framework source checkouts are development-only and are not shipped. Their disk usage depends heavily on toolchain versions, shared caches and which targets have been built, so they are not attributed to a language here.')
$sizeNotes | Set-Content -LiteralPath (Join-Path $Directory 'SIZE.md') -Encoding utf8

# Standalone SVG, with independent scales and zero baselines for each metric.
$panels = @(
    @{ Field='PrivateWorkingSetMB'; Label='Private working set (MiB)' },
    @{ Field='AllWindowsReadyMs'; Label='All windows responsive (ms)' },
    @{ Field='IdleCpuOneCorePercent'; Label='Idle CPU (% of one core)' }
)
$svg = [Collections.Generic.List[string]]::new()
$svg.Add('<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="900" viewBox="0 0 1200 900" role="img" aria-labelledby="title desc">')
$svg.Add('<title id="title">Pinny UI comparison</title><desc id="desc">Median results for one and five notes. Each panel has its own linear scale starting at zero. See README for method and limitations.</desc><rect width="1200" height="900" fill="#f4f7fb"/><g font-family="Segoe UI, sans-serif" fill="#17283e">')
$svg.Add('<text x="30" y="40" font-size="26" font-weight="700">Pinny Windows UI comparison</text><text x="30" y="67" font-size="14">New measurement series · medians · lower is better · see README for limitations</text>')
for ($p=0; $p -lt $panels.Count; $p++) {
    foreach ($count in @(1,5)) {
        $x = if ($count -eq 1) { 30 } else { 620 }; $y=100+$p*250
        $panel=$panels[$p]; $ranked=@($summary | Where-Object Notes -eq $count | Sort-Object $panel.Field)
        $max=($ranked | Measure-Object -Property $panel.Field -Maximum).Maximum
        if ($null -eq $max -or $max -le 0) { $max=1 }
        $svg.Add("<rect x='$x' y='$y' width='550' height='230' rx='12' fill='white'/><text x='$($x+15)' y='$($y+27)' font-size='17' font-weight='600'>$($panel.Label) · $count note(s)</text>")
        for ($i=0; $i -lt $ranked.Count; $i++) {
            $s=$ranked[$i]; $rowY=$y+52+$i*27; $value=$s.($panel.Field)
            $width=([double]$value/$max*270).ToString('0.0',$culture)
            $fill=if ($s.Variant -eq 'Rust Rune') { '#a568d5' } else { '#258c91' }
            $label=[Security.SecurityElement]::Escape($s.Variant)
            $svg.Add("<text x='$($x+15)' y='$($rowY+12)' font-size='13'>$label</text><rect x='$($x+120)' y='$rowY' width='270' height='15' rx='3' fill='#e9eef5'/><rect x='$($x+120)' y='$rowY' width='$width' height='15' rx='3' fill='$fill'/><text x='$($x+405)' y='$($rowY+12)' font-size='13'>$(Display $value '0.000')</text>")
        }
    }
}
$svg.Add('<text x="30" y="880" font-size="13">Private working set excludes some GPU/compositor memory. Responsive windows are not a first-paint measurement.</text></g></svg>')
$svg | Set-Content -LiteralPath (Join-Path $Directory 'comparison.svg') -Encoding utf8
