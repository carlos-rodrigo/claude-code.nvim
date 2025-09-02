# Claude Code Intelligence API Documentation

This document provides comprehensive API documentation for the Claude Code Intelligence service.

## Base URL

```
http://localhost:8080/api
```

## Authentication

All API requests require authentication using an API key. Include the API key in the request header:

```
X-API-Key: your-api-key-here
```

Alternatively, you can use the Authorization header with Bearer token:

```
Authorization: Bearer your-api-key-here
```

## Rate Limiting

API requests are subject to rate limiting based on your API key configuration. Rate limit information is included in response headers:

```
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1640995200
Retry-After: 60 (when rate limited)
```

## Response Format

All API responses follow a consistent JSON format:

### Success Response
```json
{
  "success": true,
  "data": { ... },
  "message": "Operation completed successfully",
  "timestamp": "2024-01-20T12:00:00Z"
}
```

### Error Response
```json
{
  "success": false,
  "error": "Error description",
  "message": "User-friendly error message",
  "code": "ERROR_CODE",
  "timestamp": "2024-01-20T12:00:00Z"
}
```

## Endpoints

### Session Management

#### Compress Session Content

Compress and analyze session content using AI.

**POST** `/sessions/compress`

**Request Body:**
```json
{
  "content": "Session content to compress...",
  "context": "Optional context information",
  "model": "llama3.2:8b",
  "options": {
    "target_ratio": 0.3,
    "preserve_code": true,
    "preserve_errors": true
  }
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "session_id": "sess_123456",
    "compressed_content": "Compressed session content...",
    "original_size": 5000,
    "compressed_size": 1500,
    "compression_ratio": 0.3,
    "processing_time_ms": 2500,
    "model_used": "llama3.2:8b",
    "topics": ["error handling", "API integration"],
    "decisions": ["Used async approach", "Added error logging"],
    "patterns": [{
      "type": "error_pattern",
      "description": "Connection timeout handling",
      "frequency": 3
    }]
  }
}
```

#### Search Sessions

Search through compressed sessions using semantic search.

**GET** `/sessions/search`

**Query Parameters:**
- `query` (required): Search query
- `limit` (optional): Number of results (default: 10, max: 100)
- `filters` (optional): JSON object with filters
- `sort` (optional): Sort order (relevance, date, size)

**Example:**
```
GET /sessions/search?query=error%20handling&limit=5&sort=relevance
```

**Response:**
```json
{
  "success": true,
  "data": {
    "results": [
      {
        "session_id": "sess_123456",
        "content": "Session content snippet...",
        "relevance_score": 0.95,
        "created_at": "2024-01-20T10:00:00Z",
        "topics": ["error handling"],
        "size": 1500
      }
    ],
    "total_count": 25,
    "query": "error handling",
    "search_time_ms": 45
  }
}
```

#### Get Session Details

Retrieve detailed information about a specific session.

**GET** `/sessions/{session_id}`

**Response:**
```json
{
  "success": true,
  "data": {
    "session_id": "sess_123456",
    "content": "Full session content...",
    "compressed_content": "Compressed version...",
    "metadata": {
      "created_at": "2024-01-20T10:00:00Z",
      "updated_at": "2024-01-20T10:05:00Z",
      "model_used": "llama3.2:8b",
      "compression_ratio": 0.3,
      "original_size": 5000,
      "compressed_size": 1500
    },
    "topics": ["error handling", "API integration"],
    "decisions": ["Used async approach"],
    "patterns": []
  }
}
```

#### List Sessions

List all sessions with optional filtering.

**GET** `/sessions`

**Query Parameters:**
- `limit` (optional): Number of results (default: 20, max: 100)
- `offset` (optional): Pagination offset (default: 0)
- `status` (optional): Filter by status (active, compressed, archived)
- `model` (optional): Filter by model used
- `from_date` (optional): Filter from date (ISO 8601)
- `to_date` (optional): Filter to date (ISO 8601)

