# REGS - Setup & Deployment Guide

## Quick Start

### Prerequisites
- Docker (latest)
- Docker Compose (optional)
- Git

### 1. Clone & Build

```bash
git clone <repository>
cd regs
docker build -t regs .
```

### 2. Run Container

```bash
docker run -d \
  -p 8080:8080 \
  --name regs \
  -v regs_data:/app/data \
  regs
```

### 3. Verify Running

```bash
# Check container status
docker ps | grep regs

# View logs
docker logs regs

# Should see:
# [Queue] Started 4 judge workers
# Listening and serving HTTP on :8080
```

### 4. Test API

```bash
# Register user
curl -X POST http://localhost:8080/api/users/register \
  -H "Content-Type: application/json" \
  -d '{"username":"test","password":"test123"}'

# Login
curl -X POST http://localhost:8080/api/users/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}'

# List problems
curl http://localhost:8080/api/problems
```

---

## Docker Compose Setup

Create `docker-compose.yml`:

```yaml
version: '3.9'

services:
  regs:
    build: .
    container_name: regs
    ports:
      - "8080:8080"
    volumes:
      - regs_data:/app/data
      - /var/run/docker.sock:/var/run/docker.sock
    environment:
      - REGS_DATABASE_DSN=data/regs.db
      - GIN_MODE=release
    restart: unless-stopped
    networks:
      - regs_network

volumes:
  regs_data:

networks:
  regs_network:
    driver: bridge
```

Run:
```bash
docker-compose up -d
```

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `REGS_DATABASE_DSN` | `data/regs.db` | SQLite database path |
| `GIN_MODE` | `debug` | Set to `release` in production |
| `PORT` | `8080` | Server port |

---

## Production Deployment

### 1. Security Hardening

**Change default admin password:**
```sql
UPDATE users SET password = '<new_bcrypt_hash>' WHERE username = 'admin';
```

**Generate bcrypt hash:**
```bash
docker run -it golang:1.26-alpine go run -e 'package main; import("fmt"; "golang.org/x/crypto/bcrypt"); func main() { hash, _ := bcrypt.GenerateFromPassword([]byte("your_password"), 10); fmt.Println(string(hash)) }'
```

### 2. Database Backup

```bash
# Backup
docker exec regs cp /app/data/regs.db /app/data/regs.db.backup

# Restore
docker exec regs cp /app/data/regs.db.backup /app/data/regs.db
```

### 3. Nginx Reverse Proxy

```nginx
upstream regs {
    server localhost:8080;
}

server {
    listen 80;
    server_name judge.example.com;

    location / {
        proxy_pass http://regs;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_request_buffering off;
    }
}
```

### 4. SSL/TLS

```bash
# With Let's Encrypt
certbot certonly --standalone -d judge.example.com
certbot renew --dry-run
```

Update nginx:
```nginx
listen 443 ssl;
ssl_certificate /etc/letsencrypt/live/judge.example.com/fullchain.pem;
ssl_certificate_key /etc/letsencrypt/live/judge.example.com/privkey.pem;
```

### 5. Resource Limits

Update `docker-compose.yml`:
```yaml
services:
  regs:
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 2G
        reservations:
          cpus: '1'
          memory: 1G
```

---

## Scaling

### Multiple Judge Workers

Update `main.go`:
```go
// Increase workers
service.InitQueue(8)  // 8 concurrent judges instead of 4
```

Rebuild:
```bash
docker build -t regs .
docker run -p 8080:8080 regs
```

### Database Scaling

For production, use PostgreSQL instead of SQLite:

```bash
# Set connection string
export REGS_DATABASE_DSN="postgres://user:pass@postgres:5432/regs"
```

Update `db/db.go`:
```go
import _ "gorm.io/driver/postgres"

dsn := os.Getenv("REGS_DATABASE_DSN")
db := gorm.Open(postgres.Open(dsn), &gorm.Config{})
```

---

## Monitoring

### View Logs

