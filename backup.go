package main

import (
	"archive/zip"
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/StephenBrown2/podstats/podcastaddict"
)

// BackupManager handles operations on PodcastAddict backup files.
type BackupManager struct {
	backupPath  string
	extractDir  string
	dbPath      string
	originalZip *zip.ReadCloser
	db          *sql.DB
	dbQueries   *podcastaddict.Queries
}

// NewBackupManager creates a new backup manager for the given backup file.
func NewBackupManager(backupPath string) (*BackupManager, error) {
	// Create temporary directory for extraction
	extractDir, err := os.MkdirTemp("", "podcastaddict_backup_*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	bm := &BackupManager{
		backupPath: backupPath,
		extractDir: extractDir,
		dbPath:     filepath.Join(extractDir, "podcastAddict.db"),
	}

	return bm, nil
}

// ExtractDatabase extracts the SQLite database from the backup zip file.
func (bm *BackupManager) ExtractDatabase() error {
	// Open the zip file
	r, err := zip.OpenReader(bm.backupPath)
	if err != nil {
		return fmt.Errorf("failed to open backup zip: %w", err)
	}
	bm.originalZip = r

	// Find and extract the database file
	var dbFile *zip.File
	for _, f := range r.File {
		if strings.HasSuffix(f.Name, "podcastAddict.db") {
			dbFile = f
			break
		}
	}

	if dbFile == nil {
		return fmt.Errorf("podcastAddict.db not found in backup zip")
	}

	// Extract the database file
	rc, err := dbFile.Open()
	if err != nil {
		return fmt.Errorf("failed to open database file in zip: %w", err)
	}
	defer rc.Close()

	// Create the database file in temp directory
	outFile, err := os.Create(bm.dbPath)
	if err != nil {
		return fmt.Errorf("failed to create database file: %w", err)
	}
	defer outFile.Close()

	// Copy the database content
	_, err = io.Copy(outFile, rc)
	if err != nil {
		return fmt.Errorf("failed to copy database content: %w", err)
	}

	return nil
}

