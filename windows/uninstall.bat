@echo off
set "INSTALL_DIR=%~dp0"
if "%INSTALL_DIR:~-1%"=="\" set "INSTALL_DIR=%INSTALL_DIR:~0,-1%"

:: Remove DNS Configuration
powershell -Command "Get-NetAdapter | Where-Object { $_.Status -eq 'Up' -and $_.InterfaceAlias -notmatch 'vEthernet|Virtual' } | Set-DnsClientServerAddress -ResetServerAddresses" >nul 2>&1

:: Remove Scheduled Task
schtasks /End /TN "AuraBlock" >nul 2>&1
schtasks /Delete /TN "AuraBlock" /F >nul 2>&1
taskkill /F /IM aurablock.exe >nul 2>&1

:: Remove Browser Policies
REG DELETE "HKLM\SOFTWARE\Policies\Google\Chrome\ExtensionInstallForcelist" /v 1 /f >nul 2>&1
REG DELETE "HKLM\SOFTWARE\Policies\Google\Chrome\ExtensionInstallSources" /v 1 /f >nul 2>&1
REG DELETE "HKLM\SOFTWARE\Policies\Microsoft\Edge\ExtensionInstallForcelist" /v 1 /f >nul 2>&1
REG DELETE "HKLM\SOFTWARE\Policies\Microsoft\Edge\ExtensionInstallSources" /v 1 /f >nul 2>&1

:: Remove Defender Exclusions
powershell -Command "Remove-MpPreference -ExclusionPath '%INSTALL_DIR%'" >nul 2>&1
powershell -Command "Remove-MpPreference -ExclusionProcess '%INSTALL_DIR%\aurablock.exe'" >nul 2>&1

exit /b 0
