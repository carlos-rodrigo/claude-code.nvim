package ai

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"claude-code-intelligence/internal/config"
	"claude-code-intelligence/internal/types"

	"github.com/ollama/ollama/api"
	"github.com/sirupsen/logrus"
)

// OllamaClient wraps the Ollama API client with intelligent model management
type OllamaClient struct {
	client         *api.Client
	config         *config.Config
	availableModels []api.ListModelResponse
	modelMutex     sync.RWMutex
	isConnected    bool
	logger         *logrus.Logger
}

// NewOllamaClient creates a new Ollama client instance
func NewOllamaClient(cfg *config.Config, logger *logrus.Logger) *OllamaClient {
	client, err := api.ClientFromEnvironment()
	if err != nil {
		// Create client with custom host if environment setup fails
		// Parse URL and create client
		baseURL, parseErr := url.Parse(cfg.Ollama.URL)
		if parseErr != nil {
			logger.WithError(parseErr).Warn("Failed to parse Ollama URL, using default")
			client, _ = api.ClientFromEnvironment()
		} else {
			client = api.NewClient(baseURL, http.DefaultClient)
		}
	}

	return &OllamaClient{
		client:  client,
		config:  cfg,
		logger:  logger,
	}
}

// Initialize connects to Ollama and ensures required models are available
func (o *OllamaClient) Initialize(ctx context.Context) error {
	o.logger.Info("Initializing Ollama client...")

	// Test connection
	if err := o.testConnection(ctx); err != nil {
		return fmt.Errorf("failed to connect to Ollama: %w", err)
	}

	// Refresh available models
	if err := o.refreshAvailableModels(ctx); err != nil {
		return fmt.Errorf("failed to list models: %w", err)
	}

	// Ensure primary and fallback models are available
	if err := o.ensureModelsAvailable(ctx); err != nil {
		o.logger.Warn("Some models may not be available:", err)
		// Don't fail initialization - we can still try to use what's available
	}

	o.isConnected = true
	o.logger.WithField("models", len(o.availableModels)).Info("Ollama client initialized successfully")

	return nil
}

// testConnection verifies Ollama is running and accessible
func (o *OllamaClient) testConnection(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := o.client.List(ctx)
	if err != nil {
		return fmt.Errorf("Ollama is not running or accessible at %s: %w", o.config.Ollama.URL, err)
	}

	return nil
}

// refreshAvailableModels fetches the list of currently available models
func (o *OllamaClient) refreshAvailableModels(ctx context.Context) error {
	o.modelMutex.Lock()
	defer o.modelMutex.Unlock()

	resp, err := o.client.List(ctx)
	if err != nil {
		return err
	}

	o.availableModels = resp.Models
	
	modelNames := make([]string, len(o.availableModels))
	for i, model := range o.availableModels {
		modelNames[i] = model.Name
	}
	
	o.logger.WithField("models", modelNames).Debug("Available models refreshed")
	return nil
}

