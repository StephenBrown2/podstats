package main

import (
	"cmp"
	"context"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/filepicker"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
)

// Screen types.
type screen int

const (
	fileSelectScreen screen = iota
	configScreen
	tagSelectScreen
	processingScreen
	resultsScreen
	histogramScreen
	podcastDetailScreen
	backupUpdateScreen
)

const (
	keyEsc      = "esc"
	keyEnter    = "enter"
	keyShiftTab = "shift+tab"
	yesStr      = "yes"
)

// UI element height constants for layout calculations.
const (
	fileSelectHeaderLines = 3 // title + subtitle + blank line
	fileSelectFooterLines = 2 // help text line + possible error line
	configHeaderLines     = 4 // title + file path + 2 blank lines
	tagSelectFooterLines  = 2 // blank line + help text
	resultsFooterLines    = 2 // blank line + help text
	histogramHeaderLines  = 2 // title + blank line
	histogramFooterLines  = 7 // legend (5 lines) + blank + help
)

// Main TUI model.
type tuiModel struct {
	filePicker        filepicker.Model
	err               error
	speedSettings     map[string]float64
	cache             *CacheData
	cacheFile         string
	configModel       configModel
	opmlFile          string
	resultsModel      resultsModel
	histogramModel    histogramModel
	tagSelectModel    tagSelectModel
	backupUpdateModel backupUpdateModel
	processingModel   processingModel
	detailModel       detailModel
	screen            screen
	defaultSpeed      float64
	width             int
	height            int
	isBackupFile      bool
}

// Config screen model for cache options.
type configModel struct {
	form    *huh.Form
	options struct {
		useCachedSpeed      bool
		useCachedUnlistened bool
	}
}

// Tag selection screen model for backup files.
type tagSelectModel struct {
	list        list.Model
	selectedTag string
	tags        []string
	spinner     spinner.Model
	showAllTags bool
}

// Tag item for list display.
type tagItem struct {
	name        string
	isAllOption bool
}

func (t tagItem) FilterValue() string { return t.name }
func (t tagItem) Title() string       { return t.name }
func (t tagItem) Description() string { //nolint:revive // receiver unused by design for list interface
	return "" // No description to keep the list compact
}

// Processing screen model.
type processingModel struct {
	progress     progress.Model
	currentTitle string
	stats        []PodcastStats
	podcasts     []Outline
	current      int
	total        int
	finished     bool
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
	inputs  []textinput.Model
	podcast PodcastStats
	focused int
}

// Backup update screen model.
type backupUpdateModel struct {
	errorMsg string
	spinner  spinner.Model
	updating bool
	complete bool
	success  bool
}

// Messages.
type podcastProcessedMsg struct {
	error    error
	stats    PodcastStats
	index    int
	total    int
	hasError bool
}

type processingSetupMsg struct {
	podcasts []Outline
}

type tagsLoadedMsg struct {
	tags []string
}

type speedSettingsLoadedMsg struct {
	speedSettings map[string]float64
	defaultSpeed  float64
}

type processingCompleteMsg struct {
	stats []PodcastStats
}

type backupUpdateStartMsg struct{}

type backupUpdateCompleteMsg struct {
	error   string
	success bool
}

type errorMsg struct {
	err error
}

