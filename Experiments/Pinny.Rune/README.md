# Pinny Rune experiment

An isolated Rust/Rune UI experiment for the [new comparison series](../../Benchmarks/COMPARISON.md).
It does not replace the active Go app or modify the five frozen comparisons.

Rune is pinned to [`eb5ae6c9f77b1a0051d8e1e5ee936ebea086bb90`](https://github.com/anomalyco/rune/tree/eb5ae6c9f77b1a0051d8e1e5ee936ebea086bb90)
in `rune-revision.txt`. It is a pre-release GPU-rendered framework. Its source and
required upstream patches are materialized under ignored `dist/rune-source/`;
framework/dependency/runtime files are not copied into tracked source. Cargo.lock
records the application dependency resolution.

## Scope

- Separate Light-theme note windows with a 32-point header, Segoe UI text, multiline
  editing, New, Menu, Pin and Close, plus Light/Dark/Paper and Quit menu items.
- `--data-dir PATH` reads the same `notes.json` fixture as the other saved-note
  implementations, including UTF-8 BOM input. It never saves edits or accesses
  the active application's default data folder.
- `--notes COUNT` creates the matching two-line synthetic workload in memory.
  Without either flag it opens one blank in-memory note. Count is limited to 1–100.
- Pin requests always-on-top through Rune. New windows share the event loop/GPU
  device, with per-window rendering state. No sync, tray or background networking.

This is a UI-cost prototype. JSON persistence, full product behavior, accessibility
quality, IME behavior and desktop integration are not considered equivalent simply
because controls are present. Windows smoke-check results belong in the run notes.

## Windows build

Install Rust's GNU Windows toolchain matching Rune's upstream toolchain:

```powershell
rustup toolchain install 1.99.1-x86_64-pc-windows-gnu --profile minimal --component rustfmt
```

Rune's `windows-link` dependencies use raw-dylib imports. Unlike the older Rust
Win32 baseline, the GNU build needs a working MinGW-w64 assembler (`as.exe`) as
well as the import-library tools. For example use MSYS2 UCRT64 binutils and its
runtime DLL dependencies. These are development tools and are not shipped with Pinny.
The build script accepts a portable binutils directory without modifying system PATH:

```powershell
.\Benchmarks\Build-Rune.ps1 -BinutilsDirectory C:\msys64\ucrt64\bin
.\dist\native\Pinny.Rune.exe --notes 5
```

The script fetches/checks the exact revision, rejects a dirty upstream checkout,
builds with the lockfile and embeds the existing Pinny icon. The manifest uses local
paths to the pinned checkout because Rune requires workspace-root patches for
wgpu-hal, winit, parley_engine and AccessKit. A raw `cargo build` requires that
checkout to exist first. Pin changes are intentional benchmark changes and need a
new result directory with an explanation.

Release uses optimization level 3, 16 codegen units, no LTO, no incremental builds,
and stripped symbols, following Rune's upstream perf-profile choices. Only the
accessibility feature is enabled: unused audio/SVG/Linux service/Apple backend
features and embedded Inter fonts are omitted. Windows supplies Segoe UI. Agent
tooling is excluded from the measured release executable. This is different from
the tiny direct Win32 Rust baseline's LTO profile, and is documented rather than
presented as identical compiler treatment.
