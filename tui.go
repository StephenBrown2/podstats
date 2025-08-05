package main

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/v2/filepicker"
	"github.com/charmbracelet/bubbles/v2/list"
	"github.com/charmbracelet/bubbles/v2/progress"
	"github.com/charmbracelet/bubbles/v2/textinput"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
)

// Screen types.
type screen int

const (
	fileSelectScreen screen = iota
	configScreen
	processingScreen
	resultsScreen
	histogramScreen
	podcastDetailScreen
)

// Main TUI model.
type tuiModel struct {
	screen          screen
	filePicker      filepicker.Model
	configModel     configModel
	processingModel processingModel
	resultsModel    resultsModel
	histogramModel  histogramModel
	detailModel     detailModel
	cache           *CacheData
	cacheFile       string
	opmlFile        string
	width           int
	height          int
	err             error
}

// Config screen model for cache options.
type configModel struct {
	inputs  []textinput.Model
	focused int
	options struct {
		useCachedSpeed      bool
		useCachedUnlistened bool
	}
}

// Processing screen model.
type processingModel struct {
	progress     progress.Model
	current      int
	total        int
	currentTitle string
	finished     bool
	stats        []PodcastStats
	podcasts     []Outline // Store podcasts being processed
}

// Sort modes for results.
type sortMode int

const (
	sortByScore sortMode = iota
	sortByName
	sortByPriorityAsc
	sortByPriorityDesc
)

// Results screen model.
type resultsModel struct {
	list     list.Model
	selected int
	stats    []PodcastStats
	sortMode sortMode
}

// Histogram screen model.
type histogramModel struct {
	stats    []PodcastStats
	buckets  []int
	labels   []string
	maxCount int
}

// Detail screen model for individual podcast.
type detailModel struct {
	podcast PodcastStats
	inputs  []textinput.Model
	focused int
}

// Messages.
type podcastProcessedMsg struct {
	stats    PodcastStats
	index    int
	total    int
	hasError bool
	error    error
}

type processingSetupMsg struct {
	podcasts []Outline
}

type processingCompleteMsg struct {
	stats []PodcastStats
}

type errorMsg struct {
	err error
}

// Styles.
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			Padding(1, 0)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262")).
			Padding(0, 0, 1, 0)

	inputStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	buttonStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#7D56F4")).
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(0, 2).
			Bold(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")).
			Bold(true)

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FF00")).
			Bold(true)
)

func NewTUI() *tuiModel {
	// Initialize file picker
	fp := filepicker.New()
	fp.AllowedTypes = []string{".opml"}
	fp.CurrentDirectory, _ = os.Getwd()

	// Initialize config model
	configInputs := make([]textinput.Model, 2)
	configInputs[0] = textinput.New()
	configInputs[0].Placeholder = "y or n"
	configInputs[0].Focus()
	configInputs[1] = textinput.New()
	configInputs[1].Placeholder = "y or n"

	config := configModel{
		inputs:  configInputs,
		focused: 0,
	}

	// Initialize processing model
	proc := processingModel{
		progress: progress.New(progress.WithDefaultGradient()),
	}
	proc.progress.SetWidth(80) // Set default width

	// Initialize empty results model
	results := resultsModel{}

	// Initialize empty histogram model
	histogram := histogramModel{}

	return &tuiModel{
		screen:          fileSelectScreen,
		filePicker:      fp,
		configModel:     config,
		processingModel: proc,
		resultsModel:    results,
		histogramModel:  histogram,
		detailModel:     detailModel{},
		cache:           loadCache("cache.json"),
		cacheFile:       "cache.json",
	}
}

func (m *tuiModel) Init() tea.Cmd {
	return tea.Batch(
		m.filePicker.Init(),
		textinput.Blink,
	)
}

