package security

import (
	"encoding/json"
	"fmt"
	"net/mail"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// InputValidator handles input validation and sanitization
type InputValidator struct {
	logger *logrus.Logger
}

// ValidationRule defines a validation rule
type ValidationRule struct {
	Field    string
	Required bool
	Type     string
	MinLen   int
	MaxLen   int
	Pattern  *regexp.Regexp
	Custom   func(interface{}) error
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Value   string `json:"value,omitempty"`
}

// ValidationResult contains validation results
type ValidationResult struct {
	Valid  bool              `json:"valid"`
	Errors []ValidationError `json:"errors"`
}

// NewInputValidator creates a new input validator
func NewInputValidator(logger *logrus.Logger) *InputValidator {
	return &InputValidator{
		logger: logger,
	}
}

// ValidateJSON validates JSON input against rules
func (iv *InputValidator) ValidateJSON(c *gin.Context, rules []ValidationRule) (*ValidationResult, error) {
	var data map[string]interface{}
	
	if err := c.ShouldBindJSON(&data); err != nil {
		return &ValidationResult{
			Valid: false,
			Errors: []ValidationError{
				{
					Field:   "json",
					Message: "Invalid JSON format",
				},
			},
		}, err
	}

	return iv.ValidateData(data, rules), nil
}

// ValidateData validates data against rules
func (iv *InputValidator) ValidateData(data map[string]interface{}, rules []ValidationRule) *ValidationResult {
	result := &ValidationResult{
		Valid:  true,
		Errors: []ValidationError{},
	}

	for _, rule := range rules {
		if err := iv.validateField(data, rule); err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, *err)
		}
	}

	if !result.Valid {
		iv.logger.WithFields(logrus.Fields{
			"error_count": len(result.Errors),
			"fields":      iv.getErrorFields(result.Errors),
		}).Warn("Input validation failed")
	}

	return result
}

// validateField validates a single field
func (iv *InputValidator) validateField(data map[string]interface{}, rule ValidationRule) *ValidationError {
	value, exists := data[rule.Field]

	// Check required fields
	if rule.Required && (!exists || value == nil || value == "") {
		return &ValidationError{
			Field:   rule.Field,
			Message: "This field is required",
		}
	}

	// Skip validation if field doesn't exist and is not required
	if !exists || value == nil {
		return nil
	}

	// Convert value to string for validation
	valueStr := fmt.Sprintf("%v", value)

	// Type validation
	if err := iv.validateType(value, rule.Type); err != nil {
		return &ValidationError{
			Field:   rule.Field,
			Message: err.Error(),
			Value:   iv.truncateValue(valueStr),
		}
	}

	// Length validation
	if rule.MinLen > 0 && len(valueStr) < rule.MinLen {
		return &ValidationError{
			Field:   rule.Field,
			Message: fmt.Sprintf("Minimum length is %d characters", rule.MinLen),
			Value:   iv.truncateValue(valueStr),
		}
	}

	if rule.MaxLen > 0 && len(valueStr) > rule.MaxLen {
		return &ValidationError{
			Field:   rule.Field,
			Message: fmt.Sprintf("Maximum length is %d characters", rule.MaxLen),
			Value:   iv.truncateValue(valueStr),
		}
	}

	// Pattern validation
	if rule.Pattern != nil && !rule.Pattern.MatchString(valueStr) {
		return &ValidationError{
			Field:   rule.Field,
			Message: "Invalid format",
			Value:   iv.truncateValue(valueStr),
		}
	}

	// Custom validation
	if rule.Custom != nil {
		if err := rule.Custom(value); err != nil {
			return &ValidationError{
				Field:   rule.Field,
				Message: err.Error(),
				Value:   iv.truncateValue(valueStr),
			}
		}
	}

	return nil
}

