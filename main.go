package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/texttheater/golang-levenshtein/levenshtein"
)

// AppMode represents the current mode of the application
type AppMode int

const (
	FileBrowserMode AppMode = iota
	SearchInputMode
	SearchResultsMode
)

// FileItem represents a file or directory in the browser
type FileItem struct {
	Name     string
	Path     string
	IsDir    bool
	Size     int64
	ModTime  time.Time
	Selected bool
}

// SearchResult represents a single search match
type SearchResult struct {
	FilePath     string
	LineNumber   int
	LineContent  string
	MatchStart   int
	MatchEnd     int
	FileSize     int64
	LastModified time.Time
}

// SearchResults holds all search results and metadata
type SearchResults struct {
	Pattern     string
	Target      string
	Results     []SearchResult
	Suggestions []string
	Errors      []string
	TotalFiles  int
	SearchTime  time.Duration
}

// Model represents the main application model
type model struct {
	mode          AppMode
	currentDir    string
	files         []FileItem
	selectedFile  int
	searchInput   string
	searchResults SearchResults
	resultIndex   int
	viewport      struct {
		width  int
		height int
		offset int
	}
	showHelp  bool
	quitting  bool
	statusMsg string
	searching bool
}

// Styles for the TUI
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#04B575"))

	directoryStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7D56F4")).
			Bold(true)

	fileStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F8F8F2"))

	selectedStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#44475A")).
			Foreground(lipgloss.Color("#F8F8F2")).
			Bold(true)

	searchInputStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#50FA7B")).
				Background(lipgloss.Color("#282A36")).
				Padding(0, 1)

	matchStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF5F87")).
			Bold(true).
			Background(lipgloss.Color("#3C3C3C"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF5555")).
			Bold(true)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6272A4")).
			Italic(true)

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFB86C")).
			Italic(true)

	suggestionStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFB86C")).
			Italic(true)
)

func initialModel() model {
	currentDir, _ := os.Getwd()
	m := model{
		mode:       FileBrowserMode,
		currentDir: currentDir,
	}
	m.loadDirectory()
	return m
}

func (m *model) loadDirectory() {
	entries, err := os.ReadDir(m.currentDir)
	if err != nil {
		m.statusMsg = fmt.Sprintf("Error reading directory: %v", err)
		return
	}

	m.files = make([]FileItem, 0, len(entries)+1)

	// Add parent directory entry if not at root
	if m.currentDir != "/" {
		m.files = append(m.files, FileItem{
			Name:  "..",
			Path:  filepath.Dir(m.currentDir),
			IsDir: true,
		})
	}

	// Add directory entries
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		item := FileItem{
			Name:    entry.Name(),
			Path:    filepath.Join(m.currentDir, entry.Name()),
			IsDir:   entry.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime(),
		}
		m.files = append(m.files, item)
	}

	// Sort: directories first, then files, both alphabetically
	sort.Slice(m.files, func(i, j int) bool {
		if m.files[i].IsDir && !m.files[j].IsDir {
			return true
		}
		if !m.files[i].IsDir && m.files[j].IsDir {
			return false
		}
		return m.files[i].Name < m.files[j].Name
	})

	m.selectedFile = 0
	m.statusMsg = fmt.Sprintf("Loaded %d items", len(m.files))
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.viewport.width = msg.Width
		m.viewport.height = msg.Height - 8 // Reserve space for header, input, and footer
		return m, nil

	case tea.KeyMsg:
		switch m.mode {
		case FileBrowserMode:
			return m.updateFileBrowser(msg)
		case SearchInputMode:
			return m.updateSearchInput(msg)
		case SearchResultsMode:
			return m.updateSearchResults(msg)
		}
	}

	return m, nil
}

