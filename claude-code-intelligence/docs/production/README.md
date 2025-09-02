# Claude Code Intelligence - Production Documentation

This documentation provides comprehensive guidance for deploying, operating, and maintaining the Claude Code Intelligence service in production environments.

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Deployment Guide](#deployment-guide)
3. [Configuration](#configuration)
4. [Security](#security)
5. [Monitoring & Observability](#monitoring--observability)
6. [Backup & Recovery](#backup--recovery)
7. [Performance Tuning](#performance-tuning)
8. [Troubleshooting](#troubleshooting)
9. [Maintenance](#maintenance)
10. [API Reference](#api-reference)

## Architecture Overview

### System Components

The Claude Code Intelligence service consists of several key components:

- **API Server**: Go-based HTTP server handling requests
- **Database**: SQLite for data persistence (PostgreSQL support available)
- **Ollama Integration**: Local LLM processing for AI features
- **Cache Layer**: Memory and disk-based caching system
- **Monitoring**: Prometheus metrics and health checks
- **Security**: API key authentication and rate limiting

### Dependencies

- **Go 1.21+**: Application runtime
- **Ollama**: Local LLM processing
- **SQLite 3**: Database (default)
- **Docker**: Containerization
- **Kubernetes**: Orchestration (optional)

### Network Architecture

```
Internet -> Load Balancer -> Nginx -> Claude Code Intelligence -> Ollama
                                  |
                                  v
                              Database/Storage
```

## Deployment Guide

### Prerequisites

1. **Hardware Requirements**:
   - CPU: 4+ cores recommended
   - Memory: 8GB+ RAM (4GB for Ollama, 2GB for service)
   - Storage: 100GB+ SSD (models require significant space)
   - Network: Stable internet for model downloads

2. **Software Requirements**:
   - Docker 20.10+
   - Docker Compose 2.0+ (for compose deployment)
   - Kubernetes 1.20+ (for k8s deployment)
   - curl (for health checks)

### Deployment Options

#### 1. Docker Compose (Recommended for Development/Small Production)

```bash
# Clone repository
git clone <repository-url>
cd claude-code-intelligence

# Deploy with default configuration
cd deployments/scripts
./deploy.sh -t docker-compose deploy

# Check status
./deploy.sh -t docker-compose status

# View logs
./deploy.sh -t docker-compose logs
```

#### 2. Kubernetes (Recommended for Production)

```bash
# Deploy to Kubernetes
./deploy.sh -t kubernetes -e prod deploy

# Check deployment status
kubectl get pods -n claude-code-intelligence

# Monitor deployment
kubectl logs -f deployment/claude-code-intelligence -n claude-code-intelligence
```

#### 3. Docker (Simple Single Container)

```bash
# Build and deploy
./deploy.sh -t docker deploy

# Check container status
docker ps --filter name=claude-code-intelligence
```

### Environment-Specific Configurations

#### Development
- Single replica
- Debug logging enabled
- Local storage
- Minimal resource limits

#### Staging
- Multiple replicas
- Production-like configuration
- Persistent storage
- Monitoring enabled

#### Production
- High availability setup
- Resource limits enforced
- Security hardening
- Full monitoring and alerting

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `SERVER_PORT` | `8080` | HTTP server port |
| `SERVER_HOST` | `0.0.0.0` | HTTP server host |
| `LOG_LEVEL` | `info` | Logging level (debug, info, warn, error) |
| `DATABASE_PATH` | `/app/data/intelligence.db` | SQLite database path |
| `DATABASE_BACKUP_PATH` | `/app/data/backups` | Backup directory |
| `OLLAMA_HOST` | `localhost` | Ollama service hostname |
| `OLLAMA_PORT` | `11434` | Ollama service port |
| `OLLAMA_TIMEOUT` | `30s` | Ollama request timeout |
| `API_KEY_ENABLED` | `true` | Enable API key authentication |
| `RATE_LIMIT_ENABLED` | `true` | Enable rate limiting |
| `DEFAULT_RATE_LIMIT` | `100` | Default requests per minute |
| `CACHE_ENABLED` | `true` | Enable caching |
| `CACHE_SIZE_MB` | `256` | Cache size in MB |
| `METRICS_ENABLED` | `true` | Enable Prometheus metrics |
| `HEALTH_CHECK_INTERVAL` | `30s` | Health check interval |

### Configuration File

The service can be configured using a YAML file at `/app/configs/config.yaml`:

```yaml
server:
  host: "0.0.0.0"
  port: 8080
  read_timeout: "30s"
  write_timeout: "30s"
  idle_timeout: "60s"

database:
  type: "sqlite"
  path: "/app/data/intelligence.db"
  backup_path: "/app/data/backups"
  auto_backup: true
  backup_interval: "24h"
  max_backups: 7

ollama:
  host: "ollama"
  port: 11434
  timeout: "30s"
  retry_attempts: 3
  retry_delay: "5s"
  models:
    - name: "llama3.2:3b"
      preset: "fast"
    - name: "llama3.2:8b"
      preset: "balanced"

security:
  api_key_required: true
  cors_enabled: true
  rate_limiting:
    enabled: true
    default_limit: 100
    burst_limit: 150

monitoring:
  metrics_enabled: true
  health_check_interval: "30s"
  prometheus_endpoint: "/metrics"
```

## Security

### Authentication

The service uses API key-based authentication. API keys are managed through the admin interface.

#### Creating API Keys

```bash
# Create a new API key via API
curl -X POST http://localhost:8080/api/admin/keys \
  -H "X-API-Key: YOUR_ADMIN_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "client-key",
    "permissions": ["read:sessions", "write:sessions"],
    "rate_limit": 100,
    "expires_in_days": 30
  }'
```

#### Default Admin Key

A default admin API key is created on first startup and logged to the console. This key has full permissions and should be secured immediately.

### Rate Limiting

Rate limiting is implemented at multiple levels:

1. **Global**: Overall system limits
2. **Per-API-Key**: Individual client limits
3. **Per-Endpoint**: Specific endpoint limits

### Security Headers

The following security headers are automatically added:

- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY`
- `X-XSS-Protection: 1; mode=block`
- `Referrer-Policy: strict-origin-when-cross-origin`
- `Content-Security-Policy: default-src 'self'`

### Network Security

#### Firewall Rules

```bash
# Allow only necessary ports
ufw allow 8080/tcp  # Service port
ufw allow 11434/tcp # Ollama port (if external)
ufw deny 22/tcp     # Disable SSH if not needed
```

#### SSL/TLS

For production deployments, enable SSL/TLS:

```yaml
# nginx configuration
server {
    listen 443 ssl http2;
    ssl_certificate /path/to/certificate.crt;
    ssl_certificate_key /path/to/private.key;
    
    location / {
        proxy_pass http://claude-backend;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

## Monitoring & Observability

### Health Checks

The service provides multiple health check endpoints:

- `/health`: Overall service health
- `/ready`: Readiness check
- `/live`: Liveness check

### Metrics

Prometheus metrics are available at `/metrics`:

```bash
# Check metrics
curl http://localhost:8080/metrics
```

Key metrics include:

- Request count and response times
- Error rates
- Database query performance
- Ollama service health
- Cache hit rates
- Memory usage

### Logging

Structured JSON logging is used with configurable levels:

```json
{
  "time": "2024-01-20T12:00:00Z",
  "level": "info",
  "msg": "Request processed",
  "method": "POST",
  "path": "/api/sessions",
  "status": 200,
  "duration_ms": 45,
  "client_ip": "192.168.1.100"
}
```

### Alerting

Set up alerts for:

- Service unavailability
- High error rates (>5%)
- High response times (>5s)
- Database issues
- Ollama service failures
- Memory usage (>80%)
- Disk space (>90%)

## Backup & Recovery

### Automated Backups

Backups are created automatically every 24 hours by default:

```bash
# Manual backup
curl -X POST http://localhost:8080/api/backup \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"type": "manual", "description": "Pre-maintenance backup"}'
```

### Backup Storage

Backups are stored locally by default. For production, consider:

1. **Remote Storage**: AWS S3, Google Cloud Storage
2. **Multiple Locations**: Geographic redundancy
3. **Encryption**: Encrypt backups at rest
4. **Retention**: Implement retention policies

### Recovery Procedures

#### Database Recovery

```bash
# List available backups
curl -X GET http://localhost:8080/api/backup \
  -H "X-API-Key: YOUR_API_KEY"

# Restore from backup
curl -X POST http://localhost:8080/api/backup/restore \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "backup_filename": "intelligence_backup_20240120_120000_manual.db",
    "confirm": true
  }'
```

#### Complete System Recovery

1. **Restore Infrastructure**: Redeploy containers/pods
2. **Restore Database**: Use backup restoration API
3. **Verify Service**: Run health checks
4. **Update DNS**: Point traffic to new instance

## Performance Tuning

### Resource Allocation

#### Memory

- **Minimum**: 2GB for service + 4GB for Ollama
- **Recommended**: 4GB for service + 8GB for Ollama
- **Cache sizing**: Allocate 256MB-1GB for cache

#### CPU

- **Minimum**: 2 cores
- **Recommended**: 4+ cores
- **Ollama**: Benefits from more cores for inference

#### Storage

- **Type**: SSD strongly recommended
- **Size**: 100GB+ (models can be 10-50GB each)
- **IOPS**: High IOPS for database operations

### Optimization Settings

#### Database

```sql
-- SQLite optimizations
PRAGMA journal_mode = WAL;
PRAGMA synchronous = NORMAL;
PRAGMA cache_size = 10000;
PRAGMA temp_store = MEMORY;
```

#### Go Runtime

```bash
# Environment variables for Go optimization
export GOGC=100
export GOMAXPROCS=4
export GOMEMLIMIT=2GiB
```

### Scaling

#### Horizontal Scaling

- Deploy multiple service replicas
- Use load balancer for distribution
- Shared database/storage required

#### Vertical Scaling

- Increase CPU/memory allocation
- Scale Ollama resources separately
- Monitor resource utilization

## Troubleshooting

### Common Issues

#### Service Won't Start

1. **Check logs**: `docker logs claude-code-intelligence`
2. **Verify ports**: Ensure ports 8080/11434 are available
3. **Check dependencies**: Verify Ollama is running
4. **Database permissions**: Ensure write access to data directory

#### Ollama Connection Issues

1. **Network connectivity**: `curl http://ollama:11434/api/tags`
2. **Service status**: Check Ollama container/pod status
3. **Model availability**: Verify required models are installed
4. **Resource limits**: Ensure sufficient memory for models

#### High Response Times

1. **Check resource usage**: CPU/memory utilization
2. **Database performance**: Query optimization needed
3. **Ollama performance**: Model inference times
4. **Network latency**: Between components

#### Authentication Issues

1. **API key validation**: Check key format and permissions
2. **Rate limiting**: Verify not hitting limits
3. **System time**: Ensure clocks are synchronized

### Debugging

#### Enable Debug Logging

```bash
export LOG_LEVEL=debug
```

#### Database Inspection

```bash
# Connect to SQLite database
sqlite3 /app/data/intelligence.db
.tables
.schema sessions
```

#### Metrics Analysis

```bash
# Check specific metrics
curl -s http://localhost:8080/metrics | grep claude_code_requests_total
```

## Maintenance

### Regular Tasks

#### Daily
- Monitor service health
- Check error logs
- Verify backup completion

#### Weekly
- Review performance metrics
- Check storage usage
- Update documentation

#### Monthly
- Security audit
- Dependency updates
- Capacity planning review

#### Quarterly
- Disaster recovery testing
- Performance benchmarking
- Architecture review

### Updates

#### Service Updates

```bash
# Update service version
./deploy.sh -t docker-compose update
```

#### Model Updates

```bash
# Update Ollama models
docker exec claude-ollama ollama pull llama3.2:8b
```

#### Security Updates

1. **Base images**: Update container base images
2. **Dependencies**: Update Go modules
3. **System packages**: Update OS packages

### Monitoring Checklist

- [ ] Service health endpoints responding
- [ ] Error rates within acceptable limits
- [ ] Response times within SLA
- [ ] Resource utilization normal
- [ ] Backup processes running
- [ ] Log files not growing excessively
- [ ] Certificate expiration dates

## API Reference

See [API.md](./API.md) for complete API documentation.

### Key Endpoints

- `POST /api/sessions/compress` - Compress session content
- `GET /api/sessions/search` - Search sessions
- `POST /api/backup` - Create backup
- `GET /metrics` - Prometheus metrics
- `GET /health` - Health check

### Authentication

All API requests require an API key in the `X-API-Key` header:

```bash
curl -H "X-API-Key: your-api-key" http://localhost:8080/api/sessions
```

## Support

For additional support:

1. **Documentation**: Check this documentation
2. **Logs**: Review service and container logs
3. **Metrics**: Check Prometheus metrics and Grafana dashboards
4. **Health checks**: Verify all health endpoints
5. **Community**: Consult project issues and discussions