// Styles.
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4"))

	subtitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262"))

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
	fp.AllowedTypes = []string{".opml", ".zip", ".backup"}
	fp.CurrentDirectory, _ = os.Getwd()

	// Initialize config model with huh form
	config := configModel{}
	config.options.useCachedSpeed = true
	config.options.useCachedUnlistened = true
	config.form = huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Key("useCachedSpeed").
				Title("🔄 Use cached playback speed values?").
				Value(&config.options.useCachedSpeed),
			huh.NewConfirm().
				Key("useCachedUnlistened").
				Title("📊 Use cached unlistened episode counts?").
				Value(&config.options.useCachedUnlistened),
		),
	)

	// Initialize processing model
	proc := processingModel{
		progress: progress.New(progress.WithDefaultBlend()),
	}
	proc.progress.SetWidth(80) // Set default width

	// Initialize empty results model
	results := resultsModel{}

	// Initialize empty histogram model
	histogram := histogramModel{}

	// Initialize tag selection model with spinner
	tagSelect := tagSelectModel{
		spinner: spinner.New(spinner.WithSpinner(spinner.Ellipsis)),
	}
	// tagSelect.spinner.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return &tuiModel{
		screen:          fileSelectScreen,
		filePicker:      fp,
		configModel:     config,
		tagSelectModel:  tagSelect,
		processingModel: proc,
		resultsModel:    results,
		histogramModel:  histogram,
		detailModel:     detailModel{},
		cache:           loadCache("cache.json"),
		cacheFile:       "cache.json",
		defaultSpeed:    2.0, // Default speed for OPML files
	}
}

func (m *tuiModel) Init() tea.Cmd {
	return tea.Batch(
		m.filePicker.Init(),
		m.configModel.form.Init(),
		textinput.Blink,
	)
}

