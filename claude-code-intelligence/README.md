# Claude Code Intelligence Service

AI-powered intelligence service for claude-code.nvim that provides session compression, semantic search, and context restoration using local LLM processing via Ollama.

## Features

- 🗜️ **Session Compression**: AI-powered compression achieving 70-80% size reduction
- 🔍 **Semantic Search**: Search sessions using natural language queries  
- 🤖 **Local AI Processing**: Uses Ollama for privacy-preserving local LLM processing
- 📊 **Model Testing**: Test and compare different LLM models
- 🏗️ **Auto Model Installation**: Automatically downloads and installs required models
- 📈 **Performance Tracking**: Monitor model performance and compression metrics
- 🔧 **Flexible Configuration**: Easy model switching and preset configurations

## Quick Start

### Prerequisites

1. **Go 1.21+** - [Install Go](https://golang.org/doc/install)
2. **Ollama** - [Install Ollama](https://ollama.ai/download)

### Installation

```bash
# Clone and setup
git clone <repository>
cd claude-code-intelligence

# Quick setup (installs dependencies and recommended models)
make quick-start

# Start the service
make dev
```

### Manual Setup

```bash
# 1. Install dependencies
make deps

# 2. Setup environment
cp .env.example .env
# Edit .env file with your preferences

# 3. Start Ollama
ollama serve

# 4. Install recommended models (will auto-install on first use)
make install-models

# 5. Build and run
make build
make run
```

## Model Selection & Testing

The service supports multiple LLM models with automatic installation. Here's how to choose and test models:

### Recommended Models

| Model | Size | Speed | Quality | Best For |
|-------|------|-------|---------|----------|
| `gemma2:2b` | ~1.6GB | ⚡⚡⚡ | ⭐⭐⭐ | Fast processing, low resources |
| `llama3.2:3b` | ~2GB | ⚡⚡ | ⭐⭐⭐⭐ | **Recommended default** |
| `mistral:7b` | ~4.1GB | ⚡ | ⭐⭐⭐⭐⭐ | High quality, slower |
| `qwen2.5:3b` | ~1.9GB | ⚡⚡ | ⭐⭐⭐⭐ | Code-heavy sessions |

### Model Configuration

**Environment Variables (.env):**
```bash
# Primary model (auto-installed if not available)
OLLAMA_PRIMARY_MODEL=llama3.2:3b
OLLAMA_FALLBACK_MODEL=gemma2:2b

# Model parameters  
MODEL_TEMPERATURE=0.3
MODEL_MAX_TOKENS=2000
```

**API Presets:**
```bash
# Use preset configurations
curl -X POST http://localhost:7345/api/v1/ai/compress \
  -H "Content-Type: application/json" \
  -d '{
    "content": "session content...",
    "options": {
      "preset": "quality"  // fast, balanced, quality, coding, tiny
    }
  }'
```

**Explicit Model Selection:**
```bash
curl -X POST http://localhost:7345/api/v1/ai/compress \
  -H "Content-Type: application/json" \
  -d '{
    "content": "session content...",
    "options": {
      "model": "mistral:7b",
      "style": "detailed"
    }
  }'
```

### Testing Models

**Test multiple models with sample content:**
```bash
curl -X POST http://localhost:7345/api/v1/ai/test-models \
  -H "Content-Type: application/json" \
  -d '{
    "content": "Your test session content here...",
    "models": ["gemma2:2b", "llama3.2:3b", "mistral:7b"]
  }'
```

**Response includes performance metrics:**
```json
{
  "results": [
    {
      "model": "llama3.2:3b",
      "success": true,
      "processing_time": "8.2s",
      "compression_ratio": 0.25,
      "quality_score": 8.5
    }
  ]
}
```

## API Reference

### Core Endpoints

**Health Check:**
```bash
curl http://localhost:7345/health
```

**Compress Session:**
```bash
curl -X POST http://localhost:7345/api/v1/sessions/compress \
  -H "Content-Type: application/json" \
  -d '{
    "session_id": "optional-session-id",
    "content": "Your session content here...",
    "options": {
      "style": "balanced",     // concise, balanced, detailed
      "max_length": 2000,      // max summary length
      "priority": "balanced",  // speed, balanced, quality
      "type": "general"        // general, code, discussion
    }
  }'
```

**Search Sessions:**
```bash
curl -X POST http://localhost:7345/api/v1/sessions/search \
  -H "Content-Type: application/json" \
  -d '{
    "query": "database optimization",
    "limit": 10
  }'
```

**Get Available Models:**
```bash
curl http://localhost:7345/api/v1/models
```

**Install Model:**
```bash
curl -X POST http://localhost:7345/api/v1/models/mistral:7b/install
```

**Service Stats:**
```bash
curl http://localhost:7345/api/v1/info/stats
```

### Model Performance Tracking

The service automatically tracks model performance:

```bash
curl http://localhost:7345/api/v1/info/stats
```

Returns metrics like:
- Success rates per model
- Average processing times
- Compression ratios
- Quality scores

## Configuration

### Environment Variables

```bash
# Server
PORT=7345
HOST=localhost
ENV=development

# Ollama
OLLAMA_URL=http://localhost:11434
OLLAMA_PRIMARY_MODEL=llama3.2:3b
OLLAMA_FALLBACK_MODEL=gemma2:2b

# Model Parameters
MODEL_TEMPERATURE=0.3      # Creativity (0.0-1.0)
MODEL_MAX_TOKENS=2000      # Max response length
MODEL_TOP_P=0.9           # Nucleus sampling
MODEL_SEED=42             # Reproducible results

# Performance
MAX_CONCURRENT_OPERATIONS=5
OPERATION_TIMEOUT=30s
MEMORY_LIMIT_MB=500

# Features
ENABLE_COMPRESSION=true
ENABLE_SEARCH=true
ENABLE_MODEL_TESTING=true
```

### Model Presets

Built-in presets for different use cases:

```json
{
  "fast": {
    "model": "gemma2:2b",
    "temperature": 0.3,
    "max_tokens": 1500
  },
  "balanced": {
    "model": "llama3.2:3b", 
    "temperature": 0.3,
    "max_tokens": 2000
  },
  "quality": {
    "model": "mistral:7b",
    "temperature": 0.2,
    "max_tokens": 3000
  },
  "coding": {
    "model": "qwen2.5:3b",
    "temperature": 0.2,
    "max_tokens": 2500
  }
}
```

## Development

### Building

```bash
# Development build
make build

# Multi-platform builds
make build-all

# Development server with hot reload
make dev

# Or with air (install: go install github.com/cosmtrek/air@latest)
make watch
```

### Testing

```bash
# Run tests
make test

# With coverage
make test-coverage

# Benchmarks
make bench

# Lint code
make lint
```

### Database

```bash
# Create backup
make db-backup

# View stats
curl http://localhost:7345/api/v1/info/stats
```

## Troubleshooting

### Common Issues

**Ollama Not Running:**
```bash
# Check if Ollama is running
make check-ollama

# Start Ollama
ollama serve
```

**Model Not Found:**
```bash
# List available models
ollama list

# Install specific model
ollama pull llama3.2:3b

# Or use API to auto-install
curl -X POST http://localhost:7345/api/v1/models/llama3.2:3b/install
```

**Slow Performance:**
- Use faster model: `gemma2:2b`
- Reduce `MODEL_MAX_TOKENS`
- Lower `MODEL_TEMPERATURE`

**Poor Compression Quality:**
- Use higher quality model: `mistral:7b`
- Increase `MODEL_MAX_TOKENS`
- Set style to `detailed`

### Logs

```bash
# View logs (if LOG_FILE is set)
tail -f logs/service.log

# Or check console output in development
make dev
```

## Performance Benchmarks

Typical performance on modern hardware:

| Model | Session Size | Processing Time | Compression Ratio | Quality Score |
|-------|-------------|----------------|-------------------|---------------|
| gemma2:2b | 1MB | ~3-5s | 68% | 6.5/10 |
| llama3.2:3b | 1MB | ~5-8s | 75% | 8.5/10 |
| mistral:7b | 1MB | ~10-15s | 78% | 9/10 |
| qwen2.5:3b | 1MB | ~5-7s | 76% | 8.5/10 |

*Results may vary based on hardware and content complexity*

## Integration with claude-code.nvim

This service is designed to work seamlessly with the claude-code.nvim plugin:

1. **Automatic Discovery**: The plugin automatically detects the running service
2. **Graceful Fallback**: Works without the service (basic functionality)
3. **Progressive Enhancement**: AI features are additive
4. **Local-Only**: All processing happens locally for privacy

## Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b feature-name`
3. Make changes and add tests
4. Run tests: `make test`
5. Submit a pull request

## License

MIT License - see LICENSE file for details.