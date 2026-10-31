# Runs a fixed set of IPED configurations one after the other on the same input and appends one line per
# run to <Work>\matrix.log (summary, item count, new heap dumps). The output folder is deleted after each
# run to save disk space (only the item count is kept).
#
# Variants isolate the factors: base = engine built from master, lite = engine of the lite branch;
# profile t4 = default features with only 4 threads (benchmark profile created by hand in <rel>\profiles\t4).
param(
    [Parameter(Mandatory = $true)][string]$Evidence,
    [string]$Work = "D:\iped-bench",
    [string[]]$Only = @()   # run only these variant names (default: all)
)
$Only = @($Only | ForEach-Object { $_ -split "," } | Where-Object { $_ })   # accepts a comma list when started with -File
$measure = Join-Path $PSScriptRoot "measure-processing.ps1"
$runs = @(
    @{n = "M0-base-default";  rel = "rel-base"; prof = "default"; x = "" },
    @{n = "M0b-lite-default"; rel = "rel-lite"; prof = "default"; x = "" },
    @{n = "M1-base-t4";       rel = "rel-base"; prof = "t4";      x = "-Xmx2g" },
    @{n = "M2-base-lite";     rel = "rel-base"; prof = "lite";    x = "-Xmx2g" },
    @{n = "M3-lite-lite";     rel = "rel-lite"; prof = "lite";    x = "-Xmx2g" }
)
$dumps = Join-Path $Work "dumps"
Set-Location $dumps   # heap dumps are written to the current folder
foreach ($r in $runs) {
    if ($Only.Count -gt 0 -and $Only -notcontains $r.n) { continue }
    $out = Join-Path $Work ("out-" + $r.n)
    $before = @(Get-ChildItem $dumps -Filter *.hprof -ErrorAction SilentlyContinue).Count
    $line = & $measure -IpedDir (Join-Path $Work $r.rel) -Evidence $Evidence -Output $out -Profile $r.prof -ExtraArgs $r.x 2>&1 | Select-Object -Last 1
    $items = "no FileList.csv"
    $fl = Join-Path $out "FileList.csv"
    if (Test-Path $fl) { $items = ((Get-Content $fl | Measure-Object -Line).Lines - 1) }
    $after = @(Get-ChildItem $dumps -Filter *.hprof -ErrorAction SilentlyContinue).Count
    "$($r.n) | heap=$($r.x) | $line | items=$items | new_heap_dumps=$($after - $before)" | Out-File (Join-Path $Work "matrix.log") -Append -Encoding utf8
    if (Test-Path $out) { Remove-Item $out -Recurse -Force }
}
"matrix done" | Out-File (Join-Path $Work "matrix.log") -Append -Encoding utf8