// validateType validates the type of a value
func (iv *InputValidator) validateType(value interface{}, expectedType string) error {
	switch expectedType {
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("must be a string")
		}
	case "int":
		switch v := value.(type) {
		case int, int32, int64:
			return nil
		case float64:
			if v == float64(int(v)) {
				return nil
			}
			return fmt.Errorf("must be an integer")
		case string:
			if _, err := strconv.Atoi(v); err != nil {
				return fmt.Errorf("must be an integer")
			}
		default:
			return fmt.Errorf("must be an integer")
		}
	case "float":
		switch value.(type) {
		case int, int32, int64, float32, float64:
			return nil
		case string:
			if _, err := strconv.ParseFloat(value.(string), 64); err != nil {
				return fmt.Errorf("must be a number")
			}
		default:
			return fmt.Errorf("must be a number")
		}
	case "bool":
		switch value.(type) {
		case bool:
			return nil
		case string:
			if _, err := strconv.ParseBool(value.(string)); err != nil {
				return fmt.Errorf("must be a boolean")
			}
		default:
			return fmt.Errorf("must be a boolean")
		}
	case "email":
		if str, ok := value.(string); ok {
			if _, err := mail.ParseAddress(str); err != nil {
				return fmt.Errorf("must be a valid email address")
			}
		} else {
			return fmt.Errorf("must be a string")
		}
	case "url":
		if str, ok := value.(string); ok {
			if _, err := url.ParseRequestURI(str); err != nil {
				return fmt.Errorf("must be a valid URL")
			}
		} else {
			return fmt.Errorf("must be a string")
		}
	case "datetime":
		if str, ok := value.(string); ok {
			if _, err := time.Parse(time.RFC3339, str); err != nil {
				return fmt.Errorf("must be a valid datetime in RFC3339 format")
			}
		} else {
			return fmt.Errorf("must be a string")
		}
	}

	return nil
}

// SanitizeInput sanitizes user input to prevent XSS and other attacks
func (iv *InputValidator) SanitizeInput(input string) string {
	// Remove null bytes
	sanitized := strings.ReplaceAll(input, "\x00", "")
	
	// Remove or escape potentially dangerous characters
	sanitized = strings.ReplaceAll(sanitized, "<script", "&lt;script")
	sanitized = strings.ReplaceAll(sanitized, "</script", "&lt;/script")
	sanitized = strings.ReplaceAll(sanitized, "javascript:", "")
	sanitized = strings.ReplaceAll(sanitized, "vbscript:", "")
	sanitized = strings.ReplaceAll(sanitized, "onload=", "")
	sanitized = strings.ReplaceAll(sanitized, "onerror=", "")
	
	// Trim whitespace
	sanitized = strings.TrimSpace(sanitized)
	
	return sanitized
}

// ValidateSessionRequest validates session-related requests
func (iv *InputValidator) ValidateSessionRequest() []ValidationRule {
	return []ValidationRule{
		{
			Field:    "content",
			Required: true,
			Type:     "string",
			MinLen:   1,
			MaxLen:   1000000, // 1MB limit
		},
		{
			Field:    "context",
			Required: false,
			Type:     "string",
			MaxLen:   10000,
		},
		{
			Field:    "model",
			Required: false,
			Type:     "string",
			MaxLen:   100,
			Pattern:  regexp.MustCompile(`^[a-zA-Z0-9\-_:\.]+$`),
		},
		{
			Field:    "session_id",
			Required: false,
			Type:     "string",
			MaxLen:   100,
			Pattern:  regexp.MustCompile(`^[a-zA-Z0-9\-_]+$`),
		},
	}
}

// ValidateSearchRequest validates search requests
func (iv *InputValidator) ValidateSearchRequest() []ValidationRule {
	return []ValidationRule{
		{
			Field:    "query",
			Required: true,
			Type:     "string",
			MinLen:   1,
			MaxLen:   1000,
		},
		{
			Field:    "limit",
			Required: false,
			Type:     "int",
			Custom: func(value interface{}) error {
				if val, ok := value.(float64); ok {
					if val < 1 || val > 100 {
						return fmt.Errorf("must be between 1 and 100")
					}
				}
				return nil
			},
		},
		{
			Field:    "filters",
			Required: false,
			Type:     "string",
			MaxLen:   500,
		},
	}
}

