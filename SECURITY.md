# Security

Use the latest release for fixes. Older releases are not maintained as separate security branches.

## Report a vulnerability

Use GitHub's **Security > Advisories > Report a vulnerability** for a private report when enabled:

https://github.com/thegeekkid/church-device-control/security/advisories/new

If that option is unavailable, open an issue containing only a request for a private reporting channel, without exploit details or secrets. Include affected versions, impact, and reproduction steps in the private report. Do not post Kasa passwords, LAN sync keys, or real device inventories publicly.

## Local-network trust

The app sends commands directly to devices on a trusted LAN. Roku ECP and legacy Kasa control rely on the device/network's access controls; the app does not make those protocols safe for an untrusted network. Do not expose device control ports to the Internet.

LAN settings sync uses authenticated AES-GCM packets with a shared key. Anyone with that key and network access can change shared device settings. It is not a per-user authorization system. Device settings and deletion records sync; power commands, credentials, and observed state do not.

Kasa credentials and the sync key are protected locally with Windows DPAPI for the current user. DPAPI does not protect against code already running as that user. Treat settings backups and copied diagnostic text accordingly.

## Release identity

Official signed releases use Azure Artifact Signing with an OIDC identity restricted to this repository's `main`. Public pull-request CI does not receive signing permission. Signing authenticates the publisher and file integrity; it is not a guarantee that a program is free of defects. See [Signing and releases](docs/SIGNING.md).
