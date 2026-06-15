#!/bin/bash

# AuraBlock Installer Script
# Must be run with sudo/root privileges

set -e

# Colors for terminal output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0;34m' # No Color
RESET='\033[0m'

echo -e "${CYAN}==================================================${RESET}"
echo -e "${CYAN}          AuraBlock Installer - Linux Laptop      ${RESET}"
echo -e "${CYAN}==================================================${RESET}"

# Check for root privileges
if [ "$EUID" -ne 0 ]; then
  echo -e "${RED}[!] Error: Please run this script with sudo or as root.${RESET}"
  exit 1
fi

# 1. Build the Go binary
echo -e "${YELLOW}[*] Building Go Core Engine...${RESET}"
cd backend
go build -o bin/aurablock
cd ..
echo -e "${GREEN}[+] AuraBlock binary compiled successfully.${RESET}"

# 2. Create aurablock system user/group if they don't exist
echo -e "${YELLOW}[*] Configuring system users and permissions...${RESET}"
if ! id -u aurablock &>/dev/null; then
  useradd -r -s /usr/bin/nologin aurablock
  echo -e "${GREEN}[+] Created system user 'aurablock'.${RESET}"
else
  echo -e "${GREEN}[+] System user 'aurablock' already exists.${RESET}"
fi

# 3. Create install directories
echo -e "${YELLOW}[*] Creating installation directory at /etc/aurablock...${RESET}"
mkdir -p /etc/aurablock
mkdir -p /etc/aurablock/dist

# 4. Stop service if running to prevent Text File Busy errors
if systemctl is-active --quiet aurablock; then
  echo -e "${YELLOW}[*] Stopping active AuraBlock service...${RESET}"
  systemctl stop aurablock
fi

# 5. Copy binary and frontend assets
echo -e "${YELLOW}[*] Copying binary to /usr/local/bin...${RESET}"
install -m 755 backend/bin/aurablock /usr/local/bin/aurablock

# Grant net bind capability to allow binding to port 53 without root privileges
echo -e "${YELLOW}[*] Setting network binding capabilities (cap_net_bind_service) on binary...${RESET}"
setcap 'cap_net_bind_service=+ep' /usr/local/bin/aurablock

