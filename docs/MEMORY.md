# Desktop memory investigation — 16 October 2026

The active `Pinny.Go` app was measured on Windows x64, using Go 1.27.1
and a Ryzen 5 7600X. These results are separate from the frozen experiment
charts in `Benchmarks/`. The baseline was commit `70a8cd4`; the optimized build
uses the accompanying working-tree changes. Real notes and sync credentials
were not used.

## Findings and changes

1. `wide` previously kept every string and its UTF-16 buffer forever, including
   renamed titles, opened URLs, paths and error messages. It now caches at most
   128 strings, each at most 256 UTF-8 bytes. Longer strings are temporary.
   `call` and `comCall` retain temporary arguments through the native call;
   tooltip text has an owning Go pointer for its longer native lifetime.
2. Link detection repeatedly converted the entire prefix before every match
   into runes and UTF-16. For many links this was quadratic work and generated
   large temporary allocations. Offsets now advance through the text once,
   counting UTF-16 units without allocating those intermediate buffers.
3. Notes without links now skip link geometry and character hit-test buffers.

## Focused benchmarks

Before and after used the same fixtures and 2,000 iterations on Windows.
Numbers are one measurement per fixture; timings vary with system activity.
Allocation counts describe cumulative allocation per operation, not live RAM.

| Fixture | Before | After |
| --- | ---: | ---: |
| Heap retained after 2,000 distinct 4 KiB `wide` inputs and GC | 27.46 MiB | 11.82 KiB |
| Allocated per scan of a 1,000-link note | 124.60 MiB | 0.277 MiB |
| Time per scan of that note | 61.08 ms | 1.18 ms |
| Allocated per native hover hit test on 120,000 characters without links | 1.806 MiB | 0.368 MiB |
| Time per hover hit test | 3.27 ms | 2.50 ms |

The string benchmark is a stress test of the conversion helper, not a claim
that ordinary note editing leaks 27 MiB. Initial note bodies do not use that
cache. Retained-heap deltas include runtime/pool noise. Three additional
200-iteration link scans allocated about 290 KB/op each after the change.

## Whole-process measurements

`scripts/measure-memory.ps1` starts isolated persisted datasets, waits for all
note windows, settles for three seconds, and samples memory and native handles.
Each scenario runs three times. It also samples CPU over one idle second and
stops only its own child process. No live sync request is made.

Median results for 64-character notes:

| Open notes | Resident working set before / after | Private committed bytes before / after |
| --- | ---: | ---: |
| 1 | 15.840 / 15.691 MiB | 46.469 / 46.344 MiB |
| 5 | 16.027 / 15.965 MiB | 46.566 / 46.594 MiB |
| 20 | 16.387 / 16.387 MiB | 46.727 / 46.754 MiB |

Idle memory is effectively unchanged. The first one-note run of each binary
had a roughly 53 MiB working set; the other short-note runs were 15–17 MiB.
All short-note runs reported zero CPU time over the one-second idle sample.
GDI objects rose from 45 to 64 between one and twenty notes; USER objects rose
from 10 to about 124. This is note-count scaling, not a long-duration leak test.

With five 20,000-character notes, median resident memory fell from 20.680 MiB
to 18.680 MiB, and private committed memory fell from 50.902 to 49.031 MiB.
The small sample and variable GC timing limit how broadly this result applies.

Working set includes shared resident DLL/code pages. Private bytes measure
private committed memory, including memory not currently resident. Neither is
the same as Go live heap or Windows Task Manager's private working set.
These results exclude compositor/GPU memory and cannot be compared directly
with the older chart's private-working-set numbers.

A 120,000-character single-line persisted fixture exited before creating its
windows in the baseline build, both inside and outside the sandbox. Larger
process measurements therefore use 20,000-character notes. That startup failure
remains a separate issue; the 120,000-character hover benchmark creates the
native editor first and sets its text afterward.

## Reproduction

From the repository root on Windows:

```powershell
.\Build-Pinny.ps1
.\scripts\measure-memory.ps1
.\scripts\measure-memory.ps1 -TextCharacters 20000 -NoteCounts 1,5 -Output dist\memory-large.csv
go -C Pinny.Go test ./...
go -C Pinny.Go test -run '^$' -bench 'Benchmark(WideUnique|FindLinksMany|NativeHoverLarge)$' -benchtime=2000x -benchmem
```

Raw process runs are in `docs/memory-2026-10-16.csv`. Executables and isolated
datasets stay under ignored `dist/`. The Windows tests cover cache bounds,
retained UTF-16 pointers, multiple Unicode link offsets, native link geometry,
tooltips, editing, transfers, persistence and sync integration.

## Further opportunities

The next useful target is large-note interaction: `editorText` still copies the
whole note on hover and paint, and link detection still scans the whole note.
Reading only visible native lines, with correct wrapped-line and UTF-16 offsets,
could reduce that work further without retaining a second full text copy.

Autosave also snapshots all open notes and marshals the entire JSON file after
an edit. Large datasets could benefit from reducing serialization buffers or
tracking changed note snapshots, while retaining atomic saves and JSON
compatibility. Trash contents are also loaded into memory at startup; lazy
loading is a possible future improvement if real datasets show large trash files.
These changes need workload measurements before implementation.
