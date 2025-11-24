package main

import (
	"bufio"
	"cmp"
	"context"
	"database/sql"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/StephenBrown2/podstats/podcastaddict"
)

// OPML structures.
type OPML struct {
	XMLName xml.Name `xml:"opml"`
	Head    Head     `xml:"head"`
	Body    Body     `xml:"body"`
}

type Head struct {
	Title string `xml:"title"`
}

type Body struct {
	Outlines []Outline `xml:"outline"`
}

type Outline struct {
	Text      string    `xml:"text,attr"`
	Title     string    `xml:"title,attr"`
	SortTitle string    `xml:"-"`
	Type      string    `xml:"type,attr"`
	XMLURL    string    `xml:"xmlUrl,attr"`
	HTMLURL   string    `xml:"htmlUrl,attr"`
	Outlines  []Outline `xml:"outline"`
}

// RSS/Atom feed structures.
type RSS struct {
	XMLName xml.Name `xml:"rss"`
	Channel Channel  `xml:"channel"`
}

type Atom struct {
	XMLName xml.Name `xml:"feed"`
	Title   string   `xml:"title"`
	Entries []Entry  `xml:"entry"`
}

type Channel struct {
	Title       string `xml:"title"`
	Description string `xml:"description"`
	Items       []Item `xml:"item"`
	Value       *Value `xml:"value"`
}

type Item struct {
	Title       string    `xml:"title"`
	Description string    `xml:"description"`
	PubDate     string    `xml:"pubDate"`
	Enclosure   Enclosure `xml:"enclosure"`
	Duration    string    `xml:"duration"`
}

type Entry struct {
	Title     string `xml:"title"`
	Published string `xml:"published"`
	Updated   string `xml:"updated"`
}

type Enclosure struct {
	URL    string `xml:"url,attr"`
	Length string `xml:"length,attr"`
	Type   string `xml:"type,attr"`
}

type Value struct {
	Text           string           `xml:",chardata"`
	Type           string           `xml:"type,attr"`
	Method         string           `xml:"method,attr"`
	Suggested      string           `xml:"suggested,attr"`
	ValueRecipient []ValueRecipient `xml:"valueRecipient"`
}

type ValueRecipient struct {
	Text    string `xml:",chardata"`
	Name    string `xml:"name,attr"`
	Type    string `xml:"type,attr"`
	Address string `xml:"address,attr"`
	Split   string `xml:"split,attr"`
}

// Statistics structure.
type PodcastStats struct {
	Title                string
	SortTitle            string
	URL                  string
	UnlistenedEpisodes   int
	AvgEpisodeLengthMins float64
	AvgDaysBetween       float64
	DaysSinceLatest      float64
	PlaybackSpeed        float64
	CompositeScore       float64
}

// Cache structure for storing user input.
type PodcastCache struct {
	URL                string  `json:"url"`
	UnlistenedEpisodes int     `json:"unlistened_episodes"`
	PlaybackSpeed      float64 `json:"playback_speed"`
	LastUpdated        string  `json:"last_updated"`
}

type CacheData struct {
	Podcasts []PodcastCache `json:"podcasts"`
}

// Common leading articles to ignore/dim when sorting or displaying titles.
var leadingArticles = []string{"A", "An", "The", "This", "Ye"}

// loadCache loads the cache from the cache file.
func loadCache(cacheFile string) *CacheData {
	cache := &CacheData{Podcasts: []PodcastCache{}}

	file, err := os.Open(cacheFile)
	if err != nil {
		return cache // Return empty cache if file doesn't exist
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return cache
	}

	json.Unmarshal(data, cache)
	return cache
}

// saveCache saves the cache to the cache file.
func saveCache(cacheFile string, cache *CacheData) error {
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(cacheFile, data, 0o644)
}

// getCachedUnlistenedCount gets the cached unlistened count for a podcast URL.
func getCachedUnlistenedCount(cache *CacheData, url string) (int, bool) {
	for _, podcast := range cache.Podcasts {
		if podcast.URL == url {
			return podcast.UnlistenedEpisodes, true
		}
	}
	return 0, false
}

// getCachedPlaybackSpeed gets the cached playback speed for a podcast URL.
func getCachedPlaybackSpeed(cache *CacheData, url string) (float64, bool) {
	for _, podcast := range cache.Podcasts {
		if podcast.URL == url {
			// Return 2.0 as default if playback speed is 0 (for backward compatibility)
			if podcast.PlaybackSpeed == 0 {
				return 2.0, false
			}
			return podcast.PlaybackSpeed, true
		}
	}
	return 2.0, false
}

// updateCache updates the cache with new unlistened count for a podcast.
func updateCache(cache *CacheData, url string, unlistenedCount int) {
	// Look for existing entry
	for i, podcast := range cache.Podcasts {
		if podcast.URL == url {
			cache.Podcasts[i].UnlistenedEpisodes = unlistenedCount
			cache.Podcasts[i].LastUpdated = time.Now().Format(time.DateTime)
			return
		}
	}

	// Add new entry if not found
	cache.Podcasts = append(cache.Podcasts, PodcastCache{
		URL:                url,
		UnlistenedEpisodes: unlistenedCount,
		PlaybackSpeed:      2.0, // Default playback speed
		LastUpdated:        time.Now().Format(time.DateTime),
	})
}

