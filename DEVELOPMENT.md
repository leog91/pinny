# Development across operating systems

Use one repository with independently built components. WSL can be the primary
environment for editing, Git, cloud development, and deployment. Windows is
needed to run and verify the native desktop app, not to edit its source.

## Component boundaries

| Location | Purpose | Development environment |
| --- | --- | --- |
| `Pinny.Go/` | Active Go/Win32 desktop client | Edit anywhere; cross-compile from Linux/macOS; run tests and UI on Windows |
| `cloud/` | Reserved for Worker API, D1 migrations and simple website | WSL, Linux, macOS or Windows; implementation not yet added |
| `docs/` | Sync plan and future shared API specification | Any OS |
| `mobile/` | Future mobile client | Create when needed; tooling depends on target platform |
| `Experiments/`, `Benchmarks/` | Frozen comparisons and Windows measurement tools | Keep separate from active development |

There is no root build that requires every platform's SDK. Each component owns
its dependencies and commands. Do not make cloud/web development depend on a
Windows desktop build. Mobile clients communicate through the shared API; they
do not import Win32 UI code.

## WSL workflow

The existing Windows checkout is accessible at
`/mnt/c/<Windows checkout path>`. For this checkout, that is
`/mnt/c/Users/leog-desk/Documents/repo/pinny`. You can keep editing and committing
from WSL there. Windows Git configuration is not required for that workflow.

For a primarily Linux workflow, a checkout under `~/projects/pinny` is preferable
for filesystem-heavy tooling such as npm. Moving the current checkout is not
required. If you use separate checkouts, transfer work using commits; do not
expect uncommitted changes to appear in both. Use one OS's Git per checkout.

Install Go inside WSL to use the Linux Go toolchain. From the repository root:

```bash
bash scripts/build-windows.sh
```

This cross-compiles `dist/Pinny.exe` for Windows x64. It does not run the app,
execute tests, or embed the release icon. It works independently of the current
shell's working directory and does not require an executable bit on the script.

When using the current `/mnt/c` checkout, WSL can launch the result through
Windows interoperability:

```bash
./dist/Pinny.exe
```

For a Linux-filesystem checkout, copy the result to a Windows-accessible location
to run it if needed. Run against an isolated `--data-dir` when checking changes
that affect saved notes.

On a machine without Windows, compilation is still possible, but native GUI and
desktop test verification must happen on a Windows machine or Windows CI runner.

## Windows verification and release packaging

With Windows Go installed, use PowerShell from the repository root:

```powershell
go -C Pinny.Go test ./...
.\Build-Pinny.ps1
```

The release script embeds the icon using Windows resource APIs. Existing
interactive smoke checks remain available under `Benchmarks/`; see the README
for their commands. CI executes desktop tests and the release build on Windows,
and separately cross-compiles the executable and tests on Linux. Compiling tests
on Linux is not equivalent to executing them.

You can invoke Windows release packaging from WSL without moving Git operations
to Windows. From the repository root in the Windows-mounted checkout:

```bash
powershell.exe -NoProfile -File "$(wslpath -w "$PWD/Build-Pinny.ps1")"
```

This still uses Windows Go and Windows PowerShell. Linux Go alone is sufficient
for the Bash development cross-build, but not this Windows packaging command.

All existing active Go sources have Windows build constraints. A plain Linux
`go test ./...` in `Pinny.Go/` therefore has no runnable package. As portable sync
logic is introduced, put it in independent packages without Windows imports or
build constraints, so its tests can execute on Linux and Windows. Keep Win32
window updates, dialogs, and Windows-specific storage integration in the client.

## Cloud, website and future clients

Develop the future `cloud/` component with Node and Wrangler in your preferred
OS. Install dependencies in that OS; do not share `node_modules` between Windows
and Linux. Commit dependency lockfiles. Keep Worker settings and D1 bindings in
`cloud/wrangler.jsonc`, and schema changes in versioned SQL migrations. Keep
local development data separate from the deployed database.

Keep secret values out of Git. Local `.dev.vars`/`.env` files and Wrangler state
are ignored; safe example files may be committed. Authenticate Wrangler in the
environment where you deploy. A deployed client receives app credentials, never
Cloudflare account credentials.

The [sync plan](docs/SYNC.md) records the agreed Worker/D1 inbox, 15-minute sync,
automatic-sync toggle, manual action and delivery requirements. This behavior is
planned, not implemented. PWA, mobile applications and full two-way editing remain
later work.

The shared API should carry note identity/content and task status. Window
position, size, pin state and theme remain desktop presentation settings.
Document the API independently of any client so another OS can develop and test
the website or mobile client without running Pinny's Windows UI.

## Source conventions

`.gitattributes` normalizes source and scripts to LF across checkouts. Use matching
filename capitalization in imports and scripts; Linux filesystems are case
sensitive. Avoid absolute machine paths in build scripts and application code.