// Update handles TUI state updates and message routing.
// Central TUI update dispatcher; splitting would harm message flow.
//
//nolint:funlen,ireturn,maintidx // Bubble Tea Model interface; central dispatcher
func (m *tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.processingModel.progress.SetWidth(max(10, msg.Width-4))
		// Resize lists to fill available terminal height
		if m.resultsModel.list.Width() > 0 {
			listHeight := m.calculateResultsListHeight()
			m.resultsModel.list.SetSize(m.width, listHeight)
		}

		if m.tagSelectModel.list.Width() > 0 {
			listHeight := m.calculateTagSelectListHeight()
			m.tagSelectModel.list.SetSize(m.width, listHeight)
		}

		// Also resize the file picker if available
		filePickerHeight := m.calculateFilePickerHeight()
		m.filePicker.SetHeight(filePickerHeight)

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if m.screen == resultsScreen || m.screen == podcastDetailScreen || m.screen == fileSelectScreen {
				return m, tea.Quit
			}
		case keyEsc:
			switch m.screen {
			case configScreen:
				m.screen = fileSelectScreen
			case tagSelectScreen:
				m.screen = configScreen
			case processingScreen:
				// Can't escape processing
			case resultsScreen:
				// Let the results screen handle Esc for filtering
			case histogramScreen:
				// Don't handle Esc globally for histogram - use 'r' instead
				// This prevents accidentally exiting the TUI
			case podcastDetailScreen:
				m.screen = resultsScreen
			case fileSelectScreen:
				// No-op; already at the first screen
			case backupUpdateScreen:
				// Esc handling is managed in updateBackupUpdateScreen
			}
		}

	case errorMsg:
		m.err = msg.err

	case tagsLoadedMsg:
		// Initialize tag selection screen
		m.tagSelectModel.tags = msg.tags
		m.tagSelectModel.showAllTags = true

		// Create list items with "All podcasts" option first
		items := make([]list.Item, len(msg.tags)+1)

		items[0] = tagItem{name: "All podcasts", isAllOption: true}
		for i, tag := range msg.tags {
			items[i+1] = tagItem{name: tag, isAllOption: false}
		}

		delegate := list.NewDefaultDelegate()
		delegate.SetHeight(1)
		delegate.SetSpacing(0)
		delegate.ShowDescription = false
		delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
			Foreground(lipgloss.Color("#7D56F4")).
			BorderLeftForeground(lipgloss.Color("#7D56F4"))

		// Initialize list with calculated height for available space
		listHeight := m.calculateTagSelectListHeight()
		m.tagSelectModel.list = list.New(items, delegate, m.width, listHeight)
		m.tagSelectModel.list.Title = "🏷️  Tag Selection - " + m.opmlFile
		m.tagSelectModel.list.Styles.Title = titleStyle
		m.tagSelectModel.list.SetShowTitle(true)
		// Hide built-in list help/status to avoid double footer
		m.tagSelectModel.list.SetShowHelp(false)
		m.tagSelectModel.list.SetShowStatusBar(false)

	case speedSettingsLoadedMsg:
		// Store speed settings for use during processing
		m.speedSettings = msg.speedSettings
		m.defaultSpeed = msg.defaultSpeed

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
		// Make each item more compact to fit more per page
		delegate.SetHeight(2)
		delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
			Foreground(lipgloss.Color("#7D56F4")).
			BorderLeftForeground(lipgloss.Color("#7D56F4"))
		delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
			Foreground(lipgloss.Color("#626262"))

		// Initialize list with calculated height for available space
		listHeight := m.calculateResultsListHeight()
		m.resultsModel.list = list.New(items, delegate, m.width, listHeight)
		m.resultsModel.list.Title = "🎧 Podcast Rankings"
		m.resultsModel.list.Styles.Title = titleStyle
		m.resultsModel.list.SetShowTitle(true)
		// Hide built-in list help/status to avoid double footer; keep pagination
		m.resultsModel.list.SetShowHelp(false)
		m.resultsModel.list.SetShowStatusBar(false)

		m.screen = resultsScreen

	case backupUpdateStartMsg:
		// Initialize backup update model and switch to backup update screen
		m.backupUpdateModel = backupUpdateModel{
			updating: false,
			complete: false,
			spinner:  spinner.New(spinner.WithSpinner(spinner.Ellipsis)),
		}
		// m.backupUpdateModel.spinner.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
		m.screen = backupUpdateScreen
	}

	// Handle screen-specific updates
	switch m.screen {
	case fileSelectScreen:
		m.filePicker, cmd = m.filePicker.Update(msg)
		cmds = append(cmds, cmd)

		if didSelect, path := m.filePicker.DidSelectFile(msg); didSelect {
			m.opmlFile = path
			// Check if this is a backup file
			lowerPath := strings.ToLower(path)
			m.isBackupFile = strings.HasSuffix(lowerPath, ".backup.zip") || strings.HasSuffix(lowerPath, ".backup")
			m.screen = configScreen
		}

	case configScreen:
		cmd = m.updateConfigScreen(msg)
		cmds = append(cmds, cmd)

	case tagSelectScreen:
		cmd = m.updateTagSelectScreen(msg)
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
		m.updateHistogramScreen(msg)

	case podcastDetailScreen:
		cmd = m.updateDetailScreen(msg)
		cmds = append(cmds, cmd)

	case backupUpdateScreen:
		cmd = m.updateBackupUpdateScreen(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *tuiModel) View() tea.View {
	switch m.screen {
	case fileSelectScreen:
		return m.fileSelectView()
	case configScreen:
		return m.configView()
	case tagSelectScreen:
		return m.tagSelectView()
	case processingScreen:
		return m.processingView()
	case resultsScreen:
		return m.resultsView()
	case histogramScreen:
		return m.histogramView()
	case podcastDetailScreen:
		return m.detailView()
	case backupUpdateScreen:
		return m.backupUpdateView()
	}

	return tea.NewView("")
}

func (m *tuiModel) fileSelectView() tea.View {
	var s strings.Builder

	s.WriteString(titleStyle.Render("🎧 PodStats TUI"))
	s.WriteString("\n")
	s.WriteString(subtitleStyle.Render("Select an OPML file or PodcastAddict backup file to analyze"))
	s.WriteString("\n")
	s.WriteString(m.filePicker.View())
	s.WriteString("\n")
	s.WriteString("💡 ")
	s.WriteString(lipgloss.NewStyle().Italic(true).Render("Navigate with arrow keys, Enter to select, q to quit"))

	if m.err != nil {
		s.WriteString("\n\n")
		s.WriteString(errorStyle.Render(fmt.Sprintf("❌ Error: %v", m.err)))
	}

	return tea.NewView(s.String())
}

func (m *tuiModel) configView() tea.View {
	var s strings.Builder

	s.WriteString(titleStyle.Render("⚙️  Configuration"))
	s.WriteString("\n")
	s.WriteString(subtitleStyle.Render("File: " + m.opmlFile))
	s.WriteString("\n\n")

	s.WriteString(m.configModel.form.View())

	return tea.NewView(s.String())
}

func (m *tuiModel) tagSelectView() tea.View {
	var s strings.Builder
	// Title is now embedded in list.Title to save vertical space

	if len(m.tagSelectModel.tags) == 0 {
		s.WriteString("🏷 Loading tags")
		s.WriteString(m.tagSelectModel.spinner.View())
	} else {
		s.WriteString(m.tagSelectModel.list.View())
	}

	s.WriteString("\n\n💡 ")
	s.WriteString(lipgloss.NewStyle().Italic(true).Render("Enter to select, Esc to go back"))

	return tea.NewView(s.String())
}

func (m *tuiModel) processingView() tea.View {
	var s strings.Builder

	s.WriteString(titleStyle.Render("🔄 Processing Podcasts"))
	s.WriteString("\n")

	// Multi-podcast processing state requires conditional updates.
	if !m.processingModel.finished { //nolint:nestif
		s.WriteString(fmt.Sprintf("Analyzing podcast %d of %d\n",
			m.processingModel.current, m.processingModel.total))

		if m.processingModel.currentTitle != "" {
			prefix := "Current: "
			maxWidth := m.width - lipgloss.Width(prefix)

			truncatedTitle := m.processingModel.currentTitle
			if maxWidth > 3 && lipgloss.Width(m.processingModel.currentTitle) > maxWidth {
				// Truncate by rune count to handle multi-byte chars
				runes := []rune(m.processingModel.currentTitle)
				for lipgloss.Width(string(runes)) > maxWidth-3 && len(runes) > 0 {
					runes = runes[:len(runes)-1]
				}

				truncatedTitle = string(runes) + "..."
			}

			// Use lipgloss to ensure the line is exactly the right width
			currentLine := lipgloss.NewStyle().
				Width(m.width).
				Render(prefix + truncatedTitle)
			s.WriteString(currentLine)
			s.WriteString("\n")
		}

		s.WriteString(fmt.Sprintf("Processed: %d podcasts\n", len(m.processingModel.stats)))
		s.WriteString("\n")

		percent := 0.0
		if m.processingModel.total > 0 {
			percent = float64(m.processingModel.current) / float64(m.processingModel.total)
		}

		s.WriteString(m.processingModel.progress.ViewAs(percent))
		s.WriteString(fmt.Sprintf("\n\n⏳ Progress: %.1f%% complete", percent*100))
	} else {
		s.WriteString(successStyle.Render("✅ Processing complete!"))
		s.WriteString(fmt.Sprintf("\n📊 Successfully analyzed %d podcasts", len(m.processingModel.stats)))
		s.WriteString("\n🎯 Ready to view results and histogram!")
		s.WriteString("\n💡 ")
		s.WriteString(lipgloss.NewStyle().Italic(true).Render("Press any key to view results"))
	}

	return tea.NewView(s.String())
}

func (m *tuiModel) resultsView() tea.View {
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

	// Combine sort mode into the list title to save vertical space
	m.resultsModel.list.Title = "🎧 Podcast Rankings - Sort: " + sortModeText
	// Title visibility is handled at construction; avoid resizing here to prevent flicker

	s.WriteString(m.resultsModel.list.View())
	s.WriteString("\n💡 ")

	// Show different help text based on file type
	var helpText string
	if m.isBackupFile {
		helpText = "↑↓ navigate | Enter edit | h histogram | s cycle-sort | n name | " +
			"p priority↑ | P priority↓ | u update-backup | Esc file | q quit"
	} else {
		helpText = "↑↓ navigate | Enter edit | h histogram | s cycle-sort | n name | " +
			"p priority↑ | P priority↓ | Esc file | q quit"
	}

	s.WriteString(lipgloss.NewStyle().Italic(true).Render(helpText))

	return tea.NewView(s.String())
}

func (m *tuiModel) histogramView() tea.View { //nolint:funlen // Complex view rendering; splitting would hurt readability
	var s strings.Builder

	s.WriteString(titleStyle.Render("📊 Composite Score Histogram"))
	s.WriteString(" - ")
	s.WriteString(subtitleStyle.Render(fmt.Sprintf("%d podcasts", len(m.histogramModel.stats))))
	s.WriteString("\n")

	if len(m.histogramModel.stats) == 0 {
		s.WriteString("No podcast data to display")

		return tea.NewView(s.String())
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

	return tea.NewView(s.String())
}

func (m *tuiModel) detailView() tea.View {
	var s strings.Builder

	s.WriteString(titleStyle.Render("🎙️  Podcast Details"))
	s.WriteString("\n")
	s.WriteString(subtitleStyle.Render(m.detailModel.podcast.Title))
	s.WriteString("\n")

	labels := []string{
		"📊 Unlistened Episodes:",
		"⏩ Playback Speed:",
	}

	for i := range m.detailModel.inputs {
		s.WriteString(labels[i] + "\n")

		if i == m.detailModel.focused {
			s.WriteString(inputStyle.Render(m.detailModel.inputs[i].View()))
		} else {
			s.WriteString(m.detailModel.inputs[i].View())
		}

		s.WriteString("\n\n")
	}

	s.WriteString("📈 Current Stats:\n")
	s.WriteString(fmt.Sprintf("  ⏱️  Avg Length: %.1f minutes (speed-adjusted)\n", m.detailModel.podcast.AvgEpisodeLengthMins))
	s.WriteString(fmt.Sprintf("  📅 Avg Days Between: %.1f\n", m.detailModel.podcast.AvgDaysBetween))
	s.WriteString("  📆 Days Since Latest: ")
	s.WriteString(colorDaysSince(m.detailModel.podcast.DaysSinceLatest, false))
	s.WriteString("\n")
	s.WriteString(fmt.Sprintf("  🎯 Composite Score: %.2f\n", m.detailModel.podcast.CompositeScore))

	s.WriteString("\n\n💡 ")
	s.WriteString(lipgloss.NewStyle().Italic(true).Render("Tab/Shift+Tab to navigate, Enter to save changes, Esc to go back"))

	if m.err != nil {
		s.WriteString("\n\n")
		s.WriteString(errorStyle.Render(fmt.Sprintf("❌ Error: %v", m.err)))
	}

	return tea.NewView(s.String())
}

// Podcast list item.
type podcastItem struct {
	PodcastStats
	BucketNumber int
}

func (i podcastItem) FilterValue() string {
	return i.SortTitle
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
	inputs[0].SetValue(strconv.Itoa(stats.UnlistenedEpisodes))
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
	p := tea.NewProgram(m)

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running TUI: %v", err)
		os.Exit(1)
	}
}

func (m *tuiModel) startProcessing() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		var (
			podcasts []Outline
			err      error
		)

		if m.isBackupFile {
			// Extract podcasts from backup file with tag filter
			if m.tagSelectModel.selectedTag != "" {
				podcasts, err = extractPodcastsFromBackupWithTag(m.opmlFile, m.tagSelectModel.selectedTag)
			} else {
				podcasts, err = extractPodcastsFromBackup(m.opmlFile)
			}
		} else {
			// Parse OPML file
			podcasts, err = parseOPML(m.opmlFile)
		}

		if err != nil {
			return errorMsg{err}
		}

		// Send initial setup message
		return processingSetupMsg{
			podcasts: podcasts,
		}
	})
}