func (m *tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.processingModel.progress.SetWidth(msg.Width - 4)

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if m.screen == resultsScreen || m.screen == podcastDetailScreen || m.screen == fileSelectScreen {
				return m, tea.Quit
			}
		case "esc":
			switch m.screen {
			case configScreen:
				m.screen = fileSelectScreen
			case processingScreen:
				// Can't escape processing
			case resultsScreen:
				// Let the results screen handle Esc for filtering
			case histogramScreen:
				// Don't handle Esc globally for histogram - use 'r' instead
				// This prevents accidentally exiting the TUI
			case podcastDetailScreen:
				m.screen = resultsScreen
			}
		}

	case errorMsg:
		m.err = msg.err

	case processingSetupMsg:
		m.processingModel.podcasts = msg.podcasts
		m.processingModel.total = len(msg.podcasts)
		m.processingModel.current = 0
		m.processingModel.stats = []PodcastStats{}
		return m, m.processNextPodcast()

	case podcastProcessedMsg:
		m.processingModel.current = msg.index + 1

		if !msg.hasError {
			m.processingModel.stats = append(m.processingModel.stats, msg.stats)
			if len(msg.stats.Title) > 0 {
				m.processingModel.currentTitle = msg.stats.Title
			}
		} else {
			// Log error but continue processing
			fmt.Printf("Error processing podcast %d: %v\n", msg.index, msg.error)
		}

		// Continue processing next podcast
		return m, m.processNextPodcast()

	case processingCompleteMsg:
		m.processingModel.finished = true
		m.resultsModel.stats = msg.stats

		// Generate histogram data
		m.histogramModel = m.generateHistogram(msg.stats)

		// Create list items
		items := make([]list.Item, len(msg.stats))
		for i, stat := range msg.stats {
			bucketNumber := m.calculateBucketNumber(stat.CompositeScore)
			items[i] = podcastItem{stat, bucketNumber}
		}

		delegate := list.NewDefaultDelegate()
		delegate.SetHeight(3)
		delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
			Foreground(lipgloss.Color("#7D56F4")).
			BorderLeftForeground(lipgloss.Color("#7D56F4"))
		delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
			Foreground(lipgloss.Color("#626262"))

		m.resultsModel.list = list.New(items, delegate, m.width, m.height-8)
		m.resultsModel.list.Title = "🎧 Podcast Rankings"
		m.resultsModel.list.Styles.Title = titleStyle

		m.screen = resultsScreen
	}

	// Handle screen-specific updates
	switch m.screen {
	case fileSelectScreen:
		m.filePicker, cmd = m.filePicker.Update(msg)
		cmds = append(cmds, cmd)

		if didSelect, path := m.filePicker.DidSelectFile(msg); didSelect {
			m.opmlFile = path
			m.screen = configScreen
		}

	case configScreen:
		cmd = m.updateConfigScreen(msg)
		cmds = append(cmds, cmd)

	case processingScreen:
		if model, c := m.processingModel.progress.Update(msg); c != nil {
			m.processingModel.progress = model
			cmd = c
		}
		cmds = append(cmds, cmd)

		// Handle any key press when processing is finished
		if m.processingModel.finished {
			if keyMsg, ok := msg.(tea.KeyMsg); ok {
				if keyMsg.String() != "" {
					// Move to results screen
					return m, nil
				}
			}
		}

	case resultsScreen:
		cmd = m.updateResultsScreen(msg)
		cmds = append(cmds, cmd)

	case histogramScreen:
		cmd = m.updateHistogramScreen(msg)
		cmds = append(cmds, cmd)

	case podcastDetailScreen:
		cmd = m.updateDetailScreen(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

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

// Sort and update the results list based on current sort mode.
func (m *tuiModel) sortAndUpdateResults() {
	stats := make([]PodcastStats, len(m.resultsModel.stats))
	copy(stats, m.resultsModel.stats)

	switch m.resultsModel.sortMode {
	case sortByScore:
		// Sort by composite score descending (highest scores first)
		sort.Slice(stats, func(i, j int) bool {
			return stats[i].CompositeScore > stats[j].CompositeScore
		})
	case sortByName:
		// Sort alphabetically by title
		sort.Slice(stats, func(i, j int) bool {
			return strings.ToLower(stats[i].Title) < strings.ToLower(stats[j].Title)
		})
	case sortByPriorityAsc:
		// Sort by priority ascending (bucket 1, 2, 3... 10), then by name
		sort.Slice(stats, func(i, j int) bool {
			bucket1 := m.calculateBucketNumber(stats[i].CompositeScore)
			bucket2 := m.calculateBucketNumber(stats[j].CompositeScore)
			if bucket1 != bucket2 {
				return bucket1 < bucket2
			}
			// Secondary sort by name within same priority
			return strings.ToLower(stats[i].Title) < strings.ToLower(stats[j].Title)
		})
	case sortByPriorityDesc:
		// Sort by priority descending (bucket 10, 9, 8... 1), then by name
		sort.Slice(stats, func(i, j int) bool {
			bucket1 := m.calculateBucketNumber(stats[i].CompositeScore)
			bucket2 := m.calculateBucketNumber(stats[j].CompositeScore)
			if bucket1 != bucket2 {
				return bucket1 > bucket2
			}
			// Secondary sort by name within same priority
			return strings.ToLower(stats[i].Title) < strings.ToLower(stats[j].Title)
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

func (m *tuiModel) updateConfigScreen(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			// Process configuration and start analysis
			speed := strings.ToLower(m.configModel.inputs[0].Value())
			unlistened := strings.ToLower(m.configModel.inputs[1].Value())

			m.configModel.options.useCachedSpeed = speed == "y" || speed == "yes"
			m.configModel.options.useCachedUnlistened = unlistened == "y" || unlistened == "yes"

			m.screen = processingScreen
			// Ensure progress bar has proper width
			if m.width > 0 {
				m.processingModel.progress.SetWidth(m.width - 4)
			}
			return m.startProcessing()

		case "tab", "shift+tab", "up", "down":
			if msg.String() == "up" || msg.String() == "shift+tab" {
				m.configModel.focused--
			} else {
				m.configModel.focused++
			}

			if m.configModel.focused > len(m.configModel.inputs)-1 {
				m.configModel.focused = 0
			} else if m.configModel.focused < 0 {
				m.configModel.focused = len(m.configModel.inputs) - 1
			}

			for i := range m.configModel.inputs {
				if i == m.configModel.focused {
					m.configModel.inputs[i].Focus()
				} else {
					m.configModel.inputs[i].Blur()
				}
			}
		}
	}

	for i := range m.configModel.inputs {
		var cmd tea.Cmd
		m.configModel.inputs[i], cmd = m.configModel.inputs[i].Update(msg)
		cmds = append(cmds, cmd)
	}

	return tea.Batch(cmds...)
}

func (m *tuiModel) updateResultsScreen(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			// Check if the list is filtered - if so, clear filter instead of going back
			if m.resultsModel.list.IsFiltered() {
				m.resultsModel.list.ResetFilter()
				return nil
			} else {
				// No filter active, go back to file select
				m.screen = fileSelectScreen
				return nil
			}
		case "enter":
			// Show detail view for selected podcast
			if m.resultsModel.list.SelectedItem() != nil {
				item := m.resultsModel.list.SelectedItem().(podcastItem)
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
		}
	}

	m.resultsModel.list, cmd = m.resultsModel.list.Update(msg)
	return cmd
}

func (m *tuiModel) updateDetailScreen(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			// Save changes and go back
			if err := m.saveDetailChanges(); err != nil {
				m.err = err
			} else {
				m.screen = resultsScreen
			}

		case "tab", "shift+tab", "up", "down":
			if msg.String() == "up" || msg.String() == "shift+tab" {
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

func (m *tuiModel) updateHistogramScreen(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			// Return to results screen
			m.screen = resultsScreen
		}
	}
	return nil
}

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
		return fmt.Errorf("invalid unlistened count: %s", unlistedStr)
	}

	speed, err := strconv.ParseFloat(speedStr, 64)
	if err != nil {
		return fmt.Errorf("invalid playback speed: %s", speedStr)
	}

	if speed <= 0 {
		return fmt.Errorf("playback speed must be positive")
	}

	if unlistened < 0 {
		return fmt.Errorf("unlistened count cannot be negative")
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

func (m *tuiModel) View() string {
	switch m.screen {
	case fileSelectScreen:
		return m.fileSelectView()
	case configScreen:
		return m.configView()
	case processingScreen:
		return m.processingView()
	case resultsScreen:
		return m.resultsView()
	case histogramScreen:
		return m.histogramView()
	case podcastDetailScreen:
		return m.detailView()
	}
	return ""
}

func (m *tuiModel) fileSelectView() string {
	var s strings.Builder

	s.WriteString(titleStyle.Render("🎧 PodStats TUI"))
	s.WriteString("\n")
	s.WriteString(subtitleStyle.Render("Select an OPML file to analyze your podcasts"))
	s.WriteString("\n\n")
	s.WriteString(m.filePicker.View())
	s.WriteString("\n\n")
	s.WriteString("💡 ")
	s.WriteString(lipgloss.NewStyle().Italic(true).Render("Navigate with arrow keys, Enter to select, q to quit"))

	if m.err != nil {
		s.WriteString("\n\n")
		s.WriteString(errorStyle.Render(fmt.Sprintf("❌ Error: %v", m.err)))
	}

	return s.String()
}

func (m *tuiModel) configView() string {
	var s strings.Builder

	s.WriteString(titleStyle.Render("⚙️  Configuration"))
	s.WriteString("\n")
	s.WriteString(subtitleStyle.Render(fmt.Sprintf("File: %s", m.opmlFile)))
	s.WriteString("\n\n")

	s.WriteString("Configure cache usage options:\n\n")

	labels := []string{
		"🔄 Use cached playback speed values? (y/n):",
		"📊 Use cached unlistened episode counts? (y/n):",
	}

	for i, input := range m.configModel.inputs {
		s.WriteString(labels[i] + "\n")
		if i == m.configModel.focused {
			s.WriteString(inputStyle.Render(input.View()))
		} else {
			s.WriteString(input.View())
		}
		s.WriteString("\n\n")
	}

	s.WriteString("💡 ")
	s.WriteString(lipgloss.NewStyle().Italic(true).Render("Tab/Shift+Tab to navigate, Enter to start, Esc to go back"))

	return s.String()
}

func (m *tuiModel) processingView() string {
	var s strings.Builder

	s.WriteString(titleStyle.Render("🔄 Processing Podcasts"))
	s.WriteString("\n\n")

	if !m.processingModel.finished {
		s.WriteString(fmt.Sprintf("📡 Analyzing podcast %d of %d\n",
			m.processingModel.current, m.processingModel.total))
		if m.processingModel.currentTitle != "" {
			truncatedTitle := m.processingModel.currentTitle
			if len(truncatedTitle) > 60 {
				truncatedTitle = truncatedTitle[:57] + "..."
			}
			s.WriteString(fmt.Sprintf("🎙️  Current: %s\n", truncatedTitle))
		}
		s.WriteString(fmt.Sprintf("✅ Processed: %d podcasts\n", len(m.processingModel.stats)))
		s.WriteString("\n")

		percent := 0.0
		if m.processingModel.total > 0 {
			percent = float64(m.processingModel.current) / float64(m.processingModel.total)
		}
		s.WriteString(m.processingModel.progress.ViewAs(percent))
		s.WriteString(fmt.Sprintf("\n\n⏳ Progress: %.1f%% complete", percent*100))
	} else {
		s.WriteString(successStyle.Render("✅ Processing complete!"))
		s.WriteString(fmt.Sprintf("\n\n📊 Successfully analyzed %d podcasts", len(m.processingModel.stats)))
		s.WriteString("\n\n🎯 Ready to view results and histogram!")
		s.WriteString("\n\n💡 ")
		s.WriteString(lipgloss.NewStyle().Italic(true).Render("Press any key to view results"))
	}

	return s.String()
}

func (m *tuiModel) resultsView() string {
	var s strings.Builder

	// Show current sort mode
	var sortModeText string
	switch m.resultsModel.sortMode {
	case sortByScore:
		sortModeText = "Score (High to Low)"
	case sortByName:
		sortModeText = "Name (A-Z)"
	case sortByPriorityAsc:
		sortModeText = "Priority (1-10)"
	case sortByPriorityDesc:
		sortModeText = "Priority (10-1)"
	}

	s.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4")).Render(fmt.Sprintf("Sort: %s", sortModeText)))
	s.WriteString("\n\n")

	s.WriteString(m.resultsModel.list.View())
	s.WriteString("\n💡 ")
	s.WriteString(lipgloss.NewStyle().Italic(true).Render("↑↓ navigate | Enter edit | h histogram | s cycle-sort | n name | p priority↑ | P priority↓ | Esc file | q quit"))

	return s.String()
}

func (m *tuiModel) histogramView() string {
	var s strings.Builder

	s.WriteString(titleStyle.Render("📊 Composite Score Histogram"))
	s.WriteString("\n")
	s.WriteString(subtitleStyle.Render(fmt.Sprintf("Distribution of %d podcasts", len(m.histogramModel.stats))))
	s.WriteString("\n\n")

	if len(m.histogramModel.stats) == 0 {
		s.WriteString("No podcast data to display")
		return s.String()
	}

	// Display histogram bars
	barWidth := 50
	if m.width > 0 && m.width < 80 {
		barWidth = m.width - 30 // Adjust for smaller terminals
	}

	for i, count := range m.histogramModel.buckets {
		label := m.histogramModel.labels[i]
		bucketNumber := 10 - i // Reverse the bucket numbering for priority display
		barLength := 0
		if m.histogramModel.maxCount > 0 {
			barLength = (count * barWidth) / m.histogramModel.maxCount
		}

		// Create bar with color gradient - continuous scale from red to green
		bar := strings.Repeat("█", barLength)
		barStyle := lipgloss.NewStyle()

		// Create a smooth gradient from red (bucket 1) to green (bucket 10)
		// Using HSL-like interpolation for smooth transitions
		switch bucketNumber {
		case 1:
			barStyle = barStyle.Foreground(lipgloss.Color("#FF0000")) // Pure red - most intimidating
		case 2:
			barStyle = barStyle.Foreground(lipgloss.Color("#FF3300")) // Red-orange
		case 3:
			barStyle = barStyle.Foreground(lipgloss.Color("#FF6600")) // Orange-red
		case 4:
			barStyle = barStyle.Foreground(lipgloss.Color("#FF9900")) // Orange
		case 5:
			barStyle = barStyle.Foreground(lipgloss.Color("#FFCC00")) // Yellow-orange
		case 6:
			barStyle = barStyle.Foreground(lipgloss.Color("#FFFF00")) // Pure yellow
		case 7:
			barStyle = barStyle.Foreground(lipgloss.Color("#CCFF00")) // Yellow-green
		case 8:
			barStyle = barStyle.Foreground(lipgloss.Color("#99FF00")) // Green-yellow
		case 9:
			barStyle = barStyle.Foreground(lipgloss.Color("#66FF00")) // Light green
		case 10:
			barStyle = barStyle.Foreground(lipgloss.Color("#00FF00")) // Pure green - most manageable
		}

		s.WriteString(fmt.Sprintf("%2d: %s |%s %d\n",
			bucketNumber, label, barStyle.Render(bar), count))
	}

	s.WriteString("\n📈 Legend:\n")
	s.WriteString("• Higher scores = More episodes/longer/less frequent/older\n")
	s.WriteString("• Score = Unlistened Episodes × Avg Length (mins) + Avg Days Between + Days Since Latest\n")
	s.WriteString("• Colors: ")
	s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00")).Render("Green (manageable)"))
	s.WriteString(" > ")
	s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFF00")).Render("Yellow"))
	s.WriteString(" > ")
	s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#FF8800")).Render("Orange"))
	s.WriteString(" > ")
	s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")).Render("Red (intimidating)"))

	s.WriteString("\n\n💡 ")
	s.WriteString(lipgloss.NewStyle().Italic(true).Render("Press 'r' to return to results"))

	return s.String()
}

func (m *tuiModel) detailView() string {
	var s strings.Builder

	s.WriteString(titleStyle.Render("🎙️  Podcast Details"))
	s.WriteString("\n")
	s.WriteString(subtitleStyle.Render(m.detailModel.podcast.Title))
	s.WriteString("\n\n")

	labels := []string{
		"📊 Unlistened Episodes:",
		"⏩ Playback Speed:",
	}

	for i, input := range m.detailModel.inputs {
		s.WriteString(fmt.Sprintf("%s\n", labels[i]))
		if i == m.detailModel.focused {
			s.WriteString(inputStyle.Render(input.View()))
		} else {
			s.WriteString(input.View())
		}
		s.WriteString("\n\n")
	}

	s.WriteString("📈 Current Stats:\n")
	s.WriteString(fmt.Sprintf("  ⏱️  Avg Length: %.1f minutes (speed-adjusted)\n", m.detailModel.podcast.AvgEpisodeLengthMins))
	s.WriteString(fmt.Sprintf("  📅 Avg Days Between: %.1f\n", m.detailModel.podcast.AvgDaysBetween))
	s.WriteString(fmt.Sprintf("  📆 Days Since Latest: %.1f\n", m.detailModel.podcast.DaysSinceLatest))
	s.WriteString(fmt.Sprintf("  🎯 Composite Score: %.2f\n", m.detailModel.podcast.CompositeScore))

	s.WriteString("\n\n💡 ")
	s.WriteString(lipgloss.NewStyle().Italic(true).Render("Tab/Shift+Tab to navigate, Enter to save changes, Esc to go back"))

	if m.err != nil {
		s.WriteString("\n\n")
		s.WriteString(errorStyle.Render(fmt.Sprintf("❌ Error: %v", m.err)))
	}

	return s.String()
}

// Podcast list item.
type podcastItem struct {
	PodcastStats
	BucketNumber int
}

func (i podcastItem) FilterValue() string {
	return i.PodcastStats.Title
}

func (i podcastItem) Title() string {
	return i.PodcastStats.Title
}

func (i podcastItem) Description() string {
	return fmt.Sprintf("Unlistened: %d | Speed: %.1fx | Avg: %.1f mins | Score: %.2f | Priority: %d",
		i.UnlistenedEpisodes, i.PlaybackSpeed, i.AvgEpisodeLengthMins, i.CompositeScore, i.BucketNumber)
}

func newDetailModel(stats PodcastStats) detailModel {
	inputs := make([]textinput.Model, 2)

	inputs[0] = textinput.New()
	inputs[0].SetValue(fmt.Sprintf("%d", stats.UnlistenedEpisodes))
	inputs[0].Focus()

	inputs[1] = textinput.New()
	inputs[1].SetValue(fmt.Sprintf("%.1f", stats.PlaybackSpeed))

	return detailModel{
		podcast: stats,
		inputs:  inputs,
		focused: 0,
	}
}

func runTUI() {
	m := NewTUI()
	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running TUI: %v", err)
		os.Exit(1)
	}
}

func (m *tuiModel) startProcessing() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		// Parse OPML file
		podcasts, err := parseOPML(m.opmlFile)
		if err != nil {
			return errorMsg{err}
		}

		// Send initial setup message
		return processingSetupMsg{
			podcasts: podcasts,
		}
	})
}

func (m *tuiModel) processNextPodcast() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		if m.processingModel.current >= len(m.processingModel.podcasts) {
			// All done, save cache and complete
			saveCache(m.cacheFile, m.cache)
			return processingCompleteMsg{stats: m.processingModel.stats}
		}

		// Process current podcast
		podcast := m.processingModel.podcasts[m.processingModel.current]
		stats, err := analyzePodcastTUI(podcast, m.cache,
			m.configModel.options.useCachedUnlistened,
			m.configModel.options.useCachedSpeed)
		if err != nil {
			// Skip errored podcasts but continue processing
			return podcastProcessedMsg{
				stats:    PodcastStats{}, // empty stats for error
				index:    m.processingModel.current,
				total:    len(m.processingModel.podcasts),
				hasError: true,
				error:    err,
			}
		}

		return podcastProcessedMsg{
			stats:    stats,
			index:    m.processingModel.current,
			total:    len(m.processingModel.podcasts),
			hasError: false,
		}
	})
}
