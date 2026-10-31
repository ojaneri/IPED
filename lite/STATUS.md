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
