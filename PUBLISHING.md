# Publishing Guide

This guide is for publishing Open Drug Bridge as a separate public repository
without carrying private repository history or personal commit metadata.

## Privacy Boundary

Do not push the existing parent repository or its `.git` history to the public
repository. The current history may contain personal author names, personal
email addresses, old project names, and unrelated work.

For public distribution, copy only the `open-drug-bridge` source tree into a new
directory and initialize a fresh git repository there.

GitHub privacy settings can hide the email address used for future commits, but
they do not remove email addresses already stored in existing git commits.

## Before Creating the Public Repository

1. Create or sign in to the GitHub account that will own the public repository.
2. In GitHub email settings, enable:
   - Keep my email addresses private
   - Block command line pushes that expose my email
3. Copy that account's GitHub-provided `noreply` email address.
4. Use an SSH key or GitHub CLI login that belongs only to that account.
5. Do not reuse the current repository remote.

## Fresh Repository Commands

Use the helper script from the private checkout. Replace the values in angle
brackets before running.

```powershell
.\scripts\publishing\prepare_public_repo.ps1 `
  -Destination "$env:USERPROFILE\Documents\GitHub\open-drug-bridge-public" `
  -GitUserName "<public-display-name>" `
  -GitUserEmail "<github-noreply-email>" `
  -RemoteUrl "git@github.com:<anonymous-account>/open-drug-bridge.git" `
  -Commit
```

Then verify and push:

```powershell
Set-Location "$env:USERPROFILE\Documents\GitHub\open-drug-bridge-public"
git log --format="%h %an <%ae> %s"
git remote -v
rg -n "<private-name>|<private-account>|<private-email>|<private-repo-name>|<private-path>" .
git push -u origin main
```

Manual equivalent:

```powershell
$src = "C:\path\to\private\open-drug-bridge"
$dst = "$env:USERPROFILE\Documents\GitHub\open-drug-bridge-public"

robocopy $src $dst /E /XD .git .tools dist release /XF *.exe *.log coverage.out
Set-Location $dst

git init -b main
git config user.name "<public-display-name>"
git config user.email "<github-noreply-email>"
git config user.useConfigOnly true

git add .
git commit -m "Initial public release"
git remote add origin git@github.com:<anonymous-account>/open-drug-bridge.git
git push -u origin main
```

If you use HTTPS instead of SSH, make sure the credential manager is logged in
to the public account, not your personal account.

## Verify Before Push

Check that the new repository has no old personal metadata.

```powershell
git log --format="%h %an <%ae> %s"
git remote -v
rg -n "<private-name>|<private-account>|<private-email>|<private-repo-name>|<private-path>" .
```

Expected result:

- `git log` shows only the public display name and GitHub `noreply` email.
- `git remote -v` points to the public account's repository.
- `rg` finds no personal account, personal email, or private parent path.

## Build Release Binary

Use `-trimpath` for local builds so debug path metadata does not include the
local checkout path.

```powershell
go test ./...
go vet ./...
go build -trimpath -ldflags="-s -w" -o dist\drug-storage-bridge.exe .\cmd\drug-storage-bridge
```

For GitHub Actions release artifacts, use the same build flags.

## What Not To Publish

Do not include:

- `%APPDATA%\OpenDrugBridge\config.yaml`
- Windows DPAPI credential files
- PostgreSQL passwords or access tokens
- Clinic or patient data
- `dist/` or `release/` artifacts built from the private checkout
- The private parent repository's `.git` directory