// updateCacheWithPlaybackSpeed updates the cache with both unlistened count and playback speed.
func updateCacheWithPlaybackSpeed(cache *CacheData, url string, unlistenedCount int, playbackSpeed float64) {
	// Look for existing entry
	for i, podcast := range cache.Podcasts {
		if podcast.URL == url {
			cache.Podcasts[i].UnlistenedEpisodes = unlistenedCount
			cache.Podcasts[i].PlaybackSpeed = playbackSpeed
			cache.Podcasts[i].LastUpdated = time.Now().Format(time.DateTime)
			return
		}
	}

	// Add new entry if not found
	cache.Podcasts = append(cache.Podcasts, PodcastCache{
		URL:                url,
		UnlistenedEpisodes: unlistenedCount,
		PlaybackSpeed:      playbackSpeed,
		LastUpdated:        time.Now().Format(time.DateTime),
	})
}

func main() {
	// Check if we should run in TUI mode
	if len(os.Args) == 2 && os.Args[1] == "--tui" {
		runTUI()
		return
	}

	// Check for backup command
	if len(os.Args) >= 2 && os.Args[1] == "--backup" {
		handleBackupCommand()
		return
	}

	// Original CLI mode
	if len(os.Args) < 2 || len(os.Args) > 3 {
		fmt.Println("Usage:")
		fmt.Println("  podstats --tui                                          # Run TUI mode")
		fmt.Println("  podstats [--cached[=all|speed|unlistened]] <opml_file> # Analyze OPML file")
		fmt.Println("  podstats [--cached[=all|speed|unlistened]] <backup_file> # Analyze and update backup file")
		fmt.Println("  podstats --backup <command> [args...]                  # Backup operations")
		fmt.Println("")
		fmt.Println("File Types:")
		fmt.Println("  OPML files (.opml): Analyze podcasts and display results")
		fmt.Println("  Backup files (.backup): Analyze and update podcast priorities in backup zip")
		fmt.Println("")
		fmt.Println("Backup commands:")
		fmt.Println("  podstats --backup stats <backup_file>                  # Show podcast stats from backup")
		fmt.Println("  podstats --backup speeds <backup_file>                 # Show podcast speed settings from backup")
		fmt.Println("  podstats --backup update <backup_file> <feed_url> <priority> # Update podcast priority")
		os.Exit(1)
	}

	var useCachedSpeed bool
	var useCachedUnlistened bool
	var inputFile string
	var isBackupFile bool

	// Parse arguments
	args := os.Args[1:]

	// Check for --cached parameter
	if len(args) > 0 && strings.HasPrefix(args[0], "--cached") {
		// Parse the cached parameter
		cachedParam := "all" // default
		if strings.Contains(args[0], "=") {
			parts := strings.Split(args[0], "=")
			if len(parts) == 2 {
				cachedParam = parts[1]
			}
		}

		switch cachedParam {
		case "all":
			useCachedSpeed = true
			useCachedUnlistened = true
		case "speed":
			useCachedSpeed = true
			useCachedUnlistened = false
		case "unlistened":
			useCachedSpeed = false
			useCachedUnlistened = true
		default:
			fmt.Printf("Invalid cached parameter: %s. Valid options: all, speed, unlistened\n", cachedParam)
			fmt.Println("Usage: podstats [--cached[=all|speed|unlistened]] <opml_file|backup_file>")
			os.Exit(1)
		}

		args = args[1:] // Remove the --cached flag from args
	}

	// The next argument should be the input file
	if len(args) == 0 {
		fmt.Println("Error: Input file required (OPML or backup file)")
		os.Exit(1)
	}
	inputFile = args[0]

	// Determine file type based on extension
	lowerInputFile := strings.ToLower(inputFile)
	if strings.HasSuffix(lowerInputFile, ".backup.zip") || strings.HasSuffix(lowerInputFile, ".backup") {
		isBackupFile = true
		fmt.Printf("Detected backup file: %s\n", inputFile)
	} else if strings.HasSuffix(lowerInputFile, ".opml") {
		isBackupFile = false
		fmt.Printf("Detected OPML file: %s\n", inputFile)
	} else {
		fmt.Printf("Warning: Unrecognized file extension, treating as OPML file: %s\n", inputFile)
		isBackupFile = false
	}

	if len(args) > 1 {
		fmt.Printf("Unknown arguments: %v\n", args[1:])
		fmt.Println("Usage: podstats [--cached[=all|speed|unlistened]] <opml_file|backup_file>")
		os.Exit(1)
	}

	cacheFile := "cache.json"

	// Load cache
	cache := loadCache(cacheFile)

	var allStats []PodcastStats
	var podcasts []Outline

	if isBackupFile {
		// Extract podcasts from backup file first
		fmt.Printf("Extracting podcast list from backup file: %s\n", inputFile)
		var err error
		podcasts, err = extractPodcastsFromBackup(inputFile)
		if err != nil {
			log.Fatalf("Error extracting podcasts from backup: %v", err)
		}
		fmt.Printf("Found %d podcasts in backup file\n", len(podcasts))
	} else {
		// Parse OPML file
		fmt.Printf("Parsing OPML file: %s\n", inputFile)
		var err error
		podcasts, err = parseOPML(inputFile)
		if err != nil {
			log.Fatalf("Error parsing OPML: %v", err)
		}
		fmt.Printf("Found %d podcasts in OPML file\n", len(podcasts))
	}

	fmt.Printf("Cache file: %s\n\n", cacheFile)

	// Ensure podcasts are processed in trimmed-name order (ignoring articles)
	if len(podcasts) > 0 {
		// Populate SortTitle defensively and sort
		for i := range podcasts {
			if podcasts[i].SortTitle == "" {
				title := podcasts[i].Title
				if title == "" {
					title = podcasts[i].Text
				}
				podcasts[i].SortTitle = trimArticles(title)
			}
		}
		slices.SortStableFunc(podcasts, func(a, b Outline) int {
			return strings.Compare(a.SortTitle, b.SortTitle)
		})
	}

	// Analyze each podcast
	for i, podcast := range podcasts {
		// Get the display title (fallback to text if title is empty)
		displayTitle := podcast.Title
		if displayTitle == "" {
			displayTitle = podcast.Text
		}
		displayTitle = strings.ReplaceAll(displayTitle, "&amp;", "&") // Clean up title
		fmt.Printf("[%d/%d] Analyzing: %s\n", i+1, len(podcasts), dimArticleInTitle(displayTitle))
		fmt.Printf("  URL: %s\n", podcast.XMLURL)

		stats, err := analyzePodcastCLI(podcast, cache, useCachedUnlistened, useCachedSpeed)
		if err != nil {
			fmt.Printf("  Error: %v\n\n", err)
			continue
		}

		fmt.Printf("  Episodes: %d unlistened\n", stats.UnlistenedEpisodes)
		fmt.Printf("  Playback speed: %.1fx\n", stats.PlaybackSpeed)
		fmt.Printf("  Avg length: %.1f minutes (adjusted for playback speed)\n", stats.AvgEpisodeLengthMins)
		fmt.Printf("  Avg days between: %.1f\n", stats.AvgDaysBetween)
		fmt.Printf("  Days since latest: %s\n", colorDaysSince(stats.DaysSinceLatest, false))
		fmt.Printf("  Composite score: %.2f\n\n", stats.CompositeScore)

		allStats = append(allStats, stats)
	}

	// Save cache
	if err := saveCache(cacheFile, cache); err != nil {
		fmt.Printf("Warning: Could not save cache: %v\n", err)
	}

	// Generate and display histogram
	fmt.Printf("=== PODCAST STATISTICS HISTOGRAM ===\n\n")
	displayHistogram(allStats)

	// Update backup file if this was a backup file
	if isBackupFile {
		fmt.Printf("\n=== UPDATING BACKUP FILE ===\n\n")
		updateBackupWithAnalysis(inputFile, allStats)
	}
}

