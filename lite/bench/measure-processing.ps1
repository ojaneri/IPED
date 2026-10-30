# Measures peak memory of an IPED processing run (whole process tree) on Windows.
#
# Usage:
#   .\measure-processing.ps1 -IpedDir D:\IPED\target\release\iped-4.4.0-SNAPSHOT `
#       -Evidence D:\corpus\disk.E01 -Output D:\corpus\out-lite -Profile lite [-ExtraArgs "-Xmx3g"]
#
# Samples the working set of iped.exe/java children every 2s and writes a CSV plus a summary line.
# After the run, compare the summary with the baseline run (-Profile default) in lite\RESULTS.md.
param(
    [Parameter(Mandatory = $true)][string]$IpedDir,
    [Parameter(Mandatory = $true)][string]$Evidence,
    [Parameter(Mandatory = $true)][string]$Output,
    [string]$Profile = "default",
    [string]$ExtraArgs = "",
    [int]$SampleSeconds = 2
)

$ErrorActionPreference = "Stop"
$jar = Join-Path $IpedDir "iped.jar"
$java = Join-Path $IpedDir "jre\bin\java.exe"
if (-not (Test-Path $jar)) { throw "iped.jar not found in $IpedDir" }
if (-not (Test-Path $java)) { $java = "java" }

$args = @("-jar", $jar, "-d", $Evidence, "-o", $Output, "--nogui", "--nologfile")
if ($Profile -ne "default") { $args += @("-profile", $Profile) }
if ($ExtraArgs -ne "") { $args += $ExtraArgs.Split(" ") }

$csv = Join-Path $Output "memory-samples.csv"
New-Item -ItemType Directory -Force -Path $Output | Out-Null
"time_s,process_count,total_ws_mb,max_single_ws_mb" | Out-File $csv -Encoding ascii

function Get-Tree([int]$rootPid) {
    $all = Get-CimInstance Win32_Process | Select-Object ProcessId, ParentProcessId
    $ids = New-Object System.Collections.Generic.HashSet[int]
    [void]$ids.Add($rootPid)
    $changed = $true
    while ($changed) {
        $changed = $false
        foreach ($p in $all) {
            if ($ids.Contains([int]$p.ParentProcessId) -and -not $ids.Contains([int]$p.ProcessId)) {
                [void]$ids.Add([int]$p.ProcessId); $changed = $true
            }
        }
    }
    return $ids
}

$sw = [Diagnostics.Stopwatch]::StartNew()
$proc = Start-Process -FilePath $java -ArgumentList $args -PassThru -NoNewWindow
$peakTotal = 0; $peakSingle = 0; $peakCount = 0

while (-not $proc.HasExited) {
    Start-Sleep -Seconds $SampleSeconds
    $ids = Get-Tree $proc.Id
    $total = 0; $single = 0; $count = 0
    foreach ($id in $ids) {
        $p = Get-Process -Id $id -ErrorAction SilentlyContinue
        if ($p) {
            $mb = $p.WorkingSet64 / 1MB
            $total += $mb; $count++
            if ($mb -gt $single) { $single = $mb }
        }
    }
    "{0},{1},{2:N0},{3:N0}" -f [int]$sw.Elapsed.TotalSeconds, $count, $total, $single | Out-File $csv -Append -Encoding ascii
    if ($total -gt $peakTotal) { $peakTotal = $total }
    if ($single -gt $peakSingle) { $peakSingle = $single }
    if ($count -gt $peakCount) { $peakCount = $count }
}

"profile=$Profile exit=$($proc.ExitCode) elapsed_s=$([int]$sw.Elapsed.TotalSeconds) peak_tree_ws_mb=$([int]$peakTotal) peak_single_proc_ws_mb=$([int]$peakSingle) max_processes=$peakCount"
