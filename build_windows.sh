#!/bin/bash
set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

echo -e "${CYAN}==================================================${NC}"
echo -e "${CYAN}       Building AuraBlock Windows Assets          ${NC}"
echo -e "${CYAN}==================================================${NC}"

# 1. Cross-compile Go backend for Windows
echo -e "${YELLOW}[*] Cross-compiling Go binary for Windows (amd64)...${NC}"
cd backend
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o ../aurablock.exe
cd ..
echo -e "${GREEN}[+] aurablock.exe successfully built!${NC}"

# 2. Build the extension
echo -e "${YELLOW}[*] Packaging Chrome/Edge Extension...${NC}"
python3 scripts/build_extension.py
echo -e "${GREEN}[+] Extension packaged!${NC}"

echo ""
echo -e "${GREEN}All Windows files are ready!${NC}"
echo "To generate the final single-file installer (AuraBlock-Setup-v1.0.0.exe):"
echo "1. If you are on Windows: Download 'Inno Setup' and double-click windows/setup.iss to compile it."
echo "2. If you are on Linux: You can install 'wine' and run Inno Setup through wine."
echo -e "3. ${CYAN}Recommended:${NC} Use GitHub Actions to automatically compile the installer for you."
