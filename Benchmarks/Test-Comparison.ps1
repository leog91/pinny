$ErrorActionPreference = 'Stop'
$tokens = $null; $parseErrors = $null
$ast = [Management.Automation.Language.Parser]::ParseFile((Join-Path $PSScriptRoot 'Measure-Comparison.ps1'), [ref]$tokens, [ref]$parseErrors)
if ($parseErrors) { throw $parseErrors }
# Load the sampler alone: regression checks must not launch benchmark windows.
$sampler = $ast.Find({ param($node) $node -is [Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -eq 'Get-ProcessCounters' }, $true)
Invoke-Expression $sampler.Extent.Text
$script:counterCalls = 0
function Get-Counter {
    [CmdletBinding()]param([string[]]$Counter)
    $script:counterCalls++
    Write-Error 'An unrelated process exited during the wildcard query'
    [pscustomobject]@{ CounterSamples = @(
        [pscustomobject]@{ Status=99; Path='\process(gone)\id process'; InstanceName='gone'; CookedValue=42 },
        [pscustomobject]@{ Status=0; Path='\process(pinny)\id process'; InstanceName='pinny'; CookedValue=42 },
        [pscustomobject]@{ Status=$(if ($script:counterCalls -eq 1) { 99 } else { 0 }); Path='\process(pinny)\working set - private'; InstanceName='pinny'; CookedValue=123456 }
    ) }
}
$value = Get-ProcessCounters 42
if ($value -ne 123456 -or $script:counterCalls -ne 2) { throw 'Sampler failed valid-target filtering/retry regression' }
Remove-Item Function:\Get-Counter

$fixture = Join-Path $PSScriptRoot ('.data\report-test-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $fixture -Force | Out-Null
$rows = @(foreach ($count in @(1,5)) {
    foreach ($value in @(2,8)) {
        [pscustomobject]@{ Variant='Fixture'; Notes=$count; PrivateWorkingSetMB=$value; PrivateBytesMB=$value; StartupMs=$value; AllWindowsReadyMs=$value; IdleCpuOneCorePercent=0; GpuDedicatedMiB=''; GpuSharedMiB=''; GpuEngineSumPercent=''; WindowGeometry='320x320@96dpi' }
    }
})
$rows | Export-Csv -LiteralPath (Join-Path $fixture 'results.csv') -NoTypeInformation
@{ StartedUtc='2026-11-16T00:00:00Z'; Repetitions=2; Warmup=$true; OS='fixture'; Cpu=@{Name='fixture'}; Gpu=@{Name='fixture'}; SettleSeconds=1; CpuSeconds=2; Seed=1; Binaries=@(@{Variant='Fixture';Bytes=1048576;Input='fixture'}); SourceSizes=@(@{Variant='Fixture';Files=1;Lines=10;Bytes=1024}) } |
    ConvertTo-Json -Depth 5 | Set-Content -LiteralPath (Join-Path $fixture 'environment.json')
& (Join-Path $PSScriptRoot 'Write-ComparisonReport.ps1') -Directory $fixture
$summary = @(Import-Csv -LiteralPath (Join-Path $fixture 'summary.csv'))
if ($summary.Count -ne 2 -or [double]$summary[0].PrivateWorkingSetMB -ne 5) { throw 'Even-sample median regression' }
if ($summary[0].GpuDedicatedMiB -ne '') { throw 'Missing GPU counters must remain unavailable' }
if ((Get-Content -LiteralPath (Join-Path $fixture 'README.md') -Raw) -notmatch 'unavailable') { throw 'Missing-GPU report label regression' }
[xml](Get-Content -LiteralPath (Join-Path $fixture 'comparison.svg') -Raw) | Out-Null
if (-not (Test-Path -LiteralPath (Join-Path $fixture 'SIZE.md'))) { throw 'Size side note missing' }
Write-Host 'Passed: target-counter validation/retry, even median, unavailable GPU values, Markdown/size report and SVG XML.'
