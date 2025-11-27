// Package main provides podcast statistics analysis and TUI.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"charm.land/huh/v2"
)

var (
	ErrUnableToParseAsFeed = errors.New("unable to parse as RSS or Atom feed")
	ErrNoEpisodesFound     = errors.New("no episodes found")
	ErrInvalidNumber       = errors.New("please enter a valid number")
	ErrInvalidPositiveNum  = errors.New("please enter a positive number")
)

// Default playback speed used when no cached or backup value is available.
const defaultSpeed = 2.0

// Reusable prompt helpers to reduce duplication in CLI mode.
func promptConfirm(title, affirmative, negative string, dest *bool) error {
	return huh.NewConfirm().
		Title(title).
		Affirmative(affirmative).
		Negative(negative).
		Value(dest).
		Run()
}

func promptInt(title string, dest *string) error {
	return huh.NewInput().
		Title(title).
		Value(dest).
		Validate(func(s string) error {
			if _, err := strconv.Atoi(s); err != nil {
				return ErrInvalidNumber
			}

			return nil
		}).
		Run()
}

func promptSpeed(def float64, dest *string) error {
	return huh.NewInput().
		Title(fmt.Sprintf("What playback speed do you use? (default %.1f)", def)).
		Placeholder(fmt.Sprintf("%.1f", def)).
		Value(dest).
		Validate(func(s string) error {
			if s == "" {
				return nil
			}

			speed, err := strconv.ParseFloat(s, 64)
			if err != nil || speed <= 0 {
				return ErrInvalidPositiveNum
			}

			return nil
		}).
		Run()
}

// Helper to compute unlistened count with optional cached flow.
func resolveUnlistenedCount(cache *CacheData, url string, useCached bool, defaultCount int) int {
	cachedCount, found := getCachedUnlistenedCount(cache, url)
	if !found {
		var input string
		if err := promptInt(fmt.Sprintf("How many episodes are unlistened? (default %d)", defaultCount), &input); err == nil && input != "" {
			val, _ := strconv.Atoi(input)

			return val
		}

		return defaultCount
	}

	fmt.Printf("  Cached unlistened episodes: %d\n", cachedCount)

	if useCached {
		fmt.Printf("  Using cached unlistened count (--cached=unlistened or --cached=all)\n")

		return cachedCount
	}

	var useCachedAns bool
	if err := promptConfirm(
		fmt.Sprintf("Use cached unlistened count of %d?", cachedCount),
		"Yes",
		"No, enter new value",
		&useCachedAns,
	); err != nil {
		fmt.Printf("  Error reading input, using cached value (%d)\n", cachedCount)

		return cachedCount
	}

	if useCachedAns {
		return cachedCount
	}

	var input string
	if err := promptInt("How many episodes are unlistened?", &input); err != nil {
		fmt.Printf("  Error reading input, using cached value (%d)\n", cachedCount)

		return cachedCount
	}

	if input == "" {
		return defaultCount
	}

	val, _ := strconv.Atoi(input)

	return val
}

func resolvePlaybackSpeedWithCache(cachedSpeed float64, hasCache bool, useCached bool) float64 {
	if !hasCache {
		var input string
		if err := promptSpeed(defaultSpeed, &input); err != nil {
			fmt.Printf("  Error reading input, using default (%.1fx)\n", defaultSpeed)

			return defaultSpeed
		}

		if input == "" {
			fmt.Printf("  Using default playback speed (%.1fx)\n", defaultSpeed)

			return defaultSpeed
		}

		val, _ := strconv.ParseFloat(input, 64)

		return val
	}

	fmt.Printf("  Cached playback speed: %.1fx\n", cachedSpeed)

	if useCached {
		fmt.Printf("  Using cached playback speed (--cached=speed or --cached=all)\n")

		return cachedSpeed
	}

	var useCachedAns bool
	if err := promptConfirm(
		fmt.Sprintf("Use cached playback speed of %.1fx?", cachedSpeed),
		"Yes",
		"No, enter new speed",
		&useCachedAns,
	); err != nil {
		fmt.Printf("  Error reading input, using cached speed (%.1fx)\n", cachedSpeed)

		return cachedSpeed
	}

	if useCachedAns {
		return cachedSpeed
	}

	var input string
	if err := promptSpeed(defaultSpeed, &input); err != nil {
		fmt.Printf("  Error reading input, using default (%.1fx)\n", defaultSpeed)

		return defaultSpeed
	}

	if input == "" {
		fmt.Printf("  Using default playback speed (%.1fx)\n", defaultSpeed)

		return defaultSpeed
	}

	val, _ := strconv.ParseFloat(input, 64)

	return val
}

