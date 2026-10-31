# IPED lite: where we stopped

Saved 2026-10-30 (the machine was going to be turned off at 22:00). Full plan:
`C:\Users\osvaldo\.claude\plans\fluffy-yawning-teapot.md` (outside the repo, copy kept in
`lite/PLAN.md`).

Everything is on the local branch `lite` (created from `master`). Nothing was pushed, by decision:
no push, PR or any remote action without an explicit OK.

## Done (commits on `lite`)

1. `e657de77f` profile `lite` (config only), `-profile` help text, `lite/bench/measure-processing.ps1`.
2. `483e0123b` `textCacheMaxMemoryChars` (ParsingTaskConfig), used by `TextCache`; lite uses 1M chars.
3. `9dfcf3358` `HashIndex` (compact hash set) replaces the HashMap in `DuplicateTask`; `ExportFileTask`
   uses 4096 fixed locks chosen from the hash instead of one object per hash.
4. Later commits: see `git log lite`.

Tests added: `TextCacheTest`, `HashIndexTest`; `DuplicateTaskTest` adapted. Run with
`mvn -o -pl iped-engine test -Dtest=...` using Maven at `C:\tools\apache-maven-3.9.9` and
`JAVA_HOME=C:\Program Files\BellSoft\LibericaJDK-11-Full` (mvn is not on PATH).

## Measurements

See `lite/RESULTS.md`. Short version on a 127 MB UFDR, 16 GB machine: lite profile cut time from
105 s to 43 s and peak private memory from 3.9 GB to 2.0 GB, same 5,822 items. The code changes made
so far do not show on this input.

Bench environment (outside the repo, can be deleted):

- `D:\iped-bench\rel-base`: release with the `master` engine jar; `D:\iped-bench\rel-lite`: release
  with the `lite` engine jar. Both have `profiles\lite`.
- `D:\iped-bench\src-master`: git worktree (detached, `master`) used to build the baseline engine jar.
  Remove with `git worktree remove --force D:\iped-bench\src-master` when done.
- `D:\iped-bench\out-*`: outputs of the runs. They contain derived data of a real case: delete them
  when finished.
- To refresh `rel-lite` after code changes: `mvn -o -q -pl iped-engine package -DskipTests`, then copy
  `iped-engine\target\iped-engine-4.4.0-SNAPSHOT.jar` into `D:\iped-bench\rel-lite\lib\`.

## Last thing started (2026-10-30 21:32)

The user left the choice of the larger input to me, so I started the 4.7 GB `.mbox` (path kept in
`D:\iped-bench\mbox-path.txt`, not printed here because it contains personal addresses) with both
engines at the same time, lite profile, `-Xmx800m`:

- `D:\iped-bench\out-mboxB800` (master engine) and `out-mboxC800` (lite engine), summaries in
  `run-mboxB800.log` and `run-mboxC800.log`, memory curves in `out-mbox*.memory.csv`.
- If the machine was shut down while they ran, those output folders are partial: delete them before
  repeating (the script refuses an existing output folder). Times of the two concurrent runs are not
  comparable with each other, only the memory peaks and whether each one finishes.
- The outputs of the earlier UFDR runs were deleted (numbers are in `RESULTS.md`).

## Open issue found at the end of the session

The `master` engine run alone on the mbox (`-Xmx800m`, lite profile) stopped after 163 s without a
`FileList.csv` (details at the end of `RESULTS.md`). Tomorrow: read `D:\iped-bench\diag-mboxB800c.out` (a rerun
with stdout captured; may be partial if the machine was shut down), then run the `lite` engine alone with
the same command and stdout captured. This is the experiment that decides if the code changes help at tight
heap. Run with `java -jar iped.jar -d <mbox> -o <new out> --nogui -profile lite -Xmx800m` from the release
folder, without `--nologfile`.

Update 21:50: heap dumps (`java_pid*.hprof`, now in `D:\iped-bench\dumps`, delete after use) show that
the 800 MB mbox runs had OutOfMemory errors, even the ones that "finished". So the mbox needs a larger
heap; plan: find the smallest `-Xmx` (1500, 2000 MB) at which each engine finishes with no OOM and the same
item count. Always check for new `java_pid*.hprof` after a run (they are written to the current folder).

## Next steps (in order)

1. **Better corpus.** The tests so far use one small UFDR. The code changes need many items and big
   texts. Candidates found on the PC: other UFDRs in `D:\entrelobos\...\Relatórios Cellebrite\`
   (up to 19 GB), a 4.7 GB `.mbox` under `F:\entrelobos\...\retencao\`, `D:\IPED-releases\*.zip`.
   No E01/dd image and no PST were found. These are real cases: process read-only, output to
   `D:\iped-bench`, do not print contents.
2. Etapa 3 remaining: `SleuthkitReader` (`ArrayList<Integer>` to int arrays, clear `idTotrackIDMap`),
   `UfedXmlReader` (`pathToParent` holds whole `IItem`s, ZIP byte cache size), `ParsingTask.idToItemMap`
   cleanup, `ProcessingQueues` cap for priority > 0, configurable `ExportFileTask` SQLite cache and
   H2 cache (needs a proper config class first).
3. Etapa 2: JVM flags and heap factor (`Bootstrap.java:70`, `BootstrapUI.java:28`). The measurements
   say about 500 MB per run is outside the heap.
4. Etapa 4 (analysis UI) and Etapa 5 (MCP server `iped-mcp`), Etapa 6 (slim distribution), Etapa 7
   (Rust triage tool): see the plan.

## Facts worth remembering

- Heap per thread rule: the engine refuses to start with less than 200 MB x numThreads
  (`Statistics.java:408-417`).
- `LocalConfig.txt` can be shipped inside a profile (fastmode does it); properties files merge key by
  key over the defaults.
- Do not add Claude/AI attribution to commits, PRs or comments (user rule). Do not push.
