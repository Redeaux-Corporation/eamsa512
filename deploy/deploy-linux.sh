#!/bin/bash
# EAMSA 512 - Linux Deployment Script
# Supports: Ubuntu 20.04+, CentOS 8+, Debian 11+
# Usage: sudo bash deploy-linux.sh [production|staging|development]

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
ENVIRONMENT="${1:-production}"
INSTALL_DIR="/opt/eamsa512"
CONFIG_DIR="/etc/eamsa512"
LOG_DIR="/var/log/eamsa512"
DATA_DIR="/var/lib/eamsa512"
USER="eamsa512"
GROUP="eamsa512"
VERSION="1.0.0"

echo -e "${GREEN}╔════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║     EAMSA 512 Linux Deployment Script v1.0.0          ║${NC}"
echo -e "${GREEN}║     Environment: ${ENVIRONMENT}${NC}"
echo -e "${GREEN}╚════════════════════════════════════════════════════════╝${NC}"
echo ""

# Check if running as root
if [[ $EUID -ne 0 ]]; then
   echo -e "${RED}[ERROR] This script must be run as root${NC}"
   exit 1
fi

# Detect Linux distribution
if [ -f /etc/os-release ]; then
    . /etc/os-release
    DISTRO=$ID
else
    echo -e "${RED}[ERROR] Cannot detect Linux distribution${NC}"
    exit 1
fi

echo -e "${YELLOW}[INFO] Detected distribution: $DISTRO${NC}"

# Update system
echo -e "${YELLOW}[STEP 1/10] Updating system packages...${NC}"
case $DISTRO in
    ubuntu|debian)
        apt-get update
        apt-get install -y curl wget git golang-go build-essential sqlite3
        ;;
    centos|fedora|rhel)
        yum update -y
        yum install -y curl wget git golang gcc sqlite-devel
        ;;
    *)
        echo -e "${RED}[ERROR] Unsupported distribution: $DISTRO${NC}"
        exit 1
        ;;
esac

# Create service user
echo -e "${YELLOW}[STEP 2/10] Creating service user...${NC}"
if ! id "$USER" &>/dev/null; then
    useradd --system --home-dir /var/lib/eamsa512 --shell /bin/false $USER
    echo -e "${GREEN}[OK] User $USER created${NC}"
else
    echo -e "${GREEN}[OK] User $USER already exists${NC}"
fi

# Create directories
echo -e "${YELLOW}[STEP 3/10] Creating directories...${NC}"
mkdir -p "$INSTALL_DIR"
mkdir -p "$CONFIG_DIR"
mkdir -p "$LOG_DIR"
mkdir -p "$DATA_DIR"
mkdir -p "$CONFIG_DIR/certs"

# Generate TLS certificates
echo -e "${YELLOW}[STEP 4/10] Generating TLS certificates...${NC}"
if [ ! -f "$CONFIG_DIR/certs/tls.crt" ]; then
    openssl req -x509 -newkey rsa:4096 \
        -keyout "$CONFIG_DIR/certs/tls.key" \
        -out "$CONFIG_DIR/certs/tls.crt" \
        -days 365 -nodes \
        -subj "/CN=eamsa512.local/O=EAMSA512/C=US"
    echo -e "${GREEN}[OK] TLS certificates generated${NC}"
else
    echo -e "${GREEN}[OK] TLS certificates already exist${NC}"
fi

# Download/Build application
echo -e "${YELLOW}[STEP 5/10] Building EAMSA 512...${NC}"
if [ ! -d "$INSTALL_DIR/src" ]; then
    cd "$INSTALL_DIR"
    git clone https://github.com/yourusername/eamsa512.git src || \
    mkdir -p src && echo "WARNING: Could not clone repository"
fi

cd "$INSTALL_DIR/src"
if [ -f "go.mod" ]; then
    go mod download
    go build -o "$INSTALL_DIR/eamsa512" .
    echo -e "${GREEN}[OK] EAMSA 512 built successfully${NC}"
else
    echo -e "${YELLOW}[WARNING] Could not build - go.mod not found${NC}"
fi

# Set permissions
echo -e "${YELLOW}[STEP 6/10] Setting permissions...${NC}"
chown -R $USER:$GROUP "$INSTALL_DIR"
chown -R $USER:$GROUP "$CONFIG_DIR"
chown -R $USER:$GROUP "$LOG_DIR"
chown -R $USER:$GROUP "$DATA_DIR"
chmod 750 "$INSTALL_DIR"
chmod 750 "$CONFIG_DIR"
chmod 750 "$LOG_DIR"
chmod 750 "$DATA_DIR"
chmod 600 "$CONFIG_DIR/certs/tls.key"

# Create configuration file
echo -e "${YELLOW}[STEP 7/10] Creating configuration file...${NC}"
cat > "$CONFIG_DIR/eamsa512.yaml" <<EOF
# EAMSA 512 Configuration
server:
  host: 0.0.0.0
  port: 8080
  environment: $ENVIRONMENT
  tls_enabled: true
  tls_cert_path: $CONFIG_DIR/certs/tls.crt
  tls_key_path: $CONFIG_DIR/certs/tls.key
  read_timeout: 30s
  write_timeout: 30s
  max_body_size: 1048576

database:
  type: sqlite3
  path: $DATA_DIR/eamsa512.db
  max_connections: 25

encryption:
  block_size: 64
  key_size: 32
  nonce_size: 16
  rounds: 16

key_rotation:
  enabled: true
  interval_days: 365
  retention_cycles: 3
  max_key_age_days: 730
  min_key_age_days: 30