func (m model) updateFileBrowser(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit

	case "up", "k":
		if m.selectedFile > 0 {
			m.selectedFile--
			m.adjustViewport()
		}

	case "down", "j":
		if m.selectedFile < len(m.files)-1 {
			m.selectedFile++
			m.adjustViewport()
		}

	case "enter":
		if len(m.files) > 0 {
			selected := m.files[m.selectedFile]
			if selected.IsDir {
				m.currentDir = selected.Path
				m.loadDirectory()
			} else {
				// Toggle file selection
				m.files[m.selectedFile].Selected = !m.files[m.selectedFile].Selected
				m.statusMsg = fmt.Sprintf("Toggled selection: %s", selected.Name)
			}
		}

	case "space":
		if len(m.files) > 0 && !m.files[m.selectedFile].IsDir {
			m.files[m.selectedFile].Selected = !m.files[m.selectedFile].Selected
			m.statusMsg = fmt.Sprintf("Toggled selection: %s", m.files[m.selectedFile].Name)
		}

	case "s", "/":
		m.mode = SearchInputMode
		m.searchInput = ""
		m.statusMsg = "Enter search pattern..."

	case "a":
		// Select all files (not directories)
		count := 0
		for i := range m.files {
			if !m.files[i].IsDir {
				m.files[i].Selected = true
				count++
			}
		}
		m.statusMsg = fmt.Sprintf("Selected %d files", count)

	case "A":
		// Deselect all
		for i := range m.files {
			m.files[i].Selected = false
		}
		m.statusMsg = "Deselected all files"

	case "r":
		m.loadDirectory()

	case "h", "?":
		m.showHelp = !m.showHelp

	case "home", "g":
		m.selectedFile = 0
		m.viewport.offset = 0

	case "end", "G":
		m.selectedFile = len(m.files) - 1
		m.adjustViewport()
	}

	return m, nil
}

func (m model) updateSearchInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc":
		m.mode = FileBrowserMode
		m.statusMsg = "Search cancelled"

	case "enter":
		if m.searchInput != "" {
			m.performSearch()
			m.mode = SearchResultsMode
		}

	case "backspace":
		if len(m.searchInput) > 0 {
			m.searchInput = m.searchInput[:len(m.searchInput)-1]
		}

	default:
		if len(msg.String()) == 1 {
			m.searchInput += msg.String()
		}
	}

	return m, nil
}

func (m model) updateSearchResults(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q", "esc":
		m.mode = FileBrowserMode
		m.statusMsg = "Returned to file browser"

	case "up", "k":
		if m.resultIndex > 0 {
			m.resultIndex--
			m.adjustViewport()
		}

	case "down", "j":
		if m.resultIndex < len(m.searchResults.Results)-1 {
			m.resultIndex++
			m.adjustViewport()
		}

	case "home", "g":
		m.resultIndex = 0
		m.viewport.offset = 0

	case "end", "G":
		m.resultIndex = len(m.searchResults.Results) - 1
		m.adjustViewport()

	case "s", "/":
		m.mode = SearchInputMode
		m.searchInput = ""
		m.statusMsg = "Enter new search pattern..."

	case "h", "?":
		m.showHelp = !m.showHelp
	}

	return m, nil
}

func (m *model) adjustViewport() {
	var currentIndex int
	switch m.mode {
	case FileBrowserMode:
		currentIndex = m.selectedFile
	case SearchResultsMode:
		currentIndex = m.resultIndex
	default:
		return
	}

	if currentIndex < m.viewport.offset {
		m.viewport.offset = currentIndex
	} else if currentIndex >= m.viewport.offset+m.viewport.height {
		m.viewport.offset = currentIndex - m.viewport.height + 1
	}
}

func (m *model) performSearch() {
	m.searching = true
	m.statusMsg = "Searching..."

	// Get selected files or current directory
	var targets []string
	selectedCount := 0
	for _, file := range m.files {
		if file.Selected && !file.IsDir {
			targets = append(targets, file.Path)
			selectedCount++
		}
	}

	// If no files selected, search current directory
	if selectedCount == 0 {
		targets = append(targets, m.currentDir)
	}

	// Perform search
	startTime := time.Now()
	results := SearchResults{
		Pattern: m.searchInput,
		Target:  strings.Join(targets, ", "),
	}

	for _, target := range targets {
		if fileInfo, err := os.Stat(target); err == nil {
			if fileInfo.IsDir() {
				m.searchInFolder(m.searchInput, target, &results)
			} else {
				results.TotalFiles++
				m.searchInFile(m.searchInput, target, &results)
			}
		}
	}

	// Sort results
	sort.Slice(results.Results, func(i, j int) bool {
		if results.Results[i].FilePath == results.Results[j].FilePath {
			return results.Results[i].LineNumber < results.Results[j].LineNumber
		}
		return results.Results[i].FilePath < results.Results[j].FilePath
	})

	results.SearchTime = time.Since(startTime)
	m.searchResults = results
	m.resultIndex = 0
	m.searching = false
	m.statusMsg = fmt.Sprintf("Found %d matches in %d files", len(results.Results), results.TotalFiles)
}