func extractPodcastsFromBackup(backupFile string) ([]Outline, error) {
	// Create backup manager to read podcast feed URLs
	bm, err := NewBackupManager(backupFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open backup file: %w", err)
	}
	defer bm.Close()

	if err := bm.ExtractDatabase(); err != nil {
		return nil, fmt.Errorf("failed to extract database: %w", err)
	}

	if err := bm.OpenDatabase(); err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Prompt user for tag selection
	selectedTag, err := promptForTag(bm)
	if err != nil {
		return nil, fmt.Errorf("failed to get tag selection: %w", err)
	}

	ctx := context.Background()

	if selectedTag == "" {
		// No tag filter, get all podcasts
		fmt.Println("Analyzing all podcasts...")
		backupStats, err := bm.GetPodcastStats(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get podcast stats from backup: %w", err)
		}

		// Convert backup stats to Outline format for analysis
		var podcasts []Outline
		for _, stat := range backupStats {
			podcast := Outline{
				Title:  stat.Name,
				XMLURL: stat.FeedUrl,
				Text:   stat.Name, // fallback
			}
			podcasts = append(podcasts, podcast)
		}
		// Ensure SortTitle is populated using trimArticles for consistent sorting
		for i := range podcasts {
			podcasts[i].SortTitle = trimArticles(podcasts[i].Title)
		}
		return podcasts, nil
	} else {
		// Filter by selected tag
		fmt.Printf("Analyzing podcasts with tag: %s\n", selectedTag)
		backupStats, err := bm.GetPodcastStatsByTag(ctx, selectedTag)
		if err != nil {
			return nil, fmt.Errorf("failed to get podcast stats by tag from backup: %w", err)
		}

		// Convert backup stats to Outline format for analysis
		var podcasts []Outline
		for _, stat := range backupStats {
			podcast := Outline{
				Title:  stat.Name,
				XMLURL: stat.FeedUrl,
				Text:   stat.Name, // fallback
			}
			podcasts = append(podcasts, podcast)
		}
		// Ensure SortTitle is populated using trimArticles for consistent sorting
		for i := range podcasts {
			podcasts[i].SortTitle = trimArticles(podcasts[i].Title)
		}
		return podcasts, nil
	}
}

