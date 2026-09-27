param([string]$ResultsPath = (Join-Path $PSScriptRoot 'results.csv'))

$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Drawing
$culture = [System.Globalization.CultureInfo]::CurrentCulture
$invariant = [System.Globalization.CultureInfo]::InvariantCulture
$rows = @(Import-Csv -LiteralPath $ResultsPath)
if ($rows.Count -eq 0) { throw "No benchmark measurements found: $ResultsPath" }
$runCount = @($rows | Where-Object { $_.Variant -eq 'WPF' -and [int]$_.Notes -eq 1 }).Count

$variants = @(
    [pscustomobject]@{ Key = 'WPF'; Label = 'C# WPF'; Color = '#8191a7' },
    [pscustomobject]@{ Key = 'WinForms'; Label = 'C# WinForms'; Color = '#4f97d0' },
    [pscustomobject]@{ Key = 'Win32'; Label = 'C# Win32'; Color = '#1ba482' },
    [pscustomobject]@{ Key = 'Rust Win32'; Label = 'Rust Win32'; Color = '#a568d5' },
    [pscustomobject]@{ Key = 'Go Win32'; Label = 'Go Win32'; Color = '#e39a44' }
)
$panels = @(
    [pscustomobject]@{ Title = 'Private working set · one note'; Notes = 1; Field = 'PrivateWorkingSetMB'; Unit = 'MiB'; Max = 120; X = 50; Y = 105 },
    [pscustomobject]@{ Title = 'Private working set · five notes'; Notes = 5; Field = 'PrivateWorkingSetMB'; Unit = 'MiB'; Max = 120; X = 710; Y = 105 },
    [pscustomobject]@{ Title = 'First-window startup · one note'; Notes = 1; Field = 'StartupMs'; Unit = 'ms'; Max = 1000; X = 50; Y = 485 },
    [pscustomobject]@{ Title = 'First-window startup · five notes'; Notes = 5; Field = 'StartupMs'; Unit = 'ms'; Max = 1000; X = 710; Y = 485 }
)

function Get-Median([string]$variant, [int]$notes, [string]$field) {
    $values = @($rows | Where-Object { $_.Variant -eq $variant -and [int]$_.Notes -eq $notes } |
        ForEach-Object { [double]::Parse($_.$field, $culture) } | Sort-Object)
    if ($values.Count -eq 0) { throw "Missing $variant / $notes note(s) in $ResultsPath" }
    return $values[[int][math]::Floor($values.Count / 2)]
}

