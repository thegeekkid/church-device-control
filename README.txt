CHURCH TV CONTROL — ROKU

QUICK START (Windows 10/11, 64-bit Intel/AMD)
1. Extract this ZIP to a folder. Double-click Church-TV-Control.exe.
   No installation, Python, .NET runtime, account, or cloud service required.
2. Connect the PC to the church network containing the TVs.
3. On each Roku TV, check Settings > System > Advanced system settings >
   Control by mobile apps > Network access. Enable network control (Default
   is usually sufficient; Permissive may be needed on some configurations).
   Menus vary by model and Roku OS version.
4. Enable Settings > System > Power > Fast TV Start for waking from standby.
   This increases standby energy use. Keep TVs plugged into AC power.
5. Click Discover TVs. Select a TV, enter its room name, and click Save name.
6. Use Selected ON/OFF or All TVs ON/OFF. Group commands ask for confirmation.
   Click Refresh status after a few seconds to read reported power state.

IF A TV IS MISSING
- Turn it on with its remote for initial setup and discovery.
- Find its IP in Settings > Network > About. Enter it and click Add by IP.
- Windows Firewall may prompt for network access. Allow the church's trusted
  private network. Discovery uses UDP multicast 239.255.255.250:1900.
  Control uses TCP port 8060. Ask church IT if Wi-Fi client isolation or VLANs
  separate the computer and TVs. Manual IP entry works across routed networks
  only when network rules permit it.
- Reserve TV IP addresses in your router/DHCP server. Rediscovery can update
  a saved TV's address using its serial number while retaining its room name.
- Network power-on requires the TV to remain reachable in standby. Fast TV
  Start and model/firmware support matter; an unplugged or deeply sleeping
  TV cannot be powered on by this app. There is no Wake-on-LAN fallback.
- This controls TVs running Roku OS, not arbitrary smart TVs or a display
  connected to an external Roku stick/player. The app rejects Roku players.
- HTTP 403 typically means the Roku's network control setting blocks access.

RESULTS AND SAVED SETTINGS
"Accepted" means Roku accepted the command, not proof the screen changed.
"Unreachable" can mean standby, disconnected Wi-Fi, or network restrictions.
Previously discovered TVs remain saved even when asleep.
Room names and TV details are stored for the current Windows user in:
%APPDATA%\ChurchRoku\tvs.json
The app makes direct local-network requests. It does not upload TV data.
Only one app instance should be used at a time to avoid saved-list conflicts.
The executable is unsigned; Windows may show a SmartScreen warning. Follow
church IT policy when running downloaded programs.

BUILD AND VALIDATION
Built from included Go source with no third-party code dependencies.
Protocol and persistence tests run against a simulated Roku endpoint.
Windows executable was cross-compiled on Linux. The native Windows UI and
physical TVs have not been tested here; test one TV before a church-wide use.

SOURCE BUILD (for a developer; NOT required to use the app)
Install Go 1.25 or later. In the source folder:
  go test -race ./...
  go vet ./...
  set GOOS=windows
  set GOARCH=amd64
  set CGO_ENABLED=0
  go build -buildvcs=false -trimpath -ldflags="-s -w -H=windowsgui" -o Church-TV-Control.exe .
The source can be adapted for a future Raspberry Pi interface; the current
native Windows UI does not run on the Pi.
