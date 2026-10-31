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
| B1g | 1 GB | ok | 40 s | 1,770 MB | 1,345 MB | 5,822 |
| C1g | 1 GB | ok | 43 s | 1,914 MB | 1,482 MB | 5,822 |
| B800 | 800 MB | ok | 40 s | 1,765 MB | 1,334 MB | 5,822 |
| C800 | 800 MB | ok | 43 s | 1,765 MB | 1,332 MB | 5,822 |

- **A1g** stops with "Insufficient heap memory available: less than 200MB per processing thread"
  (`Statistics.java:408-417`). With `numThreads = default` the baseline uses 12 threads here, so the
  smallest heap it accepts is 12 x 200 MB = 2,400 MB. The lite profile uses 4 threads (800 MB minimum).
- The peak of the lite runs barely changes between 1 GB and 800 MB, and the largest process stays
  around 1.3 GB, so about 500 MB of the process is outside the Java heap (metaspace, code cache,
  thread stacks, native libraries). That is the part Etapa 2 (JVM flags) and Etapa 6 (smaller
  distribution) should attack.
- B and C are equal within noise, so the code changes made so far (text cache limit, hash index)
  do not matter for this input. They should be tested with a corpus with many more items and large
  extracted texts (a big PST, a disk image, or a folder with many documents).
- The 200 MB per thread check also means that for an 8 GB machine the thread count, not the profile,
  decides the minimum heap.
