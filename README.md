# Pinny

Pinny is a small, single-note Windows desktop app built with C#, .NET 8, and WPF. It uses a local JSON file, with no database, web view, local server, or background service.

## Use the app

On Windows x64, double-click `dist\Pinny.exe` after publishing it. This self-contained build includes the .NET runtime and needs no console or separate .NET installation.

- Type directly into the note.
- Drag the header to move it; drag an edge or corner to resize it.
- Click **Theme** to try Light, Dark, or Paper.
- Click the diamond to toggle always-on-top; click **×** to close.

Pinny saves text, window position and size, pin state, and theme to `%LOCALAPPDATA%\Pinny\note.json`. It saves 500 ms after the last change and again on close. If the saved position is off-screen after a monitor change, Pinny moves the window back into view.

## Requirements

- **Run:** Windows 10 or 11, x64. The published executable includes the .NET runtime.
- **Build:** .NET 8 SDK. You can edit and compile on Windows, Linux, or macOS. Running and visual testing the WPF app requires Windows.
- **First build on Linux/macOS:** Internet access may be needed to download Windows targeting packs. The project sets `EnableWindowsTargeting` for this purpose.

## Build from source

From the repository root, run:

```sh
dotnet build Pinny/Pinny.csproj -c Release
```

On Windows, run a development build with:

```powershell
dotnet run --project .\Pinny\Pinny.csproj
```

To publish a standalone Windows x64 executable, run this on Windows:

```powershell
dotnet publish .\Pinny\Pinny.csproj -c Release -r win-x64 --self-contained true -p:PublishSingleFile=true -p:IncludeNativeLibrariesForSelfExtract=true -p:EnableCompressionInSingleFile=true -p:DebugType=none -p:DebugSymbols=false -o .\dist
```

The output is `dist\Pinny.exe`. `bin`, `obj`, and `dist` are ignored by Git. A GitHub source checkout will not contain the executable; attach it to a GitHub Release if you want others to download it without building.

## Project layout

- `Pinny/MainWindow.xaml` and its code-behind: window, themes, interactions, and one-shot save debounce.
- `Pinny/NoteState.cs`: fields stored for the note.
- `Pinny/NoteStorage.cs`: local JSON read and write.
- `Pinny/Assets`: app icon.
