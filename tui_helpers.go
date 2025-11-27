package main

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
)

// Helper function to calculate bucket number for a given score.
func (m *tuiModel) calculateBucketNumber(score float64) int {
	if len(m.histogramModel.stats) == 0 {
		return 1
	}

	// Find min and max scores (same logic as generateHistogram)
	minScore := m.histogramModel.stats[0].CompositeScore

	maxScore := m.histogramModel.stats[0].CompositeScore
	for _, stat := range m.histogramModel.stats {
		if stat.CompositeScore < minScore {
			minScore = stat.CompositeScore
		}

		if stat.CompositeScore > maxScore {
			maxScore = stat.CompositeScore
		}
	}

	bucketSize := (maxScore - minScore) / 10.0
	if bucketSize == 0 {
		bucketSize = 1.0
	}

	bucketIndex := int((score - minScore) / bucketSize)
	if bucketIndex >= 10 {
		bucketIndex = 9
	}

	if bucketIndex < 0 {
		bucketIndex = 0
	}

	// Convert to display bucket number (reversed)
	return 10 - bucketIndex
}

// calculateFilePickerHeight returns the height available for the file picker.
func (m *tuiModel) calculateFilePickerHeight() int {
	// Account for header (title + subtitle) and footer (help text + newline before it)
	usedHeight := fileSelectHeaderLines + fileSelectFooterLines + 1

	return max(5, m.height-usedHeight)
}

// calculateTagSelectListHeight returns the height available for the tag selection list.
func (m *tuiModel) calculateTagSelectListHeight() int {
	// The list includes its own title, so we only subtract footer lines + extra newline
	usedHeight := tagSelectFooterLines + 1

	return max(5, m.height-usedHeight)
}

// calculateResultsListHeight returns the height available for the results list.
func (m *tuiModel) calculateResultsListHeight() int {
	// The list includes its own title (1 line), plus we have footer lines.
	// Subtract extra lines for the newline before the help text.
	usedHeight := resultsFooterLines + 2

	return max(5, m.height-usedHeight)
}

// Sort and update the results list based on current sort mode.
func (m *tuiModel) sortAndUpdateResults() {
	stats := make([]PodcastStats, len(m.resultsModel.stats))
	copy(stats, m.resultsModel.stats)

	switch m.resultsModel.sortMode {
	case sortByScore:
		// Sort by composite score descending (highest scores first)
		slices.SortStableFunc(stats, func(a, b PodcastStats) int {
			return cmp.Compare(b.CompositeScore, a.CompositeScore)
		})
	case sortByName:
		// Sort alphabetically by title
		slices.SortStableFunc(stats, func(a, b PodcastStats) int {
			return strings.Compare(a.SortTitle, b.SortTitle)
		})
	case sortByPriorityAsc:
		// Sort by priority ascending (bucket 1, 2, 3... 10), then by name
		slices.SortStableFunc(stats, func(a, b PodcastStats) int {
			bucketA := m.calculateBucketNumber(a.CompositeScore)
			bucketB := m.calculateBucketNumber(b.CompositeScore)

			if bucketA == bucketB {
				// Secondary sort by name within same priority
				return strings.Compare(a.SortTitle, b.SortTitle)
			}

			return cmp.Compare(bucketA, bucketB)
		})
	case sortByPriorityDesc:
		// Sort by priority descending (bucket 10, 9, 8... 1), then by name
		slices.SortStableFunc(stats, func(a, b PodcastStats) int {
			bucketA := m.calculateBucketNumber(a.CompositeScore)

			bucketB := m.calculateBucketNumber(b.CompositeScore)
			if bucketA == bucketB {
				// Secondary sort by name within same priority
				return strings.Compare(a.SortTitle, b.SortTitle)
			}

			return cmp.Compare(bucketB, bucketA)
		})
	}

	m.resultsModel.stats = stats

	// Update the list items
	items := make([]list.Item, len(stats))
	for i, stat := range stats {
		items[i] = podcastItem{stat, m.calculateBucketNumber(stat.CompositeScore)}
	}

	m.resultsModel.list.SetItems(items)
}

