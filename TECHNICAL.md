# Pinny: technical notes and decisions

Pinny is a Windows-only desktop notes app. The product goals are a small, responsive UI, low idle CPU and memory use, fast startup, and local storage. The [main README](README.md) covers usage and build commands; the [benchmark report](Benchmarks/README.md) records measured results.

## Current status

| Build | UI implementation | Status |
| --- | --- | --- |
| [Go](Pinny.Go/go.mod) | Direct Win32 DLL calls and native `EDIT` control | Main implementation; continue development here |
| [WPF baseline](Experiments/Pinny.WpfBaseline/Pinny.WpfBaseline.csproj) | Snapshot of the simple WPF app | Frozen benchmark reference |
| [WinForms](Experiments/Pinny.WinForms/Pinny.WinForms.csproj) | Windows Forms controls with a borderless form | Frozen comparison build |
| [Win32](Experiments/Pinny.Win32/Pinny.Win32.csproj) | `user32.dll` / `gdi32.dll` calls from C#, a native `EDIT` control, and a painted header | Frozen comparison build |
| [Rust Win32](Experiments/Pinny.Rust/Cargo.toml) | Direct Win32 calls with thin Rust bindings | Independent, in-memory benchmark experiment |
| [Go Win32](Experiments/Pinny.Go/go.mod) | Direct Win32 DLL calls from Go | Frozen comparison build with local JSON persistence |

The active Go app was copied from the measured Go experiment when Go was selected as the main implementation. The experiment stays frozen so later app changes do not rewrite the benchmark comparison. The earlier WPF app is retained only as a frozen benchmark snapshot. The three frozen C# snapshots use .NET 8; the C# Win32 build uses the regular .NET runtime, not Native AOT. Rust and Go produce standalone Win32 executables. Rust is in-memory; Go saves local JSON.

## Shared design

The active Go app and the saved-note comparison builds share the same note JSON fields, while each owns its storage code. Rust remains in-memory.

- One process owns all open note windows. Each window has its own text, position, size, pin state, and Light/Dark/Paper theme.
- The active app owns its [Win32 UI](Pinny.Go/main.go) and [JSON storage code](Pinny.Go/storage.go). The Go benchmark snapshot has separate source under `Experiments/Pinny.Go/`; the three C# snapshots compile their own source under [Experiments/Baseline](Experiments/Baseline). There is no database, sync service, web view, local server, dependency injection container, or third-party UI framework.
- Changes queue a save after 500 ms of inactivity. Go and C# Win32 use `SetTimer` / `KillTimer`; WPF uses a stopped-and-restarted `DispatcherTimer` per window; WinForms uses one stopped-and-restarted forms timer. None is a continuously running polling loop.
- A save serializes the current list of notes to `notes.json.tmp` in the same directory, then moves it over `notes.json`. Saving the whole list keeps the implementation small; write and serialization work will grow with total note content.
- In the active Go app, **Quit** and the window close event save all open notes. Moving a note to Trash saves its state to `trash.json` before removing it from `notes.json`. The newest trashed note can be restored from the menu; Empty Trash requires confirmation. Trashed notes have no automatic expiry. The frozen comparisons retain their original close behavior.
- Build output (`bin`, `obj`, `dist`) is ignored by Git. The active Go release is a standalone Windows GUI executable and does not require a separately installed Go runtime. The C# comparison builds require the .NET 8 SDK to build.

### Data locations

| Build | Default file |
| --- | --- |
| Active Go | `%LOCALAPPDATA%\Pinny\notes.json` |
| WPF baseline | `%LOCALAPPDATA%\Pinny.Baseline\Wpf\notes.json` |
| WinForms baseline | `%LOCALAPPDATA%\Pinny.Baseline\WinForms\notes.json` |
| Win32 baseline | `%LOCALAPPDATA%\Pinny.Baseline\Win32\notes.json` |
| Go Win32 | `%LOCALAPPDATA%\Pinny.Go\notes.json` |

The experimental stores are separate so comparing builds does not overwrite the main notes. The active Go app deliberately uses the earlier WPF app's default folder and compatible JSON format, so existing notes can be opened without a copy. Its `trash.json` is a separate array with the same note fields. A Trash move writes this array first, so an interrupted move can leave a duplicate in Trash rather than lose the note. The C# and Go builds accept `--data-dir <folder>` for isolated runs; Rust has no data directory. The JSON array holds each saved note's ID, plain text, window coordinates, dimensions, pin state, and theme. Note text is stored as readable JSON, without encryption. The C# and Go builds can read the older single-note `note.json` if `notes.json` is absent. Saved coordinates are clamped back toward the visible desktop when the monitor layout changes. Load failures show an error without rewriting the saved file; save failures show a warning.

The app does not coordinate writes between separate processes. Do not run two copies against the same data directory.

The active Go app selects the data directory in this order: `--data-dir <folder>`, `PINNY_DATA_DIR`, then `%LOCALAPPDATA%\Pinny`. The frozen Go experiment uses `--data-dir <folder>` or its own `%LOCALAPPDATA%\Pinny.Go` default and does not read `PINNY_DATA_DIR`. A web or mobile client cannot use a Windows file path as a shared store; supporting those clients will require a sync API, an authoritative note store, and a migration path for existing local notes. Note text and identity can be shared across clients, while window position, size, and pin state are desktop-specific presentation state.

## Decisions and tradeoffs

| Decision | Reason | Revisit when |
| --- | --- | --- |
| Make Go the main build | It reproduces the Win32 note workflow with local persistence and measured low memory/startup cost. Keep the benchmark source frozen. | Usability or reliability checks reveal a material gap. |
| Freeze the comparison builds in `Experiments/` | They remain measurable while the active Go app changes. Separate source keeps later app edits out of the benchmark. | A new comparison baseline is intentionally created. |
| Use a local JSON file | A small note collection needs no database or background service. | Save times or note volume make whole-list writes a problem. |
| Use short, one-shot save debounce | Avoid disk work on every keystroke and continuous idle activity. | Reliability or responsiveness testing suggests a different interval. |
| Share only the snapshot model and storage source | Keeps comparison behavior and file format aligned without tying the snapshots to active development. | A new comparison baseline is intentionally created. |
| Defer Native AOT results | The attempted Win32 AOT publish stopped because this machine lacks the Visual Studio C++ linker required on Windows. | The linker is available and the AOT build passes functional checks. |

The [five-build benchmark](Benchmarks/README.md) measured Rust Win32 at **6.3 MiB** private working set with one synthetic note and Go Win32 at **7.3 MiB**, versus **25.1 MiB** for C# Win32, **49.0 MiB** for WinForms, and **103.2 MiB** for WPF. First-window startup was **44 ms**, **42 ms**, **136 ms**, **381 ms**, and **780 ms**, respectively. Go and the C# builds load JSON; Rust constructs its synthetic notes in memory. These results motivated the Go choice, but they describe the frozen experiment, not future active Go releases. The Win32 versions require more hand-written window, painting, and input code; WPF gives the most flexible styling.

## Checks for the Go implementation

- Try each version on high-DPI and mixed-DPI monitors, including unplugging a monitor while notes are open.
- Check keyboard use, IME input, screen readers, high-contrast settings, long notes, and the theme popup in each build. Native Win32 controls handle some of this, but the custom header needs its own validation.
- Check save behavior during sign-out or shutdown in the active Go app and C# WinForms and Win32 builds. The WPF benchmark has an explicit session-ending handler; these builds currently rely on their normal save and close paths.
- Measure real note content and longer idle periods if the comparison becomes a release decision. The current benchmark uses short synthetic notes and measures startup only to the first window handle.