func updateBackupWithAnalysis(backupFile string, allStats []PodcastStats) {
	fmt.Printf("Reading backup file: %s\n", backupFile)

	// Create backup manager to read current priorities
	bm, err := NewBackupManager(backupFile)
	if err != nil {
		fmt.Printf("Error: Failed to open backup file: %v\n", err)
		return
	}
	defer bm.Close()

	if err := bm.ExtractDatabase(); err != nil {
		fmt.Printf("Error: Failed to extract database: %v\n", err)
		return
	}

	if err := bm.OpenDatabase(); err != nil {
		fmt.Printf("Error: Failed to open database: %v\n", err)
		return
	}

	ctx := context.Background()
	backupStats, err := bm.GetPodcastStats(ctx)
	if err != nil {
		fmt.Printf("Error: Failed to get podcast stats from backup: %v\n", err)
		return
	}

	// Create a map of feed URLs to current priorities from backup
	currentPriorities := make(map[string]int64)
	for _, stat := range backupStats {
		currentPriorities[stat.FeedUrl] = stat.Priority
	}

	// Calculate new priorities based on composite scores
	// Lower composite scores get higher priorities (lower scores are better)
	priorityUpdates := make(map[string]int64)

	// Sort by composite score (ascending - better scores first)
	slices.SortStableFunc(allStats, func(a, b PodcastStats) int {
		return cmp.Compare(a.CompositeScore, b.CompositeScore)
	})

	// Assign priorities based on ranking using original priority range (1-10)
	// Best podcasts (lowest composite scores) get highest priorities
	maxPriority := int64(10)
	minPriority := int64(1)
	for i, stats := range allStats {
		// Check if this podcast exists in the backup
		if currentPriority, exists := currentPriorities[stats.URL]; exists {
			// Calculate new priority: best podcast gets maxPriority, worst gets minPriority
			priorityRange := maxPriority - minPriority
			newPriority := maxPriority - int64(i)*priorityRange/int64(len(allStats))
			if newPriority < minPriority {
				newPriority = minPriority
			}

			if newPriority != currentPriority {
				priorityUpdates[stats.URL] = newPriority
				fmt.Printf("  %s: %d -> %d (score: %.3f)\n", dimArticleInTitle(stats.Title), currentPriority, newPriority, stats.CompositeScore)
			}
		}
	}

	if len(priorityUpdates) == 0 {
		fmt.Println("No priority updates needed - all podcasts already have optimal priorities")
		return
	}

	fmt.Printf("\nUpdating %d podcast priorities...\n", len(priorityUpdates))

	// Apply the updates
	if err := UpdateBackupPriorities(backupFile, priorityUpdates); err != nil {
		fmt.Printf("Error: Failed to update backup priorities: %v\n", err)
		return
	}

	fmt.Printf("Successfully updated backup file with new priorities!\n")
}

func handleBackupCommand() {
	if len(os.Args) < 3 {
		fmt.Println("Backup command required. Available commands:")
		fmt.Println("  stats <backup_file>                     # Show podcast stats from backup")
		fmt.Println("  speeds <backup_file>                    # Show podcast speed settings from backup")
		fmt.Println("  update <backup_file> <feed_url> <priority> # Update podcast priority")
		os.Exit(1)
	}

	subcommand := os.Args[2]

	switch subcommand {
	case "stats":
		if len(os.Args) != 4 {
			fmt.Println("Usage: podstats --backup stats <backup_file>")
			os.Exit(1)
		}
		showBackupStats(os.Args[3])

	case "speeds":
		if len(os.Args) != 4 {
			fmt.Println("Usage: podstats --backup speeds <backup_file>")
			os.Exit(1)
		}
		showBackupSpeeds(os.Args[3])

	case "update":
		if len(os.Args) != 6 {
			fmt.Println("Usage: podstats --backup update <backup_file> <feed_url> <priority>")
			os.Exit(1)
		}

		backupFile := os.Args[3]
		feedURL := os.Args[4]
		priority, err := strconv.ParseInt(os.Args[5], 10, 64)
		if err != nil {
			fmt.Printf("Invalid priority value: %s\n", os.Args[5])
			os.Exit(1)
		}

		updateBackupPriority(backupFile, feedURL, priority)

	default:
		fmt.Printf("Unknown backup command: %s\n", subcommand)
		fmt.Println("Available commands: stats, speeds, update")
		os.Exit(1)
	}
}