func (m model) View() string {
	if m.quitting {
		return "Thanks for using zx! 👋\n"
	}

	var b strings.Builder

	// Header
	switch m.mode {
	case FileBrowserMode:
		title := fmt.Sprintf(" ZX File Manager - %s ", m.currentDir)
		b.WriteString(titleStyle.Render(title))
	case SearchInputMode:
		title := " ZX Search Input "
		b.WriteString(titleStyle.Render(title))
	case SearchResultsMode:
		title := fmt.Sprintf(" ZX Search Results - '%s' ", m.searchResults.Pattern)
		b.WriteString(titleStyle.Render(title))
	}
	b.WriteString("\n\n")

	// Show help if requested
	if m.showHelp {
		b.WriteString(m.renderHelp())
		return b.String()
	}

	// Main content based on mode
	switch m.mode {
	case FileBrowserMode:
		b.WriteString(m.renderFileBrowser())
	case SearchInputMode:
		b.WriteString(m.renderSearchInput())
	case SearchResultsMode:
		b.WriteString(m.renderSearchResults())
	}

	// Status bar
	b.WriteString("\n")
	if m.statusMsg != "" {
		b.WriteString(statusStyle.Render(m.statusMsg))
		b.WriteString("\n")
	}

	// Footer with shortcuts
	b.WriteString(m.renderFooter())

	return b.String()
}

func (m model) renderFileBrowser() string {
	var b strings.Builder

	if len(m.files) == 0 {
		b.WriteString(errorStyle.Render("No files in directory"))
		return b.String()
	}

	start := m.viewport.offset
	end := min(start+m.viewport.height, len(m.files))

	for i := start; i < end; i++ {
		file := m.files[i]

		// File icon and name
		icon := "📄"
		if file.IsDir {
			icon = "📁"
		}
		if file.Selected {
			icon = "✅"
		}

		// File info
		var fileInfo string
		if file.IsDir {
			fileInfo = fmt.Sprintf("%s %s", icon, file.Name)
		} else {
			fileInfo = fmt.Sprintf("%s %s (%s)", icon, file.Name, formatSize(file.Size))
		}

		// Apply styling
		if i == m.selectedFile {
			b.WriteString(selectedStyle.Render(fileInfo))
		} else if file.IsDir {
			b.WriteString(directoryStyle.Render(fileInfo))
		} else {
			b.WriteString(fileStyle.Render(fileInfo))
		}
		b.WriteString("\n")
	}

	// Navigation info
	if len(m.files) > m.viewport.height {
		navInfo := fmt.Sprintf("Showing %d-%d of %d items", start+1, end, len(m.files))
		b.WriteString("\n")
		b.WriteString(helpStyle.Render(navInfo))
	}

	return b.String()
}

func (m model) renderSearchInput() string {
	var b strings.Builder

	b.WriteString(headerStyle.Render("Enter search pattern (regex supported):"))
	b.WriteString("\n\n")

	// Search input box
	inputText := fmt.Sprintf("Search: %s█", m.searchInput)
	b.WriteString(searchInputStyle.Render(inputText))
	b.WriteString("\n\n")

	// Selected files info
	selectedCount := 0
	for _, file := range m.files {
		if file.Selected && !file.IsDir {
			selectedCount++
		}
	}

	if selectedCount > 0 {
		b.WriteString(headerStyle.Render(fmt.Sprintf("Will search in %d selected files", selectedCount)))
	} else {
		b.WriteString(headerStyle.Render(fmt.Sprintf("Will search in current directory: %s", m.currentDir)))
	}

	return b.String()
}

