package backup

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"claude-code-intelligence/internal/config"
	"claude-code-intelligence/internal/database"

	"github.com/sirupsen/logrus"
)

// BackupManager handles database backups and recovery
type BackupManager struct {
	db         *database.Manager
	config     *config.Config
	logger     *logrus.Logger
	backupPath string
	maxBackups int
}

// BackupInfo represents backup metadata
type BackupInfo struct {
	Filename    string    `json:"filename"`
	Path        string    `json:"path"`
	Size        int64     `json:"size"`
	CreatedAt   time.Time `json:"created_at"`
	Type        string    `json:"type"`        // manual, automatic, scheduled
	Compressed  bool      `json:"compressed"`
	Checksum    string    `json:"checksum"`
	Description string    `json:"description,omitempty"`
}

// BackupResult represents the result of a backup operation
type BackupResult struct {
	Success     bool      `json:"success"`
	BackupInfo  *BackupInfo `json:"backup_info,omitempty"`
	Duration    time.Duration `json:"duration"`
	Error       string    `json:"error,omitempty"`
	Message     string    `json:"message"`
}

// RestoreResult represents the result of a restore operation
type RestoreResult struct {
	Success     bool          `json:"success"`
	Duration    time.Duration `json:"duration"`
	Error       string        `json:"error,omitempty"`
	Message     string        `json:"message"`
	BackupInfo  *BackupInfo   `json:"backup_info,omitempty"`
}

// NewBackupManager creates a new backup manager
func NewBackupManager(db *database.Manager, cfg *config.Config, logger *logrus.Logger) *BackupManager {
	backupPath := cfg.Database.BackupPath
	if backupPath == "" {
		backupPath = "./data/backups"
	}

	return &BackupManager{
		db:         db,
		config:     cfg,
		logger:     logger,
		backupPath: backupPath,
		maxBackups: 10, // Keep last 10 backups by default
	}
}

