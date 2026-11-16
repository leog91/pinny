# New comparison series: Rune and the five existing options

The September 2026 benchmark remains the historical reference: [report](README.md),
[raw results](results.csv), [SVG](benchmark.svg), and [PNG](benchmark.png).
Its scripts and five experiment implementations are unchanged. New runs go into
their own folders under `runs/`; never combine September rows with a new run or
overwrite a previous run directory.

The new series adds an independent [Rune experiment](../Experiments/Pinny.Rune/README.md).
The active `Pinny.Go/` application is outside this comparison. No local notes or
cloud credentials are used. All GUI builds and measurements run locally on Windows.

## Recorded series

| Series | Status | Where to read |
| --- | --- | --- |
| 27 September 2026 | Original five-build reference; 30 measurements; unchanged | [Historical report](README.md) |
| 16 November 2026, attempt 02 | Completed six-build comparison; 60 measurements, matching window counts and 320 × 320 geometry at 96 DPI | [Results and chart](runs/2026-11-16-rune-comparison-02/README.md) · [Findings and checks](runs/2026-11-16-rune-comparison-02/NOTES.md) · [Size, for curiosity](runs/2026-11-16-rune-comparison-02/SIZE.md) |
| 16 November 2026, attempt 01 | Interrupted after eight measurements by a transient counter error; retained, excluded from rankings | [Reason and preserved data](runs/2026-11-16-rune-comparison/README.md) |

The new Rune prototype starts quickly in this workload but uses substantially more
private working-set memory than the direct Win32 builds, especially with five
windows. The new report separates memory measures, GPU counters, startup endpoints,
size trivia and remaining feature differences; it does not replace the older table.

## What changed, and why

| Area | September suite (preserved) | New comparison suite | Reason |
| --- | --- | --- | --- |
| Candidates | Five frozen implementations | Those same five plus Rust/Rune | Measure a GPU-rendered framework alongside direct Win32 and .NET options |
| Results | One historical CSV and chart | A fresh output directory per invocation | Keep earlier evidence and methods reviewable |
| Input | One/five Light notes; JSON except Rust Win32 | Same text, count, positions and requested dimensions; Rune reads the same JSON | Reduce workload differences without modifying frozen sources |
| Repetition/order | Three runs, implementation by implementation | Five measured runs by default, one warmup per implementation/count, seeded shuffle each round | Reduce fixed-order and first-use effects and expose variability |
| Startup | First visible note, 20 ms polling | First visible note and all requested notes visible/responding, 10 ms polling plus DwmFlush | Avoid claiming one window means five are ready; retain an explicitly limited startup measure |
| Settle/idle | Two-second settle, three-second CPU sample | Five-second settle, at least 15 seconds of CPU/GPU sampling | Better view of sustained idle behavior |
| CPU | CPU milliseconds | CPU milliseconds, actual elapsed duration and percent of one core | Make samples of slightly different duration comparable |
| GPU | Not measured | Per-process dedicated/shared usage and summed engine utilization, raw samples retained | GPU rendering can move costs outside the old RAM metric |
| Auditability | Raw rows and method notes | Raw rows, ranges, binary hashes, compiler versions, CPU/GPU/OS, execution order and observed window geometry/DPI | Make build and environment differences visible |
| Size | Not part of result table | A separate SIZE.md side note: EXE bytes and app source lines/bytes | Useful development trivia without confusing it with runtime cost |

The new startup endpoint is **not first rendered text**. WM_NULL checks event-loop
responsiveness; DwmFlush waits for a compositor cycle. Neither proves a framework
has finished submitting its first content frame. Equivalent first-paint instrumentation
across all six builds is future work and is not claimed by this series.

## Run it

Use the existing Windows build instructions for the frozen five:

```powershell
.\Benchmarks\Publish-Baseline.ps1
.\Benchmarks\Build-Native.ps1
```

If .NET 8 is installed in a non-default location, pass its `dotnet.exe` with
`Publish-Baseline.ps1 -Dotnet <path>`. Then follow the Rune build prerequisites:

```powershell
.\Benchmarks\Build-Rune.ps1 -BinutilsDirectory <MinGW-w64-bin-directory>
.\Benchmarks\Test-Comparison.ps1
.\Benchmarks\Measure-Comparison.ps1
```

The runner preflights all selected executable paths, then creates a new folder.
It rejects an existing output directory. Do not run builds or other experiments
while collecting results; leave the desktop unlocked and avoid interacting with
the note windows. It stops only the process launched for that measurement.
Synthetic data goes under ignored `Benchmarks/.data/`. Rune never writes to its
input; the other saved-note builds use separate per-run folders.

Defaults cover all six candidates and both note counts. A short validation run
can select fewer candidates and smaller sample windows; label it as a smoke run,
not the full comparison:

```powershell
.\Benchmarks\Measure-Comparison.ps1 -Variants 'Rust Rune' -Repetitions 1 -SettleSeconds 1 -CpuSeconds 2
```

Each completed run produces `README.md`, `SIZE.md`, `results.csv`, `summary.csv`,
`gpu-samples.csv`, `environment.json` and `comparison.svg`. An interrupted run
retains completed raw rows but is not a full result series. Regenerate a report
without rerunning measurements:

```powershell
.\Benchmarks\Write-ComparisonReport.ps1 -Directory <completed-run-directory>
```

## How close is the comparison?

| Option | Editor/rendering approach | Input and persistence in this suite | Platform scope of this implementation |
| --- | --- | --- | --- |
| C# WPF | WPF TextBox and WPF layout/rendering | Loads isolated JSON; supports saving | Windows |
| C# WinForms | Windows Forms TextBox and form controls | Loads isolated JSON; supports saving | Windows |
| C# Win32 | Painted header and native multiline EDIT | Loads isolated JSON; supports saving | Windows |
| Rust Win32 | Direct Win32 painting and native multiline EDIT | Constructs matching text in memory; no persistence | Windows |
| Go Win32 | Direct Win32 painting and native multiline EDIT | Loads isolated JSON; supports saving | Windows |
| Rust Rune | Rune text editor, retained layout and GPU renderer | Reads isolated JSON; edits remain in memory | Framework targets Windows/macOS/Linux/web; this prototype is verified only on Windows |

This compares UI implementation costs for a small shared workload. It is not a
full port or proof of feature equivalence. Rust Win32 constructs content in memory;
the others parse JSON. Rune does not save edits. Different native editing controls,
font metrics, wrapping, DPI awareness and window borders may change visible results.
The report records outer geometry and DPI so those differences cannot disappear
behind a single bar chart. Windows overlap in the original staggered arrangement;
this is not a test of five fully exposed windows or a minimized application.

CPU memory and GPU counters are separate views, with possible overlap; do not add
them into a supposed total. Missing GPU instances/counters are `unavailable`, not
zero. Engine percentages are summed across the process's engines/adapters and are
not a utilization percentage for the whole GPU. OS compositor and driver costs
outside the process remain excluded.

Use medians and observed ranges together. Warm-cache startup does not measure cold
disk, first driver use after boot, or completion of all rendering. Five runs give
more evidence than three but do not establish statistical significance. Compare
implementations within a new series; the old/new tables also differ in methods,
toolchains, dependencies and machine state.
