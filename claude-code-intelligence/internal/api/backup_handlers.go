package api

import (
	"net/http"
	"time"

	"claude-code-intelligence/internal/backup"

	"github.com/gin-gonic/gin"
)

// BackupHandlers contains handlers for backup operations
type BackupHandlers struct {
	backupManager *backup.BackupManager
	logger        loggerInterface
}

// NewBackupHandlers creates new backup handlers
func NewBackupHandlers(backupManager *backup.BackupManager, logger loggerInterface) *BackupHandlers {
	return &BackupHandlers{
		backupManager: backupManager,
		logger:        logger,
	}
}

// CreateBackup creates a new database backup
func (bh *BackupHandlers) CreateBackup(c *gin.Context) {
	var request struct {
		Type        string `json:"type"`        // manual, scheduled
		Description string `json:"description"` // Optional description
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		bh.errorResponse(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// Set default type
	if request.Type == "" {
		request.Type = "manual"
	}

	// Validate backup type
	validTypes := map[string]bool{"manual": true, "scheduled": true, "automatic": true}
	if !validTypes[request.Type] {
		bh.errorResponse(c, http.StatusBadRequest, "Invalid backup type", nil)
		return
	}

	bh.logger.WithFields(map[string]interface{}{
		"type":        request.Type,
		"description": request.Description,
		"endpoint":    "create_backup",
	}).Info("Creating backup")

	// Create backup
	result, err := bh.backupManager.CreateBackup(c.Request.Context(), request.Type, request.Description)
	if err != nil {
		bh.errorResponse(c, http.StatusInternalServerError, "Failed to create backup", err)
		return
	}

	if !result.Success {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   result.Error,
			"message": result.Message,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"message":     result.Message,
		"backup":      result.BackupInfo,
		"duration_ms": result.Duration.Milliseconds(),
		"created_at":  time.Now().UTC().Format(time.RFC3339),
	})
}

// ListBackups lists all available backups
func (bh *BackupHandlers) ListBackups(c *gin.Context) {
	bh.logger.WithFields(map[string]interface{}{
		"endpoint": "list_backups",
	}).Debug("Listing backups")

	backups, err := bh.backupManager.ListBackups()
	if err != nil {
		bh.errorResponse(c, http.StatusInternalServerError, "Failed to list backups", err)
		return
	}

	// Calculate total size
	var totalSize int64
	for _, backup := range backups {
		totalSize += backup.Size
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"count":   len(backups),
		"backups": backups,
		"summary": gin.H{
			"total_count":     len(backups),
			"total_size_bytes": totalSize,
			"total_size_mb":    float64(totalSize) / 1024 / 1024,
		},
		"retrieved_at": time.Now().UTC().Format(time.RFC3339),
	})
}

// RestoreBackup restores database from a backup
func (bh *BackupHandlers) RestoreBackup(c *gin.Context) {
	var request struct {
		BackupFilename string `json:"backup_filename" binding:"required"`
		Confirm        bool   `json:"confirm" binding:"required"` // Safety check
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		bh.errorResponse(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// Safety check
	if !request.Confirm {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Confirmation required",
			"message": "Database restore is a destructive operation. Please set 'confirm' to true.",
		})
		return
	}

	bh.logger.WithFields(map[string]interface{}{
		"backup_filename": request.BackupFilename,
		"endpoint":        "restore_backup",
	}).Warn("Starting database restore")

	// Perform restore
	result, err := bh.backupManager.RestoreFromBackup(c.Request.Context(), request.BackupFilename)
	if err != nil {
		bh.errorResponse(c, http.StatusInternalServerError, "Failed to restore backup", err)
		return
	}

	if !result.Success {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   result.Error,
			"message": result.Message,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"message":     result.Message,
		"backup_info": result.BackupInfo,
		"duration_ms": result.Duration.Milliseconds(),
		"restored_at": time.Now().UTC().Format(time.RFC3339),
	})
}

