# Troubleshooting

## A device is missing

- Confirm the PC and device are on the same trusted network. Guest Wi-Fi isolation and VLAN rules can block discovery and control.
- Turn a Roku TV on with its remote for initial setup. Enable network control and Fast TV Start.
- Enter the device's IPv4 address and use **Add Roku** or **Add Kasa**. Manual entry can work across a routed network only when its rules permit control traffic.
- Check Windows Firewall. Roku discovery uses UDP multicast `239.255.255.250:1900` and control uses TCP 8060. Kasa discovery uses UDP 9999/20002, with legacy control on TCP 9999 or KLAP on local HTTP port 80. LAN settings sync uses UDP 45439.
- Reserve device IP addresses in DHCP. A device that moved to another address may need rediscovery.

## Roku commands fail or status is unknown

HTTP 403 usually means the TV's network control setting blocks access. Check **Control by mobile apps > Network access** in the TV's settings.

An unreachable TV may be off, disconnected, or sleeping too deeply for network control. Enable Fast TV Start, keep AC power connected, and test again. The app has no Wake-on-LAN fallback. A reachable TV reporting `Ready` is treated as off/standby.

An accepted command is not proof that the screen changed. The app checks again about a second later; use **Refresh status** if the device is still transitioning.

## Kasa authentication fails

1. Confirm the official Kasa app can control the plug.
2. Check **Third-Party Compatibility** in the official app; its location and availability vary by firmware.
3. In **Settings**, enter the owning account's email and password, then **Save local settings** before retrying. Set credentials separately on every PC.
4. Open **Details** for the full error. Include the model and firmware when reporting a problem.

If startup reports that saved credentials could not be loaded, open **Settings** and re-enter the account credentials and LAN sync key. Saved devices and rooms are retained. This can happen if the protected file is damaged or copied from another Windows user or PC.

The app authenticates locally with the device; it does not use a cloud OAuth sign-in or an interactive MFA prompt. An authentication failure by itself does not establish that MFA is responsible. Only legacy XOR and KLAP v1/v2 are supported. The app tries a local setup-account fallback automatically when an ordinary account authentication mismatch permits it.

Authenticated sessions are reused, and a temporary status failure gets one retry. Changing credentials causes fresh authentication. A failed power command is not automatically retried, so confirm its result before sending it again.

## Offline plugs make checks take longer

Use **Disable selected** for occasional devices. Disabled devices skip regular refreshes and all power commands; discovery still checks them. Up to four enabled devices are checked concurrently. Each Kasa status check has a total 12-second budget including its retry; several batches can take longer overall.

If the error text is cut off, click **Details** or double-click the row. **Ctrl+C** copies the full dialog.

## Settings do not sync

- Keep the apps open, use the same sync key, and click **Sync + refresh** on each PC.
- Allow UDP 45439 on the trusted Private network. Broadcasts generally do not cross subnets, VLAN boundaries, or isolated Wi-Fi.
- Run one instance per Windows session and keep Windows clocks accurate.
- Keep a 0.4.1-or-newer copy open when mixing 0.4.0 and 0.3.x. Older 0.3.x versions cannot honor Disabled even when common settings sync.
- If a device was deleted on another PC, add it by IP to restore it. Discovery does not undo deletion markers.

Credentials are intentionally not synced. If a plug works on one PC but authentication fails on another, save credentials in that installation's Settings.

## Report a problem

[Open an issue](https://github.com/thegeekkid/church-device-control/issues) with the app version, Windows version, device model/firmware, steps to reproduce, and copied Details text. Remove credentials, sync keys, and identifying network information before posting. See [Security](../SECURITY.md) for a vulnerability report.
