; antiwave Windows installer (Inno Setup).
;
; Build (on Windows):
;   ISCC.exe antiwave.iss [/DMyAppVersion=0.1.0]
;
; The compiled antiwave.exe must be in this directory (next to this script).

#ifndef MyAppVersion
  #define MyAppVersion "0.1.0"
#endif
#define MyAppName "antiwave"
#define MyAppPublisher "fcode09"
#define MyAppExeName "antiwave.exe"

[Setup]
AppId={{9E3B2C1D-5A4F-4E8B-9C7A-2D1E3F4A5B6C}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
AppPublisherURL=https://github.com/fcode09/antiwave
DefaultDirName={autopf}\{#MyAppName}
DisableProgramGroupPage=yes
OutputBaseFilename=antiwave_{#MyAppVersion}_windows_amd64_setup
Compression=lzma
SolidCompression=yes
WizardStyle=modern
ArchitecturesInstallIn64BitMode=x64compatible
PrivilegesRequiredOverridesAllowed=dialog
ChangesEnvironment=yes
UninstallDisplayIcon={app}\{#MyAppExeName}
UninstallDisplayName={#MyAppName}

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Files]
Source: "antiwave.exe"; DestDir: "{app}"; Flags: ignoreversion

[Code]
function EnvRootKey(): Integer;
begin
  if IsAdminInstallMode then
    Result := HKLM
  else
    Result := HKCU;
end;

function EnvSubKey(): String;
begin
  if IsAdminInstallMode then
    Result := 'SYSTEM\CurrentControlSet\Control\Session Manager\Environment'
  else
    Result := 'Environment';
end;

procedure EnvAddPath(Path: string);
var
  Paths: string;
begin
  if not RegQueryStringValue(EnvRootKey(), EnvSubKey(), 'Path', Paths) then
    Paths := '';
  if Pos(';' + Uppercase(Path) + ';', ';' + Uppercase(Paths) + ';') = 0 then begin
    if Paths = '' then
      Paths := Path
    else
      Paths := Paths + ';' + Path;
    RegWriteStringValue(EnvRootKey(), EnvSubKey(), 'Path', Paths);
  end;
end;

procedure EnvRemovePath(Path: string);
var
  Paths: string;
  P: Integer;
begin
  if not RegQueryStringValue(EnvRootKey(), EnvSubKey(), 'Path', Paths) then
    exit;
  P := Pos(Uppercase(Path), Uppercase(Paths));
  if P > 0 then begin
    Delete(Paths, P, Length(Path));
    StringChangeEx(Paths, ';;', ';', True);
    if (Length(Paths) > 0) and (Paths[1] = ';') then
      Delete(Paths, 1, 1);
    if (Length(Paths) > 0) and (Paths[Length(Paths)] = ';') then
      Delete(Paths, Length(Paths), 1);
    if Paths = '' then
      RegDeleteValue(EnvRootKey(), EnvSubKey(), 'Path')
    else
      RegWriteStringValue(EnvRootKey(), EnvSubKey(), 'Path', Paths);
  end;
end;

procedure CurStepChanged(CurStep: TSetupStep);
begin
  if CurStep = ssPostInstall then
    EnvAddPath(ExpandConstant('{app}'));
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
begin
  if CurUninstallStep = usPostUninstall then
    EnvRemovePath(ExpandConstant('{app}'));
end;
