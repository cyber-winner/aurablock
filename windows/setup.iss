[Setup]
AppName=AuraBlock
AppVersion=1.0.0
AppPublisher=cyber-winner
AppPublisherURL=https://github.com/cyber-winner/aurablock
AppSupportURL=https://github.com/cyber-winner/aurablock/issues
AppUpdatesURL=https://github.com/cyber-winner/aurablock/releases
DefaultDirName={pf}\AuraBlock
DefaultGroupName=AuraBlock
LicenseFile=..\LICENSE
InfoBeforeFile=..\TERMS_AND_CONDITIONS.md
InfoAfterFile=..\PRIVACY_POLICY.md
OutputDir=..\release
OutputBaseFilename=AuraBlock-Setup-v1.0.0
Compression=lzma
SolidCompression=yes
PrivilegesRequired=admin
ArchitecturesInstallIn64BitMode=x64
UninstallDisplayIcon={app}\aurablock.exe
SetupIconFile=compiler:SetupClassicIcon.ico

[Files]
Source: "..\aurablock.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\dist\*"; DestDir: "{app}\dist"; Flags: ignoreversion recursesubdirs createallsubdirs
Source: "..\extension_build\*"; DestDir: "{app}\dist\extensions"; Flags: ignoreversion recursesubdirs createallsubdirs
Source: "post_install.bat"; DestDir: "{app}"; Flags: ignoreversion
Source: "uninstall.bat"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\PRIVACY_POLICY.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\TERMS_AND_CONDITIONS.md"; DestDir: "{app}"; Flags: ignoreversion

[Run]
Filename: "{app}\post_install.bat"; Flags: runhidden waituntilterminated
Filename: "http://localhost:8082"; Description: "Open AuraBlock Dashboard"; Flags: postinstall shellexec runasoriginaluser

[UninstallRun]
Filename: "{app}\uninstall.bat"; Flags: runhidden waituntilterminated

[Icons]
Name: "{group}\AuraBlock Web Dashboard"; Filename: "http://localhost:8082"
Name: "{group}\Terms and Conditions"; Filename: "{app}\TERMS_AND_CONDITIONS.md"
Name: "{group}\Privacy Policy"; Filename: "{app}\PRIVACY_POLICY.md"
Name: "{group}\Uninstall AuraBlock"; Filename: "{uninstallexe}"
