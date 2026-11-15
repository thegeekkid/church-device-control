# Security

Use the latest release for fixes. Older releases are not maintained as separate security branches.

## Report a vulnerability

To arrange a private report, [open an issue](https://github.com/thegeekkid/church-device-control/issues/new) requesting a private reporting channel. Leave vulnerability details and secrets out of the issue; share affected versions, impact, and reproduction steps privately once a channel is arranged.

## Local-network trust

The app sends commands directly to devices on a trusted LAN. Roku ECP and legacy Kasa control rely on the device/network's access controls; the app does not make those protocols safe for an untrusted network. Do not expose device control ports to the Internet.

LAN settings sync uses authenticated AES-GCM packets with a shared key. Anyone with that key and network access can change shared device settings. It is not a per-user authorization system. Device settings and deletion records sync; power commands, credentials, and observed state do not.

Kasa credentials and the sync key are protected locally with Windows DPAPI for the current user. DPAPI does not protect against code already running as that user. Treat settings backups and copied diagnostic text accordingly.

## Release identity

Official releases use Azure Artifact Signing with an OIDC identity restricted to this repository's `main`. Pull-request CI does not receive signing permission. See [Signing and releases](docs/SIGNING.md).