func (m model) renderSearchResults() string {
	var b strings.Builder

	// Summary
	summary := fmt.Sprintf("Found %d matches in %d files (searched in %v)",
		len(m.searchResults.Results),
		m.searchResults.TotalFiles,
		m.searchResults.SearchTime)
	b.WriteString(headerStyle.Render(summary))
	b.WriteString("\n\n")

	// Results
	if len(m.searchResults.Results) == 0 {
		b.WriteString(errorStyle.Render("No matches found."))
		b.WriteString("\n\n")

		// Show suggestions if available
		if len(m.searchResults.Suggestions) > 0 {
			b.WriteString(headerStyle.Render("Suggestions:"))
			b.WriteString("\n")
			for _, suggestion := range m.searchResults.Suggestions {
				b.WriteString("  ")
				b.WriteString(suggestionStyle.Render(suggestion))
				b.WriteString("\n")
			}
		}
	} else {
		start := m.viewport.offset
		end := min(start+m.viewport.height, len(m.searchResults.Results))

		for i := start; i < end; i++ {
			result := m.searchResults.Results[i]

			// File header
			fileHeader := fmt.Sprintf("📁 %s:%d (%s)",
				result.FilePath,
				result.LineNumber,
				result.LastModified.Format("2006-01-02 15:04"))

			if i == m.resultIndex {
				b.WriteString(selectedStyle.Render(fileHeader))
			} else {
				b.WriteString(directoryStyle.Render(fileHeader))
			}
			b.WriteString("\n")

			// Line content with highlighting
			lineContent := m.highlightMatch(result.LineContent, result.MatchStart, result.MatchEnd)
			if i == m.resultIndex {
				b.WriteString(selectedStyle.Render("    " + lineContent))
			} else {
				b.WriteString("    " + lineContent)
			}
			b.WriteString("\n\n")
		}

		// Navigation info
		if len(m.searchResults.Results) > m.viewport.height {
			navInfo := fmt.Sprintf("Showing %d-%d of %d results",
				start+1, end, len(m.searchResults.Results))
			b.WriteString(helpStyle.Render(navInfo))
			b.WriteString("\n")
		}
	}

	// Show errors if any
	if len(m.searchResults.Errors) > 0 {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render("Errors encountered:"))
		b.WriteString("\n")
		for _, err := range m.searchResults.Errors {
			b.WriteString("  ")
			b.WriteString(errorStyle.Render(err))
			b.WriteString("\n")
		}
	}

	return b.String()
}

func (m model) renderHelp() string {
	var help string

	switch m.mode {
	case FileBrowserMode:
		help = `
File Browser Mode:
  ↑/k           Move up
  ↓/j           Move down
  Enter         Enter directory / Toggle file selection
  Space         Toggle file selection
  s//           Start search
  a             Select all files
  A             Deselect all files
  r             Refresh directory
  g/Home        Go to first item
  G/End         Go to last item
  h/?           Toggle this help
  q/Ctrl+C      Quit

Navigation: Use arrow keys or vim-style keys (j/k)
Selection: Select files to search within, or search entire directory
`
	case SearchInputMode:
		help = `
Search Input Mode:
  Type          Enter search pattern (regex supported)
  Enter         Start search
  Esc/Ctrl+C    Cancel search
  Backspace     Delete character

Examples:
  func.*main     - Find function definitions containing 'main'
  TODO|FIXME     - Find TODO or FIXME comments
  error.*return  - Find error handling patterns
`
	case SearchResultsMode:
		help = `
Search Results Mode:
  ↑/k           Move up through results
  ↓/j           Move down through results
  g/Home        Go to first result
  G/End         Go to last result
  s/            Start new search
  Esc/q         Return to file browser
  h/?           Toggle this help

Navigation: Browse through search matches with context
`
	}

	return helpStyle.Render(help)
}

func (m model) renderFooter() string {
	var shortcuts string

	switch m.mode {
	case FileBrowserMode:
		shortcuts = "s:search | Enter:select | Space:toggle | a:select all | h:help | q:quit"
	case SearchInputMode:
		shortcuts = "Enter:search | Esc:cancel"
	case SearchResultsMode:
		shortcuts = "↑↓:navigate | s:new search | Esc:back | h:help"
	}

	return helpStyle.Render(shortcuts)
}

func (m model) highlightMatch(text string, start, end int) string {
	if start < 0 || end > len(text) || start >= end {
		return text
	}

	before := text[:start]
	match := text[start:end]
	after := text[end:]

	return before + matchStyle.Render(match) + after
}

