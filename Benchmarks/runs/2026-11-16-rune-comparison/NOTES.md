# Run notes: 16 November 2026

This is a fresh six-implementation series, not an update to the September data.
Read [method changes and reasons](../../COMPARISON.md) before comparing dates.
The [generated report](README.md) contains the current measurements; this file
records build/verification context and interpretation separately so regenerating
tables does not erase the narrative.

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

The environment JSON's `Tools.Rust` is the default Rust toolchain (Rust Win32),
not the explicitly selected Rune toolchain. The distinction above is intentional.
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
