#!/bin/bash
# EAMSA 512 - macOS Deployment Script
# Supports: macOS 11+ (Big Sur and later)
# Usage: bash deploy-macos.sh [production|staging|development]

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Configuration
ENVIRONMENT="${1:-production}"
INSTALL_DIR="/usr/local/opt/eamsa512"
CONFIG_DIR="/usr/local/etc/eamsa512"
LOG_DIR="/var/log/eamsa512"
DATA_DIR="/var/lib/eamsa512"
PLIST_DIR="$HOME/Library/LaunchAgents"
PLIST_FILE="$PLIST_DIR/com.eamsa512.plist"

echo -e "${GREEN}╔════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║     EAMSA 512 macOS Deployment Script v1.0.0          ║${NC}"
echo -e "${GREEN}║     Environment: ${ENVIRONMENT}${NC}"
echo -e "${GREEN}╚════════════════════════════════════════════════════════╝${NC}"
echo ""

# Check if running on macOS
if [[ "$OSTYPE" != "darwin"* ]]; then
    echo -e "${RED}[ERROR] This script must be run on macOS${NC}"
    exit 1
fi

# Check Homebrew
if ! command -v brew &> /dev/null; then
    echo -e "${YELLOW}[INFO] Installing Homebrew...${NC}"
    /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
fi

# Install dependencies
echo -e "${YELLOW}[STEP 1/8] Installing dependencies...${NC}"
brew update
brew tap-new local/eamsa512 2>/dev/null || true
brew install go git openssl sqlite@3

# Create directories
echo -e "${YELLOW}[STEP 2/8] Creating directories...${NC}"
sudo mkdir -p "$INSTALL_DIR"
sudo mkdir -p "$CONFIG_DIR/certs"
sudo mkdir -p "$LOG_DIR"
sudo mkdir -p "$DATA_DIR"
mkdir -p "$PLIST_DIR"

# Set permissions
echo -e "${YELLOW}[STEP 3/8] Setting permissions...${NC}"
sudo chown -R "$(whoami):staff" "$INSTALL_DIR"
sudo chown -R "$(whoami):staff" "$CONFIG_DIR"
sudo chown -R "$(whoami):staff" "$LOG_DIR"
sudo chown -R "$(whoami):staff" "$DATA_DIR"

# Generate TLS certificates
echo -e "${YELLOW}[STEP 4/8] Generating TLS certificates...${NC}"
if [ ! -f "$CONFIG_DIR/certs/tls.crt" ]; then
    openssl req -x509 -newkey rsa:4096 \
        -keyout "$CONFIG_DIR/certs/tls.key" \
        -out "$CONFIG_DIR/certs/tls.crt" \
        -days 365 -nodes \
        -subj "/CN=localhost/O=EAMSA512/C=US"
    chmod 600 "$CONFIG_DIR/certs/tls.key"
fi

# Build application
echo -e "${YELLOW}[STEP 5/8] Building EAMSA 512...${NC}"
if [ ! -d "$INSTALL_DIR/src" ]; then
    git clone https://github.com/yourusername/eamsa512.git "$INSTALL_DIR/src" 2>/dev/null || \
    mkdir -p "$INSTALL_DIR/src"
fi

if [ -f "$INSTALL_DIR/src/go.mod" ]; then
    cd "$INSTALL_DIR/src"
    go mod download
    go build -o "$INSTALL_DIR/eamsa512" .
fi

# Create configuration
echo -e "${YELLOW}[STEP 6/8] Creating configuration...${NC}"
cat > "$CONFIG_DIR/eamsa512.yaml" <<EOF
server:
  host: 0.0.0.0
  port: 8080
  environment: $ENVIRONMENT
  tls_enabled: true
  tls_cert_path: $CONFIG_DIR/certs/tls.crt
  tls_key_path: $CONFIG_DIR/certs/tls.key

database:
  type: sqlite3
  path: $DATA_DIR/eamsa512.db

encryption:
  block_size: 64
  key_size: 32
  nonce_size: 16
  rounds: 16

logging:
  level: info
  format: json
  file_path: $LOG_DIR/eamsa512.log
EOF

# Create LaunchAgent
echo -e "${YELLOW}[STEP 7/8] Creating LaunchAgent...${NC}"
cat > "$PLIST_FILE" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.eamsa512</string>
    <key>ProgramArguments</key>
    <array>
        <string>$INSTALL_DIR/eamsa512</string>
    </array>
    <key>WorkingDirectory</key>
    <string>$INSTALL_DIR</string>
    <key>EnvironmentVariables</key>
    <dict>
        <key>CONFIG_DIR</key>
        <string>$CONFIG_DIR</string>
        <key>LOG_DIR</key>
        <string>$LOG_DIR</string>
        <key>DATA_DIR</key>
        <string>$DATA_DIR</string>
    </dict>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>$LOG_DIR/output.log</string>
    <key>StandardErrorPath</key>
    <string>$LOG_DIR/error.log</string>
    <key>ProcessType</key>
    <string>Adaptive</string>
</dict>
</plist>
EOF

chmod 644 "$PLIST_FILE"

# Load LaunchAgent if production
echo -e "${YELLOW}[STEP 8/8] Finalizing installation...${NC}"
if [ "$ENVIRONMENT" = "production" ]; then
    launchctl load "$PLIST_FILE"
    sleep 2
    if launchctl list | grep -q com.eamsa512; then
        echo -e "${GREEN}[OK] EAMSA 512 is running${NC}"
    fi
fi

echo ""
echo -e "${GREEN}╔════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║         EAMSA 512 Deployment Complete!                ║${NC}"
echo -e "${GREEN}╚════════════════════════════════════════════════════════╝${NC}"
echo ""
echo "Installation Summary:"
echo "  Install Directory:    $INSTALL_DIR"
echo "  Config Directory:     $CONFIG_DIR"
echo "  Log Directory:        $LOG_DIR"
echo "  Data Directory:       $DATA_DIR"
echo "  Environment:          $ENVIRONMENT"
echo ""
echo "Next Steps:"
echo "  1. Review config:     cat $CONFIG_DIR/eamsa512.yaml"
echo "  2. View logs:         tail -f $LOG_DIR/eamsa512.log"
echo "  3. Test API:          curl -k https://localhost:8080/api/v1/health"
echo ""
echo "Useful Commands:"
if [ "$ENVIRONMENT" = "production" ]; then
    echo "  Start service:        launchctl start com.eamsa512"
    echo "  Stop service:         launchctl stop com.eamsa512"
    echo "  Unload service:       launchctl unload $PLIST_FILE"
fi
echo "  Check status:         launchctl list | grep eamsa512"
echo ""