func showBackupStats(backupFile string) {
	bm, err := NewBackupManager(backupFile)
	if err != nil {
		log.Fatalf("Failed to create backup manager: %v", err)
	}
	defer bm.Close()

	if err := bm.ExtractDatabase(); err != nil {
		log.Fatalf("Failed to extract database: %v", err)
	}

	if err := bm.OpenDatabase(); err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}

	// Prompt user for tag selection
	selectedTag, err := promptForTag(bm)
	if err != nil {
		log.Fatalf("Failed to get tag selection: %v", err)
	}

	ctx := context.Background()

	if selectedTag == "" {
		// Show all podcasts
		stats, err := bm.GetPodcastStats(ctx)
		if err != nil {
			log.Fatalf("Failed to get podcast stats: %v", err)
		}

		// Sort by trimmed title for readability
		slices.SortStableFunc(stats, func(a, b podcastaddict.GetPodcastStatsRow) int {
			return strings.Compare(trimArticles(a.Name), trimArticles(b.Name))
		})

		fmt.Printf("Found %d subscribed podcasts in backup:\n\n", len(stats))

		for _, stat := range stats {
			displayPodcastStat(stat.Name, stat.Author, stat.FeedUrl, stat.UnplayedEpisodes, stat.AverageDuration, stat.Frequency, stat.Priority)
		}
	} else {
		// Show podcasts filtered by tag
		stats, err := bm.GetPodcastStatsByTag(ctx, selectedTag)
		if err != nil {
			log.Fatalf("Failed to get podcast stats by tag: %v", err)
		}

		// Sort by trimmed title for readability
		slices.SortStableFunc(stats, func(a, b podcastaddict.GetPodcastStatsByTagRow) int {
			return strings.Compare(trimArticles(a.Name), trimArticles(b.Name))
		})

		fmt.Printf("Found %d podcasts with tag '%s':\n\n", len(stats), selectedTag)

		for _, stat := range stats {
			displayPodcastStat(stat.Name, stat.Author, stat.FeedUrl, stat.UnplayedEpisodes, stat.AverageDuration, stat.Frequency, stat.Priority)
		}
	}
}

func showBackupSpeeds(backupFile string) {
	bm, err := NewBackupManager(backupFile)
	if err != nil {
		log.Fatalf("Failed to create backup manager: %v", err)
	}
	defer bm.Close()

	if err := bm.ExtractDatabase(); err != nil {
		log.Fatalf("Failed to extract database: %v", err)
	}

	if err := bm.OpenDatabase(); err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}

	ctx := context.Background()
	speedSettings, defaultSpeed, err := bm.GetPodcastSpeedSettings(ctx)
	if err != nil {
		log.Fatalf("Failed to get podcast speed settings: %v", err)
	}

	// Sort by trimmed podcast name for consistent display
	sort.Slice(speedSettings, func(i, j int) bool {
		return trimArticles(speedSettings[i].PodcastName) < trimArticles(speedSettings[j].PodcastName)
	})

	fmt.Printf("Podcast Speed Settings (found %d podcasts, default speed: %.1fx):\n\n", len(speedSettings), defaultSpeed)

	// Create a table for better display
	t := table.New().
		Border(lipgloss.RoundedBorder()).
		Headers("Podcast Name", "Speed Enabled", "Speed Multiplier").
		StyleFunc(func(row, col int) lipgloss.Style {
			return lipgloss.NewStyle().Padding(0, 1)
		})

	for _, setting := range speedSettings {
		speedEnabledText := "No"
		if setting.SpeedEnabled {
			speedEnabledText = "Yes"
		}

		speedText := fmt.Sprintf("%.1fx", setting.SpeedMultiplier)
		if !setting.SpeedEnabled {
			speedText = fmt.Sprintf("%.1fx (default)", defaultSpeed)
		}

		t.Row(dimArticleInTitle(setting.PodcastName), speedEnabledText, speedText)
	}

	fmt.Println(t.Render())
}

func updateBackupPriority(backupFile, feedURL string, priority int64) {
	priorityUpdates := map[string]int64{
		feedURL: priority,
	}

	if err := UpdateBackupPriorities(backupFile, priorityUpdates); err != nil {
		log.Fatalf("Failed to update backup: %v", err)
	}

	fmt.Printf("Successfully updated priority for %s to %d\n", feedURL, priority)
}

func trimArticles(title string) string {
	// Clean up title by removing common articles
	sortTitle := strings.TrimSpace(title)
	sortTitle = strings.ReplaceAll(sortTitle, "&amp;", "&") // Clean up title
	for _, article := range leadingArticles {
		sortTitle = strings.TrimPrefix(strings.ToLower(sortTitle), strings.ToLower(article)) // Remove common prefix
	}
	return strings.TrimSpace(sortTitle) // Final trim
}

// colorDaysSince returns a colorized string for days-since-latest buckets.
func colorDaysSince(days float64, includeDays bool) string {
	val := fmt.Sprintf("%.0f days", days)
	if !includeDays {
		val = fmt.Sprintf("%.0f", days)
	}
	switch {
	case days < 100:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00")).Render(val)
	case days < 200:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFF00")).Render(val)
	case days < 300:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#FF8800")).Render(val)
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")).Render(val)
	}
}

