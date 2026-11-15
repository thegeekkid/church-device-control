# Development

The app is written in Go with a native Win32 interface. It statically links its Kasa protocol dependency, so users do not install a language runtime. The UI currently targets Windows amd64; Linux can run the portable protocol and persistence tests.

## Build on Windows

Install Go 1.27.3 or later. From a source checkout, in PowerShell:

```powershell
go test ./...
go vet ./...
$env:GOOS = 'windows'
$env:GOARCH = 'amd64'
$env:CGO_ENABLED = '0'
New-Item -ItemType Directory -Force build | Out-Null
go build -trimpath -ldflags='-s -w -H=windowsgui' -o build/Church-TV-Control.exe .
```

Developer builds are unsigned unless you configure your own signing identity. The checked-in `rsrc_windows_amd64.syso` supplies the icon, manifest, and version information; the installed Go version must satisfy `go.mod`.

Official builds record the source commit in Go's VCS metadata and verify it matches the release workflow's commit with no modified sources. **Details** shows the version and commit; `go version -m Church-TV-Control.exe` also reads the embedded metadata. Builds from source archives without Git metadata identify the commit as unrecorded.

## Tests

```sh
go test ./...
go vet ./...
```

On Linux, also use `go test -race ./...`. Windows tests exercise the actual native table, selection events, embedded icon, and DPAPI credential storage. Protocol tests use simulated endpoints, including KLAP sessions, power commands, offline devices, persistence, and frozen legacy sync readers. Physical devices are not available to CI, so report the model and firmware when validating hardware behavior.

The **CI** workflow runs for pull requests and pushes to `main`, with read-only repository access and no Azure OIDC permission. It never signs or publishes contributor code.

## Windows resources

The icon is original artwork generated with Go's standard library. From the repository root:

```sh
go run ./tools/icon
go install github.com/tc-hib/go-winres@v0.3.3
go-winres make --arch amd64
```

Commit the regenerated PNGs and resource object when they change. Version changes belong in `winres/winres.json` and `installer/setup.iss`.

## Installer and download packaging

Install Inno Setup 6 on a Windows development machine. After building the EXE:

```powershell
Copy-Item docs/START-HERE.txt build/START-HERE.txt
& 'C:\Program Files (x86)\Inno Setup 6\ISCC.exe' installer/setup.iss
./scripts/package.ps1
```

The installer includes the app, quick-start text, MIT license, and third-party notices. Packaging copies the standalone EXE and notices to `dist` and generates checksums.

The manual build/release workflow validates both installation scopes, shortcut targets, the taskbar helper, and uninstallation. For signed builds, signatures are verified before publishing. See [Signing and releases](SIGNING.md).

## Project layout

| Path | Purpose |
| --- | --- |
| `main_windows.go` | Native window, controls, settings dialog, and commands |
| `roku.go` | Roku discovery, inspection, power, and device-list state |
| `kasa.go`, `kasa_session.go` | Kasa discovery, protocol adapter, authentication, and session reuse |
| `settings*.go`, `secrets_*.go` | JSON compatibility and local protected credentials |
| `lansync.go`, `sync_bridge.go` | Encrypted settings exchange and legacy compatibility |
| `*_test.go` | Protocol, state, compatibility, and Windows UI tests |
| `winres/`, `tools/icon/` | Application resources and generated icon |
| `installer/`, `scripts/` | Installer and release packaging |
| `docs/` | User and developer documentation, plus historical release notes |

When adding shared device settings, follow the [sync compatibility contract](SYNC-PROTOCOL.txt). Unknown-field retention alone does not make a new device kind or global setting compatible with an old client; design and test its legacy behavior explicitly.