$svg = [System.Collections.Generic.List[string]]::new()
$svg.Add('<svg xmlns="http://www.w3.org/2000/svg" width="1400" height="880" viewBox="0 0 1400 880" role="img" aria-labelledby="title description">')
$svg.Add('  <title id="title">Pinny benchmark: five Windows implementations</title>')
$svg.Add('  <desc id="description">Median private working set and first-window startup for C# WPF, C# WinForms, C# Win32, Rust Win32, and Go Win32, with one and five synthetic notes.</desc>')
$svg.Add('  <rect width="1400" height="880" fill="#f4f7fb"/>')
$svg.Add('  <text x="50" y="57" fill="#17283e" font-family="Segoe UI, Arial, sans-serif" font-size="32" font-weight="700">Pinny: five Windows implementations</text>')
$svg.Add("  <text x=`"50`" y=`"84`" fill=`"#65758a`" font-family=`"Segoe UI, Arial, sans-serif`" font-size=`"15`">Median of $runCount runs · same PC · synthetic Light-theme notes · lower is better</text>")

$bitmap = [System.Drawing.Bitmap]::new(1400, 880)
$graphics = [System.Drawing.Graphics]::FromImage($bitmap)
$graphics.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::AntiAlias
$graphics.TextRenderingHint = [System.Drawing.Text.TextRenderingHint]::AntiAliasGridFit
$background = [System.Drawing.SolidBrush]::new([System.Drawing.ColorTranslator]::FromHtml('#f4f7fb'))
$white = [System.Drawing.SolidBrush]::new([System.Drawing.Color]::White)
$dark = [System.Drawing.SolidBrush]::new([System.Drawing.ColorTranslator]::FromHtml('#17283e'))
$muted = [System.Drawing.SolidBrush]::new([System.Drawing.ColorTranslator]::FromHtml('#65758a'))
$track = [System.Drawing.SolidBrush]::new([System.Drawing.ColorTranslator]::FromHtml('#e9eef5'))
$titleFont = [System.Drawing.Font]::new('Segoe UI', 25, [System.Drawing.FontStyle]::Bold)
$panelFont = [System.Drawing.Font]::new('Segoe UI', 17, [System.Drawing.FontStyle]::Bold)
$bodyFont = [System.Drawing.Font]::new('Segoe UI', 12)
$smallFont = [System.Drawing.Font]::new('Segoe UI', 10)

try {
    $graphics.FillRectangle($background, 0, 0, 1400, 880)
    $graphics.DrawString('Pinny: five Windows implementations', $titleFont, $dark, 50.0, 20.0)
    $graphics.DrawString("Median of $runCount runs · same PC · synthetic Light-theme notes · lower is better", $bodyFont, $muted, 50.0, 64.0)

    foreach ($panel in $panels) {
        $x = [int]$panel.X
        $y = [int]$panel.Y
        $svg.Add("  <rect x=`"$x`" y=`"$y`" width=`"640`" height=`"350`" rx=`"18`" fill=`"#ffffff`" stroke=`"#e3e9f1`"/>")
        $svg.Add("  <text x=`"$($x + 26)`" y=`"$($y + 42)`" fill=`"#17283e`" font-family=`"Segoe UI, Arial, sans-serif`" font-size=`"23`" font-weight=`"700`">$($panel.Title)</text>")
        $graphics.FillRectangle($white, $x, $y, 640, 350)
        $graphics.DrawString($panel.Title, $panelFont, $dark, [float]($x + 26), [float]($y + 15))

        $ranked = @($variants | ForEach-Object {
            [pscustomobject]@{
                Variant = $_
                Value = Get-Median $_.Key $panel.Notes $panel.Field
            }
        } | Sort-Object Value, @{ Expression = { $_.Variant.Label } })
        for ($i = 0; $i -lt $ranked.Count; $i++) {
            $variant = $ranked[$i].Variant
            $value = [double]$ranked[$i].Value
            $width = [int][math]::Round([math]::Min($value / $panel.Max, 1) * 330)
            $rowY = $y + 94 + $i * 48
            $display = if ($panel.Unit -eq 'MiB') { $value.ToString('0.0', $invariant) } else { $value.ToString('0', $invariant) }
            $svg.Add("  <text x=`"$($x + 26)`" y=`"$($rowY + 12)`" fill=`"$($variant.Color)`" font-family=`"Segoe UI, Arial, sans-serif`" font-size=`"15`" font-weight=`"600`">$($variant.Label)</text>")
            $svg.Add("  <rect x=`"$($x + 175)`" y=`"$rowY`" width=`"330`" height=`"15`" rx=`"7`" fill=`"#e9eef5`"/>")
            $svg.Add("  <rect x=`"$($x + 175)`" y=`"$rowY`" width=`"$width`" height=`"15`" rx=`"7`" fill=`"$($variant.Color)`"/>")
            $svg.Add("  <text x=`"$($x + 610)`" y=`"$($rowY + 12)`" text-anchor=`"end`" fill=`"#17283e`" font-family=`"Segoe UI, Arial, sans-serif`" font-size=`"15`" font-weight=`"600`">$display $($panel.Unit)</text>")

            $colorBrush = [System.Drawing.SolidBrush]::new([System.Drawing.ColorTranslator]::FromHtml($variant.Color))
            try {
                $graphics.DrawString($variant.Label, $bodyFont, $colorBrush, [float]($x + 26), [float]($rowY - 4))
                $graphics.FillRectangle($track, $x + 175, $rowY, 330, 15)
                $graphics.FillRectangle($colorBrush, $x + 175, $rowY, $width, 15)
                $graphics.DrawString("$display $($panel.Unit)", $bodyFont, $dark, [float]($x + 520), [float]($rowY - 4))
            }
            finally { $colorBrush.Dispose() }
        }
    }
    $svg.Add('  <text x="50" y="859" fill="#65758a" font-family="Segoe UI, Arial, sans-serif" font-size="13">Memory sampled 2 seconds after first window. Startup ends at first window handle; five-note startup does not mean all windows are ready.</text>')
    $svg.Add('</svg>')
    $graphics.DrawString('Memory sampled 2 seconds after first window. Startup ends at first window handle.', $smallFont, $muted, 50.0, 838.0)

    [System.IO.File]::WriteAllLines((Join-Path $PSScriptRoot 'benchmark.svg'), $svg, [System.Text.UTF8Encoding]::new($false))
    $bitmap.Save((Join-Path $PSScriptRoot 'benchmark.png'), [System.Drawing.Imaging.ImageFormat]::Png)
}
finally {
    $graphics.Dispose()
    $bitmap.Dispose()
    foreach ($item in @($background, $white, $dark, $muted, $track, $titleFont, $panelFont, $bodyFont, $smallFont)) { $item.Dispose() }
}