// User input handling with conditional navigation; splitting would fragment flow.
func (m *tuiModel) updateConfigScreen(msg tea.Msg) tea.Cmd {
	// Handle special keys before passing to form
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "q", "ctrl+c":
			return tea.Quit
		case "down":
			m.configModel.form.NextField()
		case "up":
			m.configModel.form.PrevField()
		}
	}

	form, cmd := m.configModel.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.configModel.form = f
	}

	// Check if form is complete
	if m.configModel.form.State == huh.StateCompleted {
		// For backup files, go to tag selection screen
		if m.isBackupFile {
			m.screen = tagSelectScreen
			// Start spinner while loading tags and speeds
			return tea.Batch(m.loadTags(), m.loadSpeedSettings(), m.tagSelectModel.spinner.Tick)
		}

		// For OPML files, go directly to processing
		m.screen = processingScreen
		// Ensure progress bar has proper width
		if m.width > 0 {
			m.processingModel.progress.SetWidth(m.width - 4)
		}

		return m.startProcessing()
	}

	return cmd
}

func (m *tuiModel) updateTagSelectScreen(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) { //nolint:gocritic,revive // singleCaseSwitch is idiomatic for Bubble Tea message handling
	case tea.KeyMsg:
		switch msg.String() {
		case keyEnter:
			// Get selected tag and proceed to processing
			if len(m.tagSelectModel.tags) == 0 {
				// Still loading; ignore Enter until tags are ready
				return nil
			}

			if selected := m.tagSelectModel.list.SelectedItem(); selected != nil {
				item, ok := selected.(tagItem)
				if !ok {
					return nil
				}

				if item.isAllOption {
					m.tagSelectModel.selectedTag = ""
				} else {
					m.tagSelectModel.selectedTag = item.name
				}

				m.screen = processingScreen
				// Ensure progress bar has proper width
				if m.width > 0 {
					m.processingModel.progress.SetWidth(m.width - 4)
				}

				return m.startProcessing()
			}
		case keyEsc:
			m.screen = configScreen
		}
	}

	var cmd tea.Cmd
	if len(m.tagSelectModel.tags) == 0 {
		// Tags still loading: keep spinner animating
		m.tagSelectModel.spinner, cmd = m.tagSelectModel.spinner.Update(msg)

		return cmd
	}

	m.tagSelectModel.list, cmd = m.tagSelectModel.list.Update(msg)

	return cmd
}

func (m *tuiModel) updateResultsScreen(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd

	switch msg := msg.(type) { //nolint:gocritic,revive // singleCaseSwitch is idiomatic for Bubble Tea message handling
	case tea.KeyMsg:
		switch msg.String() {
		case keyEsc:
			// Check if the list is filtered - if so, clear filter instead of going back
			if m.resultsModel.list.IsFiltered() {
				m.resultsModel.list.ResetFilter()

				return nil
			}

			// No filter active, go back to file select
			m.screen = fileSelectScreen

			return nil
		case keyEnter:
			// Show detail view for selected podcast
			if m.resultsModel.list.SelectedItem() != nil {
				item, ok := m.resultsModel.list.SelectedItem().(podcastItem)
				if !ok {
					return nil
				}

				m.detailModel = newDetailModel(item.PodcastStats)
				m.screen = podcastDetailScreen
			}
		case "h":
			// Show histogram view
			m.histogramModel = m.generateHistogram(m.resultsModel.stats)
			m.screen = histogramScreen
		case "s":
			// Cycle through sort modes
			m.resultsModel.sortMode = (m.resultsModel.sortMode + 1) % 4
			m.sortAndUpdateResults()
		case "n":
			// Sort by name
			m.resultsModel.sortMode = sortByName
			m.sortAndUpdateResults()
		case "p":
			// Sort by priority (ascending)
			m.resultsModel.sortMode = sortByPriorityAsc
			m.sortAndUpdateResults()
		case "P":
			// Sort by priority (descending)
			m.resultsModel.sortMode = sortByPriorityDesc
			m.sortAndUpdateResults()
		case "u":
			// Update backup file (only available for backup files)
			if m.isBackupFile {
				return m.prepareBackupUpdate()
			}
		}
	}

	m.resultsModel.list, cmd = m.resultsModel.list.Update(msg)

	return cmd
}

