param(
    [string]$DbName = "postgres",
    [string]$DbHost = "127.0.0.1",
    [int]$DbPort = 5432,
    [string]$ReadonlyUser = "",
    [string]$ClientIp = "127.0.0.1",
    [string]$AuthMethod = "md5",
    [string]$PsqlPath = "",
    [string]$PgHbaPath = "",
    [string]$PostgresServiceName = "",
    [switch]$DryRun
)

$ErrorActionPreference = "Stop"

function Read-Default {
    param([string]$Prompt, [string]$Default)
    $value = Read-Host "$Prompt [$Default]"
    if ([string]::IsNullOrWhiteSpace($value)) {
        return $Default
    }
    return $value.Trim()
}

function Assert-SafeAuthMethod {
    param([string]$Method)
    if ($Method -notin @("md5", "scram-sha-256")) {
        throw "Only md5 or scram-sha-256 are allowed for final read-only pg_hba lines."
    }
}

function Assert-SafeName {
    param([string]$Name, [string]$Label)
    if ($Name -notmatch '^[A-Za-z0-9_]+$') {
        throw "$Label must contain only letters, numbers, and underscore: $Name"
    }
}

function Get-HbaWarnings {
    param([string]$Path)
    if (-not $Path -or -not (Test-Path $Path)) { return @() }
    $warnings = @()
    $lines = Get-Content $Path
    foreach ($line in $lines) {
        $trimmed = $line.Trim()
        if ($trimmed.StartsWith("#") -or $trimmed -eq "") { continue }
        if ($trimmed -match '^host\s+all\s+all\s+0\.0\.0\.0/0\s+trust') {
            $warnings += "CRITICAL: broad trust line exists: $trimmed"
        } elseif ($trimmed -match '^host\s+all\s+all\s+0\.0\.0\.0/0\s+') {
            $warnings += "WARNING: broad network line exists before app-specific rules may match: $trimmed"
        }
    }
    return $warnings
}

function Find-Psql {
    if ($PsqlPath -and (Test-Path $PsqlPath)) { return $PsqlPath }
    $cmd = Get-Command psql.exe -ErrorAction SilentlyContinue
    if ($cmd) { return $cmd.Source }
    $candidates = @(
        "C:\Program Files\PostgreSQL\*\bin\psql.exe",
        "C:\Program Files (x86)\PostgreSQL\*\bin\psql.exe"
    )
    foreach ($pattern in $candidates) {
        $hit = Get-ChildItem $pattern -ErrorAction SilentlyContinue | Sort-Object FullName -Descending | Select-Object -First 1
        if ($hit) { return $hit.FullName }
    }
    throw "psql.exe not found. Pass -PsqlPath."
}

function Select-PathCandidate {
    param([string[]]$Candidates, [string]$Label)
    $resolved = @(
        $Candidates |
            Where-Object { $_ -and (Test-Path $_) } |
            ForEach-Object { (Resolve-Path $_).Path } |
            Sort-Object -Unique
    )
    if ($resolved.Count -eq 0) {
        throw "$Label not found. Pass -PgHbaPath."
    }
    if ($resolved.Count -eq 1) {
        return $resolved[0]
    }

    Write-Host "Multiple $Label candidates found:"
    for ($i = 0; $i -lt $resolved.Count; $i++) {
        Write-Host ("[{0}] {1}" -f ($i + 1), $resolved[$i])
    }
    while ($true) {
        $choice = Read-Host "Select $Label number"
        $index = 0
        if ([int]::TryParse($choice, [ref]$index) -and $index -ge 1 -and $index -le $resolved.Count) {
            return $resolved[$index - 1]
        }
        Write-Warning "Enter a number from 1 to $($resolved.Count)."
    }
}

function Get-PostgresDataDirsFromServices {
    $dirs = @()
    $services = Get-CimInstance Win32_Service -ErrorAction SilentlyContinue |
        Where-Object { $_.Name -match "postgres" -or $_.DisplayName -match "postgres" }
    foreach ($service in $services) {
        $path = $service.PathName
        if (-not $path) { continue }
        if ($path -match '-D\s+"([^"]+)"') {
            $dirs += $Matches[1]
        } elseif ($path -match '-D\s+([^\s]+)') {
            $dirs += $Matches[1].Trim('"')
        }
    }
    return $dirs
}

function Find-PgHba {
    if ($PgHbaPath) {
        if (Test-Path $PgHbaPath) {
            return (Resolve-Path $PgHbaPath).Path
        }
        throw "Invalid -PgHbaPath: $PgHbaPath"
    }

    $candidates = @()
    if ($env:PGDATA) {
        $candidates += Join-Path $env:PGDATA "pg_hba.conf"
    }
    foreach ($dir in Get-PostgresDataDirsFromServices) {
        $candidates += Join-Path $dir "pg_hba.conf"
    }
    $candidates += Get-ChildItem "C:\Program Files\PostgreSQL\*\data\pg_hba.conf" -ErrorAction SilentlyContinue | ForEach-Object { $_.FullName }
    $candidates += Get-ChildItem "C:\Program Files (x86)\PostgreSQL\*\data\pg_hba.conf" -ErrorAction SilentlyContinue | ForEach-Object { $_.FullName }
    $candidates += Get-ChildItem "C:\ProgramData\PostgreSQL\*\data\pg_hba.conf" -ErrorAction SilentlyContinue | ForEach-Object { $_.FullName }
    return Select-PathCandidate $candidates "pg_hba.conf"
}

