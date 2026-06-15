@echo off
setlocal EnableDelayedExpansion
title AuraBlock Windows Installer

:: 1. Request Admin Privileges
net session >nul 2>&1
if %errorLevel% == 0 (
    echo [+] Running with Administrator privileges.
) else (
    echo [!] Administrator privileges required. Requesting elevation...
    powershell -Command "Start-Process cmd -ArgumentList '/c %~dpnx0' -Verb RunAs"
    exit /b
)

echo ==================================================
echo          AuraBlock Installer - Windows
echo ==================================================

:: 2. Setup Directories
echo [*] Creating installation directories...
set "INSTALL_DIR=C:\Program Files\AuraBlock"
set "DATA_DIR=C:\ProgramData\AuraBlock"

if not exist "%INSTALL_DIR%" mkdir "%INSTALL_DIR%"
if not exist "%DATA_DIR%" mkdir "%DATA_DIR%"
if not exist "%INSTALL_DIR%\dist" mkdir "%INSTALL_DIR%\dist"

:: 3. Copy files
echo [*] Copying AuraBlock files...
if exist "aurablock.exe" (
    copy /Y "aurablock.exe" "%INSTALL_DIR%\aurablock.exe" >nul
) else if exist "backend\bin\aurablock.exe" (
    copy /Y "backend\bin\aurablock.exe" "%INSTALL_DIR%\aurablock.exe" >nul
) else (
    echo [!] Error: aurablock.exe not found! 
    echo Please make sure you have built the Windows executable or downloaded the Windows release.
    pause
    exit /b
)

if exist "backend\dist" (
    xcopy /E /I /Y "backend\dist\*" "%INSTALL_DIR%\dist" >nul
) else if exist "dist" (
    xcopy /E /I /Y "dist\*" "%INSTALL_DIR%\dist" >nul
)

if exist "extension_build" (
    xcopy /E /I /Y "extension_build\*" "%INSTALL_DIR%\dist\extensions" >nul
    set /p EXT_ID=<"extension_build\ext_id.txt"
) else (
    echo [!] Warning: extension_build folder not found. Browser extension will not be installed.
)

:: 4. Add Windows Defender Exclusions
echo [*] Adding Windows Defender exclusions to prevent blocking...
powershell -Command "Add-MpPreference -ExclusionPath '%INSTALL_DIR%'" >nul 2>&1
powershell -Command "Add-MpPreference -ExclusionProcess '%INSTALL_DIR%\aurablock.exe'" >nul 2>&1
echo [+] Windows Defender exclusions added.

:: 5. Create Background Scheduled Task (Auto-Startup on Boot)
echo [*] Setting up background service for Auto-Startup...
:: Stop it if it's already running
schtasks /query /tn "AuraBlock" >nul 2>&1
if %errorLevel% == 0 (
    schtasks /End /TN "AuraBlock" >nul 2>&1
    schtasks /Delete /TN "AuraBlock" /F >nul 2>&1
)
taskkill /F /IM aurablock.exe >nul 2>&1

:: Create a run script to set the correct working directory for the web server (./dist)
echo @echo off > "%INSTALL_DIR%\run.bat"
echo cd /d "%%~dp0" >> "%INSTALL_DIR%\run.bat"
echo aurablock.exe -dns-addr=0.0.0.0:53 -api-port=8082 -db-path="%DATA_DIR%\aurablock.db" >> "%INSTALL_DIR%\run.bat"

schtasks /Create /TN "AuraBlock" /TR "\"%INSTALL_DIR%\run.bat\"" /SC ONSTART /RU SYSTEM /F >nul
echo [+] Starting AuraBlock background service...
schtasks /Run /TN "AuraBlock" >nul

:: 6. Set System DNS
echo [*] Configuring System DNS to route through AuraBlock (127.0.0.1)...
powershell -Command "Get-NetAdapter | Where-Object { $_.Status -eq 'Up' -and $_.InterfaceAlias -notmatch 'vEthernet|Virtual' } | Set-DnsClientServerAddress -ServerAddresses '127.0.0.1'" >nul 2>&1
echo [+] DNS configured.

:: 7. Setup Browser Policies
if defined EXT_ID (
    echo [*] Configuring Browser Policies for Extension ID: !EXT_ID!
    
    :: Google Chrome
    REG ADD "HKLM\SOFTWARE\Policies\Google\Chrome\ExtensionInstallForcelist" /v 1 /t REG_SZ /d "!EXT_ID!;http://localhost:8082/extensions/update.xml" /f >nul
    REG ADD "HKLM\SOFTWARE\Policies\Google\Chrome\ExtensionInstallSources" /v 1 /t REG_SZ /d "http://localhost:8082/*" /f >nul
    REG ADD "HKLM\SOFTWARE\Policies\Google\Chrome" /v DnsOverHttpsMode /t REG_SZ /d "off" /f >nul
    REG ADD "HKLM\SOFTWARE\Policies\Google\Chrome" /v BuiltInDnsClientEnabled /t REG_DWORD /d 0 /f >nul

    :: Microsoft Edge
    REG ADD "HKLM\SOFTWARE\Policies\Microsoft\Edge\ExtensionInstallForcelist" /v 1 /t REG_SZ /d "!EXT_ID!;http://localhost:8082/extensions/update.xml" /f >nul
    REG ADD "HKLM\SOFTWARE\Policies\Microsoft\Edge\ExtensionInstallSources" /v 1 /t REG_SZ /d "http://localhost:8082/*" /f >nul
    REG ADD "HKLM\SOFTWARE\Policies\Microsoft\Edge" /v BuiltInDnsClientEnabled /t REG_DWORD /d 0 /f >nul

    :: Brave
    REG ADD "HKLM\SOFTWARE\Policies\BraveSoftware\Brave\ExtensionInstallForcelist" /v 1 /t REG_SZ /d "!EXT_ID!;http://localhost:8082/extensions/update.xml" /f >nul
    REG ADD "HKLM\SOFTWARE\Policies\BraveSoftware\Brave\ExtensionInstallSources" /v 1 /t REG_SZ /d "http://localhost:8082/*" /f >nul
    REG ADD "HKLM\SOFTWARE\Policies\BraveSoftware\Brave" /v DnsOverHttpsMode /t REG_SZ /d "off" /f >nul
    REG ADD "HKLM\SOFTWARE\Policies\BraveSoftware\Brave" /v BuiltInDnsClientEnabled /t REG_DWORD /d 0 /f >nul
    
    echo [+] Browser extension policies applied successfully.
)

echo.
echo ==================================================
echo             INSTALLATION COMPLETE!
echo ==================================================
echo AuraBlock is now running in the background.
echo Web Dashboard: http://localhost:8082
echo.
echo Press any key to exit...
pause >nul