func (m *tuiModel) updateDetailScreen(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(m.detailModel.inputs))

	switch msg := msg.(type) { //nolint:gocritic,revive // singleCaseSwitch is idiomatic for Bubble Tea message handling
	case tea.KeyMsg:
		switch msg.String() {
		case keyEnter:
			// Save changes and go back
			if err := m.saveDetailChanges(); err != nil {
				m.err = err
			} else {
				m.screen = resultsScreen
			}

		case "tab", keyShiftTab, "up", "down":
			if msg.String() == "up" || msg.String() == keyShiftTab {
				m.detailModel.focused--
			} else {
				m.detailModel.focused++
			}

			if m.detailModel.focused > len(m.detailModel.inputs)-1 {
				m.detailModel.focused = 0
			} else if m.detailModel.focused < 0 {
				m.detailModel.focused = len(m.detailModel.inputs) - 1
			}

			for i := range m.detailModel.inputs {
				if i == m.detailModel.focused {
					m.detailModel.inputs[i].Focus()
				} else {
					m.detailModel.inputs[i].Blur()
				}
			}
		}
	}

	for i := range m.detailModel.inputs {
		var cmd tea.Cmd

		m.detailModel.inputs[i], cmd = m.detailModel.inputs[i].Update(msg)
		cmds = append(cmds, cmd)
	}

	return tea.Batch(cmds...)
}

func (m *tuiModel) updateHistogramScreen(msg tea.Msg) {
	switch msg := msg.(type) { //nolint:gocritic,revive // singleCaseSwitch is idiomatic for Bubble Tea message handling
	case tea.KeyMsg:
		if msg.String() == "r" {
			// Return to results screen
			m.screen = resultsScreen
		}
	}
}

// Histogram calculation logic; splitting would reduce cohesion.
//
//nolint:funlen,revive // Receiver unused; keep as method for cohesion
func (m *tuiModel) generateHistogram(stats []PodcastStats) histogramModel {
	if len(stats) == 0 {
		return histogramModel{}
	}

	// Find min and max scores for dynamic range
	minScore := stats[0].CompositeScore

	maxScore := stats[0].CompositeScore
	for _, stat := range stats {
		if stat.CompositeScore < minScore {
			minScore = stat.CompositeScore
		}

		if stat.CompositeScore > maxScore {
			maxScore = stat.CompositeScore
		}
	}

	// Create 10 buckets with dynamic range
	buckets := make([]int, 10)
	labels := make([]string, 10)

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
		labels[i] = fmt.Sprintf("%*s - %*s", maxWidth, fmt.Sprintf("%.1f", low), maxWidth, fmt.Sprintf("%.1f", high))
	}

	// Distribute scores into buckets
	for _, stat := range stats {
		bucketIndex := int((stat.CompositeScore - minScore) / bucketSize)
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

	return histogramModel{
		stats:    stats,
		buckets:  buckets,
		labels:   labels,
		maxCount: maxCount,
	}
}

func (m *tuiModel) saveDetailChanges() error {
	// Parse the new values from inputs
	unlistedStr := m.detailModel.inputs[0].Value()
	speedStr := m.detailModel.inputs[1].Value()

	unlistened, err := strconv.Atoi(unlistedStr)
	if err != nil {
		return fmt.Errorf("invalid unlistened count: %s", unlistedStr) //nolint:err113 // error contains user input
	}

	speed, err := strconv.ParseFloat(speedStr, 64)
	if err != nil {
		return fmt.Errorf("invalid playback speed: %s", speedStr) //nolint:err113 // error contains user input
	}

	if speed <= 0 {
		return errors.New("playback speed must be positive") //nolint:err113 // simple validation error
	}

	if unlistened < 0 {
		return errors.New("unlistened count cannot be negative") //nolint:err113 // simple validation error
	}

	// Update cache
	updateCacheWithPlaybackSpeed(m.cache, m.detailModel.podcast.URL, unlistened, speed)

	// Update the podcast stats in results
	for i := range m.resultsModel.stats {
		if m.resultsModel.stats[i].URL == m.detailModel.podcast.URL {
			m.resultsModel.stats[i].UnlistenedEpisodes = unlistened
			m.resultsModel.stats[i].PlaybackSpeed = speed
			// Recalculate dependent values
			m.resultsModel.stats[i].AvgEpisodeLengthMins = m.detailModel.podcast.AvgEpisodeLengthMins * m.detailModel.podcast.PlaybackSpeed / speed
			m.resultsModel.stats[i].CompositeScore = calculateCompositeScore(m.resultsModel.stats[i])

			// Update the detail model with new values
			m.detailModel.podcast = m.resultsModel.stats[i]

			break
		}
	}

	// Re-sort the list if needed
	items := make([]list.Item, len(m.resultsModel.stats))
	for i, stat := range m.resultsModel.stats {
		bucketNumber := m.calculateBucketNumber(stat.CompositeScore)
		items[i] = podcastItem{stat, bucketNumber}
	}

	m.resultsModel.list.SetItems(items)

	// Save cache to file
	return saveCache(m.cacheFile, m.cache)
}
