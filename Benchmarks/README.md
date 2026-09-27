# Pinny UI benchmark

![Pinny benchmark chart](benchmark.svg)

[Download the PNG chart](benchmark.png) · [View the vector SVG](benchmark.svg)

The chart archives the 27 September 2026 comparison of the three simple **C# / .NET 8** Pinny builds on one Windows x64 PC. Values are medians of three runs. Lower is better for both measures. The corresponding source is frozen under `Experiments/`; ongoing development happens in `Pinny/`.

| UI | Private working set, 1 note | Private working set, 5 notes | First-window startup, 1 note |
| --- | ---: | ---: | ---: |
| WPF | 89.2 MiB | 92.9 MiB | 727 ms |
| WinForms | 43.1 MiB | 43.5 MiB | 259 ms |
| Win32 | 20.2 MiB | 20.3 MiB | 136 ms |

In this run, the Win32 version used about 77% less private working set than WPF with one note. It also reached its first window about 81% sooner. These are historical measurements of the simple Pinny versions, not general benchmarks of the frameworks or current measurements of the developing app.

## Test setup

- **Recorded:** 27 September 2026 on Windows x64, OS build 26200.
- **Builds:** .NET 8, Release, self-contained, single-file, compressed. Win32 uses the regular .NET runtime; **Native AOT was not measured**.
- **Data:** One or five synthetic Light-theme notes, each 320 × 320 pixels with two short lines of plain text. No personal note data was used.
- **Runs:** Three per UI and note count, run sequentially. The chart shows the median for each metric.
- **RAM:** Windows `Process\Working Set - Private`, sampled two seconds after the first window appeared. One MiB is 1,048,576 bytes. The raw CSV's `MB` columns use this same binary unit.
- **Startup:** Elapsed time from launching the process until its first top-level window handle appeared. This is a warm-cache measurement and does **not** mean every window is ready in the five-note case.
- **Idle CPU:** The script also samples CPU time for the following three seconds. It is in the raw data but omitted from the chart because that short window is noisy and cannot establish sustained idle behavior.

The three interfaces have different rendering and controls. Private working set describes the app process; it does not include all desktop compositor or GPU memory. Note content, window size, themes, OS state, and other processes can change the results.

## Raw data and reproduction

- [All 18 measurements](results-2026-09-27.csv)
- [Benchmark script](Measure-Pinny.ps1)
- [Architecture and technical decisions](../TECHNICAL.md)
- [Publish script](Publish-Baseline.ps1)
- [Experiment source and freeze policy](../Experiments/README.md)

On Windows with the .NET 8 SDK, publish the three frozen builds and then measure from the repository root:

```powershell
.\Benchmarks\Publish-Baseline.ps1
.\Benchmarks\Measure-Pinny.ps1
```

The publish script writes self-contained executables under `dist\baseline-2026-09-27\`. The measurement script writes a local `Benchmarks\results.csv` and prints fresh medians. It uses isolated synthetic note folders under `Benchmarks\.data` and stops only the processes it launches. All generated paths are ignored by Git. The archived `results-2026-09-27.csv`, chart, and table stay unchanged when rerunning the scripts. For a new published comparison, review the new run and give its CSV and chart a new date.