// dimArticleInTitle returns the title with a leading article dimmed for display purposes.
func dimArticleInTitle(title string) string {
	// Normalize basic entities and trim outer spaces
	cleaned := strings.TrimSpace(strings.ReplaceAll(title, "&amp;", "&"))
	if cleaned == "" {
		return cleaned
	}

	// Separate leading symbolic prefix (emojis, punctuation) that should remain untouched
	prefixEnd := 0
	for i, r := range cleaned {
		// Stop at first letter (A-Z or a-z) to attempt article detection
		if unicode.IsLetter(r) {
			prefixEnd = i
			break
		}
		// Continue accumulating prefix (emoji / punctuation / spaces)
		prefixEnd = i + len(string(r))
	}

	prefix := cleaned[:prefixEnd]
	remainder := cleaned[prefixEnd:]
	remainder = strings.TrimLeft(remainder, " ") // Remove any space after prefix for article matching

	if remainder == "" {
		return cleaned // Nothing to process
	}

	// Check common English articles (case-insensitive) followed by a space
	for _, art := range leadingArticles {
		n := len(art)
		if len(remainder) > n && strings.EqualFold(remainder[:n], art) && remainder[n] == ' ' {
			articlePart := remainder[:n]
			rest := strings.TrimSpace(remainder[n:])
			faintArticle := lipgloss.NewStyle().Faint(true).Render(articlePart)
			// Reconstruct: prefix (with its original spacing) + faint article + space + rest
			out := strings.TrimRight(prefix, " ")
			if out != "" {
				out += " "
			}
			return out + faintArticle + " " + rest
		}
	}

	// No article detected; return original cleaned title
	return cleaned
}

// parseOPML reads and parses an OPML file, extracting podcast feed URLs.
func parseOPML(filename string) ([]Outline, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	var opml OPML
	if err := xml.Unmarshal(data, &opml); err != nil {
		return nil, err
	}

	var podcasts []Outline
	extractOutlines(opml.Body.Outlines, &podcasts)
	for i := range podcasts {
		podcasts[i].SortTitle = trimArticles(podcasts[i].Title)
	}
	return podcasts, nil
}

// extractOutlines recursively extracts podcast outlines with XML URLs.
func extractOutlines(outlines []Outline, podcasts *[]Outline) {
	for _, outline := range outlines {
		if outline.XMLURL != "" {
			*podcasts = append(*podcasts, outline)
		}
		if len(outline.Outlines) > 0 {
			extractOutlines(outline.Outlines, podcasts)
		}
	}
}

// parseRSS parses RSS feed data.
func parseRSS(data []byte) (*RSS, error) {
	var rss RSS
	err := xml.Unmarshal(data, &rss)
	return &rss, err
}

// parseAtom parses Atom feed data.
func parseAtom(data []byte) (*Atom, error) {
	var atom Atom
	err := xml.Unmarshal(data, &atom)
	return &atom, err
}

// extractDatesFromRSS extracts publication dates from RSS items.
func extractDatesFromRSS(items []Item) []time.Time {
	var dates []time.Time
	for _, item := range items {
		if date, err := parseDate(item.PubDate); err == nil {
			dates = append(dates, date)
		}
	}
	sort.Slice(dates, func(i, j int) bool {
		return dates[i].After(dates[j])
	})
	return dates
}

// extractDatesFromAtom extracts publication dates from Atom entries.
func extractDatesFromAtom(entries []Entry) []time.Time {
	var dates []time.Time
	for _, entry := range entries {
		dateStr := entry.Published
		if dateStr == "" {
			dateStr = entry.Updated
		}
		if date, err := parseDate(dateStr); err == nil {
			dates = append(dates, date)
		}
	}
	sort.Slice(dates, func(i, j int) bool {
		return dates[i].After(dates[j])
	})
	return dates
}

// parseDate attempts to parse various date formats.
func parseDate(dateStr string) (time.Time, error) {
	formats := []string{
		time.RFC1123,
		time.RFC1123Z,
		time.RFC3339,
		"Mon, 2 Jan 2006 15:04:05 -0700",
		"Mon, 2 Jan 2006 15:04:05 MST",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05-07:00",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unable to parse date: %s", dateStr)
}

// calculateAverageLength calculates average episode length in minutes.
func calculateAverageLength(items []Item) float64 {
	totalMinutes := 0.0
	count := 0

	for _, item := range items {
		if item.Duration != "" {
			if mins := parseDuration(item.Duration); mins > 0 {
				totalMinutes += mins
				count++
			}
		}
	}

	if count == 0 {
		return 30.0 // Default assumption: 30 minutes
	}
	return totalMinutes / float64(count)
}

// parseDuration parses duration string (supports HH:MM:SS, MM:SS, seconds).
func parseDuration(duration string) float64 {
	duration = strings.TrimSpace(duration)

	// If it's just a number, assume seconds
	if seconds, err := strconv.ParseFloat(duration, 64); err == nil {
		return seconds / 60.0
	}

	// Parse HH:MM:SS or MM:SS format
	parts := strings.Split(duration, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0
	}

	var hours, minutes, seconds float64
	var err error

	if len(parts) == 3 {
		hours, err = strconv.ParseFloat(parts[0], 64)
		if err != nil {
			return 0
		}
		minutes, err = strconv.ParseFloat(parts[1], 64)
		if err != nil {
			return 0
		}
		seconds, err = strconv.ParseFloat(parts[2], 64)
		if err != nil {
			return 0
		}
	} else {
		minutes, err = strconv.ParseFloat(parts[0], 64)
		if err != nil {
			return 0
		}
		seconds, err = strconv.ParseFloat(parts[1], 64)
		if err != nil {
			return 0
		}
	}

	return hours*60 + minutes + seconds/60
}

