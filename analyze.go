package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// TUI-compatible version of analyzePodcast that doesn't require user input.
func analyzePodcastTUI(podcast Outline, cache *CacheData, useCachedUnlistened, useCachedSpeed bool, speedSettings map[string]float64, defaultSpeed float64) (PodcastStats, error) {
	stats := PodcastStats{
		Title: podcast.Title,
		URL:   podcast.XMLURL,
	}

	if stats.Title == "" {
		stats.Title = podcast.Text
	}
	stats.Title = strings.ReplaceAll(stats.Title, "&amp;", "&") // Clean up title

	// Fetch the RSS/Atom feed with proper user agent
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodGet, podcast.XMLURL, nil)
	if err != nil {
		return stats, err
	}

	// Set a browser-like user agent to avoid being blocked
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return stats, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return stats, err
	}

	// Try to parse as RSS first, then Atom
	var items []Item
	var dates []time.Time
	var hasValue bool

	if rss, err := parseRSS(data); err == nil {
		items = rss.Channel.Items
		dates = extractDatesFromRSS(items)
		hasValue = rss.Channel.Value != nil
	} else if atom, err := parseAtom(data); err == nil {
		dates = extractDatesFromAtom(atom.Entries)
	} else {
		return stats, fmt.Errorf("unable to parse as RSS or Atom feed")
	}

	if len(dates) == 0 {
		return stats, fmt.Errorf("no episodes found")
	}

	// Update title with lightning bolt if podcast supports value-for-value
	if hasValue && !strings.HasPrefix(stats.Title, "⚡") {
		stats.Title = "⚡" + stats.Title
	}

	// Handle unlistened episodes count
	var unlistenedCount int
	if cachedCount, found := getCachedUnlistenedCount(cache, podcast.XMLURL); found && useCachedUnlistened {
		unlistenedCount = cachedCount
	} else if cachedCount, found := getCachedUnlistenedCount(cache, podcast.XMLURL); found {
		// If not using cached but cache exists, use it as default
		unlistenedCount = cachedCount
	} else {
		// Default to all episodes unlistened if no cache
		unlistenedCount = len(dates)
		updateCache(cache, podcast.XMLURL, unlistenedCount)
	}

	// Validate input
	if unlistenedCount < 0 {
		unlistenedCount = 0
	} else if unlistenedCount > len(dates) {
		unlistenedCount = len(dates)
	}

	// Handle playback speed
	playbackSpeed := defaultSpeed // Use provided default speed instead of hardcoded 2.0
	if cachedSpeed, found := getCachedPlaybackSpeed(cache, podcast.XMLURL); found && useCachedSpeed {
		playbackSpeed = cachedSpeed
	} else if cachedSpeed, found := getCachedPlaybackSpeed(cache, podcast.XMLURL); found {
		// If not using cached but cache exists, use it
		playbackSpeed = cachedSpeed
	} else if speedSettings != nil {
		// Use speed from backup settings if available
		if backupSpeed, found := speedSettings[podcast.XMLURL]; found {
			playbackSpeed = backupSpeed
		}
		// Note: don't update cache with backup speed as it's more authoritative
	} else {
		// Fallback to default and update cache
		playbackSpeed = defaultSpeed
		updateCacheWithPlaybackSpeed(cache, podcast.XMLURL, unlistenedCount, playbackSpeed)
	}

	// Calculate statistics
	stats.UnlistenedEpisodes = unlistenedCount
	stats.PlaybackSpeed = playbackSpeed
	stats.AvgEpisodeLengthMins = calculateAverageLength(items) / playbackSpeed // Adjust for playback speed
	stats.AvgDaysBetween = calculateAverageDaysBetween(dates)
	stats.DaysSinceLatest = calculateDaysSinceLatest(dates)
	stats.CompositeScore = calculateCompositeScore(stats)

	return stats, nil
}

