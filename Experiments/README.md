# Frozen Pinny UI comparisons

These are the simple app versions used to compare UI technologies. Continue normal product development in `../Pinny/`.

| Folder | Purpose |
| --- | --- |
| `Pinny.WpfBaseline/` | Copy of the WPF app at the 27 September 2026 comparison point. |
| `Pinny.WinForms/` | Windows Forms version of the same note workflow. |
| `Pinny.Win32/` | C#/.NET version using Win32 window and drawing APIs. |
| `Baseline/` | Copied note model and JSON storage code shared by these three snapshots only. |

The WPF snapshot owns its XAML, code-behind, and icon. The other two builds reference only the copied baseline model, storage, and icon. None references active `../Pinny/` source. This means editing the active app will not silently change the benchmark builds.

Treat the three C# folders as frozen when adding features. If a new comparison is useful later, create a new snapshot rather than changing these reference builds. They use separate `%LOCALAPPDATA%\Pinny.Baseline\<UI>\` default folders; `--data-dir` overrides that path for isolated tests.

`Pinny.Rust/` and `Pinny.Go/` are additional independent Win32 experiments for runtime cost comparisons. Rust creates benchmark notes in memory. Go now saves and restores notes in the same JSON format as the C# builds, using its own default data directory. Both are included in the [single five-build benchmark](../Benchmarks/README.md). See [native build and run commands](../Benchmarks/README.md#rust-and-go-win32-experiments).

From the repository root on Windows, run `Benchmarks\Publish-Baseline.ps1` with the .NET 8 SDK to produce the three double-clickable executables in `dist\baseline-2026-09-27\`. See [benchmark method and results](../Benchmarks/README.md).
