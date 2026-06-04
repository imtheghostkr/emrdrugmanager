param(
    [string]$DbName = "postgres",
    [string]$DbHost = "127.0.0.1",
    [int]$DbPort = 5432,
    [string]$ReadonlyUser = "",
    [string]$ClientIp = "127.0.0.1",
    [string]$AuthMethod = "md5",
    [string]$PgHbaPath = "",
    [string]$PsqlPath = "",
    [string]$PostgresServiceName = "",
    [switch]$DryRun
)

$ErrorActionPreference = "Stop"

function Test-Admin {
    $id = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal($id)
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Read-Default {
    param([string]$Prompt, [string]$Default)
    $value = Read-Host "$Prompt [$Default]"
    if ([string]::IsNullOrWhiteSpace($value)) {
        return $Default
    }
    return $value.Trim()
}

function Assert-SafeName {
    param([string]$Name, [string]$Label)
    if ($Name -notmatch '^[A-Za-z0-9_]+$') {
        throw "$Label must contain only letters, numbers, and underscore: $Name"
    }
}

function Assert-SafeAuthMethod {
    param([string]$Method)
    if ($Method -notin @("md5", "scram-sha-256")) {
        throw "Final auth method must be md5 or scram-sha-256."
    }
}

function Assert-SafeHbaLine {
    param([string]$Line)
    if ($Line -match "0\.0\.0\.0/0\s+trust" -or $Line -match "host\s+all\s+all\s+.*\strust\s*$") {
        throw "Unsafe pg_hba line refused: $Line"
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
    foreach ($pattern in @("C:\Program Files\PostgreSQL\*\bin\psql.exe", "C:\Program Files (x86)\PostgreSQL\*\bin\psql.exe")) {
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
        $dir = Get-DataDirFromServicePath $service.PathName
        if ($dir) {
            $dirs += $dir
        }
    }
    return $dirs
}

function Get-DataDirFromServicePath {
    param([string]$PathName)
    if (-not $PathName) { return "" }
    if ($PathName -match '-D\s+"([^"]+)"') {
        return $Matches[1]
    }
    if ($PathName -match '-D\s+([^\s]+)') {
        return $Matches[1].Trim('"')
    }
    return ""
}

function Find-PostgresServiceName {
    param([string]$SelectedPgHbaPath)
    if ($PostgresServiceName) { return $PostgresServiceName }

    $hbaDir = (Resolve-Path (Split-Path $SelectedPgHbaPath -Parent)).Path
    $serviceMatches = @()
    $services = Get-CimInstance Win32_Service -ErrorAction SilentlyContinue |
        Where-Object { $_.Name -match "postgres" -or $_.DisplayName -match "postgres" }
    foreach ($service in $services) {
        $dir = Get-DataDirFromServicePath $service.PathName
        if (-not $dir -or -not (Test-Path $dir)) { continue }
        $resolved = (Resolve-Path $dir).Path
        if ($resolved -eq $hbaDir) {
            $serviceMatches += $service.Name
        }
    }
    $serviceMatches = @($serviceMatches | Sort-Object -Unique)
    if ($serviceMatches.Count -eq 0) { return "" }
    if ($serviceMatches.Count -eq 1) { return $serviceMatches[0] }

    Write-Host "Multiple PostgreSQL services match selected pg_hba.conf:"
    for ($i = 0; $i -lt $serviceMatches.Count; $i++) {
        Write-Host ("[{0}] {1}" -f ($i + 1), $serviceMatches[$i])
    }
    while ($true) {
        $choice = Read-Host "Select PostgreSQL service number"
        $index = 0
        if ([int]::TryParse($choice, [ref]$index) -and $index -ge 1 -and $index -le $serviceMatches.Count) {
            return $serviceMatches[$index - 1]
        }
        Write-Warning "Enter a number from 1 to $($serviceMatches.Count)."
    }
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

function Sql-Literal {
    param([string]$Value)
    return "'" + ($Value -replace "'", "''") + "'"
}

function Invoke-Psql {
    param([string]$Psql, [string]$Sql)
    $tmp = New-TemporaryFile
    try {
        Set-Content -Path $tmp -Value $Sql -Encoding UTF8
        & $Psql -h $DbHost -p $DbPort -d $DbName -U postgres -v ON_ERROR_STOP=1 -f $tmp
        if ($LASTEXITCODE -ne 0) {
            throw "psql exited with code $LASTEXITCODE"
        }
    } finally {
        Remove-Item $tmp -Force -ErrorAction SilentlyContinue
    }
}

function Invoke-PsqlWithFallback {
    param([string]$Psql, [string]$Sql)
    try {
        Invoke-Psql $Psql $Sql
    } catch {
        Write-Warning "psql connection failed with db=$DbName host=$DbHost port=$DbPort."
        $script:DbName = Read-Default "Database name" $DbName
        $script:DbHost = Read-Default "Database host" $DbHost
        $script:DbPort = [int](Read-Default "Database port" ([string]$DbPort))
        Assert-SafeName $DbName "DbName"
        Invoke-Psql $Psql $Sql
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
$PostgresServiceName = Find-PostgresServiceName $PgHbaPath
$tempLines = @(
    "# TEMP Open Drug Bridge bootstrap - local only",
    "host    all    postgres    127.0.0.1/32    trust",
    "host    all    postgres    ::1/128         trust"
)
$readonlyLine = "host    $DbName    $ReadonlyUser    $ClientIp/32    $AuthMethod"
($tempLines + $readonlyLine) | ForEach-Object { Assert-SafeHbaLine $_ }

Write-Host "psql: $psql"
Write-Host "database: $DbName"
Write-Host "database host: $DbHost"
Write-Host "database port: $DbPort"
Write-Host "pg_hba.conf: $PgHbaPath"
Write-Host "PostgreSQL service: $PostgresServiceName"
Get-HbaWarnings $PgHbaPath | ForEach-Object { Write-Warning $_ }
Write-Host "Temporary local-only trust lines:"
$tempLines | ForEach-Object { Write-Host $_ }
Write-Host "Final read-only line:"
Write-Host $readonlyLine

$sql = @"
SELECT version();
SELECT current_user;
"@

$createSql = "SELECT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '$ReadonlyUser');"

$grantSql = @"
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

if ($DryRun) {
    Write-Host "[DRY-RUN] Would backup pg_hba.conf, prepend local-only trust, reload/restart PostgreSQL, create/alter read-only user, insert final read-only line near the top, remove temporary trust, reload/restart again."
    Write-Host "[DRY-RUN] SQL preview:"
    Write-Host "CREATE USER/ALTER USER $ReadonlyUser WITH PASSWORD '********';"
    Write-Host $grantSql
    exit 0
}

if (-not (Test-Admin)) {
    throw "Run PowerShell as Administrator."
}

$password = Read-Host "New read-only password" -AsSecureString
$bstr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($password)
$plainPassword = [Runtime.InteropServices.Marshal]::PtrToStringBSTR($bstr)
[Runtime.InteropServices.Marshal]::ZeroFreeBSTR($bstr)
$passwordSql = Sql-Literal $plainPassword

$phrase = Read-Host "Type '로컬 임시 인증 후 복구' to continue"
if ($phrase -ne "로컬 임시 인증 후 복구") {
    throw "Confirmation phrase mismatch."
}

$timestamp = Get-Date -Format "yyyyMMdd_HHmmss"
$backup = "$PgHbaPath.opendrugbridge.bak.$timestamp"
Copy-Item $PgHbaPath $backup

try {
    $original = Get-Content $PgHbaPath -Raw
    Set-Content $PgHbaPath (($tempLines -join "`r`n") + "`r`n" + $original) -Encoding ASCII
    if ($PostgresServiceName) {
        Restart-Service $PostgresServiceName
    } else {
        Write-Warning "No -PostgresServiceName supplied. Reload/restart PostgreSQL manually now, then press Enter."
        Read-Host
    }

    Invoke-PsqlWithFallback $psql $sql
    $existsOutput = & $psql -h $DbHost -p $DbPort -d $DbName -U postgres -t -A -c $createSql
    if ($LASTEXITCODE -ne 0) {
        throw "role existence check failed with code $LASTEXITCODE"
    }
    $exists = (($existsOutput | Where-Object { $_ -match '\S' } | Select-Object -First 1) -as [string]).Trim()
    if ($exists -eq "t" -or $exists -eq "true" -or $exists -eq "1") {
        Invoke-PsqlWithFallback $psql "ALTER USER $ReadonlyUser WITH PASSWORD $passwordSql;"
    } else {
        Invoke-PsqlWithFallback $psql "CREATE USER $ReadonlyUser WITH PASSWORD $passwordSql;"
    }
    $grantSql = @"
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
    Invoke-PsqlWithFallback $psql $grantSql

    $readonlyLine = "host    $DbName    $ReadonlyUser    $ClientIp/32    $AuthMethod"
    Assert-SafeHbaLine $readonlyLine
    $cleaned = (Get-Content $PgHbaPath) | Where-Object {
        $_ -notmatch "TEMP Open Drug Bridge bootstrap" -and
        $_ -notmatch "host\s+all\s+postgres\s+127\.0\.0\.1/32\s+trust" -and
        $_ -notmatch "host\s+all\s+postgres\s+::1/128\s+trust"
    }
    Set-Content $PgHbaPath ("# Open Drug Bridge - Eghis read-only API`r`n$readonlyLine`r`n" + ($cleaned -join "`r`n")) -Encoding ASCII
    if (Select-String -Path $PgHbaPath -Pattern "TEMP Open Drug Bridge bootstrap|127\.0\.0\.1/32\s+trust|::1/128\s+trust" -Quiet) {
        throw "Temporary trust lines remain in pg_hba.conf."
    }
    if ($PostgresServiceName) {
        Restart-Service $PostgresServiceName
    }
    Write-Host "Bootstrap finished. Backup: $backup"
    Write-Host "Read-only user: $ReadonlyUser"
} catch {
    Write-Warning "Bootstrap failed: $_"
    Copy-Item $backup $PgHbaPath -Force
    if ($PostgresServiceName) {
        Restart-Service $PostgresServiceName
    } else {
        Write-Warning "Backup restored on disk. Restart/reload PostgreSQL manually now so restored pg_hba.conf is active."
    }
    throw
} finally {
    $plainPassword = $null
}
