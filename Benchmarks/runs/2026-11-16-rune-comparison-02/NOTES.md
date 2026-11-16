# Run notes: 16 November 2026

This is a fresh six-implementation series, not an update to the September data.
Read [method changes and reasons](../../COMPARISON.md) before comparing dates.
The [generated report](README.md) contains the current measurements; this file
records build/verification context and interpretation separately so regenerating
tables does not erase the narrative.

## What these results suggest

Rune is a useful experiment for richer styling and a potential cross-platform UI,
but this prototype does not match the direct Win32 implementations' memory cost.
Its median private working set rises from **52.6 MiB with one note to 157.7 MiB
with five**, compared with **2.7 MiB for Go Win32** and **1.6 MiB for Rust Win32**
at both counts after rounding. Rune's committed private bytes are **116.0/303.5 MiB**,
versus Go's **14.0/14.0 MiB**, so this is not just a difference in resident pages.
GPU memory is reported separately and must not be added to those figures.

Rune's first-visible-window medians are **44.5/50.5 ms**, and all-window
responsiveness medians are **50.2/111.3 ms**, for one/five notes. This is quick for
the workload, but the small differences from direct Win32 are affected by polling,
scheduling and different input paths. They do not establish faster first paint.
WPF's private working set is **96.1/99.1 MiB**, illustrating that window-count
scaling matters: Rune is below WPF with one note and above it with five here.

For Pinny's current small Windows note windows, these results support keeping the
active Go app while using Rune as an isolated design/portability experiment.
They do not show whether further Rune-specific tuning would reduce its per-window
cost, and do not measure cross-platform behavior or development effort.

The optional [size note](SIZE.md) records Rune's **29.06 MiB** executable alongside
Go's **2.73 MiB** and Rust Win32's **0.28 MiB**, plus physical app-source lines and
bytes for all six options. Different features and bundled runtimes limit that
comparison; source length is development trivia, not a quality score.

## Build context

- All five original experiment sources were rebuilt without source changes.
- C# uses the original locally cached .NET SDK 8.0.425 and the existing
  self-contained, single-file, compressed Release publish settings. The system
  `dotnet` command has no SDK, so publishing used its explicit cached SDK path.
- Rust Win32 uses rustc 1.98.1 GNU and its existing optimized/LTO configuration.
- Go Win32 uses Go 1.27.1 Windows/amd64 and the existing stripped GUI build flags.
- Rune uses rustc 1.99.1 GNU, pinned upstream revision
  `eb5ae6c9f77b1a0051d8e1e5ee936ebea086bb90` and the new Cargo.lock.
  Its optimized build follows Rune's perf settings (16 codegen units, no LTO,
  no incremental compilation), with symbols stripped.
- Rune enables accessibility and loads Windows Segoe UI; unused embedded Inter,
  audio, SVG and other-platform service/backend features are disabled. Agent
  tooling is absent from the measured release build.
- GNU binutils 2.47.20260726 and its runtime DLLs were unpacked under ignored
  `dist/binutils/` for development. PE import inspection found Windows system DLLs,
  not a required packaged MinGW runtime DLL. The experiment ran without adding
  that development binutils directory to the application's launch PATH.
- Rustup installed the requested toolchain successfully; its optional subsequent
  self-update failed. The experiment compiled and ran with the installed 1.99.1
  toolchain. No benchmark result depends on that updater.

The environment JSON records the default toolchain as `Tools.Rust` (Rust Win32)
and the explicitly selected Rune toolchain as `Tools.RuneRust`.
Executable hashes and sizes are captured before measurement. Build dependencies,
framework checkout, SDKs, executables and synthetic inputs are ignored by Git.

## Windows verification

Before the full timed run, the new Rune executable passed a live Windows UI check:

- A synthetic note rendered with the intended header, palette and editable text.
- Typed Unicode (`✓`) appeared, long text wrapped, and Enter inserted a new line.
- Pin toggled its visual state and requested Rune's native always-on-top setting.
- The menu opened and changing to Dark updated the settled rendered appearance.
- New created a second independent native note window in the same process.
- Quit closed both note windows.

A separate short harness smoke run measured Rune with one and five notes and
generated the raw CSV, GPU samples, Markdown tables, size note and SVG. Its output
is ignored under `Benchmarks/.data/rune-harness-smoke/`; those short samples are
not mixed into the full comparison.

This does not establish full editing, accessibility, IME, resize, drag, save/reload
or multi-monitor parity. The prototype reads JSON but does not save any edits.
The timed workload measures startup and idle cost, not typing latency or a full
application feature suite. All-window responsiveness remains distinct from first
rendered text; no first-paint ranking is claimed.

## Why this is attempt 02

The [first attempt](../2026-11-16-rune-comparison/README.md) stopped after eight
measurements when a wildcard Windows performance-counter query returned an
invalid sample. Its files remain intact and its measurements are not pooled here.
The new sampler ignores invalid entries from unrelated processes and retries
when the measured process lacks a valid ID/working-set pair. GPU samples are
also filtered by validity. Regression checks cover that failure, even-count
medians, missing GPU values and report/SVG generation. Binaries, input workload,
repetition count and timing parameters did not change between attempts.

## Completed-run checks

- All 60 measured rows are present: six variants × two note counts × five runs.
- Every row has the requested number of separate 320 × 320 windows at 96 DPI.
- Every recorded idle interval is at least 15 seconds; all compositor flushes succeeded.
- All six executable SHA-256 hashes still match the pre-run metadata.
- Regression checks, Rust formatting, generated SVG XML and whitespace checks passed.
- Git diff checks confirm that the original five experiment sources, historical
  benchmark scripts, CSV, SVG and PNG are unchanged. The root README only adds
  navigation to the new series.

Do not attribute the difference between September and November values to a single
optimization or language change. The OS build, tools/dependencies, sample windows
and machine state differ; this suite is a fresh within-run comparison.
