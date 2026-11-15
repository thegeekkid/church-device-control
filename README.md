# Church Device Control

A small Windows app for turning Roku TVs and Kasa smart plugs on and off across a local network. Name devices, organize them into rooms, and give multiple computers the same device list without a shared server.

Built for church volunteers who need a few clear controls before and after a service. It also works anywhere you have compatible devices on a trusted LAN.

## Download

**[Get the latest release](https://github.com/thegeekkid/church-device-control/releases/latest)** for Windows 10 or 11, 64-bit Intel/AMD:

- **Church-Device-Control-Setup.exe** installs for your Windows user without administrator access, or for all users with elevation. Shortcuts are optional.
- **Church-TV-Control.exe** runs directly, with no installation. The filename is retained for existing shortcuts.
- **SHA256.txt** contains checksums. License and third-party notices accompany future releases.

No Python, Go, or separate .NET runtime is needed to run the app. Official signed releases use Azure Artifact Signing; signing does not guarantee that Windows will skip every SmartScreen warning. GitHub provides **Source code (zip)** and **Source code (tar.gz)** for each release. Future releases omit the additional app-and-source ZIP.

## What it does

- Discovers Roku TVs and compatible single-outlet Kasa plugs; devices can also be added by IP.
- Controls one device, one room, or all enabled devices.
- Shows power status, IP and MAC addresses, and full connection errors.
- Disables occasional devices so they are skipped during regular refreshes and power commands. Discovery still checks them.
- Refreshes on launch, about one second after a command, and every minute while open.
- Syncs device settings directly between open installations using a shared, encrypted LAN sync key. Kasa credentials remain local to each Windows user.

## Get started

1. Connect the PC and devices to the same trusted network. Guest Wi-Fi isolation can prevent discovery and control.
2. On Roku TVs, enable network control and **Fast TV Start** so they remain reachable in standby.
3. For Kasa, save your account credentials in **Settings** and enable third-party compatibility in the official app if required by the firmware.
4. Click **Discover devices**, then assign device names and rooms with **Save device**.

See the **[documentation](docs/README.md)** for setup, controls, syncing, troubleshooting, and developer instructions.

## Compatibility

The app controls TVs running Roku OS, rather than external Roku players or arbitrary TV brands. Kasa support covers legacy XOR and authenticated KLAP v1/v2 single-outlet plugs; firmware compatibility varies. Multi-outlet strips are not supported. A plug's status describes its relay, rather than the appliance connected to it.

The native interface currently runs on Windows only. Network discovery and sync generally require the same subnet; isolated or routed networks may need IT configuration.

## Source, contributions, and license

Original project code and the generated TV icon are available under the **[MIT license](LICENSE)**. The current Kasa dependency has **no published license**; permission or a replacement is needed before the complete project can be presented as fully open source. See [third-party notices](THIRD-PARTY-NOTICES.txt).

[Contributing](CONTRIBUTING.md) · [Build from source](docs/DEVELOPMENT.md) · [Security](SECURITY.md) · [Release history](CHANGELOG.md)

This is an independent project, not affiliated with Roku or TP-Link. Their names identify compatible products.