// ensureModelsAvailable ensures required models are installed
func (o *OllamaClient) ensureModelsAvailable(ctx context.Context) error {
	requiredModels := o.getRequiredModels()
	var errors []string

	for _, model := range requiredModels {
		if !o.isModelAvailable(model) {
			o.logger.WithField("model", model).Info("Model not found, attempting to install...")
			
			if err := o.installModel(ctx, model); err != nil {
				errors = append(errors, fmt.Sprintf("%s: %v", model, err))
				o.logger.WithField("model", model).WithError(err).Error("Failed to install model")
			} else {
				o.logger.WithField("model", model).Info("Model installed successfully")
			}
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to install some models: %s", strings.Join(errors, "; "))
	}

	// Refresh available models after installation
	return o.refreshAvailableModels(ctx)
}

// getRequiredModels returns a list of models that should be available
func (o *OllamaClient) getRequiredModels() []string {
	models := make(map[string]bool)
	
	// Add primary and fallback models
	models[o.config.Ollama.PrimaryModel] = true
	models[o.config.Ollama.FallbackModel] = true
	
	// Add models from presets
	for _, preset := range o.config.ModelPresets {
		models[preset.Model] = true
	}
	
	// Convert to slice
	result := make([]string, 0, len(models))
	for model := range models {
		result = append(result, model)
	}
	
	return result
}

// isModelAvailable checks if a model is currently available
func (o *OllamaClient) isModelAvailable(modelName string) bool {
	o.modelMutex.RLock()
	defer o.modelMutex.RUnlock()

	for _, model := range o.availableModels {
		if model.Name == modelName {
			return true
		}
	}
	return false
}

// installModel downloads and installs a model from Ollama registry
func (o *OllamaClient) installModel(ctx context.Context, modelName string) error {
	o.logger.WithField("model", modelName).Info("Starting model installation...")

	req := &api.PullRequest{
		Model:  modelName,
		Stream: &[]bool{true}[0], // Convert bool to *bool
	}

	// Create a context with timeout for model pulling
	pullCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	progressFn := func(resp api.ProgressResponse) error {
		if resp.Status != "" {
			// Log progress at different intervals based on completion
			if resp.Completed > 0 && resp.Total > 0 {
				progress := float64(resp.Completed) / float64(resp.Total) * 100
				
				// Log every 25% for major progress updates
				if int(progress)%25 == 0 {
					o.logger.WithFields(logrus.Fields{
						"model":    modelName,
						"progress": fmt.Sprintf("%.1f%%", progress),
						"status":   resp.Status,
					}).Info("Model installation progress")
				}
			} else {
				// Log status changes
				o.logger.WithFields(logrus.Fields{
					"model":  modelName,
					"status": resp.Status,
				}).Debug("Model installation status")
			}
		}
		return nil
	}

	err := o.client.Pull(pullCtx, req, progressFn)
	if err != nil {
		return fmt.Errorf("failed to pull model %s: %w", modelName, err)
	}

	o.logger.WithField("model", modelName).Info("Model installation completed")
	return nil
}

// CompressSession compresses session content using the specified model
func (o *OllamaClient) CompressSession(ctx context.Context, content string, options types.CompressionOptions) (*types.CompressionResult, error) {
	startTime := time.Now()
	
	// Select the optimal model
	model := o.config.SelectModel(options)
	
	// Ensure model is available
	if err := o.ensureModelAvailable(ctx, model); err != nil {
		if options.AllowFallback && model != o.config.Ollama.FallbackModel {
			o.logger.WithFields(logrus.Fields{
				"original_model": model,
				"fallback_model": o.config.Ollama.FallbackModel,
			}).Warn("Falling back to fallback model")
			
			model = o.config.Ollama.FallbackModel
			if err := o.ensureModelAvailable(ctx, model); err != nil {
				return nil, fmt.Errorf("fallback model also unavailable: %w", err)
			}
		} else {
			return nil, fmt.Errorf("model unavailable: %w", err)
		}
	}

	// Build the compression prompt
	prompt := o.buildCompressionPrompt(content, options)
	
	// Get model parameters
	modelName, temperature, maxTokens := o.config.GetModelParams(model)
	
	// Create the chat request
	req := &api.ChatRequest{
		Model: modelName,
		Messages: []api.Message{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Stream: &[]bool{false}[0], // Convert bool to *bool
		Options: map[string]interface{}{
			"temperature": temperature,
			"num_predict": maxTokens,
			"top_p":       o.config.Ollama.TopP,
		},
	}

	if o.config.Ollama.Seed != nil {
		req.Options["seed"] = *o.config.Ollama.Seed
	}

	// Create context with timeout
	chatCtx, cancel := context.WithTimeout(ctx, o.config.Ollama.Timeout)
	defer cancel()

	o.logger.WithFields(logrus.Fields{
		"model":       modelName,
		"content_len": len(content),
		"style":       options.Style,
	}).Debug("Starting session compression")

	// Execute the chat request
	resp := &api.ChatResponse{}
	err := o.client.Chat(chatCtx, req, func(chatResp api.ChatResponse) error {
		*resp = chatResp
		return nil
	})

	processingTime := time.Since(startTime)

	if err != nil {
		o.logger.WithFields(logrus.Fields{
			"model":           modelName,
			"processing_time": processingTime,
			"error":           err,
		}).Error("Session compression failed")
		return nil, fmt.Errorf("compression failed: %w", err)
	}

	// Calculate compression metrics
	originalSize := len(content)
	compressedSize := len(resp.Message.Content)
	compressionRatio := float64(compressedSize) / float64(originalSize)

	result := &types.CompressionResult{
		Summary:          resp.Message.Content,
		Model:            modelName,
		ProcessingTime:   processingTime,
		OriginalSize:     originalSize,
		CompressedSize:   compressedSize,
		CompressionRatio: compressionRatio,
		QualityScore:     o.estimateQuality(resp.Message.Content, content),
	}

	o.logger.WithFields(logrus.Fields{
		"model":             modelName,
		"processing_time":   processingTime,
		"compression_ratio": fmt.Sprintf("%.2f%%", (1-compressionRatio)*100),
		"quality_score":     result.QualityScore,
	}).Info("Session compression completed")

	return result, nil
}

// ensureModelAvailable ensures a specific model is available, installing if needed
func (o *OllamaClient) ensureModelAvailable(ctx context.Context, modelName string) error {
	if o.isModelAvailable(modelName) {
		return nil
	}

	o.logger.WithField("model", modelName).Info("Model not available, installing...")
	return o.installModel(ctx, modelName)
}

// ExtractTopics extracts key topics from session content
func (o *OllamaClient) ExtractTopics(ctx context.Context, content string, maxTopics int) ([]types.Topic, error) {
	model := o.config.Ollama.PrimaryModel
	
	if err := o.ensureModelAvailable(ctx, model); err != nil {
		return nil, fmt.Errorf("model unavailable for topic extraction: %w", err)
	}

	prompt := fmt.Sprintf(`Extract the %d most important topics from this technical conversation.
Return only a JSON array of objects with 'topic' and 'relevance' (0-1) fields.

Example format: [{"topic": "database optimization", "relevance": 0.9}]

Content:
%s

Topics:`, maxTopics, o.truncateContent(content, 4000))

	req := &api.ChatRequest{
		Model: model,
		Messages: []api.Message{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Stream: &[]bool{false}[0], // Convert bool to *bool
		Options: map[string]interface{}{
			"temperature": 0.1, // Low temperature for structured output
			"num_predict": 500,
		},
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp := &api.ChatResponse{}
	err := o.client.Chat(ctx, req, func(chatResp api.ChatResponse) error {
		*resp = chatResp
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("topic extraction failed: %w", err)
	}

	// Try to parse JSON response
	var rawTopics []struct {
		Topic     string  `json:"topic"`
		Relevance float64 `json:"relevance"`
	}

	if err := json.Unmarshal([]byte(resp.Message.Content), &rawTopics); err != nil {
		// Fallback to text parsing
		o.logger.Debug("Failed to parse topics as JSON, using fallback parsing")
		return o.parseTopicsFromText(resp.Message.Content), nil
	}

	topics := make([]types.Topic, len(rawTopics))
	for i, raw := range rawTopics {
		topics[i] = types.Topic{
			Topic:          raw.Topic,
			RelevanceScore: raw.Relevance,
			Frequency:      1,
		}
	}

	return topics, nil
}

// TestModels tests multiple models with sample content
func (o *OllamaClient) TestModels(ctx context.Context, testContent string, models []string) ([]types.ModelTestResult, error) {
	if models == nil {
		// Use default test models
		models = []string{"gemma2:2b", "llama3.2:3b", "mistral:7b", "qwen2.5:3b"}
	}

	results := make([]types.ModelTestResult, 0, len(models))

	for _, model := range models {
		o.logger.WithField("model", model).Info("Testing model performance")
		
		result := types.ModelTestResult{Model: model}
		startTime := time.Now()

		// Ensure model is available (this will auto-install if needed)
		if err := o.ensureModelAvailable(ctx, model); err != nil {
			result.Success = false
			errorMsg := err.Error()
			result.Error = &errorMsg
			result.ProcessingTime = time.Since(startTime)
			results = append(results, result)
			continue
		}

		// Test compression
		options := types.CompressionOptions{
			Model:         &model,
			Style:         "balanced",
			MaxLength:     2000,
			AllowFallback: false,
		}

		compressionResult, err := o.CompressSession(ctx, testContent, options)
		if err != nil {
			result.Success = false
			errorMsg := err.Error()
			result.Error = &errorMsg
		} else {
			result.Success = true
			result.CompressionRatio = compressionResult.CompressionRatio
			result.OutputLength = compressionResult.CompressedSize
			result.QualityScore = compressionResult.QualityScore
		}

		result.ProcessingTime = time.Since(startTime)
		results = append(results, result)
	}

	return results, nil
}

// Helper methods

func (o *OllamaClient) buildCompressionPrompt(content string, options types.CompressionOptions) string {
	stylePrompts := map[string]string{
		"concise":  fmt.Sprintf("Create a very concise summary (under %d words) focusing only on key decisions and outcomes.", options.MaxLength/2),
		"balanced": fmt.Sprintf("Create a comprehensive but concise summary (under %d words) preserving important context.", options.MaxLength),
		"detailed": "Create a detailed summary preserving all important information, decisions, and technical details.",
	}

	style := options.Style
	if style == "" {
		style = "balanced"
	}

	stylePrompt := stylePrompts[style]
	if stylePrompt == "" {
		stylePrompt = stylePrompts["balanced"]
	}

	return fmt.Sprintf(`%s

Focus on:
- Key decisions made and their rationale
- Technical solutions implemented
- Important code changes or configurations
- Action items and next steps
- Problems encountered and how they were solved

Session content:
%s

Summary:`, stylePrompt, content)
}

func (o *OllamaClient) parseTopicsFromText(text string) []types.Topic {
	topics := make([]types.Topic, 0)
	scanner := bufio.NewScanner(strings.NewReader(text))
	
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		
		// Simple parsing - look for numbered or bulleted lists
		if strings.Contains(line, ":") || strings.Contains(line, "-") {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				topic := strings.TrimSpace(parts[0])
				topic = strings.TrimPrefix(topic, "-")
				topic = strings.TrimSpace(strings.TrimLeft(topic, "0123456789. "))
				
				if topic != "" {
					topics = append(topics, types.Topic{
						Topic:          topic,
						RelevanceScore: 0.5,
						Frequency:      1,
					})
				}
			}
		}
	}
	
	// Limit to reasonable number
	if len(topics) > 10 {
		topics = topics[:10]
	}
	
	return topics
}

func (o *OllamaClient) estimateQuality(summary, originalContent string) float64 {
	summaryWords := len(strings.Fields(summary))
	originalWords := len(strings.Fields(originalContent))
	
	if originalWords == 0 {
		return 0
	}
	
	compressionRatio := float64(summaryWords) / float64(originalWords)
	
	score := 5.0 // Base score
	
	// Good compression ratio
	if compressionRatio > 0.1 && compressionRatio < 0.3 {
		score += 2
	}
	
	// Contains decision indicators
	if strings.Contains(strings.ToLower(summary), "decision") || 
	   strings.Contains(strings.ToLower(summary), "decided") ||
	   strings.Contains(strings.ToLower(summary), "chose") {
		score += 1
	}
	
	// Contains technical terms
	if strings.Contains(strings.ToLower(summary), "code") ||
	   strings.Contains(strings.ToLower(summary), "function") ||
	   strings.Contains(strings.ToLower(summary), "error") {
		score += 1
	}
	
	// Has structure (lists, etc.)
	if strings.Contains(summary, "1.") || strings.Contains(summary, "-") {
		score += 1
	}
	
	// Normalize to 0-10 scale
	if score > 10 {
		score = 10
	}
	
	return score
}

func (o *OllamaClient) truncateContent(content string, maxLength int) string {
	if len(content) <= maxLength {
		return content
	}
	return content[:maxLength] + "..."
}

// GetAvailableModels returns the list of currently available models
func (o *OllamaClient) GetAvailableModels() []api.ListModelResponse {
	o.modelMutex.RLock()
	defer o.modelMutex.RUnlock()
	
	// Return a copy to avoid race conditions
	result := make([]api.ListModelResponse, len(o.availableModels))
	copy(result, o.availableModels)
	return result
}

// HealthCheck performs a health check on the Ollama service
func (o *OllamaClient) HealthCheck(ctx context.Context) types.ComponentHealth {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := o.testConnection(ctx); err != nil {
		return types.ComponentHealth{
			Status:    "unhealthy",
			Message:   err.Error(),
			LastCheck: time.Now(),
		}
	}

	return types.ComponentHealth{
		Status:    "healthy",
		Message:   fmt.Sprintf("Connected to Ollama with %d models available", len(o.availableModels)),
		LastCheck: time.Now(),
	}
}