```bash
# Real-time logs
docker logs -f regs

# Last 50 lines
docker logs --tail 50 regs

# Specific time range
docker logs --since 2025-01-15T10:00:00 regs
```

### Check Container Health

```bash
# Health check
docker exec regs curl http://localhost:8080/api/problems

# Container stats
docker stats regs

# Inspect container
docker inspect regs
```

### Database Status

```bash
# Count submissions
docker exec regs sqlite3 /app/data/regs.db "SELECT COUNT(*) FROM submissions;"

# Count users
docker exec regs sqlite3 /app/data/regs.db "SELECT COUNT(*) FROM users;"

# View queue status
docker logs regs | grep "\[Queue\]"
```

---

## Maintenance

### Update Container

```bash
# Pull latest
git pull
docker build -t regs .

# Stop old container
docker stop regs

# Start new container
docker run -d \
  -p 8080:8080 \
  --name regs \
  -v regs_data:/app/data \
  regs
```

### Clean Up

```bash
# Remove old containers
docker rm -f regs

# Remove dangling images
docker image prune -f

# Remove unused volumes
docker volume prune -f

# Full cleanup
docker system prune -af
```

### Database Maintenance

```bash
# Optimize database
docker exec regs sqlite3 /app/data/regs.db "VACUUM;"

# Check integrity
docker exec regs sqlite3 /app/data/regs.db "PRAGMA integrity_check;"
```

---

## Troubleshooting

### Port Already in Use

```bash
# Find process using port 8080
lsof -i :8080

# Kill process
kill -9 <PID>

# Or use different port
docker run -p 8081:8080 regs
```

### Container Crashes

```bash
# Check logs
docker logs regs

# Inspect
docker inspect regs

# Rebuild from scratch
docker rm -f regs
docker rmi regs
docker build -t regs .
docker run -p 8080:8080 regs
```

### Database Locked

```bash
# Reset database
docker exec regs rm /app/data/regs.db
docker restart regs

# Creates new fresh database
```

### Queue Not Processing

```bash
# Check worker threads
docker logs regs | grep "Worker"

# Check for errors
docker logs regs | grep "ERROR"

# Check Docker daemon
docker ps
```

### High Memory Usage

```bash
# Check container memory
docker stats regs

# Reduce workers
# Edit main.go: service.InitQueue(2)  # Reduce from 4
# Rebuild docker image
```

---

## API Testing

### Using curl

```bash
# Register
curl -X POST http://localhost:8080/api/users/register \
  -H "Content-Type: application/json" \
  -d '{"username":"alice","password":"pass123"}'

# Login
TOKEN=$(curl -s -X POST http://localhost:8080/api/users/login \
  -H "Content-Type: application/json" \
  -d '{"username":"alice","password":"pass123"}' | jq -r '.token')

# Use token
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/users/me
```

### Using Postman

1. Create new collection
2. Add requests:
   - `POST` Register
   - `POST` Login
   - `GET` Problems
   - etc.
3. Set `Authorization` header with Bearer token

### Using Python

```python
import requests

BASE = "http://localhost:8080/api"

# Register
resp = requests.post(f"{BASE}/users/register", json={
    "username": "alice",
    "password": "pass123"
})
print(resp.json())

# Login
resp = requests.post(f"{BASE}/users/login", json={
    "username": "alice",
    "password": "pass123"
})
token = resp.json()["token"]

# Get profile
resp = requests.get(
    f"{BASE}/users/me",
    headers={"Authorization": f"Bearer {token}"}
)
print(resp.json())
```

---

## Performance Tips

1. **Increase workers** for high load
2. **Use PostgreSQL** instead of SQLite
3. **Enable compression** in nginx
4. **Cache static responses** with Redis
5. **Use CDN** for file downloads
6. **Monitor queue depth** and adjust workers
7. **Archive old submissions** to reduce DB size

---

## Support

For issues, check:
1. Logs: `docker logs regs`
2. Database: `docker exec regs sqlite3 /app/data/regs.db`
3. Docker status: `docker ps`
4. Network: `docker network ls`

---

## License

Internal use only.
