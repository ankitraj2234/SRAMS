; SRAMS v2.0.0 Production Installer - PostgreSQL Edition
; Enterprise-grade installation with bundled PostgreSQL database
; Zero Trust Security Architecture
; Inno Setup 6.x Required

#define MyAppName "SRAMS Enterprise"
#define MyAppVersion "2.0.0"
#define MyAppPublisher "SRAMS Development Team"
#define MyAppURL "https://localhost:3000"
#define MyAppExeName "srams-server.exe"

[Setup]
AppId={{9FAE8E23-4C5D-5F6G-0B2C-3D4E5F6G7H8I}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
AppPublisherURL={#MyAppURL}
DefaultDirName={autopf}\{#MyAppName}
DefaultGroupName={#MyAppName}
AllowNoIcons=yes
OutputDir=output
OutputBaseFilename=SRAMS-Enterprise-Setup-{#MyAppVersion}
Compression=lzma2/ultra64
SolidCompression=yes
WizardStyle=modern
PrivilegesRequired=admin
ArchitecturesInstallIn64BitMode=x64
UninstallDisplayName={#MyAppName} Enterprise Audit System
DisableDirPage=no
DisableProgramGroupPage=yes
MinVersion=10.0
SetupLogging=yes
CloseApplications=yes
; Larger disk space required for PostgreSQL
ExtraDiskSpaceRequired=536870912

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Messages]
WelcomeLabel2=This will install [name/ver] on your computer.%n%nSRAMS Enterprise Edition includes:%n- Bundled PostgreSQL 16 database%n- Zero Trust security with Row-Level Security%n- Append-only audit logs%n- Enterprise user management%n%nNo external database installation required!

[Files]
; Backend
Source: "output\srams-server.exe"; DestDir: "{app}\backend"; Flags: ignoreversion

; Frontend
Source: "output\frontend\*"; DestDir: "{app}\frontend"; Flags: ignoreversion recursesubdirs createallsubdirs

; Desktop Launcher
Source: "..\desktop-launcher\dist\SRAMS Admin-win32-x64\*"; DestDir: "{app}\admin-launcher"; Flags: ignoreversion recursesubdirs createallsubdirs

; PostgreSQL Portable (downloaded separately or bundled)
; You need to download PostgreSQL Windows binaries and extract to pgsql folder
Source: "pgsql\*"; DestDir: "{app}\pgsql"; Flags: ignoreversion recursesubdirs createallsubdirs

; Database migrations
Source: "..\backend\internal\db\postgres\migrations\*"; DestDir: "{app}\migrations"; Flags: ignoreversion

; Tools
Source: "tools\nssm.exe"; DestDir: "{app}\tools"; Flags: ignoreversion

; Scripts
Source: "scripts\*.ps1"; DestDir: "{app}\scripts"; Flags: ignoreversion
Source: "scripts\*.bat"; DestDir: "{app}\scripts"; Flags: ignoreversion

[Dirs]
Name: "{app}\data"; Permissions: admins-full
Name: "{app}\data\postgres"; Permissions: admins-full
Name: "{app}\documents"; Permissions: users-modify
Name: "{app}\logs"; Permissions: users-modify  
Name: "{app}\certs"; Permissions: admins-full
Name: "{app}\config"; Permissions: admins-full

[Icons]
Name: "{commondesktop}\SRAMS Control Panel"; Filename: "{app}\admin-launcher\SRAMS Admin.exe"; Comment: "SRAMS Super Admin Control Panel"; Tasks: desktopicon
Name: "{group}\SRAMS Control Panel"; Filename: "{app}\admin-launcher\SRAMS Admin.exe"; Comment: "Super Admin Control Panel"
Name: "{group}\SRAMS Configuration"; Filename: "{app}\config"
Name: "{group}\SRAMS Logs"; Filename: "{app}\logs"
Name: "{group}\{cm:UninstallProgram,{#MyAppName}}"; Filename: "{uninstallexe}"

[Tasks]
Name: "desktopicon"; Description: "Create Super Admin desktop shortcut"; GroupDescription: "Additional options:"

[Run]
; Step 1: Generate configuration files with user-provided credentials
Filename: "powershell.exe"; Parameters: "-ExecutionPolicy Bypass -File ""{app}\scripts\generate-postgres-config.ps1"" -InstallPath ""{app}"" -DbPassword ""{code:GetDbPassword}"" -JwtSecret ""{code:GetJwtSecret}"" -AdminEmail ""{code:GetAdminEmail}"" -AdminPassword ""{code:GetAdminPassword}"" -AdminName ""{code:GetAdminFullName}"""; Flags: waituntilterminated; StatusMsg: "Generating configuration..."

; Step 2: Run complete PostgreSQL setup (init, config, start, create db, migrations)
Filename: "powershell.exe"; Parameters: "-ExecutionPolicy Bypass -File ""{app}\scripts\setup-postgres-complete.ps1"" -InstallPath ""{app}"""; Flags: waituntilterminated; StatusMsg: "Setting up PostgreSQL database (this may take a minute)..."

; Step 3: Install backend as Windows service
Filename: "powershell.exe"; Parameters: "-ExecutionPolicy Bypass -File ""{app}\scripts\install-postgres-service.ps1"" -InstallPath ""{app}"""; Flags: waituntilterminated; StatusMsg: "Installing Windows service..."

; Step 4: Start backend service (this will auto-seed super admin on first start)
Filename: "powershell.exe"; Parameters: "-ExecutionPolicy Bypass -Command ""Start-Service srams-backend -ErrorAction SilentlyContinue; Start-Sleep -Seconds 8"""; Flags: runhidden waituntilterminated; StatusMsg: "Starting backend service and creating Super Admin..."

; Step 5: Launch Desktop Control Panel
Filename: "{app}\admin-launcher\SRAMS Admin.exe"; Flags: nowait postinstall shellexec runascurrentuser; Description: "Launch SRAMS Control Panel"

[Registry]
Root: HKLM; Subkey: "SOFTWARE\Microsoft\Windows NT\CurrentVersion\AppCompatFlags\Layers"; ValueType: string; ValueName: "{app}\admin-launcher\SRAMS Admin.exe"; ValueData: "~ RUNASADMIN"; Flags: uninsdeletevalue

[UninstallRun]
; Stop services and cleanup
Filename: "powershell.exe"; Parameters: "-ExecutionPolicy Bypass -File ""{app}\scripts\uninstall-postgres.ps1"" -InstallPath ""{app}"""; Flags: runhidden waituntilterminated; RunOnceId: "UninstallSRAMS"

[Code]
var
  SecurityPage: TInputQueryWizardPage;
  AdminPage: TInputQueryWizardPage;
  DatabasePage: TInputQueryWizardPage;
  SummaryPage: TOutputMsgMemoWizardPage;
  
  DbPassword: String;
  JwtSecret: String;
  AdminEmail: String;
  AdminPassword: String;
  AdminFullName: String;

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

function GetDbPassword(Param: String): String;
begin
  Result := DbPassword;
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
  // Database Configuration Page
  DatabasePage := CreateInputQueryPage(wpSelectDir,
    'PostgreSQL Database', 'Configure the bundled PostgreSQL database',
    'SRAMS includes a secure PostgreSQL 16 database with Row-Level Security. Enter a database password below.');
  DatabasePage.Add('Database Password (min 12 characters):', True);
  DatabasePage.Values[0] := GenerateRandomSecret(16);

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
    if Length(DatabasePage.Values[0]) < 12 then
    begin
      MsgBox('Database password must be at least 12 characters for security.', mbError, MB_OK);
      Result := False;
      Exit;
    end;
    DbPassword := DatabasePage.Values[0];
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
    SummaryText := SummaryText + 'Database: PostgreSQL 16 (Bundled)' + #13#10;
    SummaryText := SummaryText + 'Security: Row-Level Security (RLS)' + #13#10;
    SummaryText := SummaryText + 'Audit: Append-only with immutability triggers' + #13#10;
    SummaryText := SummaryText + 'Location: ' + WizardDirValue + '\data\postgres' + #13#10 + #13#10;
    SummaryText := SummaryText + '=== SUPER ADMIN ACCOUNT ===' + #13#10;
    SummaryText := SummaryText + 'Name: ' + AdminFullName + #13#10;
    SummaryText := SummaryText + 'Email: ' + AdminEmail + #13#10 + #13#10;
    SummaryText := SummaryText + '=== ZERO TRUST SECURITY ===' + #13#10;
    SummaryText := SummaryText + '- Row-Level Security enforced at DB level' + #13#10;
    SummaryText := SummaryText + '- Append-only audit logs (cannot be deleted)' + #13#10;
    SummaryText := SummaryText + '- Session-bound database context' + #13#10;
    SummaryText := SummaryText + '- Role-based database users' + #13#10 + #13#10;
    SummaryText := SummaryText + 'IMPORTANT: Save your credentials securely!';
    SummaryPage.RichEditViewer.Text := SummaryText;
  end;
end;

function UpdateReadyMemo(Space, NewLine, MemoUserInfoInfo, MemoDirInfo, MemoTypeInfo, MemoComponentsInfo, MemoGroupInfo, MemoTasksInfo: String): String;
begin
  Result := MemoDirInfo + NewLine + NewLine +
            'Database: PostgreSQL 16 with Zero Trust RLS' + NewLine +
            'Super Admin: ' + AdminEmail + NewLine + NewLine +
            'Enterprise-grade security included!';
end;
