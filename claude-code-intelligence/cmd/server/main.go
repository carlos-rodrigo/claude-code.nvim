package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"claude-code-intelligence/internal/ai"
	"claude-code-intelligence/internal/api"
	"claude-code-intelligence/internal/cache"
	"claude-code-intelligence/internal/config"
	"claude-code-intelligence/internal/database"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logrus.WithError(err).Fatal("Failed to load configuration")
	}

	// Set up logger
	logger := setupLogger(cfg)
	logger.WithField("config", cfg.Server).Info("Starting claude-code-intelligence service")

	// Initialize database
	db := database.NewManager(cfg, logger)
	if err := db.Initialize(context.Background()); err != nil {
		logger.WithError(err).Fatal("Failed to initialize database")
	}
	defer db.Close()

	// Initialize Ollama client
	ollama := ai.NewOllamaClient(cfg, logger)
	if err := ollama.Initialize(context.Background()); err != nil {
		logger.WithError(err).Fatal("Failed to initialize Ollama client")
	}

	// Initialize Phase 2 components
	contextBuilder := ai.NewContextBuilder(db, ollama, logger)
	memorySystem := ai.NewMemorySystem(db, ollama, logger)
	cacheConfig := &cache.CacheConfig{
		MemoryCacheSize: 1000,
		DiskCacheSize:   100 * 1024 * 1024, // 100MB
		DefaultTTL:      15 * time.Minute,
		EvictionPolicy:  "LRU",
		CachePath:       "./data/cache",
	}
	cacheManager := cache.NewCacheManager(cacheConfig, logger)

	// Create HTTP server with advanced features
	server := setupServer(cfg, db, ollama, contextBuilder, memorySystem, cacheManager, logger)

	// Start server
	go func() {
		addr := fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port)
		logger.WithField("address", addr).Info("Starting HTTP server")
		
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.WithError(err).Fatal("Failed to start server")
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.WithError(err).Fatal("Server forced to shutdown")
	}

	logger.Info("Server exited")
}

func setupLogger(cfg *config.Config) *logrus.Logger {
	logger := logrus.New()

	// Set log level
	level, err := logrus.ParseLevel(cfg.Logging.Level)
	if err != nil {
		level = logrus.InfoLevel
	}
	logger.SetLevel(level)

	// Set log format
	if cfg.Logging.Format == "json" {
		logger.SetFormatter(&logrus.JSONFormatter{})
	} else {
		logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
		})
	}

	// Set log output
	if cfg.Logging.File != "" {
		// Create logs directory
		if err := os.MkdirAll("logs", 0755); err == nil {
			if file, err := os.OpenFile(cfg.Logging.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666); err == nil {
				logger.SetOutput(file)
			}
		}
	}

	return logger
}

func setupServer(cfg *config.Config, db *database.Manager, ollama *ai.OllamaClient, contextBuilder *ai.ContextBuilder, memorySystem *ai.MemorySystem, cacheManager *cache.CacheManager, logger *logrus.Logger) *http.Server {
	// Set Gin mode
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	// Create Gin router
	r := gin.New()

	// Add middleware
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	// CORS configuration
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowOrigins = cfg.Security.CORSOrigins
	corsConfig.AllowMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}
	corsConfig.AllowHeaders = []string{"Origin", "Content-Length", "Content-Type", "Authorization"}
	r.Use(cors.New(corsConfig))

	// Rate limiting middleware (simple implementation)
	r.Use(rateLimitMiddleware(cfg.Security.RateLimitRPS))

	// Create handlers
	handlers := api.NewHandlers(db, ollama, cfg, logger)
	advancedHandlers := api.NewAdvancedHandlers(handlers, contextBuilder, memorySystem, cacheManager)

	// Health check routes
	r.GET("/health", handlers.HealthCheck)
	r.GET("/api/health", handlers.HealthCheck)

	// API routes
	v1 := r.Group("/api/v1")
	{
		// Session management
		sessions := v1.Group("/sessions")
		{
			sessions.POST("", handlers.CreateSession)
			sessions.GET("", handlers.ListSessions)
			sessions.GET("/:id", handlers.GetSession)
			sessions.POST("/compress", handlers.CompressSession)
			sessions.POST("/search", handlers.SearchSessions)
		}

		// AI operations
		ai := v1.Group("/ai")
		{
			ai.POST("/compress", handlers.CompressSession)
			ai.POST("/extract-topics", handlers.ExtractTopics)
			ai.POST("/test-models", handlers.TestModels)
		}

		// Model management
		models := v1.Group("/models")
		{
			models.GET("", handlers.GetAvailableModels)
			models.POST("/:model/install", handlers.InstallModel)
		}

		// Service information
		info := v1.Group("/info")
		{
			info.GET("/config", handlers.GetConfig)
			info.GET("/stats", handlers.GetStats)
		}

		// Phase 2: Advanced AI Features
		context := v1.Group("/context")
		{
			context.POST("/build", advancedHandlers.BuildContext)
			context.POST("/restore/:id", advancedHandlers.RestoreSession)
		}

		// Memory system
		memory := v1.Group("/memory")
		{
			memory.POST("/consolidate/:id", advancedHandlers.ConsolidateProjectMemory)
			memory.GET("/:id", advancedHandlers.GetProjectMemory)
		}

		// Advanced search
		search := v1.Group("/search")
		{
			search.POST("/advanced", advancedHandlers.AdvancedSearch)
		}

		// Analytics
		analytics := v1.Group("/analytics")
		{
			analytics.GET("/sessions", advancedHandlers.GetSessionAnalytics)
			analytics.GET("/timeline/:id", advancedHandlers.GetProjectTimeline)
			analytics.GET("/relationships/:id", advancedHandlers.GetSessionRelationships)
		}

		// Visualization
		visualization := v1.Group("/visualization")
		{
			visualization.GET("/session/:id", advancedHandlers.GetSessionVisualization)
			visualization.GET("/project/:id/graph", advancedHandlers.GetProjectGraph)
			visualization.GET("/project/:id/heatmap", advancedHandlers.GetProjectHeatmap)
			visualization.GET("/flow/:id", advancedHandlers.GetWorkflowFlow)
		}

		// Cache management
		cache := v1.Group("/cache")
		{
			cache.GET("/stats", advancedHandlers.GetCacheStats)
			cache.DELETE("/clear", advancedHandlers.ClearCache)
		}
	}

	// Create HTTP server
	return &http.Server{
		Addr:         fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port),
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}

// Simple rate limiting middleware
func rateLimitMiddleware(rps int) gin.HandlerFunc {
	// This is a simple implementation - in production you might want to use
	// a more sophisticated rate limiter like github.com/ulule/limiter
	requests := make(map[string][]time.Time)
	
	return func(c *gin.Context) {
		if rps <= 0 {
			c.Next()
			return
		}

		clientIP := c.ClientIP()
		now := time.Now()
		windowStart := now.Add(-time.Second)

		// Clean old requests
		if clientRequests, exists := requests[clientIP]; exists {
			var validRequests []time.Time
			for _, reqTime := range clientRequests {
				if reqTime.After(windowStart) {
					validRequests = append(validRequests, reqTime)
				}
			}
			requests[clientIP] = validRequests
		}

		// Check rate limit
		if len(requests[clientIP]) >= rps {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Rate limit exceeded",
			})
			c.Abort()
			return
		}

		// Add current request
		requests[clientIP] = append(requests[clientIP], now)
		c.Next()
	}
}