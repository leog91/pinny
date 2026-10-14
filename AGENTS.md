# Repository guidance

- Read `DEVELOPMENT.md` for the WSL-first and cross-platform workflow.
- `Pinny.Go/` is the active Windows desktop app. Keep `Experiments/` frozen unless
  the task explicitly concerns those comparisons.
- Keep cloud/web and future client commands independent of Windows desktop tools.
- Keep portable logic in packages without Win32 imports; retain platform build
  constraints for Windows integration. Cross-compilation does not verify runtime
  behavior; execute desktop tests and UI checks on Windows.
- Use `bash scripts/build-windows.sh` for a development cross-build and
  `Build-Pinny.ps1` on Windows for release packaging. Run desktop Go tests from
  `Pinny.Go/` on Windows. See `.github/workflows/checks.yml` for CI coverage.
- Preserve existing JSON compatibility and local notes when changing storage.
- Keep credentials and generated dependency/runtime files out of Git. Cloudflare
  account credentials must never be shipped in a client.