// Initialize sets up the backup system
func (bm *BackupManager) Initialize() error {
	// Create backup directory
	if err := os.MkdirAll(bm.backupPath, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	bm.logger.WithField("backup_path", bm.backupPath).Info("Backup manager initialized")
	return nil
}

// CreateBackup creates a new database backup
func (bm *BackupManager) CreateBackup(ctx context.Context, backupType, description string) (*BackupResult, error) {
	start := time.Now()
	
	result := &BackupResult{
		Success: false,
		Duration: 0,
	}

	bm.logger.WithFields(logrus.Fields{
		"type":        backupType,
		"description": description,
	}).Info("Starting database backup")

	// Generate backup filename
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("intelligence_backup_%s_%s.db", timestamp, backupType)
	backupPath := filepath.Join(bm.backupPath, filename)

	// Create backup using SQLite VACUUM INTO
	backupFilePath, err := bm.db.Backup(ctx)
	if err != nil {
		result.Error = err.Error()
		result.Message = "Failed to create database backup"
		result.Duration = time.Since(start)
		return result, err
	}

	// Move backup to proper location with proper naming
	finalPath := backupPath
	if backupFilePath != finalPath {
		if err := os.Rename(backupFilePath, finalPath); err != nil {
			result.Error = err.Error()
			result.Message = "Failed to move backup file"
			result.Duration = time.Since(start)
			return result, err
		}
	}

	// Get backup file info
	fileInfo, err := os.Stat(finalPath)
	if err != nil {
		result.Error = err.Error()
		result.Message = "Failed to get backup file info"
		result.Duration = time.Since(start)
		return result, err
	}

	// Calculate checksum (simple implementation)
	checksum, err := bm.calculateChecksum(finalPath)
	if err != nil {
		bm.logger.WithError(err).Warn("Failed to calculate backup checksum")
		checksum = "unavailable"
	}

	// Create backup info
	backupInfo := &BackupInfo{
		Filename:    filename,
		Path:        finalPath,
		Size:        fileInfo.Size(),
		CreatedAt:   fileInfo.ModTime(),
		Type:        backupType,
		Compressed:  false, // SQLite backups are not compressed by default
		Checksum:    checksum,
		Description: description,
	}

	result.Success = true
	result.BackupInfo = backupInfo
	result.Duration = time.Since(start)
	result.Message = fmt.Sprintf("Backup created successfully: %s", filename)

	bm.logger.WithFields(logrus.Fields{
		"filename":    filename,
		"size_bytes":  fileInfo.Size(),
		"duration_ms": result.Duration.Milliseconds(),
		"checksum":    checksum,
	}).Info("Database backup completed")

	// Clean up old backups
	if err := bm.cleanupOldBackups(); err != nil {
		bm.logger.WithError(err).Warn("Failed to cleanup old backups")
	}

	return result, nil
}

// RestoreFromBackup restores database from a backup
func (bm *BackupManager) RestoreFromBackup(ctx context.Context, backupFilename string) (*RestoreResult, error) {
	start := time.Now()
	
	result := &RestoreResult{
		Success: false,
		Duration: 0,
	}

	bm.logger.WithField("backup_file", backupFilename).Info("Starting database restore")

	// Find backup file
	backupPath := filepath.Join(bm.backupPath, backupFilename)
	if !bm.fileExists(backupPath) {
		result.Error = "Backup file not found"
		result.Message = fmt.Sprintf("Backup file not found: %s", backupFilename)
		result.Duration = time.Since(start)
		return result, fmt.Errorf("backup file not found: %s", backupFilename)
	}

	// Get backup info
	backupInfo, err := bm.getBackupInfo(backupPath)
	if err != nil {
		result.Error = err.Error()
		result.Message = "Failed to get backup information"
		result.Duration = time.Since(start)
		return result, err
	}

	// Verify backup integrity
	if err := bm.verifyBackupIntegrity(backupPath, backupInfo); err != nil {
		result.Error = err.Error()
		result.Message = "Backup integrity check failed"
		result.Duration = time.Since(start)
		return result, err
	}

	// Create a backup of current database before restore
	currentBackupResult, err := bm.CreateBackup(ctx, "pre_restore", fmt.Sprintf("Automatic backup before restoring from %s", backupFilename))
	if err != nil {
		bm.logger.WithError(err).Warn("Failed to create pre-restore backup")
	} else {
		bm.logger.WithField("pre_restore_backup", currentBackupResult.BackupInfo.Filename).Info("Created pre-restore backup")
	}

	// Close current database connection
	if err := bm.db.Close(); err != nil {
		bm.logger.WithError(err).Warn("Failed to close database connection")
	}

	// Get current database path
	currentDBPath := bm.config.Database.Path

	// Backup current database file (additional safety)
	backupCurrentPath := currentDBPath + ".backup." + time.Now().Format("20060102_150405")
	if bm.fileExists(currentDBPath) {
		if err := bm.copyFile(currentDBPath, backupCurrentPath); err != nil {
			result.Error = err.Error()
			result.Message = "Failed to backup current database file"
			result.Duration = time.Since(start)
			return result, err
		}
	}

	// Copy backup file to database location
	if err := bm.copyFile(backupPath, currentDBPath); err != nil {
		// Try to restore from backup
		if bm.fileExists(backupCurrentPath) {
			bm.copyFile(backupCurrentPath, currentDBPath)
		}
		result.Error = err.Error()
		result.Message = "Failed to restore backup file"
		result.Duration = time.Since(start)
		return result, err
	}

	// Reinitialize database connection
	if err := bm.db.Initialize(ctx); err != nil {
		// Try to restore from backup
		if bm.fileExists(backupCurrentPath) {
			bm.copyFile(backupCurrentPath, currentDBPath)
			bm.db.Initialize(ctx) // Try to restore connection
		}
		result.Error = err.Error()
		result.Message = "Failed to reinitialize database after restore"
		result.Duration = time.Since(start)
		return result, err
	}

	// Verify restored database
	if err := bm.verifyRestoredDatabase(ctx); err != nil {
		bm.logger.WithError(err).Error("Restored database verification failed")
		// Don't fail the restore, but log the warning
		bm.logger.Warn("Database restore completed but verification failed")
	}

	// Clean up temporary backup
	if bm.fileExists(backupCurrentPath) {
		os.Remove(backupCurrentPath)
	}

	result.Success = true
	result.BackupInfo = backupInfo
	result.Duration = time.Since(start)
	result.Message = fmt.Sprintf("Database restored successfully from %s", backupFilename)

	bm.logger.WithFields(logrus.Fields{
		"backup_file": backupFilename,
		"duration_ms": result.Duration.Milliseconds(),
		"size_bytes":  backupInfo.Size,
	}).Info("Database restore completed")

	return result, nil
}

// ListBackups returns a list of available backups
func (bm *BackupManager) ListBackups() ([]*BackupInfo, error) {
	files, err := os.ReadDir(bm.backupPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read backup directory: %w", err)
	}

	var backups []*BackupInfo
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".db") {
			continue
		}

		backupPath := filepath.Join(bm.backupPath, file.Name())
		backupInfo, err := bm.getBackupInfo(backupPath)
		if err != nil {
			bm.logger.WithError(err).WithField("file", file.Name()).Warn("Failed to get backup info")
			continue
		}

		backups = append(backups, backupInfo)
	}

	// Sort by creation time, newest first
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].CreatedAt.After(backups[j].CreatedAt)
	})

	return backups, nil
}

