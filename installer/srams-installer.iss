; SRAMS v1.0.0 Production Installer - SQLite Edition
; Simplified installation with embedded encrypted SQLite database
; Inno Setup 6.x Required

#define MyAppName "SRAMS"
#define MyAppVersion "1.0.0"
#define MyAppPublisher "SRAMS Development Team"
#define MyAppURL "https://localhost:3000"
#define MyAppExeName "srams-server.exe"

[Setup]
AppId={{8FAE7D12-3B4C-4E5F-9A1B-2C3D4E5F6A7B}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
AppPublisherURL={#MyAppURL}
DefaultDirName={autopf}\{#MyAppName}
DefaultGroupName={#MyAppName}
AllowNoIcons=yes
OutputDir=output
OutputBaseFilename=SRAMS-Setup-{#MyAppVersion}
Compression=lzma2/ultra64
SolidCompression=yes
WizardStyle=modern
PrivilegesRequired=admin
ArchitecturesInstallIn64BitMode=x64
UninstallDisplayName={#MyAppName} Secure Audit System
DisableDirPage=no
DisableProgramGroupPage=yes
MinVersion=10.0
SetupLogging=yes
CloseApplications=yes

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Messages]
WelcomeLabel2=This will install [name/ver] on your computer.%n%nSRAMS is a Secure Role-Based Audit Management System with an embedded encrypted SQLite database.%n%nNO external database installation required!%n%nThis installer will:%n- Install the backend service%n- Deploy the frontend application%n- Create your Super Admin account

[Files]
; Backend
Source: "output\srams-server.exe"; DestDir: "{app}\backend"; Flags: ignoreversion

; Frontend
Source: "output\frontend\*"; DestDir: "{app}\frontend"; Flags: ignoreversion recursesubdirs createallsubdirs

; Desktop Launcher - Super Admin Control Panel
Source: "..\desktop-launcher\dist\SRAMS Admin-win32-x64\*"; DestDir: "{app}\admin-launcher"; Flags: ignoreversion recursesubdirs createallsubdirs

; Tools
Source: "tools\nssm.exe"; DestDir: "{app}\tools"; Flags: ignoreversion

; Scripts
Source: "scripts\*.ps1"; DestDir: "{app}\scripts"; Flags: ignoreversion

[Dirs]
Name: "{app}\data"; Permissions: admins-full
Name: "{app}\documents"; Permissions: users-modify
Name: "{app}\logs"; Permissions: users-modify  
Name: "{app}\certs"; Permissions: admins-full
Name: "{app}\config"; Permissions: admins-full

[Icons]
; Super Admin Desktop Shortcut - launches the desktop control panel
Name: "{commondesktop}\SRAMS Control Panel"; Filename: "{app}\admin-launcher\SRAMS Admin.exe"; Comment: "SRAMS Super Admin Control Panel"; Tasks: desktopicon

; Start Menu shortcuts
Name: "{group}\SRAMS Control Panel"; Filename: "{app}\admin-launcher\SRAMS Admin.exe"; Comment: "Super Admin Control Panel"
Name: "{group}\SRAMS Configuration"; Filename: "{app}\config"
Name: "{group}\SRAMS Logs"; Filename: "{app}\logs"
Name: "{group}\{cm:UninstallProgram,{#MyAppName}}"; Filename: "{uninstallexe}"

[Tasks]
Name: "desktopicon"; Description: "Create Super Admin desktop shortcut"; GroupDescription: "Additional options:"

[Run]
; Generate configuration for SQLite
Filename: "powershell.exe"; Parameters: "-ExecutionPolicy Bypass -File ""{app}\scripts\generate-sqlite-config.ps1"" -InstallPath ""{app}"" -EncryptionKey ""{code:GetEncryptionKey}"" -JwtSecret ""{code:GetJwtSecret}"" -AdminEmail ""{code:GetAdminEmail}"" -AdminPassword ""{code:GetAdminPassword}"" -AdminName ""{code:GetAdminFullName}"""; Flags: runhidden waituntilterminated; StatusMsg: "Generating configuration..."

; Install backend service
Filename: "powershell.exe"; Parameters: "-ExecutionPolicy Bypass -File ""{app}\scripts\install-sqlite-service.ps1"" -InstallPath ""{app}"""; Flags: runhidden waituntilterminated; StatusMsg: "Installing Windows service..."

; Start backend service
Filename: "powershell.exe"; Parameters: "-ExecutionPolicy Bypass -Command ""Start-Service srams-backend -ErrorAction SilentlyContinue; Start-Sleep -Seconds 5"""; Flags: runhidden waituntilterminated; StatusMsg: "Starting backend service..."

; Create Super Admin
Filename: "powershell.exe"; Parameters: "-ExecutionPolicy Bypass -File ""{app}\scripts\create-superadmin.ps1"" -InstallPath ""{app}"""; Flags: runhidden waituntilterminated; StatusMsg: "Creating Super Admin account..."

; Launch Desktop Control Panel
Filename: "{app}\admin-launcher\SRAMS Admin.exe"; Flags: nowait postinstall shellexec runascurrentuser; Description: "Launch SRAMS Control Panel"

[Registry]
; Make desktop app always request admin rights
Root: HKLM; Subkey: "SOFTWARE\Microsoft\Windows NT\CurrentVersion\AppCompatFlags\Layers"; ValueType: string; ValueName: "{app}\admin-launcher\SRAMS Admin.exe"; ValueData: "~ RUNASADMIN"; Flags: uninsdeletevalue

[UninstallRun]
Filename: "powershell.exe"; Parameters: "-ExecutionPolicy Bypass -File ""{app}\scripts\uninstall-sqlite.ps1"" -InstallPath ""{app}"""; Flags: runhidden waituntilterminated; RunOnceId: "UninstallSRAMS"

[Code]
var
  // Wizard pages
  SecurityPage: TInputQueryWizardPage;
  AdminPage: TInputQueryWizardPage;
  DatabasePage: TInputQueryWizardPage;
  SummaryPage: TOutputMsgMemoWizardPage;
  
  // Configuration values
  EncryptionKey: String;
  JwtSecret: String;
  AdminEmail: String;
  AdminPassword: String;
  AdminFullName: String;
  DbMaxSizeMB: Integer;

function GenerateRandomSecret(Len: Integer): String;
var
  I: Integer;
  Chars: String;
begin
  Chars := 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789';
  Result := '';
  for I := 1 to Len do
    Result := Result + Chars[Random(Length(Chars)) + 1];
end;

function GetEncryptionKey(Param: String): String;
begin
  Result := EncryptionKey;
end;

function GetJwtSecret(Param: String): String;
begin
  Result := JwtSecret;
end;

function GetAdminEmail(Param: String): String;
begin
  Result := AdminEmail;
end;

function GetAdminPassword(Param: String): String;
begin
  Result := AdminPassword;
end;

function GetAdminFullName(Param: String): String;
begin
  Result := AdminFullName;
end;

procedure InitializeWizard();
begin
  // Database Encryption Page
  DatabasePage := CreateInputQueryPage(wpSelectDir,
    'Database Encryption', 'Secure your database with AES-256 encryption',
    'Enter a strong encryption key for the SQLite database. This key encrypts all data at rest and is required to access the database.');
  DatabasePage.Add('Database Encryption Key (min 16 characters):', True);
  DatabasePage.Add('Maximum Database Size (MB):', False);
  DatabasePage.Values[0] := GenerateRandomSecret(32);
  DatabasePage.Values[1] := '5120';  // 5GB default

  // Security Page
  SecurityPage := CreateInputQueryPage(DatabasePage.ID,
    'Security Configuration', 'JWT Secret for API authentication',
    'The JWT secret is used to sign authentication tokens. A strong random secret is pre-generated.');
  SecurityPage.Add('JWT Secret (32+ characters):', True);
  SecurityPage.Values[0] := GenerateRandomSecret(48);

  // Admin Page
  AdminPage := CreateInputQueryPage(SecurityPage.ID,
    'Super Admin Account', 'Create the initial Super Admin user',
    'The Super Admin has full access to all system features and can manage other users.');
  AdminPage.Add('Full Name:', False);
  AdminPage.Add('Email:', False);
  AdminPage.Add('Password (min 8 characters):', True);
  AdminPage.Add('Confirm Password:', True);
  AdminPage.Values[0] := 'System Administrator';
  AdminPage.Values[1] := 'admin@srams.local';

  // Summary Page
  SummaryPage := CreateOutputMsgMemoPage(AdminPage.ID,
    'Installation Summary', 'Review your settings before installation',
    'Click Install to begin the installation with these settings:',
    '');
end;

function NextButtonClick(CurPageID: Integer): Boolean;
var
  SummaryText: String;
begin
  Result := True;
  
  // Validate Database Page
  if CurPageID = DatabasePage.ID then
  begin
    if Length(DatabasePage.Values[0]) < 16 then
    begin
      MsgBox('Encryption key must be at least 16 characters.', mbError, MB_OK);
      Result := False;
      Exit;
    end;
    EncryptionKey := DatabasePage.Values[0];
    DbMaxSizeMB := StrToIntDef(DatabasePage.Values[1], 5120);
  end;
  
  // Validate Security Page
  if CurPageID = SecurityPage.ID then
  begin
    if Length(SecurityPage.Values[0]) < 32 then
    begin
      MsgBox('JWT Secret must be at least 32 characters.', mbError, MB_OK);
      Result := False;
      Exit;
    end;
    JwtSecret := SecurityPage.Values[0];
  end;
  
  // Validate Admin Page
  if CurPageID = AdminPage.ID then
  begin
    if Length(AdminPage.Values[1]) < 5 then
    begin
      MsgBox('Please enter a valid email address.', mbError, MB_OK);
      Result := False;
      Exit;
    end;
    if Length(AdminPage.Values[2]) < 8 then
    begin
      MsgBox('Password must be at least 8 characters.', mbError, MB_OK);
      Result := False;
      Exit;
    end;
    if AdminPage.Values[2] <> AdminPage.Values[3] then
    begin
      MsgBox('Passwords do not match.', mbError, MB_OK);
      Result := False;
      Exit;
    end;
    AdminFullName := AdminPage.Values[0];
    AdminEmail := AdminPage.Values[1];
    AdminPassword := AdminPage.Values[2];
    
    // Build summary
    SummaryText := 'Installation Directory: ' + WizardDirValue + #13#10 + #13#10;
    SummaryText := SummaryText + '=== DATABASE CONFIGURATION ===' + #13#10;
    SummaryText := SummaryText + 'Database: Embedded SQLite (AES-256 Encrypted)' + #13#10;
    SummaryText := SummaryText + 'Max Size: ' + IntToStr(DbMaxSizeMB) + ' MB' + #13#10;
    SummaryText := SummaryText + 'Location: ' + WizardDirValue + '\data\srams.db' + #13#10 + #13#10;
    SummaryText := SummaryText + '=== SUPER ADMIN ACCOUNT ===' + #13#10;
    SummaryText := SummaryText + 'Name: ' + AdminFullName + #13#10;
    SummaryText := SummaryText + 'Email: ' + AdminEmail + #13#10 + #13#10;
    SummaryText := SummaryText + '=== SECURITY ===' + #13#10;
    SummaryText := SummaryText + 'Database Encryption: AES-256-GCM' + #13#10;
    SummaryText := SummaryText + 'Key Derivation: Argon2id' + #13#10;
    SummaryText := SummaryText + 'JWT Token: RS256 compatible' + #13#10 + #13#10;
    SummaryText := SummaryText + 'IMPORTANT: Save your encryption key securely!' + #13#10;
    SummaryText := SummaryText + 'You will need it if the configuration is lost.';
    SummaryPage.RichEditViewer.Text := SummaryText;
  end;
end;

function UpdateReadyMemo(Space, NewLine, MemoUserInfoInfo, MemoDirInfo, MemoTypeInfo, MemoComponentsInfo, MemoGroupInfo, MemoTasksInfo: String): String;
begin
  Result := MemoDirInfo + NewLine + NewLine +
            'Database: Embedded SQLite with AES-256 encryption' + NewLine +
            'Super Admin: ' + AdminEmail + NewLine + NewLine +
            'No external database required - fully self-contained installation!';
end;
