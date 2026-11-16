# Size, for curiosity

A development side note, not a performance ranking. These numbers describe these particular implementations and packaging choices.

| Implementation | Published EXE MiB | App source files | Source lines | Source KiB |
| --- | ---: | ---: | ---: | ---: |
| WPF | 68.31 | 4 | 359 | 12.9 |
| WinForms | 68.31 | 5 | 284 | 11.3 |
| Win32 | 33.49 | 4 | 629 | 30.2 |
| Rust Win32 | 0.28 | 1 | 483 | 20.4 |
| Go Win32 | 2.73 | 2 | 915 | 25.5 |
| Rust Rune | 29.06 | 1 | 291 | 8.6 |

- Source lines are physical lines including blanks, comments and any tests in the counted app files. Only .cs, .xaml, .go and .rs files are counted; generated bin/obj/target trees, dependencies, lockfiles, icons and project files are excluded. Shared C# model/storage source is counted in each implementation that compiles it.
- C# executables bundle a compressed .NET runtime. Rust and Go executables include linked runtime/library code but still depend on Windows system components. Rune also uses graphics drivers. EXE size is not installed or in-memory size.
- Rune app source is small partly because the framework supplies layout, text editing, rendering and shell code. Direct Win32 source implements much of that itself. The Rune experiment also has fewer storage features. Lines of code are not a measure of equivalent functionality or maintenance quality.
- SDK installations, compiler caches and framework source checkouts are development-only and are not shipped. Their disk usage depends heavily on toolchain versions, shared caches and which targets have been built, so they are not attributed to a language here.
