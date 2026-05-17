$ErrorActionPreference = "Stop"

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$devBinary = (Join-Path $repoRoot "tmp\erg-server.exe")
$devBinaryLower = $devBinary.ToLowerInvariant()

$processes = Get-CimInstance Win32_Process -Filter "Name = 'erg-server.exe'" | Where-Object {
    ($_.ExecutablePath -and $_.ExecutablePath.ToLowerInvariant() -eq $devBinaryLower) -or
    ($_.CommandLine -and $_.CommandLine.ToLowerInvariant().Contains($devBinaryLower))
}

if (-not $processes) {
    Write-Host "dev-preflight: no stale tmp\erg-server.exe process found"
    exit 0
}

foreach ($process in $processes) {
    Write-Host ("dev-preflight: stopping stale tmp\erg-server.exe pid={0}" -f $process.ProcessId)
    Stop-Process -Id $process.ProcessId -Force -ErrorAction Stop
}

Start-Sleep -Milliseconds 300

$remaining = Get-CimInstance Win32_Process -Filter "Name = 'erg-server.exe'" | Where-Object {
    ($_.ExecutablePath -and $_.ExecutablePath.ToLowerInvariant() -eq $devBinaryLower) -or
    ($_.CommandLine -and $_.CommandLine.ToLowerInvariant().Contains($devBinaryLower))
}

if ($remaining) {
    $ids = ($remaining | ForEach-Object { $_.ProcessId }) -join ", "
    Write-Error "dev-preflight: stale process still running: $ids"
    exit 1
}

Write-Host "dev-preflight: stale process stopped"
