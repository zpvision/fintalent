param(
    [string]$SourceUrl = 'https://rosstat.gov.ru/opendata/7708234640-okvedva/data-20260801T1408-structure-20180402T1704.csv'
)

$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
$dataDir = Join-Path $projectRoot 'data'
$target = Join-Path $dataDir 'okved.csv'
New-Item -ItemType Directory -Force -Path $dataDir | Out-Null

$temporary = Join-Path ([System.IO.Path]::GetTempPath()) ('fintalent-okved-' + [guid]::NewGuid().ToString('N') + '.csv')
try {
    & curl.exe -k -L --fail --max-time 120 -sS -o $temporary $SourceUrl
    if ($LASTEXITCODE -ne 0) { throw "OKVED download failed: curl exit code $LASTEXITCODE" }
    $lines = [System.IO.File]::ReadAllLines($temporary, [System.Text.Encoding]::UTF8)
    if ($lines.Count -lt 3000 -or $lines[0] -notmatch '^"A";') { throw 'Invalid OKVED data file' }
    [System.IO.File]::WriteAllLines($target, $lines, (New-Object System.Text.UTF8Encoding($false)))
    Write-Output "OKVED updated: $($lines.Count) rows, $target"
}
finally {
    Remove-Item -LiteralPath $temporary -Force -ErrorAction SilentlyContinue
}