func (m *tuiModel) loadTags() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		backupManager, err := NewBackupManager(m.opmlFile)
		if err != nil {
			return errorMsg{err}
		}

		defer func() { _ = backupManager.Close() }()

		if err = backupManager.ExtractDatabase(); err != nil {
			return errorMsg{err}
		}

		if err = backupManager.OpenDatabase(); err != nil {
			return errorMsg{err}
		}

		tags, err := backupManager.GetTags(context.Background())
		if err != nil {
			return errorMsg{err}
		}

		return tagsLoadedMsg{tags: tags}
	})
}

func (m *tuiModel) loadSpeedSettings() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		backupManager, err := NewBackupManager(m.opmlFile)
		if err != nil {
			return errorMsg{err}
		}

		defer func() { _ = backupManager.Close() }()

		if err = backupManager.ExtractDatabase(); err != nil {
			return errorMsg{err}
		}

		if err = backupManager.OpenDatabase(); err != nil {
			return errorMsg{err}
		}

		speedSettings, defaultSpeed, err := backupManager.GetPodcastSpeedSettingsByURL(context.Background())
		if err != nil {
			return errorMsg{err}
		}

		return speedSettingsLoadedMsg{
			speedSettings: speedSettings,
			defaultSpeed:  defaultSpeed,
		}
	})
}

