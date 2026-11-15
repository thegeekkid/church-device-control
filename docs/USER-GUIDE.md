# User guide

Church Device Control is a Windows control panel for Roku TVs and compatible Kasa smart plugs on your local network. Each row is a device; a room groups rows for a shared on/off command.

## Install or run

Download from the [latest release](https://github.com/thegeekkid/church-device-control/releases/latest). Use Windows 10 or 11 on a 64-bit Intel/AMD PC.

- **Installer:** run `Church-Device-Control-Setup.exe`. Choose **Install for me only** for a user-profile installation without administrator access, or **all users** for Program Files with elevation. Desktop and Start menu shortcuts are optional.
- **Portable app:** save `Church-TV-Control.exe` to a folder and double-click it. No separate runtime is needed.

For taskbar pinning, the installer's optional helper opens a folder containing a shortcut. Right-click the shortcut and choose **Pin to taskbar**; **Show more options** may be needed. Windows controls pinning, so the installer does not silently pin it.

Close running copies before upgrading. Replace the portable executable or run the newer installer. Saved devices, rooms, and local credentials are retained. Uninstall also preserves settings. An all-users installation shares the executable, but each Windows user has their own settings and credentials.

## Prepare the network and devices

Put the PC and devices on the same trusted LAN. Guest Wi-Fi often blocks devices from communicating with one another. If discovery fails, check Wi-Fi client isolation, VLANs, and Windows Firewall with your network administrator.

### Roku TVs

1. Use the TV's remote to connect it to the network initially.
2. Enable network control under **Settings > System > Advanced system settings > Control by mobile apps > Network access**. Default is usually sufficient; some configurations need Permissive. Menus vary by model and Roku OS version.
3. Enable **Settings > System > Power > Fast TV Start** for reliable waking from standby. This increases standby energy use. Keep the TV connected to AC power.

Only TVs running Roku OS are supported. A Roku streaming stick or player cannot supply reliable power control and screen status for an attached TV. An unplugged or deeply sleeping TV cannot be woken by this app.

### Kasa plugs

1. Set up the plug in the official Kasa app and confirm it can turn on and off there.
2. Enable **Third-Party Compatibility** in the official app if your firmware requires it.
3. In this app, open **Settings**, enter the owning TP-Link/Kasa account email and password, and click **Save local settings**.

The app supports legacy XOR and authenticated KLAP v1/v2 single-outlet plugs. Model and firmware support varies; multi-outlet strips and other authentication protocols are not supported. The plug's state reports its relay, rather than the power state of the appliance attached to it.

Credentials are protected with Windows DPAPI for the current user. Enter them separately on every installation that will control Kasa devices; they are never sent through settings sync. Some firmware uses a local setup-account fallback automatically. Keep your normal account email and password in Settings.

## Discover and name devices

Click **Discover devices**. Previously saved devices stay listed when offline. Discovery also checks disabled devices without re-enabling them.

If a device is missing, enter its IPv4 address and click **Add Roku** or **Add Kasa**. Roku TVs show their address in **Settings > Network > About**. Reserving addresses in your router's DHCP settings helps keep devices reachable. Rediscovery can update a known device's address when its identity matches.

Select a row, enter **Device name** and **Room**, then click **Save device**. The device name identifies the individual TV or plug, such as “Cafe TV.” A room, such as “Lobby,” can contain several TVs and plugs. Room names match without case sensitivity. Clear or change the Room field to move a device out of its previous room.

**Remove selected** deletes a saved device and syncs the deletion. To restore a deleted device, add it by IP; discovery alone does not undo a deletion.

## Read status and send commands

| Indicator | Meaning |
| --- | --- |
| Green / On | The device reported on. |
| Red / Off | The device reported off or standby. Roku's `Ready` state is treated as off. |
| Yellow | Unknown, pending, unreachable, or a failed operation. Unreachable does not establish that a device is off. |
| Gray / Disabled | Regular refreshes and power commands skip this device. |

The table shows device name, room, type, IP address, MAC address, reported power, and the last result. MAC addresses remain cached while a device is unreachable; a blank field means the app has not received one. Resize columns by dragging their dividers and use the horizontal scrollbar when needed.

Use **Selected ON/OFF**, **Room ON/OFF**, or **All devices ON/OFF**. For a room command, select one of its members or enter the room name. Group commands ask for confirmation; check room membership when it includes plugs connected to other appliances.

An accepted command means the device accepted the request. The app waits about one second and checks status to confirm what the device reports. Status is also checked on launch and once a minute while open. **Refresh status** starts a check immediately. Device requests run in the background with up to four status checks in parallel; a Kasa check, including its retry, has a total 12-second limit.

For full error text, select a device and click **Details**, or double-click its row. Press **Ctrl+C** in the dialog to copy its contents.

## Disable occasional devices

Select a row and click **Disable selected**. Its status changes to gray **Disabled**, and its previous power reading is hidden. It will receive no regular refreshes or power commands, including selected, room, and all-device commands. Discovery still checks its address and metadata.

To use it again, click **Enable selected**. Enabling starts a check if the app is idle; otherwise refresh once the current operation finishes. A request already in progress when you disable a device may finish.

The disabled setting is saved and synced. Older 0.3.x installations do not understand it and may still refresh or control that device. Upgrade an installation before relying on Disabled there.

## Sync several installations

1. On the first PC, open **Settings**, click **Generate key**, then **Save local settings**.
2. Copy the key into **LAN sync key** in Settings on each other PC and save.
3. Allow the app through Windows Firewall on the trusted Private network. Your administrator may need to allow UDP port **45439** in both directions.
4. Keep the apps open. **Sync + refresh** exchanges settings immediately. Open copies also exchange settings and check their devices every minute.
5. Test a name or room change in each direction before relying on sync.

The shared key protects packets using authenticated AES-GCM encryption. Treat it as a secret: anyone with the key and LAN access can change shared settings. Sync carries device names, rooms, addresses, types, identities, disabled flags, and deletions. It carries neither Kasa credentials nor observed power states, and it never sends power commands on another PC's behalf. Each installation controls devices directly.

Different device edits merge; for conflicting edits to common settings on one device, the newest timestamp wins. Disabled has a separate revision. Keep PC clocks accurate. An offline PC retains its list and catches up when it reconnects. Run one copy per Windows session to avoid listener and saved-file conflicts.

LAN broadcasts generally stay on one subnet. There is no manual cross-subnet peer list. Routed VLANs and isolated Wi-Fi require network configuration.

### Upgrading at different times

Version 0.4.1 exchanges common settings with 0.3.x and bridges 0.4.0's original sync-format mismatch. Keep a 0.4.1-or-newer copy open while updating the other PCs. A 0.4.0/0.3.x pair without an updated peer still has its original one-way limitation.

New versions preserve unknown optional device settings. Old versions can exchange settings they understand, but need updating to implement new behavior. The compatibility contract is described in the [sync protocol](SYNC-PROTOCOL.txt).

## Where settings live

| File | Contents |
| --- | --- |
| `%APPDATA%\ChurchRoku\tvs.json` | Device settings, revisions, and deletion markers |
| `%APPDATA%\ChurchRoku\local-secrets.bin` | DPAPI-protected credentials and sync key for this Windows user |
| `%APPDATA%\ChurchRoku\tvs-v0.2-backup.json` | Backup made when migrating an old device list |

The legacy folder name is retained to preserve upgrades. Do not copy local-secrets.bin to another PC as a migration method; configure that PC's credentials and sync key through Settings. Do not run an older executable against the same settings folder concurrently.

The app makes direct local-network requests and has no telemetry upload. Roku and TP-Link devices and their official apps may have their own cloud services. See [troubleshooting](TROUBLESHOOTING.md) if setup does not work.