**Response:**
```json
{
  "success": true,
  "data": {
    "sessions": [
      {
        "session_id": "sess_123456",
        "status": "compressed",
        "created_at": "2024-01-20T10:00:00Z",
        "model_used": "llama3.2:8b",
        "compression_ratio": 0.3,
        "size": 1500,
        "topics": ["error handling"]
      }
    ],
    "pagination": {
      "total_count": 150,
      "limit": 20,
      "offset": 0,
      "has_more": true
    }
  }
}
```

### Advanced Features

#### Build Context

Build intelligent context from multiple related sessions.

**POST** `/context/build`

**Request Body:**
```json
{
  "query": "API error handling patterns",
  "max_sessions": 10,
  "max_tokens": 4000,
  "include_patterns": true,
  "filters": {
    "status": "compressed",
    "topics": ["error handling", "API"],
    "min_quality": 0.7
  },
  "sort_by": "relevance"
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "context": "Built context from multiple sessions...",
    "sessions_used": [
      {
        "session_id": "sess_123456",
        "relevance_score": 0.95,
        "contribution": "Error handling patterns"
      }
    ],
    "total_sessions": 8,
    "context_quality": 0.85,
    "token_count": 3500,
    "patterns_identified": [
      {
        "type": "error_pattern",
        "description": "Retry with exponential backoff",
        "frequency": 5,
        "sessions": ["sess_123456", "sess_789012"]
      }
    ]
  }
}
```

#### Consolidate Project Memory

Consolidate knowledge across project sessions.

**POST** `/memory/consolidate`

**Request Body:**
```json
{
  "project_context": "Web API development",
  "include_topics": true,
  "include_decisions": true,
  "include_patterns": true,
  "min_confidence": 0.6
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "consolidated_topics": [
      {
        "topic": "error handling",
        "sessions": ["sess_1", "sess_2"],
        "key_points": ["Use retry logic", "Log errors"],
        "confidence": 0.9
      }
    ],
    "key_decisions": [
      {
        "decision": "Use async/await pattern",
        "context": "API calls",
        "sessions": ["sess_1", "sess_3"],
        "reasoning": "Better error handling and readability"
      }
    ],
    "identified_patterns": [
      {
        "pattern": "API error handling",
        "type": "solution_pattern",
        "frequency": 8,
        "effectiveness": 0.85
      }
    ],
    "project_timeline": [
      {
        "phase": "Initial API setup",
        "sessions": ["sess_1", "sess_2"],
        "key_outcomes": ["Basic structure", "Error handling"]
      }
    ]
  }
}
```

### Analytics and Visualization

#### Get Analytics Dashboard

Retrieve comprehensive analytics data.

**GET** `/analytics/dashboard`

**Query Parameters:**
- `timeframe` (optional): Time period (7d, 30d, 90d, all)
- `granularity` (optional): Data granularity (hour, day, week)

**Response:**
```json
{
  "success": true,
  "data": {
    "summary": {
      "total_sessions": 150,
      "total_compressions": 140,
      "avg_compression_ratio": 0.32,
      "total_size_saved_mb": 45.2,
      "active_models": ["llama3.2:8b", "llama3.2:3b"]
    },
    "compression_stats": {
      "by_model": {
        "llama3.2:8b": {"count": 100, "avg_ratio": 0.30},
        "llama3.2:3b": {"count": 40, "avg_ratio": 0.35}
      },
      "by_time": [
        {"date": "2024-01-20", "compressions": 15, "avg_ratio": 0.31}
      ]
    },
    "topic_analysis": {
      "most_common": [
        {"topic": "error handling", "frequency": 25},
        {"topic": "API integration", "frequency": 18}
      ],
      "trending": [
        {"topic": "authentication", "growth": 0.45}
      ]
    },
    "performance_metrics": {
      "avg_processing_time_ms": 2200,
      "cache_hit_rate": 0.75,
      "error_rate": 0.02
    }
  }
}
```

#### Get Session Visualization

Get visualization data for a specific session.

**GET** `/analytics/sessions/{session_id}/visualization`