// extractPodcastsFromBackupWithTag extracts podcasts from backup filtered by tag.
func extractPodcastsFromBackupWithTag(backupFile string, tag string) ([]Outline, error) {
	bm, err := NewBackupManager(backupFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open backup file: %w", err)
	}

	defer func() { _ = bm.Close() }()

	if err = bm.ExtractDatabase(); err != nil {
		return nil, fmt.Errorf("failed to extract database: %w", err)
	}

	if err = bm.OpenDatabase(); err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	ctx := context.Background()

	if tag == "" {
		// No tag filter, get all podcasts
		backupStats, e := bm.GetPodcastStats(ctx)
		if e != nil {
			return nil, fmt.Errorf("failed to get podcast stats from backup: %w", e)
		}

		podcasts := make([]Outline, 0, len(backupStats))
		for _, stat := range backupStats {
			podcast := Outline{
				Title:  stat.Name,
				XMLURL: stat.FeedUrl,
				Text:   stat.Name,
			}
			podcasts = append(podcasts, podcast)
		}

		return podcasts, nil
	}

	// Filter by selected tag
	backupStats, err := bm.GetPodcastStatsByTag(ctx, tag)
	if err != nil {
		return nil, fmt.Errorf("failed to get podcast stats by tag from backup: %w", err)
	}

	podcasts := make([]Outline, 0, len(backupStats))
	for _, stat := range backupStats {
		podcast := Outline{
			Title:  stat.Name,
			XMLURL: stat.FeedUrl,
			Text:   stat.Name,
		}
		podcasts = append(podcasts, podcast)
	}

	return podcasts, nil
}

