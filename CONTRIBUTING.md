# Contributing

Bug reports, documentation fixes, and focused pull requests are welcome. For a device-compatibility problem, include the app version, Windows version, device model and firmware, steps to reproduce, and full text from **Details**. Remove account credentials, sync keys, and identifying network data before posting.

Start a discussion in an issue before a large change. Keep pull requests focused and describe what behavior changes and how you tested it. Hardware validation is useful because CI uses simulated devices.

See [Development](docs/DEVELOPMENT.md) for the Go build and test commands. Run `go test ./...` and `go vet ./...`; Windows changes also need Windows CI. Shared settings changes must follow the [sync compatibility contract](docs/SYNC-PROTOCOL.txt) and preserve mixed-version behavior.

Use your own local settings for hardware tests; do not commit `tvs.json`, `local-secrets.bin`, real credentials, or LAN sync keys. Tests should use clearly artificial fixtures.

Original project contributions are made under the repository's MIT license. Do not copy code, icons, or other assets without a compatible license and the required notices. The current Kasa dependency has an [unresolved license](THIRD-PARTY-NOTICES.txt); do not assume that public source is sufficient permission.

Contributor CI does not sign or publish builds. Official releases are a maintainer operation from trusted `main`. Changes to workflows, dependencies, authentication, and sync deserve particular care in review.

For a vulnerability, follow [Security](SECURITY.md) rather than posting sensitive details in a public issue.