**Response:**
```json
{
  "success": true,
  "data": {
    "topic_network": {
      "nodes": [
        {"id": "error_handling", "size": 10, "type": "topic"},
        {"id": "api_calls", "size": 8, "type": "topic"}
      ],
      "edges": [
        {"source": "error_handling", "target": "api_calls", "weight": 0.7}
      ]
    },
    "decision_flow": [
      {
        "decision": "Use async pattern",
        "context": "API calls",
        "outcomes": ["Better error handling", "Improved performance"]
      }
    ],
    "complexity_metrics": {
      "cognitive_load": 0.6,
      "decision_count": 8,
      "topic_diversity": 0.75,
      "interaction_density": 0.45
    }
  }
}
```

### Backup and Recovery

#### Create Backup

Create a database backup.

**POST** `/backup`

**Request Body:**
```json
{
  "type": "manual",
  "description": "Pre-deployment backup"
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "backup_info": {
      "filename": "intelligence_backup_20240120_120000_manual.db",
      "path": "/app/data/backups/intelligence_backup_20240120_120000_manual.db",
      "size": 52428800,
      "created_at": "2024-01-20T12:00:00Z",
      "type": "manual",
      "checksum": "sha256:abc123...",
      "description": "Pre-deployment backup"
    },
    "duration_ms": 1500
  }
}
```

#### List Backups

List all available backups.

**GET** `/backup`

**Response:**
```json
{
  "success": true,
  "data": {
    "backups": [
      {
        "filename": "intelligence_backup_20240120_120000_manual.db",
        "size": 52428800,
        "created_at": "2024-01-20T12:00:00Z",
        "type": "manual",
        "description": "Pre-deployment backup"
      }
    ],
    "summary": {
      "total_count": 5,
      "total_size_bytes": 250000000,
      "total_size_mb": 238.4
    }
  }
}
```

#### Restore from Backup

Restore database from a backup.

**POST** `/backup/restore`

**Request Body:**
```json
{
  "backup_filename": "intelligence_backup_20240120_120000_manual.db",
  "confirm": true
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "backup_info": {
      "filename": "intelligence_backup_20240120_120000_manual.db",
      "size": 52428800,
      "created_at": "2024-01-20T12:00:00Z"
    },
    "duration_ms": 3000
  },
  "message": "Database restored successfully"
}
```

### Cache Management

#### Get Cache Stats

Retrieve cache statistics and performance metrics.

**GET** `/cache/stats`

**Response:**
```json
{
  "success": true,
  "data": {
    "memory_cache": {
      "size_bytes": 67108864,
      "size_mb": 64,
      "entries": 1250,
      "hit_rate": 0.85,
      "miss_rate": 0.15
    },
    "disk_cache": {
      "size_bytes": 134217728,
      "size_mb": 128,
      "entries": 500,
      "hit_rate": 0.65
    },
    "overall": {
      "total_requests": 10000,
      "total_hits": 7500,
      "total_misses": 2500,
      "hit_rate": 0.75
    }
  }
}
```

#### Clear Cache

Clear cache contents.

**DELETE** `/cache`

**Query Parameters:**
- `type` (optional): Cache type (memory, disk, all)

**Response:**
```json
{
  "success": true,
  "data": {
    "cleared": {
      "memory_entries": 1250,
      "disk_entries": 500,
      "total_size_freed_mb": 192
    }
  },
  "message": "Cache cleared successfully"
}
```

### Security and Administration

#### Create API Key

Create a new API key (requires admin permissions).

**POST** `/admin/keys`

**Request Body:**
```json
{
  "name": "client-key",
  "permissions": ["read:sessions", "write:sessions"],
  "rate_limit": 100,
  "expires_in_days": 30
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "api_key": {
      "key": "sk_abc123def456...",
      "name": "client-key",
      "permissions": ["read:sessions", "write:sessions"],
      "rate_limit": 100,
      "created_at": "2024-01-20T12:00:00Z",
      "expires_at": "2024-02-19T12:00:00Z"
    }
  },
  "message": "API key created successfully",
  "warning": "Store this API key securely. It will not be shown again."
}
```