func (m *tuiModel) processNextPodcast() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		if m.processingModel.current >= len(m.processingModel.podcasts) {
			// All done, save cache and complete
			_ = saveCache(m.cacheFile, m.cache)

			return processingCompleteMsg{stats: m.processingModel.stats}
		}

		// Process current podcast
		podcast := m.processingModel.podcasts[m.processingModel.current]

		stats, err := analyzePodcastTUI(podcast, m.cache,
			m.configModel.options.useCachedUnlistened,
			m.configModel.options.useCachedSpeed,
			m.speedSettings,
			m.defaultSpeed)
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

// prepareBackupUpdate calculates priority updates and switches to backup update screen.
func (m *tuiModel) prepareBackupUpdate() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		// Create backup manager to read current priorities
		bm, err := NewBackupManager(m.opmlFile)
		if err != nil {
			return backupUpdateCompleteMsg{success: false, error: fmt.Sprintf("Failed to open backup file: %v", err)}
		}

		defer func() { _ = bm.Close() }()

		if err = bm.ExtractDatabase(); err != nil {
			return backupUpdateCompleteMsg{success: false, error: fmt.Sprintf("Failed to extract database: %v", err)}
		}

		if err = bm.OpenDatabase(); err != nil {
			return backupUpdateCompleteMsg{success: false, error: fmt.Sprintf("Failed to open database: %v", err)}
		}

		ctx := context.Background()

		backupStats, err := bm.GetPodcastStats(ctx)
		if err != nil {
			return backupUpdateCompleteMsg{success: false, error: fmt.Sprintf("Failed to get podcast stats: %v", err)}
		}

		// Create a map of feed URLs to current priorities from backup
		currentPriorities := make(map[string]int64)
		for _, stat := range backupStats {
			currentPriorities[stat.FeedUrl] = stat.Priority
		}

		// Calculate new priorities based on composite scores
		priorityUpdates := make(map[string]int64)

		// Sort by composite score (ascending - better scores first)
		allStats := make([]PodcastStats, len(m.resultsModel.stats))
		copy(allStats, m.resultsModel.stats)
		slices.SortStableFunc(allStats, func(a, b PodcastStats) int {
			return cmp.Compare(a.CompositeScore, b.CompositeScore)
		})

		// Assign priorities based on ranking using original priority range (1-11)
		maxPriority := int64(11)
		minPriority := int64(1)

		for i, stats := range allStats {
			// Check if this podcast exists in the backup
			if currentPriority, exists := currentPriorities[stats.URL]; exists {
				// Calculate new priority: best podcast gets maxPriority, worst gets minPriority
				priorityRange := maxPriority - minPriority
				newPriority := max(maxPriority-int64(i)*priorityRange/int64(len(allStats)), minPriority)

				if newPriority != currentPriority {
					priorityUpdates[stats.URL] = newPriority
				}
			}
		}

		return backupUpdateStartMsg{}
	})
}

