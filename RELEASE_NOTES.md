# Drug Storage Bridge 0.1.0

Initial Windows preview release for Eghis PostgreSQL drug inventory automation.

## Included

- Local Web UI and HTTP API.
- Eghis drug search, prescription usage query, stock calculation, order planning, and XLSX export.
- Windows DPAPI-protected password storage.
- Read-only PostgreSQL account validation with superuser/write-account save blocking.
- Optional LAN/VPN binding with CIDR and access-token controls.
- Eghis read-only account creation scripts, including a guarded local-auth bootstrap script.

## Known Limits

- Only the Eghis adapter is implemented.
- Setup-changing actions must be performed from the server PC itself.
- Remote access must be limited to clinic LAN or VPN. Router port forwarding and public IP exposure are unsupported.
- NIMS stock calculation depends on Eghis NIMS report tables being present and populated.