// calculateAverageDaysBetween calculates average days between episodes.
func calculateAverageDaysBetween(dates []time.Time) float64 {
	if len(dates) < 2 {
		return 7.0 // Default assumption: weekly
	}

	totalDays := 0.0
	for i := range len(dates) - 1 {
		days := dates[i].Sub(dates[i+1]).Hours() / 24
		totalDays += days
	}

	return totalDays / float64(len(dates)-1)
}

// calculateDaysSinceLatest calculates days since the most recent episode.
func calculateDaysSinceLatest(dates []time.Time) float64 {
	if len(dates) == 0 {
		return 365.0 // Default: very old
	}
	return time.Since(dates[0]).Hours() / 24
}

// calculateCompositeScore creates a single score from all metrics.
func calculateCompositeScore(stats PodcastStats) float64 {
	// return float64(stats.UnlistenedEpisodes)*stats.AvgEpisodeLengthMins - stats.AvgDaysBetween - stats.DaysSinceLatest
	return stats.AvgEpisodeLengthMins
}

// displayHistogram creates and displays a histogram of composite scores.
func displayHistogram(allStats []PodcastStats) {
	if len(allStats) == 0 {
		fmt.Println("No podcast data to display")
		return
	}

	// Find min and max scores for dynamic range
	minScore := allStats[0].CompositeScore
	maxScore := allStats[0].CompositeScore
	for _, stats := range allStats {
		if stats.CompositeScore < minScore {
			minScore = stats.CompositeScore
		}
		if stats.CompositeScore > maxScore {
			maxScore = stats.CompositeScore
		}
	}

	// Create 10 buckets with dynamic range
	buckets := make([]int, 10)
	bucketLabels := make([]string, 10)

	bucketSize := (maxScore - minScore) / 10.0
	if bucketSize == 0 {
		bucketSize = 1.0 // Prevent division by zero if all scores are the same
	}

	// Calculate the maximum width needed for formatting numbers
	maxWidth := 0
	for i := range 10 {
		low := minScore + float64(i)*bucketSize
		high := minScore + float64(i+1)*bucketSize
		lowStr := fmt.Sprintf("%.1f", low)
		highStr := fmt.Sprintf("%.1f", high)
		if len(lowStr) > maxWidth {
			maxWidth = len(lowStr)
		}
		if len(highStr) > maxWidth {
			maxWidth = len(highStr)
		}
	}

	for i := range 10 {
		low := minScore + float64(i)*bucketSize
		high := minScore + float64(i+1)*bucketSize
		bucketLabels[i] = fmt.Sprintf("%*s - %*s", maxWidth, fmt.Sprintf("%.1f", low), maxWidth, fmt.Sprintf("%.1f", high))
	}

	// Distribute scores into buckets
	for _, stats := range allStats {
		bucketIndex := int((stats.CompositeScore - minScore) / bucketSize)
		if bucketIndex >= 10 {
			bucketIndex = 9
		}
		if bucketIndex < 0 {
			bucketIndex = 0
		}
		buckets[bucketIndex]++
	}

	// Find max count for scaling
	maxCount := 0
	for _, count := range buckets {
		if count > maxCount {
			maxCount = count
		}
	}

	// Display histogram
	fmt.Printf("Composite Score Distribution (%d podcasts):\n\n", len(allStats))

	barWidth := 50
	for i, count := range buckets {
		label := bucketLabels[i]
		bucketNumber := 10 - i // Reverse the bucket numbering
		barLength := 0
		if maxCount > 0 {
			barLength = (count * barWidth) / maxCount
		}

		bar := strings.Repeat("█", barLength)
		fmt.Printf("%2d: %s |%s %d\n", bucketNumber, label, bar, count)
	}

	fmt.Printf("\nLegend: Higher scores = More episodes/longer/less frequent/older\n")
	fmt.Printf("Score = Unlistened Episodes * Avg Length (mins) + Avg Days Between + Days Since Latest\n")

	// Output podcasts sorted by bucket ranking and name
	fmt.Printf("\n=== PODCAST RANKINGS ===\n\n")

	// Create a slice to hold podcast info with bucket numbers
	type PodcastRanking struct {
		Title                string
		SortTitle            string
		Bucket               int
		UnlistenedEpisodes   int
		AvgEpisodeLengthMins float64
		AvgDaysBetween       float64
		DaysSinceLatest      float64
		PlaybackSpeed        float64
	}

	var rankings []PodcastRanking

	// Calculate bucket for each podcast and create rankings
	for _, stats := range allStats {
		bucketIndex := int((stats.CompositeScore - minScore) / bucketSize)
		if bucketIndex >= 10 {
			bucketIndex = 9
		}
		if bucketIndex < 0 {
			bucketIndex = 0
		}
		bucketNumber := 10 - bucketIndex // Reverse the bucket numbering

		title := stats.Title
		if title == "" {
			title = stats.URL // Fallback to URL if no title
		}

		rankings = append(rankings, PodcastRanking{
			Title:                title,
			SortTitle:            stats.SortTitle,
			Bucket:               bucketNumber,
			UnlistenedEpisodes:   stats.UnlistenedEpisodes,
			AvgEpisodeLengthMins: stats.AvgEpisodeLengthMins,
			AvgDaysBetween:       stats.AvgDaysBetween,
			DaysSinceLatest:      stats.DaysSinceLatest,
			PlaybackSpeed:        stats.PlaybackSpeed,
		})
	}

	// Sort by bucket (descending) then by title (ascending), using trimmed articles
	slices.SortStableFunc(rankings, func(a, b PodcastRanking) int {
		if a.Bucket == b.Bucket {
			ti := a.SortTitle
			tj := b.SortTitle
			if ti == "" { // Fallback for safety
				ti = trimArticles(a.Title)
			}
			if tj == "" {
				tj = trimArticles(b.Title)
			}
			return strings.Compare(ti, tj)
		}
		// Descending bucket: higher bucket first
		return cmp.Compare(b.Bucket, a.Bucket)
	})

	// Create a table for better display
	tbl := table.New().Wrap(true).
		Headers("Priority", "Title", "Unlistened", "Speed", "Adj Length", "Avg Days Between", "Days Since Latest").
		StyleFunc(func(row, col int) lipgloss.Style {
			style := lipgloss.NewStyle().Padding(0, 1)
			align := lipgloss.Left
			switch col {
			case 0, 2, 3, 4, 5, 6:
				align = lipgloss.Center
			case 1:
				style = style.Width(39) // Adjust width for title column
			}
			return style.Align(align)
		})
	lastRanking := 0
	for _, ranking := range rankings {
		if ranking.Bucket != lastRanking {
			// Add a separator row if the bucket changes
			if lastRanking != 0 {
				tbl.Row("--", "---", "--", "----", "----", "----", "----")
			}
		}
		tbl.Row(
			fmt.Sprintf("%2d", ranking.Bucket),
			dimArticleInTitle(ranking.Title),
			fmt.Sprintf("%d", ranking.UnlistenedEpisodes),
			fmt.Sprintf("%.1fx", ranking.PlaybackSpeed),
			fmt.Sprintf("%.1f mins", ranking.AvgEpisodeLengthMins),
			fmt.Sprintf("%.0f days", ranking.AvgDaysBetween),
			colorDaysSince(ranking.DaysSinceLatest, true),
		)
		lastRanking = ranking.Bucket // Capture the last ranking for display
	}
	// Display the sorted rankings
	fmt.Println(tbl)
}