// updateBackupUpdateScreen handles the backup update screen.
func (m *tuiModel) updateBackupUpdateScreen(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "Y":
			if !m.backupUpdateModel.updating && !m.backupUpdateModel.complete {
				m.backupUpdateModel.updating = true

				return tea.Batch(m.performBackupUpdate(), m.backupUpdateModel.spinner.Tick)
			}
		case "n", "N", keyEsc:
			if !m.backupUpdateModel.updating {
				m.screen = resultsScreen

				return nil
			}
		case keyEnter:
			if m.backupUpdateModel.complete {
				m.screen = resultsScreen

				return nil
			}
		}
	case backupUpdateCompleteMsg:
		m.backupUpdateModel.updating = false
		m.backupUpdateModel.complete = true

		m.backupUpdateModel.success = msg.success
		if !msg.success {
			m.backupUpdateModel.errorMsg = msg.error
		}
	}
	// Animate spinner while updating
	if m.backupUpdateModel.updating {
		var cmd tea.Cmd

		m.backupUpdateModel.spinner, cmd = m.backupUpdateModel.spinner.Update(msg)

		return cmd
	}

	return nil
}

// performBackupUpdate actually performs the backup update.
func (m *tuiModel) performBackupUpdate() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		// Calculate priority updates (same logic as prepareBackupUpdate but actually apply them)
		bm, err := NewBackupManager(m.opmlFile)
		if err != nil {
			return backupUpdateCompleteMsg{success: false, error: fmt.Sprintf("Failed to open backup file: %v", err)}
		}

		defer func() { _ = bm.Close() }()

		if err = bm.ExtractDatabase(); err != nil {
			return backupUpdateCompleteMsg{success: false, error: fmt.Sprintf("Failed to extract database: %v", err)}
		}

		if err = bm.OpenDatabase(); err != nil {
			return backupUpdateCompleteMsg{success: false, error: fmt.Sprintf("Failed to open database: %v", err)}
		}

		ctx := context.Background()

		backupStats, err := bm.GetPodcastStats(ctx)
		if err != nil {
			return backupUpdateCompleteMsg{success: false, error: fmt.Sprintf("Failed to get podcast stats: %v", err)}
		}

		// Create a map of feed URLs to current priorities from backup
		currentPriorities := make(map[string]int64)
		for _, stat := range backupStats {
			currentPriorities[stat.FeedUrl] = stat.Priority
		}

		// Calculate new priorities based on composite scores
		priorityUpdates := make(map[string]int64)

		// Sort by composite score (ascending - better scores first)
		allStats := make([]PodcastStats, len(m.resultsModel.stats))
		copy(allStats, m.resultsModel.stats)
		slices.SortStableFunc(allStats, func(a, b PodcastStats) int {
			return cmp.Compare(a.CompositeScore, b.CompositeScore)
		})

		// Assign priorities based on ranking using original priority range (1-11)
		maxPriority := int64(11)
		minPriority := int64(1)

		for i, stats := range allStats {
			// Check if this podcast exists in the backup
			if currentPriority, exists := currentPriorities[stats.URL]; exists {
				// Calculate new priority: best podcast gets maxPriority, worst gets minPriority
				priorityRange := maxPriority - minPriority
				newPriority := max(maxPriority-int64(i)*priorityRange/int64(len(allStats)), minPriority)

				if newPriority != currentPriority {
					priorityUpdates[stats.URL] = newPriority
				}
			}
		}

		if len(priorityUpdates) == 0 {
			return backupUpdateCompleteMsg{success: true, error: "No priority updates needed - all podcasts already have optimal priorities"}
		}

		// Apply the updates
		if err := UpdateBackupPriorities(m.opmlFile, priorityUpdates); err != nil {
			return backupUpdateCompleteMsg{success: false, error: fmt.Sprintf("Failed to update backup priorities: %v", err)}
		}

		return backupUpdateCompleteMsg{success: true, error: fmt.Sprintf("Successfully updated %d podcast priorities!", len(priorityUpdates))}
	})
}