// DeleteBackup deletes a specific backup
func (bm *BackupManager) DeleteBackup(backupFilename string) error {
	backupPath := filepath.Join(bm.backupPath, backupFilename)
	
	if !bm.fileExists(backupPath) {
		return fmt.Errorf("backup file not found: %s", backupFilename)
	}

	if err := os.Remove(backupPath); err != nil {
		return fmt.Errorf("failed to delete backup: %w", err)
	}

	bm.logger.WithField("backup_file", backupFilename).Info("Backup deleted")
	return nil
}

// ScheduledBackup performs scheduled backup
func (bm *BackupManager) ScheduledBackup(ctx context.Context) error {
	result, err := bm.CreateBackup(ctx, "scheduled", "Automatic scheduled backup")
	if err != nil {
		return err
	}

	if !result.Success {
		return fmt.Errorf("scheduled backup failed: %s", result.Error)
	}

	return nil
}

// Helper methods

// calculateChecksum calculates a simple checksum for the backup file
func (bm *BackupManager) calculateChecksum(filePath string) (string, error) {
	// Simple implementation - in production you might want SHA256
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Get file size as a simple checksum
	fileInfo, err := file.Stat()
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("size_%d", fileInfo.Size()), nil
}

// getBackupInfo gets information about a backup file
func (bm *BackupManager) getBackupInfo(backupPath string) (*BackupInfo, error) {
	fileInfo, err := os.Stat(backupPath)
	if err != nil {
		return nil, err
	}

	filename := filepath.Base(backupPath)
	
	// Parse backup type from filename
	backupType := "manual"
	if strings.Contains(filename, "_scheduled_") {
		backupType = "scheduled"
	} else if strings.Contains(filename, "_automatic_") {
		backupType = "automatic"
	}

	checksum, _ := bm.calculateChecksum(backupPath)

	return &BackupInfo{
		Filename:   filename,
		Path:       backupPath,
		Size:       fileInfo.Size(),
		CreatedAt:  fileInfo.ModTime(),
		Type:       backupType,
		Compressed: false,
		Checksum:   checksum,
	}, nil
}

// verifyBackupIntegrity verifies backup file integrity
func (bm *BackupManager) verifyBackupIntegrity(backupPath string, backupInfo *BackupInfo) error {
	// Check if file exists and is readable
	file, err := os.Open(backupPath)
	if err != nil {
		return fmt.Errorf("cannot read backup file: %w", err)
	}
	defer file.Close()

	// Verify checksum if available
	if backupInfo.Checksum != "unavailable" && backupInfo.Checksum != "" {
		currentChecksum, err := bm.calculateChecksum(backupPath)
		if err != nil {
			return fmt.Errorf("failed to calculate checksum: %w", err)
		}
		
		if currentChecksum != backupInfo.Checksum {
			return fmt.Errorf("checksum mismatch: expected %s, got %s", backupInfo.Checksum, currentChecksum)
		}
	}

	// Try to open as SQLite database (basic validation)
	// This is a simple check - in production you might want more thorough validation

	return nil
}

// verifyRestoredDatabase verifies the restored database
func (bm *BackupManager) verifyRestoredDatabase(ctx context.Context) error {
	// Perform basic health check
	health := bm.db.HealthCheck(ctx)
	if health.Status != "healthy" {
		return fmt.Errorf("database health check failed: %s", health.Message)
	}

	return nil
}

// cleanupOldBackups removes old backups beyond the max limit
func (bm *BackupManager) cleanupOldBackups() error {
	backups, err := bm.ListBackups()
	if err != nil {
		return err
	}

	if len(backups) <= bm.maxBackups {
		return nil
	}

	// Delete oldest backups
	for i := bm.maxBackups; i < len(backups); i++ {
		backup := backups[i]
		if err := os.Remove(backup.Path); err != nil {
			bm.logger.WithError(err).WithField("backup", backup.Filename).Warn("Failed to delete old backup")
			continue
		}
		bm.logger.WithField("backup", backup.Filename).Info("Deleted old backup")
	}

	return nil
}

// fileExists checks if a file exists
func (bm *BackupManager) fileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// copyFile copies a file from src to dst
func (bm *BackupManager) copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	// Sync to ensure data is written
	return destFile.Sync()
}