// ValidateBackupRequest validates backup requests
func (iv *InputValidator) ValidateBackupRequest() []ValidationRule {
	return []ValidationRule{
		{
			Field:    "type",
			Required: false,
			Type:     "string",
			Pattern:  regexp.MustCompile(`^(manual|scheduled|automatic)$`),
		},
		{
			Field:    "description",
			Required: false,
			Type:     "string",
			MaxLen:   500,
		},
		{
			Field:    "confirm",
			Required: false,
			Type:     "bool",
		},
	}
}

// ValidateAPIKeyRequest validates API key creation requests
func (iv *InputValidator) ValidateAPIKeyRequest() []ValidationRule {
	return []ValidationRule{
		{
			Field:    "name",
			Required: true,
			Type:     "string",
			MinLen:   3,
			MaxLen:   50,
			Pattern:  regexp.MustCompile(`^[a-zA-Z0-9\-_]+$`),
		},
		{
			Field:    "permissions",
			Required: true,
			Custom: func(value interface{}) error {
				// Validate that permissions is an array of strings
				switch v := value.(type) {
				case []interface{}:
					for _, perm := range v {
						if _, ok := perm.(string); !ok {
							return fmt.Errorf("permissions must be an array of strings")
						}
					}
					if len(v) == 0 {
						return fmt.Errorf("at least one permission is required")
					}
				default:
					return fmt.Errorf("permissions must be an array")
				}
				return nil
			},
		},
		{
			Field:    "rate_limit",
			Required: false,
			Type:     "int",
			Custom: func(value interface{}) error {
				if val, ok := value.(float64); ok {
					if val < 1 || val > 10000 {
						return fmt.Errorf("must be between 1 and 10000")
					}
				}
				return nil
			},
		},
		{
			Field:    "expires_in_days",
			Required: false,
			Type:     "int",
			Custom: func(value interface{}) error {
				if val, ok := value.(float64); ok {
					if val < 1 || val > 365 {
						return fmt.Errorf("must be between 1 and 365 days")
					}
				}
				return nil
			},
		},
	}
}

// CreateValidationMiddleware creates middleware for input validation
func (iv *InputValidator) CreateValidationMiddleware(getRules func() []ValidationRule) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		if c.Request.Method == "GET" || c.Request.Method == "DELETE" {
			c.Next()
			return
		}

		rules := getRules()
		result, err := iv.ValidateJSON(c, rules)
		
		if err != nil {
			iv.logger.WithError(err).Error("JSON parsing failed")
			c.JSON(400, gin.H{
				"error":     "Invalid JSON",
				"message":   "Request body must be valid JSON",
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})
			c.Abort()
			return
		}

		if !result.Valid {
			c.JSON(400, gin.H{
				"error":      "Validation failed",
				"message":    "Request validation failed",
				"errors":     result.Errors,
				"timestamp":  time.Now().UTC().Format(time.RFC3339),
			})
			c.Abort()
			return
		}

		c.Next()
	})
}

// Helper functions

func (iv *InputValidator) truncateValue(value string) string {
	if len(value) > 50 {
		return value[:50] + "..."
	}
	return value
}

func (iv *InputValidator) getErrorFields(errors []ValidationError) []string {
	fields := make([]string, len(errors))
	for i, err := range errors {
		fields[i] = err.Field
	}
	return fields
}

// ValidateStruct validates a struct using reflection and tags
func (iv *InputValidator) ValidateStruct(s interface{}) *ValidationResult {
	result := &ValidationResult{
		Valid:  true,
		Errors: []ValidationError{},
	}

	v := reflect.ValueOf(s)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "root",
			Message: "Value must be a struct",
		})
		return result
	}

	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		value := v.Field(i)

		// Skip unexported fields
		if !value.CanInterface() {
			continue
		}

		// Check validation tags
		if err := iv.validateStructField(field, value); err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, *err)
		}
	}

	return result
}

func (iv *InputValidator) validateStructField(field reflect.StructField, value reflect.Value) *ValidationError {
	// This is a simplified implementation
	// In a real scenario, you would parse struct tags for validation rules
	
	// Example: required validation
	if field.Tag.Get("required") == "true" {
		if value.Kind() == reflect.String && value.String() == "" {
			return &ValidationError{
				Field:   field.Name,
				Message: "This field is required",
			}
		}
	}

	return nil
}