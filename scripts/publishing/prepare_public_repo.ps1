param(
    [string]$Source = (Resolve-Path "$PSScriptRoot\..\..").Path,
    [Parameter(Mandatory = $true)]
    [string]$Destination,
    [Parameter(Mandatory = $true)]
    [string]$GitUserName,
    [Parameter(Mandatory = $true)]
    [string]$GitUserEmail,
    [string]$RemoteUrl = "",
    [switch]$Commit
)

$ErrorActionPreference = "Stop"

function Assert-Command($Name) {
    if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
        throw "Required command not found: $Name"
    }
}

Assert-Command git
Assert-Command robocopy

$sourcePath = (Resolve-Path $Source).Path
$destinationPath = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($Destination)

if (Test-Path $destinationPath) {
    $existing = Get-ChildItem -LiteralPath $destinationPath -Force -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($existing) {
        throw "Destination already exists and is not empty: $destinationPath"
    }
} else {
    New-Item -ItemType Directory -Path $destinationPath | Out-Null
}

Write-Host "Copying source without private history or build artifacts..."
& robocopy $sourcePath $destinationPath /E /XD .git .tools dist release /XF *.exe *.log coverage.out
$code = $LASTEXITCODE
if ($code -gt 7) {
    throw "robocopy failed with exit code $code"
}

Push-Location $destinationPath
try {
    if (Test-Path ".git") {
        throw "Destination unexpectedly already contains .git"
    }

    & git init -b main
    & git config user.name $GitUserName
    & git config user.email $GitUserEmail
    & git config user.useConfigOnly true

    if ($RemoteUrl.Trim() -ne "") {
        & git remote add origin $RemoteUrl
    }

    & git add .

    if ($Commit) {
        & git commit -m "Initial public release"
    }

    Write-Host ""
    Write-Host "Public repository prepared at:"
    Write-Host $destinationPath
    Write-Host ""
    Write-Host "Verify before pushing:"
    Write-Host '  git log --format="%h %an <%ae> %s"'
    Write-Host "  git remote -v"
    Write-Host '  rg -n "<private-name>|<private-account>|<private-email>|<private-repo-name>|<private-path>" .'
} finally {
    Pop-Location
}
