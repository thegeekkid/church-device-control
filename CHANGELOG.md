# Release history

Download binaries and GitHub-generated source archives from [Releases](https://github.com/thegeekkid/church-device-control/releases). Historical release notes are retained as written; current setup instructions are in the [user guide](docs/USER-GUIDE.md).

## Unreleased

- License original project code and artwork under MIT; include third-party notices.
- Replace the old README with an introduction and a documentation index. Add current user, troubleshooting, development, signing, contribution, and security guides.
- Remove the extra app-and-source ZIP; include license notices with downloads.
- Add read-only contributor CI and restrict manual release builds to this repository's `main`.

## v0.4.1

Restore mixed-version settings sync, retain unknown optional device fields, and independently version Disabled changes. Older copies need updates to implement new behavior. [Full notes](docs/release-v0.4.1.txt)

## v0.4.0

Disable occasional devices, skip them during regular refresh and power commands, and bound concurrent device checks with shorter timeouts. [Full notes](docs/release-v0.4.0.txt)

## v0.3.4

Reuse authenticated Kasa sessions, retry transient status reads, and show full errors in Details. [Full notes](docs/release-v0.3.4.txt)

## v0.3.3

Fix the installer's Explorer path for the taskbar shortcut helper. [Full notes](docs/release-v0.3.3.txt)

## v0.3.2

Show and retain device MAC addresses. [Full notes](docs/release-v0.3.2.txt)

## v0.3.1

Add the Kasa local setup-account authentication fallback. [Full notes](docs/release-v0.3.1.txt)

## v0.3.0

Add Kasa plugs, rooms, direct encrypted LAN settings sync, minute-by-minute refreshes, and a dual-scope installer. [Full notes](docs/release-v0.3.0.txt)

## v0.2.3

Recognize the Roku `Ready` standby state as off. [Full notes](docs/release-v0.2.3.txt)

## v0.2.2

Refresh saved TVs on launch and after power commands. [Full notes](docs/release-v0.2.2.txt)

## v0.2.1

Publish signed builds and document Azure federation. [Full notes](docs/release-v0.2.1.txt)

## v0.2.0

Add a native status table, colored indicators, application icon, and optional Azure signing. [Full notes](docs/release-v0.2.0.txt)

## v0.1.0

Initial portable Windows Roku TV power controller. [Release](https://github.com/thegeekkid/church-device-control/releases/tag/v0.1.0)
