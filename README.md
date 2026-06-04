# Drug Storage Bridge

Drug Storage Bridge is a Windows local API and Web UI for drug-only inventory and prescription usage queries from an EMR PostgreSQL database.

Version `0.1.0` supports the Eghis PostgreSQL schema.

## Security Defaults

- The app listens on `127.0.0.1:3987` by default.
- Use a PostgreSQL read-only account.
- Do not store PostgreSQL superuser credentials in the app.
- Patient-identifying APIs are not provided.
- Remote access should use a clinic LAN or VPN such as Tailscale, ZeroTier, NetBird, or an existing hospital VPN.
- Do not expose this app with router port forwarding or a public IP.

## Responsibility Notice

Drug Storage Bridge is a read-only drug data tool. It does not replace server security administration.

PostgreSQL account management, `pg_hba.conf` changes, firewall rules, backup storage policy, and compliance with clinic security/privacy rules are the responsibility of the server administrator and owner.

## Quick Start

```powershell
.\drug-storage-bridge.exe
```

Open:

```text
http://127.0.0.1:3987/ui
```

On first launch, enter the Eghis PostgreSQL read-only account.

## Configuration

Default config path:

```text
%APPDATA%\OpenDrugBridge\config.yaml
```

Passwords are stored separately using Windows DPAPI-protected credential files. The config file stores only a credential reference.

## Remote Access

Default usage is same-PC only.

Recommended options for access from another PC:

1. Tailscale
2. ZeroTier
3. NetBird
4. Existing hospital VPN
5. Clinic LAN with Windows Firewall IP restrictions

Do not use:

- Router port forwarding
- Public IP direct exposure
- Firewall allow-all rules

Example for Tailscale:

```yaml
server:
  host: 0.0.0.0
  port: 3987
  access_token_required: true
  allowed_cidrs:
    - 100.64.0.0/10
  access_token_hash: "PUT_SHA256_HASH_HERE"
```

Create an access token hash on the server PC:

```powershell
.\drug-storage-bridge.exe --hash-token "long-random-token"
```

Store only the printed hash in `config.yaml`. Keep the original token private. Setup-changing APIs require a temporary setup token that is exposed only to loopback requests, so DB credentials should be configured on the server PC itself.

## Server Setup Scripts

Eghis scripts are under:

```text
scripts/eghis/
```

Server-local defaults:

- Database host: `127.0.0.1`
- Database port: `5432`
- Database name: `postgres`

Use `create_eghis_drug_readonly.ps1` if you know the PostgreSQL superuser password.

Use `bootstrap_eghis_drug_readonly.ps1` only if you own/administer the server but do not know the PostgreSQL superuser password. It temporarily adds local-only trust authentication, creates a read-only account, and removes the temporary trust lines.

Run scripts in dry-run mode first. The scripts auto-detect `pg_hba.conf`; if multiple files are found, choose the matching PostgreSQL installation by number.

```powershell
.\scripts\eghis\bootstrap_eghis_drug_readonly.ps1 -DryRun
.\scripts\eghis\bootstrap_eghis_drug_readonly.ps1
```

During execution, enter the new read-only username and password. If the default local connection does not work, the script asks for database name, host, and port and retries once.

The bootstrap script is intentionally conservative:

- It requires Administrator PowerShell for actual changes.
- It inserts temporary `trust` authentication only for local `postgres` connections.
- It refuses broad `trust` lines.
- It backs up `pg_hba.conf` and restores it on failure.
- It warns if broad `0.0.0.0/0` lines already exist.

## API

```text
GET  /health
GET  /version
GET  /ui
GET  /api/setup/status
POST /api/setup/test-connection
POST /api/setup/save
GET  /api/drugs/search?q=
GET  /api/drugs/{code}
GET  /api/drugs/{code}/stock
GET  /api/stocks
GET  /api/usage?days=30
GET  /api/usage?from=YYYYMMDD&to=YYYYMMDD
GET  /api/user-codes/{code}/stock
GET  /api/user-codes/{code}/usage?days=30
GET  /api/user-codes/{code}/usage?from=YYYYMMDD&to=YYYYMMDD
GET  /api/inventory/order-plan?from=YYYYMMDD&to=YYYYMMDD&target_days=45
GET  /api/inventory/order-plan.xlsx?from=YYYYMMDD&to=YYYYMMDD&target_days=45
```

Examples:

```powershell
Invoke-RestMethod "http://127.0.0.1:3987/api/stocks"
Invoke-RestMethod "http://127.0.0.1:3987/api/usage?days=30"
Invoke-RestMethod "http://127.0.0.1:3987/api/usage?from=20260501&to=20260531"
Invoke-RestMethod "http://127.0.0.1:3987/api/user-codes/651900680/stock"
Invoke-RestMethod "http://127.0.0.1:3987/api/user-codes/651900680/usage?days=90"
```

## Build

```powershell
go mod tidy
go test ./...
go build -trimpath -ldflags="-s -w" -o dist\drug-storage-bridge.exe .\cmd\drug-storage-bridge
```

## Publishing

If publishing this project as a separate public repository, do not reuse the
private parent repository history. Follow [PUBLISHING.md](PUBLISHING.md) to
create a fresh repository with public-account commit metadata.
