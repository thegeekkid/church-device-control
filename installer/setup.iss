#define AppVersion "0.4.1"
[Setup]
AppId={{507DE0A3-7082-4830-A477-DC1A5325DC31}
AppName=Church Device Control
AppVersion={#AppVersion}
DefaultDirName={autopf}\Church Device Control
DefaultGroupName=Church Device Control
PrivilegesRequired=lowest
PrivilegesRequiredOverridesAllowed=dialog commandline
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
OutputDir=..\dist
OutputBaseFilename=Church-Device-Control-Setup
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
UninstallDisplayIcon={app}\Church-TV-Control.exe
CloseApplications=yes
RestartApplications=no
SetupMutex=ChurchDeviceControlSetup
[Tasks]
Name: desktopicon; Description: "Create a desktop shortcut"; Flags: unchecked
Name: startmenuicon; Description: "Create a Start menu shortcut"
Name: taskbarhelp; Description: "Show the app shortcut after setup so I can pin it to the taskbar"; Flags: unchecked
[Files]
Source: "..\build\Church-TV-Control.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\build\START-HERE.txt"; DestDir: "{app}"; DestName: "START-HERE.txt"; Flags: ignoreversion
Source: "..\LICENSE"; DestDir: "{app}"; DestName: "LICENSE.txt"; Flags: ignoreversion
Source: "..\THIRD-PARTY-NOTICES.txt"; DestDir: "{app}"; Flags: ignoreversion
[Icons]
Name: "{autodesktop}\Church Device Control"; Filename: "{app}\Church-TV-Control.exe"; Tasks: desktopicon
Name: "{group}\Church Device Control"; Filename: "{app}\Church-TV-Control.exe"; Tasks: startmenuicon
Name: "{app}\Church Device Control"; Filename: "{app}\Church-TV-Control.exe"; Tasks: taskbarhelp
[Run]
Filename: "{app}\Church-TV-Control.exe"; Description: "Open Church Device Control"; Flags: nowait postinstall skipifsilent
Filename: "{win}\explorer.exe"; Parameters: "/select,""{app}\Church Device Control.lnk"""; Tasks: taskbarhelp; Flags: postinstall skipifsilent nowait runasoriginaluser; Description: "Show shortcut (right-click it, then choose Pin to taskbar; Show more options may be needed)"