#### List API Keys

List all API keys (requires admin permissions).

**GET** `/admin/keys`

**Response:**
```json
{
  "success": true,
  "data": {
    "api_keys": [
      {
        "key": "sk_abc123...",
        "name": "client-key",
        "permissions": ["read:sessions"],
        "rate_limit": 100,
        "created_at": "2024-01-20T12:00:00Z",
        "last_used": "2024-01-20T14:30:00Z",
        "enabled": true
      }
    ],
    "statistics": {
      "total_keys": 5,
      "enabled_keys": 4,
      "expired_keys": 1
    }
  }
}
```

#### Revoke API Key

Revoke an API key (requires admin permissions).

**DELETE** `/admin/keys/{key}`

**Response:**
```json
{
  "success": true,
  "message": "API key revoked successfully"
}
```

### Health and Monitoring

#### Health Check

Check overall service health.

**GET** `/health`

**Response:**
```json
{
  "status": "healthy",
  "timestamp": "2024-01-20T12:00:00Z",
  "version": "1.0.0",
  "uptime_seconds": 86400,
  "components": {
    "database": {
      "status": "healthy",
      "response_time_ms": 2,
      "last_check": "2024-01-20T11:59:55Z"
    },
    "ollama": {
      "status": "healthy",
      "response_time_ms": 150,
      "models_loaded": ["llama3.2:8b"],
      "last_check": "2024-01-20T11:59:58Z"
    },
    "cache": {
      "status": "healthy",
      "hit_rate": 0.75,
      "size_mb": 64
    }
  }
}
```

#### Readiness Check

Check if service is ready to handle requests.

**GET** `/ready`

**Response:**
```json
{
  "ready": true,
  "timestamp": "2024-01-20T12:00:00Z",
  "checks": {
    "database": true,
    "ollama": true,
    "configuration": true
  }
}
```

#### Liveness Check

Basic liveness probe.

**GET** `/live`

**Response:**
```json
{
  "alive": true,
  "timestamp": "2024-01-20T12:00:00Z",
  "uptime_seconds": 86400
}
```

## Error Codes

| Code | Description |
|------|-------------|
| `AUTHENTICATION_REQUIRED` | API key is required |
| `INVALID_API_KEY` | API key is invalid or expired |
| `INSUFFICIENT_PERMISSIONS` | API key lacks required permissions |
| `RATE_LIMIT_EXCEEDED` | Rate limit exceeded |
| `VALIDATION_FAILED` | Request validation failed |
| `RESOURCE_NOT_FOUND` | Requested resource not found |
| `SERVICE_UNAVAILABLE` | External service unavailable |
| `INTERNAL_ERROR` | Internal server error |
| `DATABASE_ERROR` | Database operation failed |
| `CACHE_ERROR` | Cache operation failed |

## Rate Limits

Default rate limits by endpoint category:

| Category | Default Limit | Burst Limit |
|----------|---------------|-------------|
| Read Operations | 100 req/min | 150 req/min |
| Write Operations | 50 req/min | 75 req/min |
| Admin Operations | 20 req/min | 30 req/min |
| Analytics | 30 req/min | 45 req/min |

## Examples

### Compress a session

```bash
curl -X POST http://localhost:8080/api/sessions/compress \
  -H "X-API-Key: your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "content": "User reported error connecting to API...",
    "context": "Bug report session",
    "model": "llama3.2:8b"
  }'
```

### Search sessions

```bash
curl -X GET "http://localhost:8080/api/sessions/search?query=API%20error&limit=5" \
  -H "X-API-Key: your-api-key"
```

### Create backup

```bash
curl -X POST http://localhost:8080/api/backup \
  -H "X-API-Key: your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "type": "manual",
    "description": "Weekly backup"
  }'
```

### Get analytics

```bash
curl -X GET "http://localhost:8080/api/analytics/dashboard?timeframe=30d" \
  -H "X-API-Key: your-api-key"
```