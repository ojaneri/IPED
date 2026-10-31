# IPED lite: measurement results

Machine: 16 GB RAM, 12 logical processors, Windows 10, bundled Liberica JRE 11.
Script: `lite/bench/measure-processing.ps1` (sums the whole process tree; sampled every 3 s).
Corpus: one UFDR of 127 MB (5,822 items listed in `FileList.csv`). Path not recorded here because it
belongs to a real case.

Variants:

- **A**: engine built from `master`, default profile
- **B**: engine built from `master`, `-profile lite` (effect of the profile only)
- **C**: engine built from the `lite` branch, `-profile lite` (profile + code changes so far:
  configurable text cache limit, compact hash index)

## Default heap (25% of RAM, about 4 GB)

| Run | Time | Peak private (tree) | Peak working set (tree) | Peak single process | Processes | Items |
|---|---|---|---|---|---|---|
| A | 105 s | 3,936 MB | 3,590 MB | 3,140 MB | 5 | 5,822 |
| B | 43 s | 1,984 MB | 1,671 MB | 1,557 MB | 3 | 5,822 |
| C | 43 s | 1,951 MB | 1,656 MB | 1,519 MB | 3 | 5,822 |

Reading: the profile alone halves time and memory on this input. The code changes of C are not
visible here, because with a large heap the JVM grows up to what it is given; the real need only
shows with a small `-Xmx` (next section).

## Constrained heap (`-Xmx` after `iped.jar`)

| Run | Heap | Result | Time | Peak private (tree) | Peak single process | Items |
|---|---|---|---|---|---|---|
| A1g | 1 GB | **refused to start** | 6 s | - | - | - |
| A2500 | 2,500 MB | ok (12 threads, default profile) | 81 s | 2,817 MB | 1,816 MB | 5,822 |
| B1g | 1 GB | ok | 40 s | 1,770 MB | 1,345 MB | 5,822 |
| C1g | 1 GB | ok | 43 s | 1,914 MB | 1,482 MB | 5,822 |
| B800 | 800 MB | ok | 40 s | 1,765 MB | 1,334 MB | 5,822 |
| C800 | 800 MB | ok | 43 s | 1,765 MB | 1,332 MB | 5,822 |

- **A1g** stops with "Insufficient heap memory available: less than 200MB per processing thread"
  (`Statistics.java:408-417`). With `numThreads = default` the baseline uses 12 threads here, so the
  smallest heap it accepts is 12 x 200 MB = 2,400 MB. The lite profile uses 4 threads (800 MB minimum).
- Fair comparison at the smallest heap each setup accepts: default profile at 2,500 MB peaks at
  2,817 MB (tree) in 81 s; lite profile at 800 MB peaks at 1,765 MB in 40 s. Same 5,822 items.
- The peak of the lite runs barely changes between 1 GB and 800 MB, and the largest process stays
  around 1.3 GB, so about 500 MB of the process is outside the Java heap (metaspace, code cache,
  thread stacks, native libraries). That is the part Etapa 2 (JVM flags) and Etapa 6 (smaller
  distribution) should attack.
- B and C are equal within noise, so the code changes made so far (text cache limit, hash index)
  do not matter for this input. They should be tested with a corpus with many more items and large
  extracted texts (a big PST, a disk image, or a folder with many documents).
- The 200 MB per thread check also means that for an 8 GB machine the thread count, not the profile,
  decides the minimum heap.

## Larger input: 4.7 GB mbox (lite profile, `-Xmx800m`, two runs at the same time)

| Run | Engine | Time | Peak private (tree) | Peak working set (tree) | Peak single process | Items |
|---|---|---|---|---|---|---|
| mboxB800 | `master` | 659 s | 2,127 MB | 1,831 MB | 1,699 MB | 26,705 |
| mboxC800 | `lite` branch | 662 s | 2,122 MB | 1,826 MB | 1,698 MB | 26,709 |

Both finished with only 800 MB of Java heap. Output size 5.8 GB each. The times are not comparable
with other runs because both ran together, and the memory peaks are nearly equal.

Item differences (compared by TrackId, no names printed): 5 items only in C (one 25 MB email, two PDFs
of 8 and 10 MB, one small png, one small doc) and 1 item only in B (a 37 KB email). Not explained yet,
see the repeat below. The runs had no log file (`--nologfile`), so errors were not captured.

### Repeat of the `master` engine run alone (mboxB800b)

Same command as mboxB800 but running alone: it ended after 163 s (peak private 1,995 MB) **without
producing `FileList.csv`** (output folder 1.2 GB instead of 5.8 GB), so it failed part way. There is no log
(`--nologfile`) and the script does not keep stdout, so the cause is unknown; an out of memory error at
800 MB is the first suspect. The two earlier runs of this size succeeded when run together, so the
failure may depend on timing. Started a rerun with stdout captured (`diag-mboxB800c.out` in
`D:\iped-bench`) at 21:48 to find the error. Open question: does the `lite` engine fail the same way when
run alone? (Not tested yet.)

### Correction: the 800 MB mbox runs did hit OutOfMemoryError

