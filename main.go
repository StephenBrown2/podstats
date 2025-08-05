package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss/v2"
	"github.com/charmbracelet/lipgloss/v2/table"
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
	Text     string    `xml:"text,attr"`
	Title    string    `xml:"title,attr"`
	Type     string    `xml:"type,attr"`
	XMLURL   string    `xml:"xmlUrl,attr"`
	HTMLURL  string    `xml:"htmlUrl,attr"`
	Outlines []Outline `xml:"outline"`
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

	// Original CLI mode
	if len(os.Args) < 2 || len(os.Args) > 4 {
		fmt.Println("Usage:")
		fmt.Println("  podstats --tui                                          # Run TUI mode")
		fmt.Println("  podstats [--cached[=all|speed|unlistened]] <opml_file> # Run CLI mode")
		os.Exit(1)
	}

	var useCachedSpeed bool
	var useCachedUnlistened bool
	var opmlFile string

	if len(os.Args) >= 3 && strings.HasPrefix(os.Args[1], "--cached") {
		// Parse the cached parameter
		cachedParam := "all" // default
		if strings.Contains(os.Args[1], "=") {
			parts := strings.Split(os.Args[1], "=")
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
			fmt.Println("Usage: podstats [--cached[=all|speed|unlistened]] <opml_file>")
			os.Exit(1)
		}

		opmlFile = os.Args[2]
	} else {
		opmlFile = os.Args[1]
	}

	cacheFile := "cache.json"

	// Load cache
	cache := loadCache(cacheFile)

	// Parse OPML file
	fmt.Printf("Parsing OPML file: %s\n", opmlFile)
	podcasts, err := parseOPML(opmlFile)
	if err != nil {
		log.Fatalf("Error parsing OPML: %v", err)
	}

	fmt.Printf("Found %d podcasts in OPML file\n", len(podcasts))
	fmt.Printf("Cache file: %s\n\n", cacheFile)

	// Analyze each podcast
	var allStats []PodcastStats
	for i, podcast := range podcasts {
		// Get the display title (fallback to text if title is empty)
		displayTitle := podcast.Title
		if displayTitle == "" {
			displayTitle = podcast.Text
		}
		displayTitle = strings.ReplaceAll(displayTitle, "&amp;", "&") // Clean up title
		fmt.Printf("[%d/%d] Analyzing: %s\n", i+1, len(podcasts), displayTitle)
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
		fmt.Printf("  Days since latest: %.1f\n", stats.DaysSinceLatest)
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
	return float64(stats.UnlistenedEpisodes)*stats.AvgEpisodeLengthMins + stats.AvgDaysBetween + stats.DaysSinceLatest
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
			Bucket:               bucketNumber,
			UnlistenedEpisodes:   stats.UnlistenedEpisodes,
			AvgEpisodeLengthMins: stats.AvgEpisodeLengthMins,
			AvgDaysBetween:       stats.AvgDaysBetween,
			DaysSinceLatest:      stats.DaysSinceLatest,
			PlaybackSpeed:        stats.PlaybackSpeed,
		})
	}

	// Sort by bucket (descending) then by title (ascending)
	sort.Slice(rankings, func(i, j int) bool {
		if rankings[i].Bucket == rankings[j].Bucket {
			return rankings[i].Title < rankings[j].Title
		}
		return rankings[i].Bucket > rankings[j].Bucket
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
			ranking.Title,
			fmt.Sprintf("%d", ranking.UnlistenedEpisodes),
			fmt.Sprintf("%.1fx", ranking.PlaybackSpeed),
			fmt.Sprintf("%.1f mins", ranking.AvgEpisodeLengthMins),
			fmt.Sprintf("%.0f days", ranking.AvgDaysBetween),
			fmt.Sprintf("%.0f days", ranking.DaysSinceLatest),
		)
		lastRanking = ranking.Bucket // Capture the last ranking for display
	}
	// Display the sorted rankings
	fmt.Println(tbl)
}
