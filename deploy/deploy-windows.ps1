@echo off
REM EAMSA 512 - Windows Deployment Script
REM Supports: Windows Server 2019+, Windows 10 Pro/Enterprise
REM Usage: powershell -ExecutionPolicy Bypass -File deploy-windows.ps1 -Environment production

#Requires -RunAsAdministrator
#Requires -Version 5.1

param(
    [ValidateSet('production', 'staging', 'development')]
    [string]$Environment = 'production',
    [string]$InstallPath = 'C:\Program Files\EAMSA512',
    [string]$ConfigPath = 'C:\ProgramData\EAMSA512\config',
    [string]$DataPath = 'C:\ProgramData\EAMSA512\data',
    [string]$LogPath = 'C:\ProgramData\EAMSA512\logs'
)

# Color codes
$Red = [System.Console]::ForegroundColor = 'Red'
$Green = [System.Console]::ForegroundColor = 'Green'
$Yellow = [System.Console]::ForegroundColor = 'Yellow'
$Reset = [System.Console]::ResetColor()

function Write-Success {
    param([string]$Message)
    Write-Host "[OK] $Message" -ForegroundColor Green
}

function Write-Error-Custom {
    param([string]$Message)
    Write-Host "[ERROR] $Message" -ForegroundColor Red
}

function Write-Info {
    param([string]$Message)
    Write-Host "[INFO] $Message" -ForegroundColor Yellow
}

Write-Host "╔════════════════════════════════════════════════════════╗" -ForegroundColor Green
Write-Host "║     EAMSA 512 Windows Deployment Script v1.0.0        ║" -ForegroundColor Green
Write-Host "║     Environment: $Environment" -ForegroundColor Green
Write-Host "╚════════════════════════════════════════════════════════╝" -ForegroundColor Green
Write-Host ""

# Check admin rights
if (-not ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]'Administrator')) {
    Write-Error-Custom "This script must be run as Administrator"
    exit 1
}

# Step 1: Install prerequisites
Write-Info "STEP 1/9 Installing prerequisites..."
try {
    # Check for Chocolatey
    if (-not (Get-Command choco -ErrorAction SilentlyContinue)) {
        Write-Info "Installing Chocolatey..."
        [System.Net.ServicePointManager]::SecurityProtocol = [System.Net.ServicePointManager]::SecurityProtocol -bor 3072
        iex ((New-Object System.Net.WebClient).DownloadString('https://community.chocolatey.org/install.ps1'))
    }
    
    # Install Go
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        Write-Info "Installing Go..."
        choco install golang -y
    }
    
    # Install Git
    if (-not (Get-Command git -ErrorAction SilentlyContinue)) {
        Write-Info "Installing Git..."
        choco install git -y
    }
    
    # Install OpenSSL
    if (-not (Get-Command openssl -ErrorAction SilentlyContinue)) {
        Write-Info "Installing OpenSSL..."
        choco install openssl -y
    }
    
    Write-Success "Prerequisites installed"
}
catch {
    Write-Error-Custom "Failed to install prerequisites: $_"
    exit 1
}

# Step 2: Create directories
Write-Info "STEP 2/9 Creating directories..."
try {
    $dirs = @($InstallPath, $ConfigPath, $DataPath, $LogPath, "$ConfigPath\certs")
    foreach ($dir in $dirs) {
        if (-not (Test-Path $dir)) {
            New-Item -ItemType Directory -Path $dir -Force | Out-Null
        }
    }
    Write-Success "Directories created"
}
catch {
    Write-Error-Custom "Failed to create directories: $_"
    exit 1
}

