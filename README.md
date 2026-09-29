# Pinny

Pinny is a small Windows desktop notes app built with Go and direct Win32 calls. It uses local JSON files, with no database, web view, local server, or background service.

`Pinny.Go/` is the main app for ongoing development and personal use. `Experiments/` contains frozen benchmark comparisons, including the WPF version and an independent snapshot of Go. See [technical notes and decisions](TECHNICAL.md).

## Use the app

On Windows x64, double-click `dist\Pinny.exe` after building it. The executable does not require a separate Go or .NET installation.

- Type directly into any note. Each note has its own text, position, size, pin state, and theme.
- Click **+** to create another note. New notes open slightly offset from the current one.
- Drag a note's header to move it; drag an edge or corner to resize it.
- Click the diamond to toggle always-on-top for that note.
- Click **⋯** to choose Light, Dark, or Paper, manage Trash, or quit while keeping all notes.
- Click the red **×**, choose **⋯ → Quit Pinny (keep notes)**, or close any note window from the taskbar to exit while keeping every note.

### Deleting and recovering notes

Choose **⋯ → Move this note to Trash** to remove one note. Pinny saves that note to `trash.json` before removing it from `notes.json`; it stays recoverable across restarts. If it was the last open note, Pinny exits and opens a blank note on the next launch. Choose **⋯ → View Trash...** to browse deleted notes, read their contents, restore a selected note, permanently delete one note, or empty all of Trash. The list shows the newest deletion first. Both permanent deletion actions ask for confirmation. **⋯ → Restore last deleted note** remains a shortcut for the newest note. Trash is never emptied automatically.

Pinny saves open notes to `%LOCALAPPDATA%\Pinny\notes.json` and trashed notes to `%LOCALAPPDATA%\Pinny\trash.json`. Changes save 500 ms after the last edit, move, resize, pin toggle, or theme change, and again when quitting. The Go app reads notes saved by the earlier WPF version in the same folder. When upgrading from the single-note version, Pinny loads the old `%LOCALAPPDATA%\Pinny\note.json` if `notes.json` does not exist. The old file is left in place as a backup. If a saved position is off-screen after a monitor change, Pinny moves that window back into view.

### Choose a notes folder

Choose **⋯ → Choose notes folder...** to pick a folder in the app. Pinny copies all current notes and Trash there, saves future changes there, and remembers the folder for future launches. The original files are kept. To protect another collection, the destination must not already contain `notes.json`, `trash.json`, or the older `note.json`. The choice is stored in `%LOCALAPPDATA%\Pinny\settings.json`.

You can also set the `PINNY_DATA_DIR` user environment variable or pass `--data-dir <folder>` for one launch. The selection order is command-line option, environment variable, remembered folder, then the default `%LOCALAPPDATA%\Pinny`. No `.env` file is required or read by the app.

In PowerShell, `$env:PINNY_DATA_DIR = 'C:\Notes\Pinny'` sets it for the current session. Then run `.\dist\Pinny.exe`.

Changing the environment variable or command-line option does not copy existing notes. Close Pinny and copy `notes.json` and `trash.json` to the new folder if you want to retain them, or use the folder picker in the app. Keep only one running app pointed at a notes folder: the current JSON storage does not merge simultaneous edits. The archived builds under `Experiments/` keep their own default folders and ignore `PINNY_DATA_DIR`.

### Export and import

Choose **⋯ → Export notes...** to save one readable JSON backup containing all open notes and Trash. It preserves each note's text, ID, position, size, pin state, and theme, including current edits. The backup has `Version`, `Notes`, and `Trash` fields.

Choose **⋯ → Import notes...** to add a backup to your current collection. Existing notes and Trash are kept, and conflicting note IDs are replaced with new IDs. The importer also accepts a plain `notes.json` array from earlier versions. Invalid files are rejected before changing the collection. Imported window positions are moved into view if needed on the current computer.

## Requirements

- **Run:** Windows 10 or 11, x64.
- **Build the main app:** Go for Windows x64 and PowerShell. The release script embeds the app icon.
- **Build the C# comparisons:** .NET 8 SDK. Rust/Cargo is needed only for the Rust comparison.

## Build from source

From the repository root in PowerShell, run:

```powershell
.\Build-Pinny.ps1
.\dist\Pinny.exe
```

This produces the Windows GUI executable `dist\Pinny.exe` with the Pinny icon. To build without the release script during development:

```powershell
Push-Location .\Pinny.Go
go run .
Pop-Location
```

`bin`, `obj`, and `dist` are ignored by Git. A GitHub source checkout will not contain the executable; attach it to a GitHub Release if you want others to download it without building.

## Project layout