// Search functions (same as before but as methods)
func (m *model) searchInFile(pattern, filePath string, results *SearchResults) {
	file, err := os.Open(filePath)
	if err != nil {
		results.Errors = append(results.Errors, fmt.Sprintf("Unable to open file: %s", filePath))
		return
	}
	defer file.Close()

	re, err := regexp.Compile(pattern)
	if err != nil {
		results.Errors = append(results.Errors, fmt.Sprintf("Invalid regex pattern: %s", err))
		return
	}

	fileInfo, err := file.Stat()
	if err != nil {
		results.Errors = append(results.Errors, fmt.Sprintf("Unable to get file info: %s", filePath))
		return
	}

	scanner := bufio.NewScanner(file)
	lineNum := 1
	var suggestions []string

	for scanner.Scan() {
		line := scanner.Text()

		// Check for exact match
		if matches := re.FindAllStringIndex(line, -1); len(matches) > 0 {
			for _, match := range matches {
				result := SearchResult{
					FilePath:     filePath,
					LineNumber:   lineNum,
					LineContent:  line,
					MatchStart:   match[0],
					MatchEnd:     match[1],
					FileSize:     fileInfo.Size(),
					LastModified: fileInfo.ModTime(),
				}
				results.Results = append(results.Results, result)
			}
		} else {
			// Store potential similar matches for suggestions
			if levenshtein.DistanceForStrings([]rune(strings.ToLower(line)), []rune(strings.ToLower(pattern)), levenshtein.DefaultOptions) <= 3 {
				suggestions = append(suggestions, line)
			}
		}
		lineNum++
	}

	if err := scanner.Err(); err != nil {
		results.Errors = append(results.Errors, fmt.Sprintf("Error reading file %s: %s", filePath, err))
	}

	// Add unique suggestions
	for _, suggestion := range suggestions {
		if !contains(results.Suggestions, suggestion) && len(results.Suggestions) < 10 {
			results.Suggestions = append(results.Suggestions, suggestion)
		}
	}
}

func (m *model) searchInFolder(pattern, folderPath string, results *SearchResults) {
	err := filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			results.Errors = append(results.Errors, fmt.Sprintf("Unable to access: %s", path))
			return nil
		}

		// Skip directories and hidden files
		if !info.IsDir() && !strings.HasPrefix(filepath.Base(path), ".") {
			results.TotalFiles++
			m.searchInFile(pattern, path, results)
		}
		return nil
	})

	if err != nil {
		results.Errors = append(results.Errors, fmt.Sprintf("Failed to scan folder: %s", folderPath))
	}
}

// Helper functions
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func formatSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

func main() {
	// If arguments provided, use legacy command-line mode
	if len(os.Args) >= 3 {
		pattern := os.Args[1]
		target := os.Args[2]

		// Perform search and show results in TUI
		results := performLegacySearch(pattern, target)
		p := tea.NewProgram(legacyResultsModel(results), tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error running program: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Interactive TUI mode
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running program: %v\n", err)
		os.Exit(1)
	}
}

// Legacy functions for backward compatibility
func performLegacySearch(pattern, target string) SearchResults {
	startTime := time.Now()

	results := SearchResults{
		Pattern: pattern,
		Target:  target,
	}

	// Validate pattern
	if _, err := regexp.Compile(pattern); err != nil {
		results.Errors = append(results.Errors, fmt.Sprintf("Invalid regex pattern: %s", err))
		results.SearchTime = time.Since(startTime)
		return results
	}

	// Check if target exists
	fileInfo, err := os.Stat(target)
	if err != nil {
		results.Errors = append(results.Errors, fmt.Sprintf("File or folder not found: %s", target))
		results.SearchTime = time.Since(startTime)
		return results
	}

	m := &model{} // Create a temporary model for search methods
	if fileInfo.IsDir() {
		m.searchInFolder(pattern, target, &results)
	} else {
		results.TotalFiles = 1
		m.searchInFile(pattern, target, &results)
	}

	// Sort results by file path and line number
	sort.Slice(results.Results, func(i, j int) bool {
		if results.Results[i].FilePath == results.Results[j].FilePath {
			return results.Results[i].LineNumber < results.Results[j].LineNumber
		}
		return results.Results[i].FilePath < results.Results[j].FilePath
	})

	results.SearchTime = time.Since(startTime)
	return results
}

func legacyResultsModel(results SearchResults) model {
	m := model{
		mode:          SearchResultsMode,
		searchResults: results,
		resultIndex:   0,
	}
	return m
}
