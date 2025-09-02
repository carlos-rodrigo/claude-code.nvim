package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"claude-code-intelligence/internal/types"
	
	"github.com/joho/godotenv"
)

// Config holds all configuration for the service
type Config struct {
	Server     ServerConfig     `json:"server"`
	Ollama     OllamaConfig     `json:"ollama"`
	Database   DatabaseConfig   `json:"database"`
	Embeddings EmbeddingsConfig `json:"embeddings"`
	Performance PerformanceConfig `json:"performance"`
	Security   SecurityConfig   `json:"security"`
	Logging    LoggingConfig    `json:"logging"`
	Features   FeatureConfig    `json:"features"`
	ModelPresets map[string]types.ModelPreset `json:"model_presets"`
}

type ServerConfig struct {
	Port string `json:"port"`
	Host string `json:"host"`
	Env  string `json:"env"`
}

type OllamaConfig struct {
	URL           string        `json:"url"`
	PrimaryModel  string        `json:"primary_model"`
	FallbackModel string        `json:"fallback_model"`
	Timeout       time.Duration `json:"timeout"`
	Temperature   float64       `json:"temperature"`
	MaxTokens     int           `json:"max_tokens"`
	TopP          float64       `json:"top_p"`
	Seed          *int          `json:"seed,omitempty"`
}

type DatabaseConfig struct {
	Path       string `json:"path"`
	BackupPath string `json:"backup_path"`
	PoolSize   int    `json:"pool_size"`
}

type EmbeddingsConfig struct {
	URL     string        `json:"url"`
	Model   string        `json:"model"`
	Timeout time.Duration `json:"timeout"`
}

type PerformanceConfig struct {
	MaxConcurrentOps   int           `json:"max_concurrent_operations"`
	OperationTimeout   time.Duration `json:"operation_timeout"`
	MemoryLimitMB      int           `json:"memory_limit_mb"`
	CompressionBatchSize int         `json:"compression_batch_size"`
}

type SecurityConfig struct {
	CORSOrigins   []string `json:"cors_origins"`
	RateLimitRPS  int      `json:"rate_limit_rps"`
	RateLimitBurst int     `json:"rate_limit_burst"`
}

type LoggingConfig struct {
	Level  string `json:"level"`
	Format string `json:"format"`
	File   string `json:"file"`
}

type FeatureConfig struct {
	Compression   bool `json:"compression"`
	Search        bool `json:"search"`
	Embeddings    bool `json:"embeddings"`
	ModelTesting  bool `json:"model_testing"`
}

var GlobalConfig *Config

// Load loads configuration from environment variables and .env file
func Load() (*Config, error) {
	// Load .env file if it exists
	_ = godotenv.Load()

	config := &Config{
		Server: ServerConfig{
			Port: getEnvOrDefault("PORT", "7345"),
			Host: getEnvOrDefault("HOST", "localhost"),
			Env:  getEnvOrDefault("ENV", "development"),
		},
		Ollama: OllamaConfig{
			URL:           getEnvOrDefault("OLLAMA_URL", "http://localhost:11434"),
			PrimaryModel:  getEnvOrDefault("OLLAMA_PRIMARY_MODEL", "llama3.2:3b"),
			FallbackModel: getEnvOrDefault("OLLAMA_FALLBACK_MODEL", "gemma2:2b"),
			Timeout:       getEnvDurationOrDefault("OLLAMA_TIMEOUT", 30*time.Second),
			Temperature:   getEnvFloatOrDefault("MODEL_TEMPERATURE", 0.3),
			MaxTokens:     getEnvIntOrDefault("MODEL_MAX_TOKENS", 2000),
			TopP:          getEnvFloatOrDefault("MODEL_TOP_P", 0.9),
		},
		Database: DatabaseConfig{
			Path:       getEnvOrDefault("DB_PATH", "./data/intelligence.db"),
			BackupPath: getEnvOrDefault("DB_BACKUP_PATH", "./data/backups"),
			PoolSize:   getEnvIntOrDefault("DB_POOL_SIZE", 10),
		},
		Embeddings: EmbeddingsConfig{
			URL:     getEnvOrDefault("EMBEDDINGS_URL", "http://localhost:8080"),
			Model:   getEnvOrDefault("EMBEDDINGS_MODEL", "sentence-transformers/all-MiniLM-L6-v2"),
			Timeout: getEnvDurationOrDefault("EMBEDDINGS_TIMEOUT", 10*time.Second),
		},
		Performance: PerformanceConfig{
			MaxConcurrentOps:     getEnvIntOrDefault("MAX_CONCURRENT_OPERATIONS", 5),
			OperationTimeout:     getEnvDurationOrDefault("OPERATION_TIMEOUT", 30*time.Second),
			MemoryLimitMB:        getEnvIntOrDefault("MEMORY_LIMIT_MB", 500),
			CompressionBatchSize: getEnvIntOrDefault("COMPRESSION_BATCH_SIZE", 10),
		},
		Security: SecurityConfig{
			CORSOrigins:    strings.Split(getEnvOrDefault("CORS_ORIGINS", "*"), ","),
			RateLimitRPS:   getEnvIntOrDefault("RATE_LIMIT_RPS", 10),
			RateLimitBurst: getEnvIntOrDefault("RATE_LIMIT_BURST", 20),
		},
		Logging: LoggingConfig{
			Level:  getEnvOrDefault("LOG_LEVEL", "info"),
			Format: getEnvOrDefault("LOG_FORMAT", "json"),
			File:   getEnvOrDefault("LOG_FILE", "./logs/service.log"),
		},
		Features: FeatureConfig{
			Compression:  getEnvBoolOrDefault("ENABLE_COMPRESSION", true),
			Search:       getEnvBoolOrDefault("ENABLE_SEARCH", true),
			Embeddings:   getEnvBoolOrDefault("ENABLE_EMBEDDINGS", true),
			ModelTesting: getEnvBoolOrDefault("ENABLE_MODEL_TESTING", true),
		},
		ModelPresets: getModelPresets(),
	}

	// Set seed if provided
	if seedStr := os.Getenv("MODEL_SEED"); seedStr != "" {
		if seed, err := strconv.Atoi(seedStr); err == nil {
			config.Ollama.Seed = &seed
		}
	}

	GlobalConfig = config
	return config, nil
}