// TUI-compatible version: non-interactive; uses cache/defaults only.
func analyzePodcastTUI( //nolint:funlen
	podcast Outline,
	cache *CacheData,
	useCachedUnlistened, useCachedSpeed bool,
	speedSettings map[string]float64,
	defSpeed float64,
) (PodcastStats, error) {
	stats := PodcastStats{Title: podcast.Title, SortTitle: podcast.SortTitle, URL: podcast.XMLURL}
	if stats.Title == "" {
		stats.Title = podcast.Text
	}

	stats.Title = strings.ReplaceAll(stats.Title, "&amp;", "&") // Clean up title

	// Fetch the RSS/Atom feed with proper user agent
	client := &http.Client{}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, podcast.XMLURL, nil)
	if err != nil {
		return stats, err
	}

	// Set a browser-like user agent to avoid being blocked
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return stats, err
	}

	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return stats, err
	}

	// Try to parse as RSS first, then Atom
	var (
		items    []Item
		dates    []time.Time
		hasValue bool
	)
	if rss, err := parseRSS(data); err == nil {
		items = rss.Channel.Items
		dates = extractDatesFromRSS(items)
		hasValue = rss.Channel.Value != nil
	} else if atom, err := parseAtom(data); err == nil {
		dates = extractDatesFromAtom(atom.Entries)
	} else {
		return stats, ErrUnableToParseAsFeed
	}

	if len(dates) == 0 {
		return stats, ErrNoEpisodesFound
	}

	if hasValue && !strings.HasPrefix(stats.Title, "⚡") {
		stats.Title = "⚡ " + stats.Title
	}

	// Handle unlistened count: respect cache preference from config
	unlistenedCount := len(dates)

	if cachedCount, found := getCachedUnlistenedCount(cache, podcast.XMLURL); found {
		if useCachedUnlistened {
			unlistenedCount = cachedCount
		} else {
			// Use all episodes if user chose not to use cached value
			unlistenedCount = len(dates)
		}
	} else {
		updateCache(cache, podcast.XMLURL, unlistenedCount)
	}

	// Validate input
	if unlistenedCount < 0 {
		unlistenedCount = 0
	} else if unlistenedCount > len(dates) {
		unlistenedCount = len(dates)
	}

	// Handle playback speed: respect cache preference and check speedSettings
	playbackSpeed := defSpeed
	if speed, found := speedSettings[podcast.XMLURL]; found {
		// Use speed from backup database if available
		playbackSpeed = speed
	} else if cachedSpeed, found := getCachedPlaybackSpeed(cache, podcast.XMLURL); found {
		if useCachedSpeed {
			playbackSpeed = cachedSpeed
		}
		// Otherwise use default
	}

	stats.UnlistenedEpisodes = unlistenedCount
	stats.PlaybackSpeed = playbackSpeed
	stats.AvgEpisodeLengthMins = calculateAverageLength(items) / playbackSpeed // Adjust for playback speed
	stats.AvgDaysBetween = calculateAverageDaysBetween(dates)
	stats.DaysSinceLatest = calculateDaysSinceLatest(dates)
	stats.CompositeScore = calculateCompositeScore(stats)

	return stats, nil
}

// CLI version: interactive prompts using huh helpers.
func analyzePodcastCLI( //nolint:funlen
	podcast Outline,
	cache *CacheData,
	useCachedUnlistened, useCachedSpeed bool,
) (PodcastStats, error) {
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

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, podcast.XMLURL, nil)
	if err != nil {
		return stats, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return stats, err
	}

	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return stats, err
	}

	// Try to parse as RSS first, then Atom
	var (
		items    []Item
		dates    []time.Time
		hasValue bool
	)

	if rss, err := parseRSS(data); err == nil {
		items = rss.Channel.Items
		dates = extractDatesFromRSS(items)
		hasValue = rss.Channel.Value != nil
	} else if atom, err := parseAtom(data); err == nil {
		dates = extractDatesFromAtom(atom.Entries)
	} else {
		return stats, ErrUnableToParseAsFeed
	}

	if len(dates) == 0 {
		return stats, ErrNoEpisodesFound
	}

	// Update title with lightning bolt if podcast supports value-for-value
	if hasValue && !strings.HasPrefix(stats.Title, "⚡") {
		stats.Title = "⚡ " + stats.Title
	}

	fmt.Printf("  Total episodes in feed: %d\n", len(dates))

	unlistenedCount := resolveUnlistenedCount(cache, podcast.XMLURL, useCachedUnlistened, len(dates))
	if unlistenedCount < 0 {
		unlistenedCount = 0
	} else if unlistenedCount > len(dates) {
		unlistenedCount = len(dates)
	}

	updateCache(cache, podcast.XMLURL, unlistenedCount)

	cachedSpeed, hasCache := getCachedPlaybackSpeed(cache, podcast.XMLURL)
	playbackSpeed := resolvePlaybackSpeedWithCache(cachedSpeed, hasCache, useCachedSpeed)

	updateCacheWithPlaybackSpeed(cache, podcast.XMLURL, unlistenedCount, playbackSpeed)

	stats.UnlistenedEpisodes = unlistenedCount
	stats.PlaybackSpeed = playbackSpeed
	stats.AvgEpisodeLengthMins = calculateAverageLength(items) / playbackSpeed // Adjust for playback speed
	stats.AvgDaysBetween = calculateAverageDaysBetween(dates)
	stats.DaysSinceLatest = calculateDaysSinceLatest(dates)
	stats.CompositeScore = calculateCompositeScore(stats)

	return stats, nil
}
