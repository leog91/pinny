# Pinny: technical notes and decisions

Pinny is a Windows-only desktop notes app. The product goals are a small, responsive UI, low idle CPU and memory use, fast startup, and local storage. The [main README](README.md) covers usage and build commands; the [benchmark report](Benchmarks/README.md) records measured results.

## Current status

| Build | UI implementation | Status |
| --- | --- | --- |
| [WPF](Pinny/Pinny.csproj) | XAML and WPF controls | Main implementation; continue development here |
| [WPF baseline](Experiments/Pinny.WpfBaseline/Pinny.WpfBaseline.csproj) | Snapshot of the simple WPF app | Frozen benchmark reference |
| [WinForms](Experiments/Pinny.WinForms/Pinny.WinForms.csproj) | Windows Forms controls with a borderless form | Frozen comparison build |
| [Win32](Experiments/Pinny.Win32/Pinny.Win32.csproj) | `user32.dll` / `gdi32.dll` calls from C#, a native `EDIT` control, and a painted header | Frozen comparison build |
| [Rust Win32](Experiments/Pinny.Rust/Cargo.toml) | Direct Win32 calls with thin Rust bindings | Independent, in-memory benchmark experiment |
| [Go Win32](Experiments/Pinny.Go/go.mod) | Direct Win32 DLL calls from Go | Independent comparison build with local JSON persistence |

The active WPF app and three frozen snapshots are C# / .NET 8 applications. The C# Win32 build uses the regular .NET runtime, not Native AOT. Rust and Go produce standalone native Win32 executables. Rust is in-memory; Go saves local JSON. No framework switch has been chosen. The WPF app remains the main build while we assess resource use against UI quality, reliability, and maintenance cost.

## Shared design

This section describes the active WPF app and three C# snapshots. Go has a compatible independent JSON store; Rust remains in-memory.

- One process owns all open note windows. Each window has its own text, position, size, pin state, and Light/Dark/Paper theme.
- The active app owns its [note model](Pinny/NoteState.cs) and [JSON storage code](Pinny/NoteStorage.cs). The three snapshots compile separate copies under [Experiments/Baseline](Experiments/Baseline), so future edits to the active app cannot change benchmark behavior. There is no database, sync service, web view, local server, dependency injection container, or third-party UI framework.
- Changes queue a save after 500 ms of inactivity. WPF uses a stopped-and-restarted `DispatcherTimer` per window; WinForms uses one stopped-and-restarted forms timer; Win32 uses `SetTimer` / `KillTimer`. None is a continuously running polling loop.
- A save serializes the current list of notes to `notes.json.tmp` in the same directory, then moves it over `notes.json`. Saving the whole list keeps the implementation small; write and serialization work will grow with total note content.
- Closing a note removes only that note from the saved list. **Quit** saves the open notes before exiting. Closing the last note leaves an empty list, so the next launch starts with a blank note.
- Build output (`bin`, `obj`, `dist`) is ignored by Git. Published Windows x64 builds are self-contained, single-file executables; they do not require a separately installed .NET runtime. Source builds require the .NET 8 SDK.

### Data locations

| Build | Default file |
| --- | --- |
| Active WPF | `%LOCALAPPDATA%\Pinny\notes.json` |
| WPF baseline | `%LOCALAPPDATA%\Pinny.Baseline\Wpf\notes.json` |
| WinForms baseline | `%LOCALAPPDATA%\Pinny.Baseline\WinForms\notes.json` |
| Win32 baseline | `%LOCALAPPDATA%\Pinny.Baseline\Win32\notes.json` |
| Go Win32 | `%LOCALAPPDATA%\Pinny.Go\notes.json` |

The stores are separate so comparing builds does not overwrite the main WPF notes. All four C# builds and Go accept `--data-dir <folder>` for isolated runs; Rust has no data directory. The JSON array holds each saved note's ID, plain text, window coordinates, dimensions, pin state, and theme. Note text is stored as readable JSON, without encryption. The C# and Go builds can read the older single-note `note.json` if `notes.json` is absent. Saved coordinates are clamped back toward the visible desktop when the monitor layout changes. Load failures show an error without rewriting the saved file; save failures show a warning.

The app does not coordinate writes between separate processes. Do not run two copies against the same data directory.

The active WPF app selects its data directory in this order: `--data-dir <folder>`, `PINNY_DATA_DIR`, then `%LOCALAPPDATA%\Pinny`. Go uses `--data-dir <folder>` or its own default location; it does not read `PINNY_DATA_DIR`. A web or mobile client cannot use a Windows file path as a shared store; supporting those clients will require a sync API, an authoritative note store, and a migration path for existing local notes. Note text and identity can be shared across clients, while window position, size, and pin state are desktop-specific presentation state.

## Decisions and tradeoffs

| Decision | Reason | Revisit when |
| --- | --- | --- |
| Keep WPF as the main build for now | It already delivers the complete note workflow and is the easiest of these versions to style and extend. | A comparison build meets the desired UI quality and manual checks. |
| Freeze the three C# comparison builds in `Experiments/` | They remain measurable while the active WPF app changes. A copied WPF snapshot removes the dependency on `Pinny/`. | A framework is selected or a new comparison baseline is intentionally created. |
| Use a local JSON file | A small note collection needs no database or background service. | Save times or note volume make whole-list writes a problem. |
| Use short, one-shot save debounce | Avoid disk work on every keystroke and continuous idle activity. | Reliability or responsiveness testing suggests a different interval. |
| Share only the snapshot model and storage source | Keeps comparison behavior and file format aligned without tying the snapshots to active development. | A new comparison baseline is intentionally created. |
| Defer Native AOT results | The attempted Win32 AOT publish stopped because this machine lacks the Visual Studio C++ linker required on Windows. | The linker is available and the AOT build passes functional checks. |

The [five-build benchmark](Benchmarks/README.md) measured Rust Win32 at **6.3 MiB** private working set with one synthetic note and Go Win32 at **7.3 MiB**, versus **25.1 MiB** for C# Win32, **49.0 MiB** for WinForms, and **103.2 MiB** for WPF. First-window startup was **44 ms**, **42 ms**, **136 ms**, **381 ms**, and **780 ms**, respectively. Go and the C# builds load JSON; Rust constructs its synthetic notes in memory. These results support a runtime-cost comparison, while UI quality, accessibility, and maintenance still need evaluation. The Win32 versions require more hand-written window, painting, and input code; WPF gives the most flexible styling.

## Checks still worth doing before selecting a UI

- Try each version on high-DPI and mixed-DPI monitors, including unplugging a monitor while notes are open.
- Check keyboard use, IME input, screen readers, high-contrast settings, long notes, and the theme popup in each build. Native Win32 controls handle some of this, but the custom header needs its own validation.
- Check save behavior during sign-out or shutdown in the C# WinForms and Win32 builds. The WPF app has an explicit session-ending handler; these two builds currently rely on their normal save and close paths.
- Measure real note content and longer idle periods if the comparison becomes a release decision. The current benchmark uses short synthetic notes and measures startup only to the first window handle.