// SelectModel selects the optimal model based on context
func (c *Config) SelectModel(options types.CompressionOptions) string {
	// Use explicit model if specified
	if options.Model != nil {
		return *options.Model
	}

	// Use preset if specified
	if options.Preset != nil {
		if preset, exists := c.ModelPresets[*options.Preset]; exists {
			return preset.Model
		}
	}

	// Priority-based selection
	switch options.Priority {
	case "speed":
		if preset, exists := c.ModelPresets["fast"]; exists {
			return preset.Model
		}
	case "quality":
		if preset, exists := c.ModelPresets["quality"]; exists {
			return preset.Model
		}
	}

	// Type-based selection
	if options.Type == "code" {
		if preset, exists := c.ModelPresets["coding"]; exists {
			return preset.Model
		}
	}

	// Default to primary model
	return c.Ollama.PrimaryModel
}

// GetModelParams returns parameters for a specific model or preset
func (c *Config) GetModelParams(modelOrPreset string) (string, float64, int) {
	// Check if it's a preset
	if preset, exists := c.ModelPresets[modelOrPreset]; exists {
		return preset.Model, preset.Temperature, preset.MaxTokens
	}

	// Return default parameters with specified model
	return modelOrPreset, c.Ollama.Temperature, c.Ollama.MaxTokens
}

// IsDevelopment returns true if running in development mode
func (c *Config) IsDevelopment() bool {
	return c.Server.Env == "development"
}

// IsProduction returns true if running in production mode
func (c *Config) IsProduction() bool {
	return c.Server.Env == "production"
}

// Utility functions
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getEnvFloatOrDefault(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseFloat(value, 64); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getEnvBoolOrDefault(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		parsed, err := strconv.ParseBool(value)
		if err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getEnvDurationOrDefault(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if parsed, err := time.ParseDuration(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getModelPresets() map[string]types.ModelPreset {
	return map[string]types.ModelPreset{
		"fast": {
			Name:        "fast",
			Model:       "gemma2:2b",
			Temperature: 0.3,
			MaxTokens:   1500,
			Description: "Fast processing with good quality",
		},
		"balanced": {
			Name:        "balanced",
			Model:       "llama3.2:3b",
			Temperature: 0.3,
			MaxTokens:   2000,
			Description: "Balanced speed and quality",
		},
		"quality": {
			Name:        "quality",
			Model:       "mistral:7b",
			Temperature: 0.2,
			MaxTokens:   3000,
			Description: "High quality output, slower processing",
		},
		"coding": {
			Name:        "coding",
			Model:       "qwen2.5:3b",
			Temperature: 0.2,
			MaxTokens:   2500,
			Description: "Optimized for code and technical content",
		},
		"tiny": {
			Name:        "tiny",
			Model:       "gemma2:2b",
			Temperature: 0.4,
			MaxTokens:   1000,
			Description: "Minimal resource usage",
		},
	}
}