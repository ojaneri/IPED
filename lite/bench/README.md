# IPED lite: memory benchmarks

Reference measurements for the low-memory work (see the plan: stages 0 to 7).

## Corpus (not tracked in git)

Use the same inputs for every run so numbers are comparable:

- one E01 or dd image with NTFS
- one UFDR
- one large PST
- one nested ZIP

## Processing

```powershell
.\measure-processing.ps1 -IpedDir <release dir> -Evidence <input> -Output <out dir> -Profile default
.\measure-processing.ps1 -IpedDir <release dir> -Evidence <input> -Output <out dir> -Profile lite
```

The script prints a summary line (exit code, elapsed time, peak working set of the whole process
tree, number of processes) and writes `memory-samples.csv` in the output folder.

For heap after GC and native memory, run the child JVM with
`-XX:NativeMemoryTracking=summary` (pass it after `iped.jar` in `-ExtraArgs`) and use
`jcmd <pid> GC.heap_info` and `jcmd <pid> VM.native_memory summary` while it runs.

## Analysis (fixed script)

Open the case, then in this order: search `*`, sort by name, filter one category, enable the
timeline table, open the metadata panel on `hash`, open the gallery. Record the peak working set
of the process after each step.

## Results

Record every run in `RESULTS.md` (date, commit, profile, corpus, summary line).