- `Pinny.Go/main.go`: active Win32 note windows, controls, and save coordination.
- `Pinny.Go/storage.go`: active JSON storage and single-note migration.
- `Pinny.Go/Pinny.ico`: active app icon.
- `Build-Pinny.ps1`: main Go release build with icon.
- `Experiments/Pinny.WpfBaseline/`, `Pinny.WinForms/`, `Pinny.Win32/`: three standalone source snapshots of the simple app.
- `Experiments/Pinny.Rust/` and `Pinny.Go/`: frozen native Win32 comparisons; Rust is in-memory, while Go saves local JSON. See [build and benchmark commands](Benchmarks/README.md#rust-and-go-win32-experiments).
- `Experiments/Baseline/`: model and storage source shared only by those snapshots; see [experiment structure](Experiments/README.md).
- `Benchmarks/`: five-build comparison and scripts to build and measure the frozen experiments.
- `TECHNICAL.md`: architecture and current technical decisions.

## Performance experiments

The benchmark compares the five builds under `Experiments/`. It does not measure future changes to the active Go app under `Pinny.Go/`.

![Pinny benchmark: memory and first-window startup for five implementations](Benchmarks/benchmark.png)

[Benchmark methods and results](Benchmarks/README.md) · [All 30 raw measurements](Benchmarks/results.csv)

With one synthetic note, median private working set was 103.2 MiB for C# WPF, 49.0 MiB for C# WinForms, 25.1 MiB for C# Win32, 6.3 MiB for Rust Win32, and 7.3 MiB for Go Win32. First-window startup was 780, 381, 136, 44, and 42 ms respectively. Rust keeps notes in memory; Go loads and saves the same JSON note format as the C# builds.

| Version | Interface | Note storage | Open after publishing |
| --- | --- | --- | --- |
| WPF baseline | Borderless WPF window | `%LOCALAPPDATA%\Pinny.Baseline\Wpf\notes.json` | `dist\baseline-2026-09-27\wpf\Pinny.exe` |
| WinForms baseline | Borderless Windows Forms window | `%LOCALAPPDATA%\Pinny.Baseline\WinForms\notes.json` | `dist\baseline-2026-09-27\winforms\Pinny.WinForms.exe` |
| Win32 baseline | Borderless note window with a painted header and native edit control | `%LOCALAPPDATA%\Pinny.Baseline\Win32\notes.json` | `dist\baseline-2026-09-27\win32\Pinny.Win32.exe` |
| Rust Win32 | Painted Win32 window and native edit control | None | `dist\native\Pinny.Rust.exe` |
| Go Win32 | Painted Win32 window and native edit control | `%LOCALAPPDATA%\Pinny.Go\notes.json` | `dist\native\Pinny.Go.exe` |

All three snapshots can open multiple notes, edit plain text, move and resize windows, pin individual notes, choose Light/Dark/Paper, delete individual notes, quit while retaining all notes, and restore saved notes. The Win32 version has a painted header with New, Menu, Pin, and Close controls and a native popup menu. Each snapshot has its own default data directory, separate from the active app. All support `--data-dir <folder>` for isolated testing.

Build the snapshots with the .NET 8 SDK on Windows:

```powershell
dotnet build .\Experiments\Pinny.WpfBaseline\Pinny.WpfBaseline.csproj -c Release
dotnet build .\Experiments\Pinny.WinForms\Pinny.WinForms.csproj -c Release
dotnet build .\Experiments\Pinny.Win32\Pinny.Win32.csproj -c Release
```

Publish all three self-contained executables from the repository root:

```powershell
.\Benchmarks\Publish-Baseline.ps1
```

To build and measure all five implementations in PowerShell:

```powershell
.\Benchmarks\Publish-Baseline.ps1
.\Benchmarks\Build-Native.ps1
.\Benchmarks\Measure-Pinny.ps1
```

To smoke-test the WinForms, C# Win32, and frozen Go versions with synthetic notes (edit, move, resize, pin, restart, create and delete a note):

```powershell
.\Benchmarks\Smoke-Persistence.ps1
```

To check the active Go app's save, Trash, restore, and safe-close behavior using isolated test data:

```powershell
.\Benchmarks\Smoke-Persistence.ps1 -Variants active-go
```

To check native picker cancellation and startup from a remembered folder, then exercise JSON transfers and failure recovery with live note windows and isolated test data:

```powershell
.\Benchmarks\Smoke-Transfer.ps1
Push-Location .\Pinny.Go
go test ./...
Pop-Location
```

The benchmark script uses synthetic notes and updates the single tracked CSV and chart under `Benchmarks/`. Published executables and temporary synthetic folders are ignored by Git. Native AOT was not measured because the required C++ linker is not installed on this machine.