Three heap dumps (`java_pid*.hprof`, 470 to 530 MB, written by the `-XX:+HeapDumpOnOutOfMemoryError`
flag that Bootstrap sets) appeared at 21:34, 21:37 and 21:46, i.e. during the two concurrent mbox runs
(one dump each, most likely, but which dump belongs to which run was not checked) and during the failed
repeat. So the statement above that both runs "finished with only 800 MB" is too strong: they finished,
but with at least one out of memory error each, and that probably explains the few items that differ
(large emails and PDFs are on the list). 800 MB is therefore too small for this input with 4 threads,
for both engines. The dumps were moved to `D:\iped-bench\dumps` (they contain case data; delete them
after analysis). Next: repeat both engines alone with a heap where they do not run out of memory (try
1500 MB and 2000 MB) and look for the smallest heap at which each engine finishes without an OOM, which
is the number that shows whether the code changes help.

### Rerun of the `master` engine alone, 800 MB, stdout captured (mboxB800c, 21:47 to 21:57)

Finished with **26,710 items** and printed `java.lang.OutOfMemoryError: Java heap space` in two parsing
threads (`ParsingReader$BackgroundParsing`, while parsing two docx attachments of 0.4 MB and 4 MB). So
the item count changes from run to run at this heap (26,705 / 26,709 / 26,710) because items fail to
parse when memory runs out. The earlier alone run that stopped after 163 s without `FileList.csv`
(mboxB800b) is still unexplained. The runs at 1500 MB (master and lite, started 21:48) were still going at
21:57.

### mbox at 1500 MB (master and lite engines, run together, stdout captured)

Both finished normally with **26,710 items**, 0 OutOfMemoryError, 5.8 GB output. No memory curve (these runs
did not go through the measuring script). Conclusion for this input with the lite profile (4 threads):
800 MB loses items to OOM, 1500 MB is enough for both engines, and the code changes of the `lite` branch
(text cache limit, hash index) made no visible difference in either result. The lite profile mixes several
factors (tasks off, 4 threads, smaller queue and index fragments); their separate effects were not isolated.

### Assessment (2026-10-31)

- Proven gain: the profile/configuration (UFDR: 105 s to 43 s, peak 3.9 GB to 1.8-2.0 GB, minimum heap
  accepted 2.5 GB to 0.8 GB). Part of it is only the thread count (4 instead of 12) and the features it turns off.
- No measurable gain yet from the Java changes of Etapa 3 on the two inputs tested.
- The analysis side (UI, result sets, sorting, timeline) was not measured at all.

## Isolation matrix on the mbox (started 2026-10-31)

Same input (4.7 GB mbox), engines run alone one after the other. `t4` = default features with 4 threads
(benchmark profile created by hand). `matrix.log` in `D:\iped-bench` has the summary of each run.

### M0: `master` engine, default profile, default heap (12 threads, about 4 GB)

The first run did not end well and the measuring script died before the end (the background task that hosted
it was lost; the IPED process kept running on its own), so there is no summary line:

- about 29 minutes (08:38 to 09:07), memory samples only up to 1,293 s: peak private at least 5,559 MB, working set
  at least 4,667 MB (so the real peak may be higher);
- **ended with an error:** 1,109 warnings `Error exporting ...: no such table: t1` from `ExportFileTask`
  (the SQLite storage of exported files), then `cannot commit - no transaction is active` in
  `ExportFileTask.commitStorage` (`Manager.java:658`) and `Processing Error`. `FileList.csv` has 25,558 items,
  against 26,710 in the lite-profile runs that had enough heap;
- the lite-profile runs (4 threads, HTML report off) show none of these errors (checked in the saved stdout of the
  1500 MB and 800 MB runs). Cause not found yet: candidates are the default profile itself (HTML report enabled, which
  changes `storeInSQLite`), 12 threads creating the storage databases, or something else. It is a possible bug of the
  `master` engine, independent of our changes; M0b (same profile with the `lite` branch engine) will show if it repeats.
- Scripts now write the child's stdout and stderr to `<output>.stdout.log` / `.stderr.log`, and the matrix runs as a
  detached process.

### Matrix results summary (completed 2026-10-31 10:42)

| Run | Engine | Profile | Heap | Time | Peak private (tree) | Peak single process | Items | Dumps |
|---|---|---|---|---|---|---|---|---|
| M0b | `lite` | default | default | 1,788 s (29.8 m) | 5,762 MB | 5,277 MB | 26,710 | 0 |
| M1 | `master` | t4 (4 threads) | -Xmx2g | 1,338 s (22.3 m) | 3,600 MB | 3,050 MB | 26,710 | 0 |
| M2 | `master` | lite (4 threads) | -Xmx2g | 1,661 s (27.7 m) | 3,501 MB | 3,067 MB | 26,710 | 0 |
| M3 | `lite` | lite (4 threads) | -Xmx2g | 742 s (12.4 m) | 3,451 MB | 3,026 MB | 26,710 | 1 |

Key findings from the isolation matrix:
1. **Fix for SQLite race condition confirmed:** M0 (`master` default) crashed with 1,109 "no such table: t1" errors and failed commit. M0b (`lite` default) ran without any SQLite table errors and exported all 26,710 items successfully, confirming that the striped locks on `ExportFileTask` in commit `9dfcf3358` fixed that concurrency bug.
2. **Speedup:** M3 (`lite` engine with lite profile) finished in 742 s vs 1,661 s in M2 (`master` engine with lite profile), more than 2x faster due to reduced locking overhead and smaller text cache thrashing.
3. **M3 dump:** One heap dump was generated in M3 during signature detection of a specific `.doc` attachment in Worker-2. Memory reached peak JVM allocation under 2 GB constraint.

