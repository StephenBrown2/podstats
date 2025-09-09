# PodcastAddict Backup Database Management

This feature allows you to extract, examine, and modify the SQLite database contained within PodcastAddict backup files.

## Features

### Database Extraction and Analysis
- Extract SQLite database from PodcastAddict backup zip files
- View podcast statistics including:
  - Podcast name and author
  - Feed URL
  - Unlistened episode count
  - Average episode duration
  - Publication frequency
  - Current priority

### Database Modification
- Update podcast priorities
- Automatically repackage the modified database back into a new backup file
- Preserves all other backup contents (preferences, settings, etc.)

## Usage

### View Podcast Statistics from Backup
```bash
./podstats --backup stats <backup_file>
```

Example:
```bash
./podstats --backup stats PodcastAddict_autoBackup_20250731_012309.backup.zip
```

This will show all subscribed podcasts with their current statistics and priorities.

### Update Podcast Priority
```bash
./podstats --backup update <backup_file> <feed_url> <priority>
```

Example:
```bash
./podstats --backup update PodcastAddict_autoBackup_20250731_012309.backup.zip "https://coinstories.libsyn.com/rsscoin" 15
```

This will:
1. Extract the database from the backup
2. Update the priority for the specified podcast
3. Create a new backup file with timestamp: `PodcastAddict_autoBackup_20250731_012309_updated_20250805_162623.backup.zip`
4. Preserve all other backup contents

## Technical Details

### Database Operations
- Uses SQLite driver (`modernc.org/sqlite`) for database access
- Generated database operations using `sqlc` for type safety
- Supports both read and write operations on the extracted database

### File Handling
- Extracts database to temporary directory during processing
- Automatically cleans up temporary files after completion
- Preserves original zip file structure and metadata
- Replaces only the database file while keeping all other backup contents intact

### Error Handling
- Validates that feed URLs exist in the database before attempting updates
- Provides clear error messages for common issues
- Automatically verifies updates were successful

## Safety Notes

- Always keep a backup of your original file before making modifications
- The tool creates new backup files with timestamps rather than overwriting originals
- All database operations are transactional and rolled back on error

## Dependencies

- `modernc.org/sqlite`: SQLite database driver
- Generated database operations in `podcastaddict/` package using `sqlc`