function Add-ReadOnlyHbaLine {
    param([string]$Path)
    $line = "host    $DbName    $ReadonlyUser    $ClientIp/32    $AuthMethod"
    if ($line -match "0\.0\.0\.0/0" -or $line -match "\s+trust\s*$") {
        throw "Unsafe pg_hba line refused: $line"
    }
    if ($DryRun) {
        Write-Host "[DRY-RUN] Would insert pg_hba line near the top, before broad host rules:"
        Write-Host $line
        return
    }
    $timestamp = Get-Date -Format "yyyyMMdd_HHmmss"
    Copy-Item $Path "$Path.opendrugbridge.bak.$timestamp"
    $original = Get-Content $Path -Raw
    Set-Content $Path ("# Open Drug Bridge - Eghis read-only API`r`n$line`r`n" + $original) -Encoding ASCII
}

function Invoke-PsqlSql {
    param([string]$Sql)
    $tmp = New-TemporaryFile
    try {
        Set-Content -Path $tmp -Value $Sql -Encoding UTF8
        & $psql -h $DbHost -p $DbPort -d $DbName -U postgres -f $tmp
        if ($LASTEXITCODE -ne 0) {
            throw "psql exited with code $LASTEXITCODE"
        }
    } finally {
        Remove-Item $tmp -Force -ErrorAction SilentlyContinue
    }
}

function Invoke-PsqlSqlWithFallback {
    param([scriptblock]$SqlFactory)
    try {
        Invoke-PsqlSql (& $SqlFactory)
    } catch {
        Write-Warning "psql connection failed with db=$DbName host=$DbHost port=$DbPort."
        $script:DbName = Read-Default "Database name" $DbName
        $script:DbHost = Read-Default "Database host" $DbHost
        $script:DbPort = [int](Read-Default "Database port" ([string]$DbPort))
        Assert-SafeName $DbName "DbName"
        Invoke-PsqlSql (& $SqlFactory)
    }
}

Assert-SafeAuthMethod $AuthMethod
Assert-SafeName $DbName "DbName"
if ([string]::IsNullOrWhiteSpace($ReadonlyUser)) {
    $ReadonlyUser = Read-Default "New read-only username" "eghis_drug_ro"
}
Assert-SafeName $ReadonlyUser "ReadonlyUser"
$psql = Find-Psql
$PgHbaPath = Find-PgHba
Write-Host "psql: $psql"
Write-Host "database: $DbName"
Write-Host "database host: $DbHost"
Write-Host "database port: $DbPort"
Write-Host "pg_hba.conf: $PgHbaPath"
Get-HbaWarnings $PgHbaPath | ForEach-Object { Write-Warning $_ }

if ($DryRun) {
    $plainPassword = "DRY_RUN_PASSWORD"
} else {
    $password = Read-Host "New read-only password" -AsSecureString
    $bstr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($password)
    $plainPassword = [Runtime.InteropServices.Marshal]::PtrToStringBSTR($bstr)
    [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($bstr)
}

function New-SetupSql {
@"
CREATE USER $ReadonlyUser WITH PASSWORD '$plainPassword';
GRANT CONNECT ON DATABASE $DbName TO $ReadonlyUser;
GRANT USAGE ON SCHEMA public TO $ReadonlyUser;
GRANT SELECT ON TABLE h0_mst_drug TO $ReadonlyUser;
GRANT SELECT ON TABLE h0drug_stock TO $ReadonlyUser;
GRANT SELECT ON TABLE h1opdin TO $ReadonlyUser;
GRANT SELECT ON TABLE h2opd_doct_ord TO $ReadonlyUser;
GRANT SELECT ON TABLE h8_nims_buy TO $ReadonlyUser;
GRANT SELECT ON TABLE h8_nims_buy_lines TO $ReadonlyUser;
GRANT SELECT ON TABLE h8_nims_medi TO $ReadonlyUser;
GRANT SELECT ON TABLE h8_nims_medi_lines TO $ReadonlyUser;
GRANT SELECT ON TABLE h8_nims_exp TO $ReadonlyUser;
GRANT SELECT ON TABLE h8_nims_exp_lines TO $ReadonlyUser;
GRANT SELECT ON TABLE h8_nims_send TO $ReadonlyUser;
GRANT SELECT ON TABLE h8_nims_send_lines TO $ReadonlyUser;
"@
}

$sql = New-SetupSql

if ($DryRun) {
    Write-Host "[DRY-RUN] Would run SQL with password redacted:"
    Write-Host ($sql -replace [regex]::Escape($plainPassword), "********")
} else {
    Invoke-PsqlSqlWithFallback { New-SetupSql }
}

Add-ReadOnlyHbaLine $PgHbaPath
if (-not $DryRun -and $PostgresServiceName) {
    Restart-Service $PostgresServiceName
}