echo -e "${YELLOW}[*] Copying web dashboard files...${RESET}"
cp -R backend/dist/* /etc/aurablock/dist/

# Rebuild the custom AuraBlock Shield Chrome extension
echo -e "${YELLOW}[*] Packaging custom AuraBlock Shield Chrome extension...${RESET}"
python3 scripts/build_extension.py

echo -e "${YELLOW}[*] Deploying custom extension files...${RESET}"
mkdir -p /etc/aurablock/dist/extensions
cp -R extension_build/* /etc/aurablock/dist/extensions/

# Set ownership of /etc/aurablock
echo -e "${YELLOW}[*] Setting permissions on configuration files...${RESET}"
chown -R aurablock:aurablock /etc/aurablock
# Make the dist directory world-readable so that browsers can load the extension and dashboard
chmod -R 755 /etc/aurablock/dist

# 5. Create Systemd Service File
echo -e "${YELLOW}[*] Generating systemd service file...${RESET}"
cat <<EOF > /etc/systemd/system/aurablock.service
[Unit]
Description=AuraBlock Device-Wide DNS AdBlocker
After=network.target

[Service]
Type=simple
User=aurablock
Group=aurablock
WorkingDirectory=/etc/aurablock
ExecStart=/usr/local/bin/aurablock -dns-addr=0.0.0.0:53 -api-port=8082 -db-path=/etc/aurablock/aurablock.db
Restart=always
RestartSec=5
AmbientCapabilities=CAP_NET_BIND_SERVICE
CapabilityBoundingSet=CAP_NET_BIND_SERVICE

[Install]
WantedBy=multi-user.target
EOF

# 6. Enable and Start Systemd Service
echo -e "${YELLOW}[*] Enabling and starting AuraBlock daemon...${RESET}"
systemctl daemon-reload
systemctl enable aurablock
systemctl restart aurablock

echo -e "${GREEN}[+] AuraBlock service started successfully!${RESET}"

# 7. Apply Browser Managed Policies to force-install AuraBlock Shield Extension
echo -e "${YELLOW}[*] Configuring Browser Policies to force-install AuraBlock Shield Extension...${RESET}"

# Get the extension ID from the build output
EXT_ID=$(cat /home/cyber/CODES/aurablock/extension_build/ext_id.txt)
echo -e "${GREEN}[+] Detected AuraBlock Shield Extension ID: ${EXT_ID}${RESET}"

# Google Chrome
echo -e "${YELLOW}[*] Setting up Chrome policy...${RESET}"
mkdir -p /etc/opt/chrome/policies/managed
cat <<EOF > /etc/opt/chrome/policies/managed/aurablock_policy.json
{
  "ExtensionInstallForcelist": [
    "${EXT_ID};http://localhost:8082/extensions/update.xml"
  ],
  "ExtensionInstallSources": [
    "http://localhost:8082/*"
  ],
  "OverrideSecurityRestrictionsOnInsecureOrigin": [
    "http://localhost:8082"
  ],
  "DnsOverHttpsMode": "off",
  "BuiltInDnsClientEnabled": false
}
EOF

# Chromium
echo -e "${YELLOW}[*] Setting up Chromium policy...${RESET}"
mkdir -p /etc/chromium/policies/managed
cat <<EOF > /etc/chromium/policies/managed/aurablock_policy.json
{
  "ExtensionInstallForcelist": [
    "${EXT_ID};http://localhost:8082/extensions/update.xml"
  ],
  "ExtensionInstallSources": [
    "http://localhost:8082/*"
  ],
  "OverrideSecurityRestrictionsOnInsecureOrigin": [
    "http://localhost:8082"
  ],
  "DnsOverHttpsMode": "off",
  "BuiltInDnsClientEnabled": false
}
EOF

# Brave
echo -e "${YELLOW}[*] Setting up Brave policy...${RESET}"
mkdir -p /etc/brave/policies/managed
cat <<EOF > /etc/brave/policies/managed/aurablock_policy.json
{
  "ExtensionInstallForcelist": [
    "${EXT_ID};http://localhost:8082/extensions/update.xml"
  ],
  "ExtensionInstallSources": [
    "http://localhost:8082/*"
  ],
  "OverrideSecurityRestrictionsOnInsecureOrigin": [
    "http://localhost:8082"
  ],
  "DnsOverHttpsMode": "off",
  "BuiltInDnsClientEnabled": false
}
EOF

# Firefox
echo -e "${YELLOW}[*] Setting up Firefox policy...${RESET}"
mkdir -p /etc/firefox/policies
cat <<EOF > /etc/firefox/policies/policies.json
{
  "policies": {
    "Extensions": {
      "Install": [
        "https://example.com/aurablock-shield.xpi"
      ]
    }
  }
}
EOF

# 8. Configure System-Wide External Extensions
echo -e "${YELLOW}[*] Configuring System-Wide External Extensions...${RESET}"

# Create the JSON preference file pointing to our local crx path
cat <<EOF > /tmp/nbakpbkkbgiklpcmdhceahdkbkijjmfm.json
{
  "external_crx": "/etc/aurablock/dist/extensions/aurablock-shield.crx",
  "external_version": "1.0"
}
EOF

# Google Chrome external extension
mkdir -p /opt/google/chrome/extensions
cp /tmp/nbakpbkkbgiklpcmdhceahdkbkijjmfm.json /opt/google/chrome/extensions/nbakpbkkbgiklpcmdhceahdkbkijjmfm.json
chmod 644 /opt/google/chrome/extensions/nbakpbkkbgiklpcmdhceahdkbkijjmfm.json

# Brave external extension
mkdir -p /opt/brave-bin/extensions
cp /tmp/nbakpbkkbgiklpcmdhceahdkbkijjmfm.json /opt/brave-bin/extensions/nbakpbkkbgiklpcmdhceahdkbkijjmfm.json
chmod 644 /opt/brave-bin/extensions/nbakpbkkbgiklpcmdhceahdkbkijjmfm.json

# Chromium external extension
mkdir -p /usr/share/chromium/extensions
cp /tmp/nbakpbkkbgiklpcmdhceahdkbkijjmfm.json /usr/share/chromium/extensions/nbakpbkkbgiklpcmdhceahdkbkijjmfm.json
chmod 644 /usr/share/chromium/extensions/nbakpbkkbgiklpcmdhceahdkbkijjmfm.json

rm -f /tmp/nbakpbkkbgiklpcmdhceahdkbkijjmfm.json

echo -e "${GREEN}[+] Browser policies and external extensions applied successfully.${RESET}"
echo ""
echo -e "${CYAN}==================================================${RESET}"
echo -e "${GREEN}            INSTALLATION COMPLETE!                ${RESET}"
echo -e "${CYAN}==================================================${RESET}"
echo -e "AuraBlock Core is now running in the background."
echo -e "Web Dashboard: ${GREEN}http://localhost:8082${RESET}"
echo ""
echo -e "${YELLOW}HOW TO CONFIGURE YOUR SYSTEM DNS TO BLOCK ADS:${RESET}"
echo -e "Currently, AuraBlock is listening on ${CYAN}127.0.0.1:53${RESET}."
echo -e "To block ads device-wide, you must route your system DNS through it."
echo ""
echo -e "${CYAN}Option A: Direct /etc/resolv.conf Configuration (Quickest)${RESET}"
echo -e "Edit ${YELLOW}/etc/resolv.conf${RESET} and set the nameserver to localhost:"
echo -e "  ${GREEN}nameserver 127.0.0.1${RESET}"
echo -e "Note: If NetworkManager or Tailscale is running, they might overwrite it."
echo ""
echo -e "${CYAN}Option B: NetworkManager Configuration (Persistent)${RESET}"
echo -e "1. Open your network connection configuration (GUI or nmcli)."
echo -e "2. Set IPv4 DNS to ${GREEN}127.0.0.1${RESET}."
echo -e "3. Turn OFF 'Automatic DNS' or set 'Ignore automatically resolved DNS'."
echo ""
echo -e "${CYAN}Option C: Tailscale Configuration${RESET}"
echo -e "If Tailscale manages your DNS, open your Tailscale Admin Console -> DNS settings."
echo -e "Add a Custom DNS Server pointing to this device's local address (or localhost)."
echo ""
echo -e "${CYAN}Option D: systemd-resolved Configuration${RESET}"
echo -e "If you wish to run systemd-resolved, edit ${YELLOW}/etc/systemd/resolved.conf${RESET}:"
echo -e "  [Resolve]"
echo -e "  DNS=127.0.0.1"
echo -e "Then run: ${GREEN}sudo systemctl restart systemd-resolved${RESET}"
echo -e "${CYAN}==================================================${RESET}"