// Original analyzePodcast function remains unchanged for CLI mode.
func analyzePodcastCLI(podcast Outline, cache *CacheData, useCachedUnlistened, useCachedSpeed bool) (PodcastStats, error) {
	stats := PodcastStats{
		Title:     podcast.Title,
		SortTitle: podcast.SortTitle,
		URL:       podcast.XMLURL,
	}

	if stats.Title == "" {
		stats.Title = podcast.Text
	}
	stats.Title = strings.ReplaceAll(stats.Title, "&amp;", "&") // Clean up title

	// Fetch the RSS/Atom feed with proper user agent
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodGet, podcast.XMLURL, nil)
	if err != nil {
		return stats, err
	}

	// Set a browser-like user agent to avoid being blocked
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return stats, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return stats, err
	}

	// Try to parse as RSS first, then Atom
	var items []Item
	var dates []time.Time
	var hasValue bool

	if rss, err := parseRSS(data); err == nil {
		items = rss.Channel.Items
		dates = extractDatesFromRSS(items)
		hasValue = rss.Channel.Value != nil
	} else if atom, err := parseAtom(data); err == nil {
		dates = extractDatesFromAtom(atom.Entries)
	} else {
		return stats, fmt.Errorf("unable to parse as RSS or Atom feed")
	}

	if len(dates) == 0 {
		return stats, fmt.Errorf("no episodes found")
	}

	// Update title with lightning bolt if podcast supports value-for-value
	if hasValue && !strings.HasPrefix(stats.Title, "⚡") {
		stats.Title = "⚡ " + stats.Title
	}

	// Check cache for unlistened episodes count
	fmt.Printf("  Total episodes in feed: %d\n", len(dates))

	var unlistenedCount int
	if cachedCount, found := getCachedUnlistenedCount(cache, podcast.XMLURL); found {
		fmt.Printf("  Cached unlistened episodes: %d\n", cachedCount)

		if useCachedUnlistened {
			fmt.Printf("  Using cached unlistened count (--cached=unlistened or --cached=all specified)\n")
			unlistenedCount = cachedCount
		} else {
			fmt.Printf("  Use cached value? (y/n/new_number): ")

			reader := bufio.NewReader(os.Stdin)
			input, err := reader.ReadString('\n')
			if err != nil {
				fmt.Printf("  Error reading input, using cached value (%d)\n", cachedCount)
				unlistenedCount = cachedCount
			} else {
				input = strings.TrimSpace(input)
				if input == "" || strings.ToLower(input) == "y" || strings.ToLower(input) == "yes" {
					unlistenedCount = cachedCount
				} else if strings.ToLower(input) == "n" || strings.ToLower(input) == "no" {
					// Ask for new unlistened episode count
					fmt.Printf("  How many episodes have you NOT listened to? ")
					reader2 := bufio.NewReader(os.Stdin)
					episodeInput, err := reader2.ReadString('\n')
					if err != nil {
						fmt.Printf("  Error reading input, using total episodes (%d)\n", len(dates))
						unlistenedCount = len(dates)
					} else {
						episodeInput = strings.TrimSpace(episodeInput)
						if count, err := strconv.Atoi(episodeInput); err == nil {
							unlistenedCount = count
							updateCache(cache, podcast.XMLURL, unlistenedCount)
						} else {
							fmt.Printf("  Invalid input, using total episodes (%d)\n", len(dates))
							unlistenedCount = len(dates)
						}
					}
				} else if newCount, err := strconv.Atoi(input); err == nil {
					unlistenedCount = newCount
					updateCache(cache, podcast.XMLURL, unlistenedCount)
				} else {
					fmt.Printf("  Invalid input, using cached value (%d)\n", cachedCount)
					unlistenedCount = cachedCount
				}
			}
		}
	} else {
		fmt.Printf("  How many episodes have you NOT listened to? ")
		reader := bufio.NewReader(os.Stdin)
		episodeInput, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("  Error reading input, using total episodes (%d)\n", len(dates))
			unlistenedCount = len(dates)
		} else {
			episodeInput = strings.TrimSpace(episodeInput)
			if count, err := strconv.Atoi(episodeInput); err == nil {
				unlistenedCount = count
			} else {
				fmt.Printf("  Invalid input, using total episodes (%d)\n", len(dates))
				unlistenedCount = len(dates)
			}
		}
		updateCache(cache, podcast.XMLURL, unlistenedCount)
	}

	// Validate input
	if unlistenedCount < 0 {
		unlistenedCount = 0
	} else if unlistenedCount > len(dates) {
		unlistenedCount = len(dates)
	}

	// Handle playback speed
	playbackSpeed := 2.0
	if cachedSpeed, found := getCachedPlaybackSpeed(cache, podcast.XMLURL); found {
		fmt.Printf("  Cached playback speed: %.1fx\n", cachedSpeed)

		if useCachedSpeed {
			fmt.Printf("  Using cached playback speed (--cached=speed or --cached=all specified)\n")
			playbackSpeed = cachedSpeed
		} else {
			fmt.Printf("  Use cached playback speed? (y/n/new_speed): ")

			reader := bufio.NewReader(os.Stdin)
			input, err := reader.ReadString('\n')
			if err != nil {
				fmt.Printf("  Error reading input, using cached speed (%.1fx)\n", cachedSpeed)
				playbackSpeed = cachedSpeed
			} else {
				input = strings.TrimSpace(input)
				if input == "" || strings.ToLower(input) == "y" || strings.ToLower(input) == "yes" {
					playbackSpeed = cachedSpeed
				} else if strings.ToLower(input) == "n" || strings.ToLower(input) == "no" {
					// Ask for new playback speed
					fmt.Printf("  What playback speed do you use for this podcast? (default 2.0): ")
					reader2 := bufio.NewReader(os.Stdin)
					speedInput, err := reader2.ReadString('\n')
					if err != nil {
						fmt.Printf("  Error reading input, using default (2.0x)\n")
						playbackSpeed = 2.0
					} else {
						speedInput = strings.TrimSpace(speedInput)
						if speedInput == "" {
							fmt.Printf("  Using default playback speed (2.0x)\n")
							playbackSpeed = 2.0
						} else if speed, err := strconv.ParseFloat(speedInput, 64); err == nil && speed > 0 {
							playbackSpeed = speed
						} else {
							fmt.Printf("  Invalid input, using default (2.0x)\n")
							playbackSpeed = 2.0
						}
					}
					updateCacheWithPlaybackSpeed(cache, podcast.XMLURL, unlistenedCount, playbackSpeed)
				} else if newSpeed, err := strconv.ParseFloat(input, 64); err == nil && newSpeed > 0 {
					playbackSpeed = newSpeed
					updateCacheWithPlaybackSpeed(cache, podcast.XMLURL, unlistenedCount, playbackSpeed)
				} else {
					fmt.Printf("  Invalid input, using cached speed (%.1fx)\n", cachedSpeed)
					playbackSpeed = cachedSpeed
				}
			}
		}
	} else {
		fmt.Printf("  What playback speed do you use for this podcast? (default 2.0): ")
		reader := bufio.NewReader(os.Stdin)
		speedInput, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("  Error reading input, using default (2.0x)\n")
			playbackSpeed = 2.0
		} else {
			speedInput = strings.TrimSpace(speedInput)
			if speedInput == "" {
				fmt.Printf("  Using default playback speed (2.0x)\n")
				playbackSpeed = 2.0
			} else if speed, err := strconv.ParseFloat(speedInput, 64); err == nil && speed > 0 {
				playbackSpeed = speed
			} else {
				fmt.Printf("  Invalid input, using default (2.0x)\n")
				playbackSpeed = 2.0
			}
		}
		updateCacheWithPlaybackSpeed(cache, podcast.XMLURL, unlistenedCount, playbackSpeed)
	}

	// Calculate statistics
	stats.UnlistenedEpisodes = unlistenedCount
	stats.PlaybackSpeed = playbackSpeed
	stats.AvgEpisodeLengthMins = calculateAverageLength(items) / playbackSpeed // Adjust for playback speed
	stats.AvgDaysBetween = calculateAverageDaysBetween(dates)
	stats.DaysSinceLatest = calculateDaysSinceLatest(dates)
	stats.CompositeScore = calculateCompositeScore(stats)

	return stats, nil
}