# Step 3: Generate TLS certificates
Write-Info "STEP 3/9 Generating TLS certificates..."
try {
    $certPath = "$ConfigPath\certs"
    $certFile = "$certPath\tls.crt"
    $keyFile = "$certPath\tls.key"
    
    if (-not (Test-Path $certFile)) {
        Write-Info "Generating self-signed certificate..."
        & openssl req -x509 -newkey rsa:4096 `
            -keyout "$keyFile" `
            -out "$certFile" `
            -days 365 -nodes `
            -subj "/CN=eamsa512.local/O=EAMSA512/C=US"
    }
    Write-Success "TLS certificates ready"
}
catch {
    Write-Error-Custom "Failed to generate certificates: $_"
    exit 1
}

# Step 4: Download and build application
Write-Info "STEP 4/9 Building EAMSA 512..."
try {
    $srcPath = "$InstallPath\src"
    
    if (-not (Test-Path $srcPath)) {
        Write-Info "Cloning repository..."
        git clone https://github.com/yourusername/eamsa512.git $srcPath
    }
    
    if (Test-Path "$srcPath\go.mod") {
        Set-Location $srcPath
        Write-Info "Downloading Go modules..."
        & go mod download
        
        Write-Info "Building application..."
        & go build -o "$InstallPath\eamsa512.exe" .
        Write-Success "Build complete"
    }
    else {
        Write-Info "go.mod not found, skipping build"
    }
}
catch {
    Write-Error-Custom "Failed to build application: $_"
    exit 1
}

# Step 5: Create configuration file
Write-Info "STEP 5/9 Creating configuration file..."
try {
    $configFile = "$ConfigPath\eamsa512.yaml"
    
    $config = @"
# EAMSA 512 Configuration
server:
  host: 0.0.0.0
  port: 8080
  environment: $Environment
  tls_enabled: true
  tls_cert_path: $ConfigPath\certs\tls.crt
  tls_key_path: $ConfigPath\certs\tls.key
  read_timeout: 30s
  write_timeout: 30s
  max_body_size: 1048576

database:
  type: sqlite3
  path: $DataPath\eamsa512.db
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
  file_path: $LogPath\eamsa512.log
  max_size_mb: 100
  max_backups: 10

audit:
  enabled: true
  log_path: $LogPath\audit.log
  retention_days: 90

monitoring:
  enabled: true
  metrics_port: 9090
  health_check_interval: 30s
"@
    
    Set-Content -Path $configFile -Value $config
    Write-Success "Configuration file created"
}
catch {
    Write-Error-Custom "Failed to create configuration: $_"
    exit 1
}

# Step 6: Create Windows Service
Write-Info "STEP 6/9 Creating Windows Service..."
try {
    $serviceName = "EAMSA512"
    $displayName = "EAMSA 512 Encryption Service"
    $binaryPath = "$InstallPath\eamsa512.exe"
    
    # Check if service exists
    $service = Get-Service -Name $serviceName -ErrorAction SilentlyContinue
    
    if ($service) {
        Write-Info "Stopping existing service..."
        Stop-Service -Name $serviceName -Force
        Remove-Service -Name $serviceName -Force
    }
    
    Write-Info "Creating service..."
    New-Service -Name $serviceName `
        -DisplayName $displayName `
        -BinaryPathName $binaryPath `
        -StartupType Automatic `
        -Description "EAMSA 512 Encryption Service" | Out-Null
    
    # Set service recovery options
    $action = New-Object -ComObject "ServiceRecoveryActions"
    sc.exe failure $serviceName reset= 60 actions= restart/5000
    
    Write-Success "Windows Service created"
}
catch {
    Write-Error-Custom "Failed to create service: $_"
    exit 1
}

