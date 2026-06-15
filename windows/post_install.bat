@echo off
setlocal EnableDelayedExpansion
set "INSTALL_DIR=%~dp0"
:: Remove trailing slash
if "%INSTALL_DIR:~-1%"=="\" set "INSTALL_DIR=%INSTALL_DIR:~0,-1%"
set "DATA_DIR=C:\ProgramData\AuraBlock"

if not exist "%DATA_DIR%" mkdir "%DATA_DIR%"

:: Defender Exclusions
powershell -Command "Add-MpPreference -ExclusionPath '%INSTALL_DIR%'" >nul 2>&1
powershell -Command "Add-MpPreference -ExclusionProcess '%INSTALL_DIR%\aurablock.exe'" >nul 2>&1

:: Create run script for correct working directory
echo @echo off > "%INSTALL_DIR%\run.bat"
echo cd /d "%%~dp0" >> "%INSTALL_DIR%\run.bat"
echo aurablock.exe -dns-addr=0.0.0.0:53 -api-port=8082 -db-path="%DATA_DIR%\aurablock.db" >> "%INSTALL_DIR%\run.bat"

:: Scheduled Task Setup
schtasks /query /tn "AuraBlock" >nul 2>&1
if %errorLevel% == 0 (
    schtasks /End /TN "AuraBlock" >nul 2>&1
    schtasks /Delete /TN "AuraBlock" /F >nul 2>&1
)
taskkill /F /IM aurablock.exe >nul 2>&1

schtasks /Create /TN "AuraBlock" /TR "\"%INSTALL_DIR%\run.bat\"" /SC ONSTART /RU SYSTEM /F >nul
schtasks /Run /TN "AuraBlock" >nul

:: Redirect System DNS to AuraBlock
powershell -Command "Get-NetAdapter | Where-Object { $_.Status -eq 'Up' -and $_.InterfaceAlias -notmatch 'vEthernet|Virtual' } | Set-DnsClientServerAddress -ServerAddresses '127.0.0.1'" >nul 2>&1

:: Inject Browser Policies
if exist "%INSTALL_DIR%\dist\extensions\ext_id.txt" (
    set /p EXT_ID=<"%INSTALL_DIR%\dist\extensions\ext_id.txt"
    
    :: Chrome
    REG ADD "HKLM\SOFTWARE\Policies\Google\Chrome\ExtensionInstallForcelist" /v 1 /t REG_SZ /d "!EXT_ID!;http://localhost:8082/extensions/update.xml" /f >nul
    REG ADD "HKLM\SOFTWARE\Policies\Google\Chrome\ExtensionInstallSources" /v 1 /t REG_SZ /d "http://localhost:8082/*" /f >nul
    REG ADD "HKLM\SOFTWARE\Policies\Google\Chrome" /v DnsOverHttpsMode /t REG_SZ /d "off" /f >nul
    REG ADD "HKLM\SOFTWARE\Policies\Google\Chrome" /v BuiltInDnsClientEnabled /t REG_DWORD /d 0 /f >nul

    :: Edge
    REG ADD "HKLM\SOFTWARE\Policies\Microsoft\Edge\ExtensionInstallForcelist" /v 1 /t REG_SZ /d "!EXT_ID!;http://localhost:8082/extensions/update.xml" /f >nul
    REG ADD "HKLM\SOFTWARE\Policies\Microsoft\Edge\ExtensionInstallSources" /v 1 /t REG_SZ /d "http://localhost:8082/*" /f >nul
    REG ADD "HKLM\SOFTWARE\Policies\Microsoft\Edge" /v BuiltInDnsClientEnabled /t REG_DWORD /d 0 /f >nul
)
exit /b 0
