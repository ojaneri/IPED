# Measures peak memory of an IPED processing run (whole process tree) on Windows.
#
# Usage:
#   .\measure-processing.ps1 -IpedDir D:\iped-bench\rel-base `
#       -Evidence D:\corpus\disk.E01 -Output D:\iped-bench\out-base -Profile default [-ExtraArgs "-Xmx1500m"]
#
# Samples the whole process tree of iped.jar every few seconds. For each sample it records the sum of
# the working set (RAM resident) and of the private bytes (committed memory) of all processes.
# The CSV is written next to the output folder (<Output>.memory.csv), because IPED wants an empty
# output folder. Compare the summary line between runs in lite\RESULTS.md.
param(
    [Parameter(Mandatory = $true)][string]$IpedDir,
    [Parameter(Mandatory = $true)][string]$Evidence,
    [Parameter(Mandatory = $true)][string]$Output,
    [string]$Profile = "default",
    [string]$ExtraArgs = "",
    [int]$SampleSeconds = 3
)

$ErrorActionPreference = "Stop"
$jar = Join-Path $IpedDir "iped.jar"
$java = Join-Path $IpedDir "jre\bin\java.exe"
if (-not (Test-Path $jar)) { throw "iped.jar not found in $IpedDir" }
if (-not (Test-Path $java)) { $java = "java" }
if (Test-Path $Output) { throw "Output folder already exists, use a new one: $Output" }

function Quote([string]$s) { '"' + $s + '"' }

$iArgs = @("-jar", (Quote $jar), "-d", (Quote $Evidence), "-o", (Quote $Output), "--nogui", "--nologfile")
if ($Profile -ne "default") { $iArgs += @("-profile", $Profile) }
if ($ExtraArgs -ne "") { $iArgs += $ExtraArgs.Split(" ") }

$csv = "$Output.memory.csv"
"time_s,process_count,total_ws_mb,total_private_mb,max_single_private_mb" | Out-File $csv -Encoding ascii

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
$proc = Start-Process -FilePath $java -ArgumentList $iArgs -PassThru -NoNewWindow
$peakWs = 0; $peakPriv = 0; $peakSingle = 0; $peakCount = 0

while (-not $proc.HasExited) {
    Start-Sleep -Seconds $SampleSeconds
    $ids = Get-Tree $proc.Id
    $ws = 0; $priv = 0; $single = 0; $count = 0
    foreach ($id in $ids) {
        $p = Get-Process -Id $id -ErrorAction SilentlyContinue
        if ($p) {
            $count++
            $ws += $p.WorkingSet64 / 1MB
            $pm = $p.PrivateMemorySize64 / 1MB
            $priv += $pm
            if ($pm -gt $single) { $single = $pm }
        }
    }
    "{0},{1},{2:N0},{3:N0},{4:N0}" -f [int]$sw.Elapsed.TotalSeconds, $count, $ws, $priv, $single | Out-File $csv -Append -Encoding ascii
    if ($ws -gt $peakWs) { $peakWs = $ws }
    if ($priv -gt $peakPriv) { $peakPriv = $priv }
    if ($single -gt $peakSingle) { $peakSingle = $single }
    if ($count -gt $peakCount) { $peakCount = $count }
}

"profile=$Profile exit=$($proc.ExitCode) elapsed_s=$([int]$sw.Elapsed.TotalSeconds) peak_tree_ws_mb=$([int]$peakWs) peak_tree_private_mb=$([int]$peakPriv) peak_single_private_mb=$([int]$peakSingle) max_processes=$peakCount"