// DeleteBackup deletes a specific backup
func (bh *BackupHandlers) DeleteBackup(c *gin.Context) {
	backupFilename := c.Param("filename")
	if backupFilename == "" {
		bh.errorResponse(c, http.StatusBadRequest, "Backup filename is required", nil)
		return
	}

	var request struct {
		Confirm bool `json:"confirm" binding:"required"` // Safety check
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		bh.errorResponse(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// Safety check
	if !request.Confirm {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Confirmation required",
			"message": "Backup deletion is irreversible. Please set 'confirm' to true.",
		})
		return
	}

	bh.logger.WithFields(map[string]interface{}{
		"backup_filename": backupFilename,
		"endpoint":        "delete_backup",
	}).Warn("Deleting backup")

	// Delete backup
	if err := bh.backupManager.DeleteBackup(backupFilename); err != nil {
		bh.errorResponse(c, http.StatusInternalServerError, "Failed to delete backup", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    "Backup deleted successfully",
		"filename":   backupFilename,
		"deleted_at": time.Now().UTC().Format(time.RFC3339),
	})
}

// GetBackupInfo gets information about a specific backup
func (bh *BackupHandlers) GetBackupInfo(c *gin.Context) {
	backupFilename := c.Param("filename")
	if backupFilename == "" {
		bh.errorResponse(c, http.StatusBadRequest, "Backup filename is required", nil)
		return
	}

	// List all backups and find the requested one
	backups, err := bh.backupManager.ListBackups()
	if err != nil {
		bh.errorResponse(c, http.StatusInternalServerError, "Failed to get backup info", err)
		return
	}

	// Find the specific backup
	var targetBackup *backup.BackupInfo
	for _, b := range backups {
		if b.Filename == backupFilename {
			targetBackup = b
			break
		}
	}

	if targetBackup == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Backup not found",
			"message": "The specified backup file was not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"backup":  targetBackup,
	})
}

// GetBackupStats returns backup statistics
func (bh *BackupHandlers) GetBackupStats(c *gin.Context) {
	backups, err := bh.backupManager.ListBackups()
	if err != nil {
		bh.errorResponse(c, http.StatusInternalServerError, "Failed to get backup stats", err)
		return
	}

	// Calculate statistics
	stats := gin.H{
		"total_count": len(backups),
		"by_type": gin.H{
			"manual":    0,
			"scheduled": 0,
			"automatic": 0,
		},
		"total_size_bytes": int64(0),
		"oldest_backup":    (*time.Time)(nil),
		"newest_backup":    (*time.Time)(nil),
		"average_size_mb":  float64(0),
	}

	if len(backups) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"stats":   stats,
		})
		return
	}

	var totalSize int64
	var oldest, newest time.Time

	for i, backup := range backups {
		// Count by type
		if typeCount, ok := stats["by_type"].(gin.H)[backup.Type]; ok {
			stats["by_type"].(gin.H)[backup.Type] = typeCount.(int) + 1
		}

		// Calculate total size
		totalSize += backup.Size

		// Track oldest and newest
		if i == 0 {
			oldest = backup.CreatedAt
			newest = backup.CreatedAt
		} else {
			if backup.CreatedAt.Before(oldest) {
				oldest = backup.CreatedAt
			}
			if backup.CreatedAt.After(newest) {
				newest = backup.CreatedAt
			}
		}
	}

	stats["total_size_bytes"] = totalSize
	stats["total_size_mb"] = float64(totalSize) / 1024 / 1024
	stats["average_size_mb"] = float64(totalSize) / float64(len(backups)) / 1024 / 1024
	stats["oldest_backup"] = oldest.UTC().Format(time.RFC3339)
	stats["newest_backup"] = newest.UTC().Format(time.RFC3339)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"stats":   stats,
		"generated_at": time.Now().UTC().Format(time.RFC3339),
	})
}

// ScheduleBackup triggers a scheduled backup
func (bh *BackupHandlers) ScheduleBackup(c *gin.Context) {
	bh.logger.WithFields(map[string]interface{}{
		"endpoint": "schedule_backup",
	}).Info("Scheduling backup")

	if err := bh.backupManager.ScheduledBackup(c.Request.Context()); err != nil {
		bh.errorResponse(c, http.StatusInternalServerError, "Failed to create scheduled backup", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"message":     "Scheduled backup created successfully",
		"scheduled_at": time.Now().UTC().Format(time.RFC3339),
	})
}

// errorResponse sends a standardized error response
func (bh *BackupHandlers) errorResponse(c *gin.Context, statusCode int, message string, err error) {
	response := gin.H{
		"success":   false,
		"message":   message,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	if err != nil {
		response["error"] = err.Error()
		bh.logger.WithFields(map[string]interface{}{
			"error":       err.Error(),
			"message":     message,
			"status_code": statusCode,
			"path":        c.Request.URL.Path,
		}).Error("Backup operation failed")
	}

	c.JSON(statusCode, response)
}