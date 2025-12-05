# EAMSA 512 Deployment Quick Reference

**Version:** 1.0.0 | **Date:** December 4, 2025

---

## 🚀 QUICK START COMMANDS

### Linux
```bash
sudo bash deploy-linux.sh production
# Service: systemctl {start|stop|restart} eamsa512.service
```

### Windows
```powershell
powershell -ExecutionPolicy Bypass -File deploy-windows.ps1
# Service: net {start|stop} EAMSA512
```

### macOS
```bash
bash deploy-macos.sh production
# Service: launchctl {start|stop} com.eamsa512
```

### Docker
```bash
docker build -t eamsa512:1.0.0 .
docker run -d -p 8080:8080 eamsa512:1.0.0
```

### Kubernetes
```bash
kubectl apply -f k8s-deployment.yaml
kubectl get pods -n eamsa512
```

### AWS
```bash
terraform init && terraform apply
terraform output alb_dns_name
```

### Cloudflare
```bash
bash deploy-cloudflare-worker.sh
npm run deploy
```

---

## 📍 KEY PORTS & ENDPOINTS

| Component | Port | Endpoint |
|-----------|------|----------|
| API | 8080 | https://localhost:8080/api/v1/ |
| Metrics | 9090 | https://localhost:9090/metrics |
| Health | 8080 | https://localhost:8080/api/v1/health |
| Database | varies | Per deployment |

---

## 📂 CONFIG LOCATIONS

| OS | Path |
|----|------|
| **Linux** | `/etc/eamsa512/eamsa512.yaml` |
| **Windows** | `C:\ProgramData\EAMSA512\config\eamsa512.yaml` |
| **macOS** | `/usr/local/etc/eamsa512/eamsa512.yaml` |
| **Docker** | `/etc/eamsa512/eamsa512.yaml` |

---

## 📋 COMMON OPERATIONS

### Check Status
```bash
# Linux
systemctl status eamsa512.service

# Windows
sc query EAMSA512

# macOS
launchctl list | grep eamsa512

# Docker
docker ps | grep eamsa512

# Kubernetes
kubectl get pods -n eamsa512
```

### View Logs
```bash
# Linux
journalctl -u eamsa512.service -f

# Windows
Get-Content C:\ProgramData\EAMSA512\logs\eamsa512.log -Tail 50

# macOS
tail -f /var/log/eamsa512/eamsa512.log

# Docker
docker logs -f eamsa512

# Kubernetes
kubectl logs -n eamsa512 deployment/eamsa512
```

### Restart Service
```bash
# Linux
sudo systemctl restart eamsa512.service

# Windows
net stop EAMSA512 && net start EAMSA512

# macOS
launchctl stop com.eamsa512 && launchctl start com.eamsa512

# Docker
docker restart eamsa512

# Kubernetes
kubectl rollout restart deployment/eamsa512 -n eamsa512
```

### Create Backup
```bash
# Linux/macOS
/opt/eamsa512/backup.sh
tar -czf eamsa512_backup.tar.gz /var/lib/eamsa512/

# Windows
.\backup.bat

# Docker
docker exec eamsa512 tar -czf /backup/eamsa512.tar.gz /var/lib/eamsa512/

# Kubernetes
kubectl exec -n eamsa512 <pod-name> -- tar -czf /backup/eamsa512.tar.gz /var/lib/eamsa512/
```

---

## 🔍 HEALTH CHECKS

```bash
# API Health
curl -k https://localhost:8080/api/v1/health

# Metrics
curl https://localhost:9090/metrics

# Liveness
curl -k https://localhost:8080/api/v1/health/live

# Readiness
curl -k https://localhost:8080/api/v1/health/ready
```

---

## 🛠️ TROUBLESHOOTING QUICK GUIDE

| Problem | Solution |
|---------|----------|
| **Port already in use** | Check process: `lsof -i :8080` / Kill: `kill -9 <PID>` |
| **Permission denied** | Check ownership: `ls -la /var/lib/eamsa512/` / Fix: `chown -R eamsa512:eamsa512` |
| **Service won't start** | Check logs: `journalctl -xe` / `docker logs` / `kubectl logs` |
| **TLS certificate error** | Regenerate: Remove cert files and rerun deployment script |
| **Database locked** | Restart service: `systemctl restart eamsa512.service` |
| **Out of memory** | Check usage: `ps aux \| grep eamsa512` / Increase limits in config |
| **High CPU** | Check if tasks running: `top -p $(pgrep eamsa512)` |

---

## 🔐 SECURITY ESSENTIALS

✅ **Always:**
- Use TLS certificates (never HTTP in production)
- Run as non-root user
- Keep firewall rules restrictive
- Rotate keys regularly
- Backup configuration
- Monitor for suspicious activity
- Update dependencies
- Use strong passwords