// backupUpdateView renders the backup update confirmation screen.
func (m *tuiModel) backupUpdateView() tea.View {
	var s strings.Builder

	s.WriteString(titleStyle.Render("🔄 Update Backup Priorities"))
	s.WriteString("\n")

	switch {
	case !m.backupUpdateModel.updating && !m.backupUpdateModel.complete:
		s.WriteString("This will update the podcast priorities in your backup file based on the analysis results.\n")
		s.WriteString("Podcasts with better composite scores (more manageable) will get higher priorities.\n\n")
		s.WriteString("⚠️  This will modify your backup file. Do you want to continue?\n\n")
		s.WriteString(buttonStyle.Render("Y") + " Yes  " + buttonStyle.Render("N") + " No")
		s.WriteString("\n\n💡 ")
		s.WriteString(lipgloss.NewStyle().Italic(true).Render("Y to continue, N or Esc to cancel"))
	case m.backupUpdateModel.updating:
		s.WriteString("🔄 Updating backup file")
		s.WriteString(m.backupUpdateModel.spinner.View())
		s.WriteString("\n\nPlease wait while priorities are calculated and applied...")
	case m.backupUpdateModel.complete:
		if m.backupUpdateModel.success {
			s.WriteString(successStyle.Render("✅ Backup Update Complete!"))
			s.WriteString("\n")
			s.WriteString(m.backupUpdateModel.errorMsg) // This contains the success message
		} else {
			s.WriteString(errorStyle.Render("❌ Backup Update Failed"))
			s.WriteString("\n")
			s.WriteString("Error: " + m.backupUpdateModel.errorMsg)
		}

		s.WriteString("\n\n💡 ")
		s.WriteString(lipgloss.NewStyle().Italic(true).Render("Press Enter to continue"))
	}

	return tea.NewView(s.String())
}