// promptForTag prompts the user to select a tag filter
func promptForTag(bm *BackupManager) (string, error) {
	ctx := context.Background()

	// Get all tags
	tags, err := bm.GetTags(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get tags: %w", err)
	}

	if len(tags) == 0 {
		fmt.Println("No tags found in the database.")
		return "", nil
	}

	fmt.Println("\nAvailable tags:")
	fmt.Println("0. [All podcasts]")
	for i, tag := range tags {
		fmt.Printf("%d. %s\n", i+1, tag)
	}

	fmt.Print("\nSelect a tag (0 for all, or enter tag number): ")

	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return "", fmt.Errorf("failed to read input")
	}

	input := strings.TrimSpace(scanner.Text())
	if input == "" || input == "0" {
		return "", nil // No filter
	}

	choice, err := strconv.Atoi(input)
	if err != nil {
		return "", fmt.Errorf("invalid choice: %s", input)
	}

	if choice < 1 || choice > len(tags) {
		return "", fmt.Errorf("choice out of range: %d", choice)
	}

	return tags[choice-1], nil
}

// displayPodcastStat displays a single podcast's statistics in a consistent format
func displayPodcastStat(name string, author sql.NullString, feedUrl string, unplayedEpisodes int64, avgDuration sql.NullInt64, frequency sql.NullInt64, priority int64) {
	authorStr := "Unknown"
	if author.Valid {
		authorStr = author.String
	}

	avgDurationStr := "Unknown"
	if avgDuration.Valid {
		mins := avgDuration.Int64 / 1000 / 60 // Convert ms to minutes
		avgDurationStr = fmt.Sprintf("%d mins", mins)
	}

	frequencyStr := "Unknown"
	if frequency.Valid {
		frequencyStr = fmt.Sprintf("%d days", frequency.Int64)
	}

	fmt.Printf("Name: %s\n", dimArticleInTitle(name))
	fmt.Printf("  Author: %s\n", authorStr)
	fmt.Printf("  Feed URL: %s\n", feedUrl)
	fmt.Printf("  Unlistened: %d episodes\n", unplayedEpisodes)
	fmt.Printf("  Average Duration: %s\n", avgDurationStr)
	fmt.Printf("  Frequency: %s\n", frequencyStr)
	fmt.Printf("  Priority: %d\n", priority)
	fmt.Println()
}
