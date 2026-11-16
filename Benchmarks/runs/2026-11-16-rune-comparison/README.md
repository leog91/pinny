# Interrupted comparison attempt: 16 November 2026

**Incomplete: 8 of 60 planned measured runs. Do not use this attempt for rankings.**

The initial runner stopped on a transient invalid Windows performance-counter
sample while measuring Go Win32 in the second one-note round. The process cleanup
ran. Completed [raw rows](results.csv), [GPU samples](gpu-samples.csv),
[environment metadata](environment.json) and [build/UI notes](NOTES.md) are retained.
The metadata's repetition count is the planned count, not a claim of completion.

The sampler was corrected to filter out invalid entries from unrelated processes
and retry up to eight times when the measured PID's valid ID/working-set pair is
missing. GPU queries also retain only valid samples. No experiment implementation,
binary, workload, sampling duration or September result changed because of this
fix. No rows from this attempt are mixed into the rerun.

The replacement series is [2026-11-16-rune-comparison-02](../2026-11-16-rune-comparison-02/README.md).
See [method changes and reasons](../../COMPARISON.md).
