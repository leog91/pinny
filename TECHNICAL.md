# Pinny: technical notes and decisions

Pinny is a Windows-only desktop notes app. The product goals are a small, responsive UI, low idle CPU and memory use, fast startup, and local storage. The [main README](README.md) covers usage and build commands; the [benchmark report](Benchmarks/README.md) records measured results.

## Current status

| Build | UI implementation | Status |
| --- | --- | --- |
| [WPF](Pinny/Pinny.csproj) | XAML and WPF controls | Main implementation; continue development here |
| [WPF baseline](Experiments/Pinny.WpfBaseline/Pinny.WpfBaseline.csproj) | Snapshot of the simple WPF app | Frozen benchmark reference |
| [WinForms](Experiments/Pinny.WinForms/Pinny.WinForms.csproj) | Windows Forms controls with a borderless form | Frozen comparison build |
| [Win32](Experiments/Pinny.Win32/Pinny.Win32.csproj) | `user32.dll` / `gdi32.dll` calls from C#, a native `EDIT` control, and a painted header | Frozen comparison build |

**All four projects are C# / .NET 8 applications.** The Win32 build is still managed .NET code published as a self-contained executable; it is not C++ and has not been compiled with Native AOT. No framework switch has been chosen. The WPF app remains the main build while we assess the measured resource savings against UI quality and maintenance cost.

## Shared design

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

The stores are separate so comparing builds does not overwrite the main WPF notes. Every build accepts `--data-dir <folder>` for isolated runs. The JSON array holds each note's ID, plain text, window coordinates, dimensions, pin state, and theme. Note text is stored as readable JSON, without encryption. If `notes.json` is absent, the storage code can read the older single-note `note.json`; it leaves that legacy file in place. Saved coordinates are clamped back toward the visible desktop when the monitor layout changes. Load failures show an error without rewriting the saved file; save failures show a warning.

The app does not coordinate writes between separate processes. Do not run two copies against the same data directory.

## Decisions and tradeoffs

| Decision | Reason | Revisit when |
| --- | --- | --- |
| Keep WPF as the main build for now | It already delivers the complete note workflow and is the easiest of these versions to style and extend. | A comparison build meets the desired UI quality and manual checks. |
| Freeze all three comparison builds in `Experiments/` | They remain measurable while the active WPF app changes. A copied WPF snapshot removes the dependency on `Pinny/`. | A framework is selected; preserve the archived comparison for reference. |
| Use a local JSON file | A small note collection needs no database or background service. | Save times or note volume make whole-list writes a problem. |
| Use short, one-shot save debounce | Avoid disk work on every keystroke and continuous idle activity. | Reliability or responsiveness testing suggests a different interval. |
| Share only the snapshot model and storage source | Keeps comparison behavior and file format aligned without tying the snapshots to active development. | A new comparison baseline is intentionally created. |
| Defer Native AOT results | The attempted Win32 AOT publish stopped because this machine lacks the Visual Studio C++ linker required on Windows. | The linker is available and the AOT build passes functional checks. |

The [three-run benchmark](Benchmarks/README.md) recorded on 27 September 2026 measured the styled Win32 build at **20.2 MiB private working set** with one short note, versus **43.1 MiB** for WinForms and **89.2 MiB** for WPF. Its first-window time was **136 ms**, versus **259 ms** and **727 ms**. These are historical measurements on one PC, not general framework guarantees or measurements of future active WPF changes. The Win32 version requires more hand-written window, painting, and input code. WinForms is the middle option; WPF gives the most flexible styling.

## Checks still worth doing before selecting a UI

- Try each version on high-DPI and mixed-DPI monitors, including unplugging a monitor while notes are open.
- Check keyboard use, IME input, screen readers, high-contrast settings, long notes, and the theme popup in each build. Native Win32 controls handle some of this, but the custom header needs its own validation.
- Check save behavior during sign-out or shutdown in the experimental builds. The WPF app has an explicit session-ending handler; the two experiments currently rely on their normal save and close paths.
- Measure real note content and longer idle periods if the comparison becomes a release decision. The current benchmark uses short synthetic notes and measures startup only to the first window handle.
