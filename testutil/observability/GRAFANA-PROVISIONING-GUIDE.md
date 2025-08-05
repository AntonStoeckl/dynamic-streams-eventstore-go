# Grafana Dashboard Provisioning - Quick Reference Guide

## 🎯 How to Create Persistent Grafana Dashboards

### Required JSON Format for File Provisioning
```json
{
  "uid": "unique-dashboard-id",     // REQUIRED - unique identifier  
  "version": 1,                    // REQUIRED - for update tracking
  "title": "Your Dashboard Name",
  "tags": ["tag1", "tag2"],
  "timezone": "browser",
  "refresh": "5s",
  "schemaVersion": 27,
  "time": {
    "from": "now-5m",
    "to": "now"
  },
  "panels": [
    // Your panels here
  ]
}
```

### Required Provisioning Configuration
**File**: `grafana/provisioning/dashboards/dashboards.yml`
```yaml
apiVersion: 1
providers:
  - name: 'your-dashboards'
    orgId: 1
    folder: ''                     # Empty string = root folder
    type: file
    disableDeletion: false
    updateIntervalSeconds: 10
    allowUiUpdates: true
    options:
      path: /var/lib/grafana/dashboards
```

### Required Docker Configuration
**File**: `docker-compose.yml`
```yaml
grafana:
  image: grafana/grafana:latest
  volumes:
    - ./grafana/provisioning:/etc/grafana/provisioning
    - ./grafana/dashboards:/var/lib/grafana/dashboards
    - grafana-storage:/var/lib/grafana           # REQUIRED for persistence
  # ... other config

volumes:
  grafana-storage:                               # REQUIRED at file bottom
```

## 🚫 Critical Errors to Avoid

### ❌ Don't Use API JSON Format
```json
// WRONG - Will show empty panels
{
  "dashboard": { "title": "...", ... },
  "folderId": 0
}
```

### ❌ Don't Include ID Field
```json
// WRONG - Will cause conflicts  
{
  "id": 123,           // Remove this
  "uid": "my-dash",
  "title": "..."
}
```

### ❌ Don't Forget Required Fields
```json
// WRONG - Will fail to provision
{
  "title": "My Dashboard"
  // Missing "uid" and "version"
}
```

### ❌ Don't Create Subfolders Accidentally
```yaml
# WRONG - Creates unwanted EventStore folder
providers:
  - folder: 'EventStore'

# CORRECT - Root location
providers:
  - folder: ''
```

## 🔧 Quick Conversion from API Dashboard

1. **Export from API:**
```bash
curl -u admin:secretpw "http://localhost:3000/api/dashboards/uid/YOUR_UID" | jq '.dashboard' > temp.json
```

2. **Add required fields and clean:**
```bash
jq '. + {"uid": "my-dashboard-id", "version": 1} | del(.id)' temp.json > final-dashboard.json
```

3. **Place in dashboards directory:**
```bash
mv final-dashboard.json testutil/observability/grafana/dashboards/
```

4. **Restart to apply:**
```bash
docker compose restart grafana
```

## ✅ Verification Checklist

- [ ] Dashboard JSON has `uid` and `version` fields
- [ ] Dashboard JSON has NO `id` field  
- [ ] Provisioning config uses `folder: ''` for root
- [ ] Docker compose has persistent volume
- [ ] Dashboard appears after `docker compose restart grafana`
- [ ] Dashboard persists after `docker compose down && docker compose up -d`

## 📍 This Setup Results In
- ✅ **100% persistent dashboards** across all Docker operations
- ✅ **Root folder location** (no subfolders)
- ✅ **File-based version control** of dashboard definitions
- ✅ **Automatic updates** when JSON files change

## 🆘 Troubleshooting
If dashboards show empty panels:
1. Check datasource UID matches between dashboard and Grafana
2. Verify metrics exist in Prometheus: `curl http://localhost:9090/api/v1/query?query=up`
3. Check Grafana logs: `docker compose logs grafana`
4. Ensure JSON format is correct (no API wrapper)

---
*Created: 2025-08-05 | Tested with Grafana latest | EventStore Dynamic Streams Project*