logging:
  level: info
  format: json
  output: file
  file_path: $LOG_DIR/eamsa512.log
  max_size_mb: 100
  max_backups: 10

audit:
  enabled: true
  log_path: $LOG_DIR/audit.log
  retention_days: 90

monitoring:
  enabled: true
  metrics_port: 9090
  health_check_interval: 30s
EOF

chmod 640 "$CONFIG_DIR/eamsa512.yaml"

# Create systemd service file
echo -e "${YELLOW}[STEP 8/10] Creating systemd service...${NC}"
cat > /etc/systemd/system/eamsa512.service <<EOF
[Unit]
Description=EAMSA 512 Encryption Service
After=network.target
Wants=network-online.target

[Service]
Type=simple
User=$USER
Group=$GROUP
WorkingDirectory=$INSTALL_DIR
Environment="PATH=/usr/local/bin:/usr/bin:/bin"
Environment="CONFIG_DIR=$CONFIG_DIR"
Environment="LOG_DIR=$LOG_DIR"
EnvironmentFile=$CONFIG_DIR/eamsa512.env

ExecStart=$INSTALL_DIR/eamsa512
Restart=on-failure
RestartSec=5s

# Security settings
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=$LOG_DIR $DATA_DIR
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
PrivateDevices=true
ProtectClock=true

# Resource limits
LimitNOFILE=65536
LimitNPROC=65536

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=eamsa512

[Install]
WantedBy=multi-user.target
EOF

# Create environment file
echo -e "${YELLOW}[STEP 9/10] Creating environment file...${NC}"
cat > "$CONFIG_DIR/eamsa512.env" <<EOF
# EAMSA 512 Environment Variables
CONFIG_DIR=$CONFIG_DIR
LOG_DIR=$LOG_DIR
DATA_DIR=$DATA_DIR
ENVIRONMENT=$ENVIRONMENT
GOMAXPROCS=0
EOF

chmod 640 "$CONFIG_DIR/eamsa512.env"

# Enable and start service
echo -e "${YELLOW}[STEP 10/10] Enabling systemd service...${NC}"
systemctl daemon-reload
systemctl enable eamsa512.service

if [ "$ENVIRONMENT" = "production" ]; then
    echo -e "${YELLOW}[INFO] Starting EAMSA 512 service...${NC}"
    systemctl start eamsa512.service
    sleep 2
    
    if systemctl is-active --quiet eamsa512.service; then
        echo -e "${GREEN}[OK] EAMSA 512 service is running${NC}"
    else
        echo -e "${RED}[ERROR] EAMSA 512 service failed to start${NC}"
        systemctl status eamsa512.service
        exit 1
    fi
fi

# Create firewall rules
echo -e "${YELLOW}[INFO] Setting up firewall rules...${NC}"
if command -v ufw &> /dev/null; then
    ufw allow 8080/tcp comment "EAMSA 512 HTTP"
    ufw allow 9090/tcp comment "EAMSA 512 Metrics"
elif command -v firewall-cmd &> /dev/null; then
    firewall-cmd --permanent --add-port=8080/tcp
    firewall-cmd --permanent --add-port=9090/tcp
    firewall-cmd --reload
fi

# Create backup script
echo -e "${YELLOW}[INFO] Creating backup script...${NC}"
cat > "$INSTALL_DIR/backup.sh" <<'EOF'
#!/bin/bash
BACKUP_DIR="/backups/eamsa512"
DATA_DIR="/var/lib/eamsa512"
mkdir -p "$BACKUP_DIR"

TIMESTAMP=$(date +%Y%m%d_%H%M%S)
tar -czf "$BACKUP_DIR/eamsa512_$TIMESTAMP.tar.gz" \
    -C /var/lib eamsa512 \
    -C /etc eamsa512

# Keep only last 30 backups
find "$BACKUP_DIR" -maxdepth 1 -type f -mtime +30 -delete

echo "Backup created: $BACKUP_DIR/eamsa512_$TIMESTAMP.tar.gz"
EOF

chmod +x "$INSTALL_DIR/backup.sh"

# Create monitoring script
echo -e "${YELLOW}[INFO] Creating monitoring script...${NC}"
cat > "$INSTALL_DIR/monitor.sh" <<'EOF'
#!/bin/bash
echo "EAMSA 512 Service Status"
echo "========================"
systemctl status eamsa512.service
echo ""
echo "Recent logs:"
journalctl -u eamsa512.service -n 20 --no-pager
echo ""
echo "System resources:"
ps aux | grep eamsa512 | grep -v grep
EOF

chmod +x "$INSTALL_DIR/monitor.sh"

# Print summary
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
echo "  Service User:         $USER"
echo "  Environment:          $ENVIRONMENT"
echo ""
echo "Next Steps:"
echo "  1. Review configuration:  cat $CONFIG_DIR/eamsa512.yaml"
echo "  2. Check service status:  systemctl status eamsa512.service"
echo "  3. View logs:             journalctl -u eamsa512.service -f"
echo "  4. Test API:              curl -k https://localhost:8080/api/v1/health"
echo ""
echo "Useful Commands:"
echo "  Start service:        systemctl start eamsa512.service"
echo "  Stop service:         systemctl stop eamsa512.service"
echo "  Restart service:      systemctl restart eamsa512.service"
echo "  Check status:         $INSTALL_DIR/monitor.sh"
echo "  Create backup:        $INSTALL_DIR/backup.sh"
echo ""

exit 0
