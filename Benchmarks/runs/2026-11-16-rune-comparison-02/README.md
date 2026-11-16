# Pinny Windows UI comparison

Recorded UTC: 2026-11-16 20:37:22. Medians of 5 measured runs per implementation and note count; warmup: True.

This is a new measurement series. The [September 2026 report](../../README.md), CSV, charts, scripts, and five original experiment sources remain unchanged. See [method changes and reasons](../../COMPARISON.md).

[Raw runs](results.csv) · [Summary CSV](summary.csv) · [GPU samples](gpu-samples.csv) · [Environment and binary hashes](environment.json) · [Chart](comparison.svg) · [Size, for curiosity](SIZE.md)

[Build, verification and interpretation notes](NOTES.md)

## Results

Memory is MiB. CPU is percent of one logical core, so 100% means one core busy. GPU memory columns are median per-run observed peaks; engine utilization is a sum across owned engine instances, not whole-device utilization.

| Implementation | Notes | Private working set | Private bytes | First visible ms | All responsive ms | Idle CPU % | GPU dedicated | GPU shared | GPU engines % |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| Rust Win32 | 1 | 1.6 | 2.3 | 33.3 | 39.3 | 0.000 | unavailable | unavailable | unavailable |
| Go Win32 | 1 | 2.7 | 14.0 | 32.0 | 45.3 | 0.000 | unavailable | unavailable | unavailable |
| Win32 | 1 | 20.7 | 23.0 | 124.7 | 137.8 | 0.000 | unavailable | unavailable | unavailable |
| WinForms | 1 | 43.4 | 46.2 | 232.3 | 251.1 | 0.000 | unavailable | unavailable | unavailable |
| Rust Rune | 1 | 52.6 | 116.0 | 44.5 | 50.2 | 0.098 | 26.9 | 31.4 | 0.000 |
| WPF | 1 | 96.1 | 132.7 | 642.9 | 650.3 | 0.503 | 7.6 | 3.2 | 0.010 |
| Rust Win32 | 5 | 1.6 | 2.3 | 33.9 | 92.8 | 0.000 | unavailable | unavailable | unavailable |
| Go Win32 | 5 | 2.7 | 14.0 | 34.7 | 109.4 | 0.000 | unavailable | unavailable | unavailable |
| Win32 | 5 | 20.7 | 23.0 | 126.3 | 184.0 | 0.000 | unavailable | unavailable | unavailable |
| WinForms | 5 | 44.2 | 46.9 | 221.3 | 325.1 | 0.000 | unavailable | unavailable | unavailable |
| WPF | 5 | 99.1 | 143.5 | 658.5 | 759.9 | 0.488 | 9.5 | 3.4 | 0.013 |
| Rust Rune | 5 | 157.7 | 303.5 | 50.5 | 111.3 | 0.000 | 91.5 | 115.9 | 0.000 |

## Variability

Ranges are observed minima–maxima, not confidence intervals. Small startup differences near the polling/scheduling interval should not drive a framework decision.

| Implementation | Notes | Runs | Private working set range MiB | First visible range ms | All responsive range ms |
| --- | ---: | ---: | ---: | ---: | ---: |
| Rust Win32 | 1 | 5 | 1.6–1.6 | 30.6–33.9 | 38.2–41.8 |
| Go Win32 | 1 | 5 | 2.6–2.8 | 29.9–33.4 | 42.9–46.3 |
| Win32 | 1 | 5 | 20.6–20.7 | 123.7–130.0 | 137.3–141.4 |
| WinForms | 1 | 5 | 43.3–43.9 | 217.7–237.7 | 241.8–263.2 |
| Rust Rune | 1 | 5 | 52.3–52.7 | 35.1–49.1 | 42.8–55.0 |
| WPF | 1 | 5 | 93.8–100.3 | 632.7–653.6 | 639.9–658.6 |
| Rust Win32 | 5 | 5 | 1.6–1.6 | 24.7–48.6 | 86.0–103.0 |
| Go Win32 | 5 | 5 | 2.7–2.8 | 30.6–45.6 | 90.2–110.7 |
| Win32 | 5 | 5 | 20.7–20.8 | 124.0–139.3 | 179.7–204.4 |
| WinForms | 5 | 5 | 43.9–44.6 | 220.1–251.4 | 311.4–343.5 |
| WPF | 5 | 5 | 98.8–99.2 | 639.9–681.6 | 729.5–779.0 |
| Rust Rune | 5 | 5 | 157.2–158.4 | 48.8–63.3 | 107.5–118.8 |

## Build and workload

OS: Microsoft Windows NT 10.0.26300.0. CPU: AMD Ryzen 5 7600X 6-Core Processor             . GPU: AMD Radeon(TM) Graphics, NVIDIA GeForce RTX 4070.

Settling: 5 seconds after all windows respond. Idle sampling: at least 15 seconds; actual duration is recorded per run. Seed: 20261116.

| Implementation | EXE MiB | Input |
| --- | ---: | --- |
| WPF | 68.31 | JSON |
| WinForms | 68.31 | JSON |
| Win32 | 33.49 | JSON |
| Rust Win32 | 0.28 | synthetic |
| Go Win32 | 2.73 | JSON |
| Rust Rune | 29.06 | JSON (read only) |

EXE size is not installed footprint: the C# builds are self-contained compressed single files; framework libraries and native drivers can allocate or extract other data. Rune uses the pinned revision in Experiments/Pinny.Rune/rune-revision.txt; its build notes describe enabled features and compiler settings.

## Geometry and remaining limits

| Implementation | Notes | Observed outer size and window DPI |
| --- | ---: | --- |
| Go Win32 | 1 | 320x320@96dpi |
| Go Win32 | 5 | 5 × 320x320@96dpi |
| Rust Rune | 1 | 320x320@96dpi |
| Rust Rune | 5 | 5 × 320x320@96dpi |
| Rust Win32 | 1 | 320x320@96dpi |
| Rust Win32 | 5 | 5 × 320x320@96dpi |
| Win32 | 1 | 320x320@96dpi |
| Win32 | 5 | 5 × 320x320@96dpi |
| WinForms | 1 | 320x320@96dpi |
| WinForms | 5 | 5 × 320x320@96dpi |
| WPF | 1 | 320x320@96dpi |
| WPF | 5 | 5 × 320x320@96dpi |

- Startup ends at visible windows / WM_NULL responsiveness plus a compositor flush. It does **not** prove first rendered text or GPU completion. First-paint instrumentation across all six implementations remains future work.
- All implementations request the same Light-note workload. Different DPI awareness, layout engines, wrapping and native controls can still produce different physical geometry; inspect the table above before treating the runs as pixel-equivalent.
- Rust Win32 generates content in memory; the others parse JSON. Rune edits remain in memory and never save the input. The others have different persistence behavior. This is a UI-cost experiment, not full application parity or a storage benchmark.
- GPU counters are best effort and per process. Unavailable includes no matching instance. Zero is a sampled zero. Driver/compositor allocations outside the process are excluded; GPU and CPU memory can overlap and must not be added into a claimed total.
- Notes are staggered and overlap, matching the original workload. Initial editor focus/caret behavior is framework-dependent; caret state and occlusion can affect idle activity. Background machine activity and warm OS/driver caches remain uncontrolled.
- No language-wide performance conclusion follows from these implementations. The active Go application is outside this frozen comparison.
