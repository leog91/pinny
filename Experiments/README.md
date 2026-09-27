# Frozen Pinny UI comparisons

These are the simple app versions used to compare UI technologies. Continue normal product development in `../Pinny/`.

| Folder | Purpose |
| --- | --- |
| `Pinny.WpfBaseline/` | Copy of the WPF app at the 27 September 2026 comparison point. |
| `Pinny.WinForms/` | Windows Forms version of the same note workflow. |
| `Pinny.Win32/` | C#/.NET version using Win32 window and drawing APIs. |
| `Baseline/` | Copied note model and JSON storage code shared by these three snapshots only. |

The WPF snapshot owns its XAML, code-behind, and icon. The other two builds reference only the copied baseline model, storage, and icon. None references active `../Pinny/` source. This means editing the active app will not silently change the benchmark builds.

Treat these folders as frozen when adding features. If a new comparison is useful later, create a new dated snapshot and record new results rather than changing the archived source or chart. The only post-measurement change to these snapshots was to put their default note data under separate `%LOCALAPPDATA%\Pinny.Baseline\<UI>\` folders; `--data-dir` still overrides that path for tests. The original measurements used isolated `--data-dir` folders, so this default-path change does not affect their method.

From the repository root on Windows, run `Benchmarks\Publish-Baseline.ps1` with the .NET 8 SDK to produce the three double-clickable executables in `dist\baseline-2026-09-27\`. See [benchmark method and results](../Benchmarks/README.md).