# Step 7: Configure firewall
Write-Info "STEP 7/9 Configuring Windows Firewall..."
try {
    # Allow port 8080 (HTTPS)
    $rule = Get-NetFirewallRule -DisplayName "EAMSA 512 Service" -ErrorAction SilentlyContinue
    if (-not $rule) {
        New-NetFirewallRule -DisplayName "EAMSA 512 Service" `
            -Direction Inbound `
            -LocalPort 8080 `
            -Protocol TCP `
            -Action Allow `
            -Enabled True | Out-Null
    }
    
    # Allow port 9090 (Metrics)
    $rule = Get-NetFirewallRule -DisplayName "EAMSA 512 Metrics" -ErrorAction SilentlyContinue
    if (-not $rule) {
        New-NetFirewallRule -DisplayName "EAMSA 512 Metrics" `
            -Direction Inbound `
            -LocalPort 9090 `
            -Protocol TCP `
            -Action Allow `
            -Enabled True | Out-Null
    }
    
    Write-Success "Firewall rules configured"
}
catch {
    Write-Error-Custom "Failed to configure firewall: $_"
}

# Step 8: Create startup scripts
Write-Info "STEP 8/9 Creating management scripts..."
try {
    # Start script
    $startScript = @"
@echo off
net start EAMSA512
if errorlevel 1 (
    echo Failed to start service
    exit /b 1
)
echo EAMSA 512 service started
"@
    Set-Content -Path "$InstallPath\start.bat" -Value $startScript
    
    # Stop script
    $stopScript = @"
@echo off
net stop EAMSA512
if errorlevel 1 (
    echo Failed to stop service
    exit /b 1
)
echo EAMSA 512 service stopped
"@
    Set-Content -Path "$InstallPath\stop.bat" -Value $stopScript
    
    # Status script
    $statusScript = @"
@echo off
echo EAMSA 512 Service Status
echo ========================
sc query EAMSA512
echo.
echo Recent Events:
wevtutil qe System /q:"System[EventID=7036 and Computer='%COMPUTERNAME%']" /f:text
"@
    Set-Content -Path "$InstallPath\status.bat" -Value $statusScript
    
    Write-Success "Management scripts created"
}
catch {
    Write-Error-Custom "Failed to create scripts: $_"
}

# Step 9: Start service
Write-Info "STEP 9/9 Starting service..."
try {
    if ($Environment -eq 'production') {
        Start-Service -Name "EAMSA512"
        Start-Sleep -Seconds 2
        
        $service = Get-Service -Name "EAMSA512"
        if ($service.Status -eq 'Running') {
            Write-Success "EAMSA 512 service is running"
        }
        else {
            Write-Error-Custom "Service failed to start"
            exit 1
        }
    }
    else {
        Write-Info "Service not started (non-production environment)"
    }
}
catch {
    Write-Error-Custom "Failed to start service: $_"
    exit 1
}

# Print summary
Write-Host ""
Write-Host "╔════════════════════════════════════════════════════════╗" -ForegroundColor Green
Write-Host "║         EAMSA 512 Deployment Complete!                ║" -ForegroundColor Green
Write-Host "╚════════════════════════════════════════════════════════╝" -ForegroundColor Green
Write-Host ""
Write-Host "Installation Summary:" -ForegroundColor Green
Write-Host "  Install Path:         $InstallPath"
Write-Host "  Config Path:          $ConfigPath"
Write-Host "  Data Path:            $DataPath"
Write-Host "  Log Path:             $LogPath"
Write-Host "  Service Name:         EAMSA512"
Write-Host "  Environment:          $Environment"
Write-Host ""
Write-Host "Next Steps:" -ForegroundColor Green
Write-Host "  1. Review config:     notepad $ConfigPath\eamsa512.yaml"
Write-Host "  2. Check service:     services.msc"
Write-Host "  3. View logs:         Get-Content $LogPath\eamsa512.log -Tail 50"
Write-Host "  4. Test API:          curl -k https://localhost:8080/api/v1/health"
Write-Host ""
Write-Host "Useful Commands:" -ForegroundColor Green
Write-Host "  Start service:        net start EAMSA512"
Write-Host "  Stop service:         net stop EAMSA512"
Write-Host "  Query service:        sc query EAMSA512"
Write-Host "  View status:          & '$InstallPath\status.bat'"
Write-Host ""

exit 0