// OpenDatabase opens the extracted database and initializes queries.
func (bm *BackupManager) OpenDatabase() error {
	db, err := sql.Open("sqlite", bm.dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	bm.db = db
	bm.dbQueries = podcastaddict.New(db)
	return nil
}

// GetPodcastStats retrieves podcast statistics from the database.
func (bm *BackupManager) GetPodcastStats(ctx context.Context) ([]podcastaddict.GetPodcastStatsRow, error) {
	if bm.dbQueries == nil {
		return nil, fmt.Errorf("database not opened")
	}

	return bm.dbQueries.GetPodcastStats(ctx)
}

// GetPodcastStatsByTag retrieves podcast statistics filtered by tag.
func (bm *BackupManager) GetPodcastStatsByTag(ctx context.Context, tag string) ([]podcastaddict.GetPodcastStatsByTagRow, error) {
	if bm.dbQueries == nil {
		return nil, fmt.Errorf("database not opened")
	}

	return bm.dbQueries.GetPodcastStatsByTag(ctx, tag)
}

// GetTags retrieves all available tags from the database.
func (bm *BackupManager) GetTags(ctx context.Context) ([]string, error) {
	if bm.dbQueries == nil {
		return nil, fmt.Errorf("database not opened")
	}

	return bm.dbQueries.GetTags(ctx)
}

// UpdatePodcastPriority updates the priority of a podcast by its feed URL.
func (bm *BackupManager) UpdatePodcastPriority(ctx context.Context, feedURL string, priority int64) error {
	if bm.dbQueries == nil {
		return fmt.Errorf("database not opened")
	}

	// First, check if the podcast exists
	stats, err := bm.GetPodcastStats(ctx)
	if err != nil {
		return fmt.Errorf("failed to get podcast stats for verification: %w", err)
	}

	found := false
	for _, stat := range stats {
		if stat.FeedUrl == feedURL {
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("podcast with feed URL %s not found in database", feedURL)
	}

	err = bm.dbQueries.UpdatePriority(ctx, podcastaddict.UpdatePriorityParams{
		FeedUrl:  feedURL,
		Priority: priority,
	})
	if err != nil {
		return fmt.Errorf("failed to execute update query: %w", err)
	}

	return nil
}

// RepackageBackup creates a new backup zip file with the modified database.
func (bm *BackupManager) RepackageBackup(outputPath string) error {
	if bm.originalZip == nil {
		return fmt.Errorf("original zip not opened")
	}

	// Create new zip file
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output zip: %w", err)
	}
	defer outFile.Close()

	zipWriter := zip.NewWriter(outFile)
	defer zipWriter.Close()

	// Copy all files from original zip, replacing the database
	for _, f := range bm.originalZip.File {
		var writer io.Writer
		var reader io.ReadCloser

		// Create new file in zip
		fileWriter, err := zipWriter.CreateHeader(&zip.FileHeader{
			Name:     f.Name,
			Method:   f.Method,
			Modified: f.Modified,
		})
		if err != nil {
			return fmt.Errorf("failed to create file in zip: %w", err)
		}
		writer = fileWriter

		// If this is the database file, use our modified version
		if strings.HasSuffix(f.Name, "podcastAddict.db") {
			dbFile, err := os.Open(bm.dbPath)
			if err != nil {
				return fmt.Errorf("failed to open modified database: %w", err)
			}
			reader = dbFile
		} else {
			// Use original file
			rc, err := f.Open()
			if err != nil {
				return fmt.Errorf("failed to open original file: %w", err)
			}
			reader = rc
		}

		// Copy content
		_, err = io.Copy(writer, reader)
		reader.Close()
		if err != nil {
			return fmt.Errorf("failed to copy file content: %w", err)
		}
	}

	return nil
}

// Close cleans up resources.
func (bm *BackupManager) Close() error {
	var errs []error

	if bm.db != nil {
		if err := bm.db.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if bm.originalZip != nil {
		if err := bm.originalZip.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if bm.extractDir != "" {
		if err := os.RemoveAll(bm.extractDir); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("cleanup errors: %v", errs)
	}

	return nil
}

// UpdateBackupPriorities provides a convenient function to update podcast priorities in a backup.
func UpdateBackupPriorities(backupPath string, priorityUpdates map[string]int64) error {
	// Create backup manager
	bm, err := NewBackupManager(backupPath)
	if err != nil {
		return fmt.Errorf("failed to create backup manager: %w", err)
	}
	defer bm.Close()

	// Extract database
	if err := bm.ExtractDatabase(); err != nil {
		return fmt.Errorf("failed to extract database: %w", err)
	}

	// Open database
	if err := bm.OpenDatabase(); err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Apply priority updates
	ctx := context.Background()
	for feedURL, priority := range priorityUpdates {
		if err := bm.UpdatePodcastPriority(ctx, feedURL, priority); err != nil {
			return fmt.Errorf("failed to update priority for %s: %w", feedURL, err)
		}
	}

	// Close database connection to ensure all changes are flushed
	if err := bm.db.Close(); err != nil {
		return fmt.Errorf("failed to close database: %w", err)
	}
	bm.db = nil
	bm.dbQueries = nil

	// Create output filename with timestamp
	timestamp := time.Now().Format("20060102_150405")
	var outputPath string

	// Handle both .backup.zip and .backup extensions
	lowerBackupPath := strings.ToLower(backupPath)
	if strings.HasSuffix(lowerBackupPath, ".backup.zip") {
		outputPath = strings.Replace(backupPath, ".backup.zip", fmt.Sprintf("_updated_%s.backup.zip", timestamp), 1)
	} else if strings.HasSuffix(lowerBackupPath, ".backup") {
		outputPath = strings.Replace(backupPath, ".backup", fmt.Sprintf("_updated_%s.backup", timestamp), 1)
	} else {
		// Fallback: append timestamp before the last dot or at the end
		if lastDot := strings.LastIndex(backupPath, "."); lastDot != -1 {
			outputPath = backupPath[:lastDot] + fmt.Sprintf("_updated_%s", timestamp) + backupPath[lastDot:]
		} else {
			outputPath = backupPath + fmt.Sprintf("_updated_%s", timestamp)
		}
	}

	// Repackage backup
	if err := bm.RepackageBackup(outputPath); err != nil {
		return fmt.Errorf("failed to repackage backup: %w", err)
	}

	fmt.Printf("Updated backup saved as: %s\n", outputPath)
	return nil
}