❌ **Never:**
- Deploy with default credentials
- Expose metrics port publicly
- Store keys in plaintext
- Skip TLS setup
- Run as root
- Ignore security warnings

---

## 📊 PERFORMANCE MONITORING

```bash
# CPU & Memory
ps aux | grep eamsa512
top -p $(pgrep eamsa512)

# Network connections
netstat -tulpn | grep 8080
ss -tulpn | grep 8080

# Open files
lsof -p $(pgrep eamsa512)

# Disk usage
du -sh /var/lib/eamsa512/
du -sh /var/log/eamsa512/

# Database size
du -sh /var/lib/eamsa512/eamsa512.db
sqlite3 /var/lib/eamsa512/eamsa512.db "PRAGMA page_count * page_size / 1024 / 1024 AS size_mb;"
```

---

## 📈 SCALING QUICK REFERENCE

| Need | Platform | Action |
|------|----------|--------|
| **Add more instances** | Kubernetes | `kubectl scale deployment eamsa512 --replicas=5 -n eamsa512` |
| **Increase resources** | AWS | Change instance type in ASG |
| **Better performance** | Docker | Add more containers or increase `docker run` resources |
| **High availability** | AWS | Terraform already sets up multi-AZ |
| **Geographic distribution** | Cloudflare | Default (global edge network) |

---

## 🔄 DEPLOYMENT WORKFLOW

```
1. Choose Platform
   ↓
2. Run Deployment Script
   ↓
3. Verify Installation
   ↓
4. Configure Application
   ↓
5. Test API Endpoints
   ↓
6. Setup Monitoring
   ↓
7. Configure Backups
   ↓
8. Production Ready ✅
```

---

## 📞 QUICK CONTACTS

- **Documentation:** See accompanying DEPLOYMENT_GUIDE.md
- **GitHub Issues:** https://github.com/yourusername/eamsa512/issues
- **Email Support:** support@eamsa512.com
- **Wiki:** https://github.com/yourusername/eamsa512/wiki

---

## 🎯 QUICK DECISION TREE

```
Choose your deployment:

├─ Local Development?
│  └─ → Use Docker Compose or macOS script
│
├─ Team Staging?
│  └─ → Use Kubernetes or single Linux instance
│
├─ Production (High Traffic)?
│  ├─ AWS? → Use Terraform deployment
│  ├─ On-Prem? → Use Linux deployment
│  └─ Global? → Use Cloudflare Workers
│
└─ Budget Constrained?
   └─ → Use Cloudflare Workers (~$10/mo) or Linux VPS (~$5/mo)
```

---

## ⚡ EMERGENCY PROCEDURES

### Service Crashed
```bash
# Linux
systemctl restart eamsa512.service

# Windows
net stop EAMSA512 && net start EAMSA512

# Docker
docker restart eamsa512
```

### Restore from Backup
```bash
# Extract backup
tar -xzf eamsa512_backup.tar.gz -C /

# Restart service
systemctl restart eamsa512.service
```

### Clear Logs
```bash
# Linux/macOS
rm /var/log/eamsa512/*.log

# Docker
docker exec eamsa512 rm /var/log/eamsa512/*.log
```

### Reset Configuration
```bash
# Restore from template
cp /etc/eamsa512/eamsa512.yaml.default /etc/eamsa512/eamsa512.yaml

# Restart service
systemctl restart eamsa512.service
```

---

## 📚 CONFIGURATION PARAMETERS

Key parameters in `eamsa512.yaml`:

```yaml
server:
  port: 8080                    # Change if port in use
  max_body_size: 1048576        # 1MB default

encryption:
  block_size: 64                # Don't change
  key_size: 32                  # 256-bit keys
  rounds: 16                    # Encryption rounds

key_rotation:
  interval_days: 365            # Annual rotation
  retention_cycles: 3           # Keep 3 versions

logging:
  level: info                   # debug/info/warn/error
  max_size_mb: 100              # Log file size limit
  max_backups: 10               # Retention count
```

---

## ✅ FINAL CHECKLIST

Before going live:
- [ ] Deployment script completed successfully
- [ ] All health checks passing
- [ ] TLS certificates installed
- [ ] Firewall rules configured
- [ ] Backups tested
- [ ] Monitoring active
- [ ] Logs flowing
- [ ] Team trained
- [ ] Runbooks prepared
- [ ] Emergency contacts documented

---

**Version:** 1.0.0  
**Status:** ✅ Production Ready  
**Last Updated:** December 4, 2025

**For full documentation, see:** DEPLOYMENT_GUIDE.md
