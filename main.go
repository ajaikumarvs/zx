package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Configuration for large data handling
const (
	MaxConcurrentFiles = 50        // Maximum concurrent file searches
	MaxResultsInMemory = 10000     // Maximum results to keep in memory
	MaxFileSize        = 100 << 20 // 100MB max file size to search
	BufferSize         = 64 << 10  // 64KB buffer for file reading
	ProgressUpdateMs   = 100       // Progress update interval in milliseconds
)

// AppMode represents the current mode of the application
type AppMode int

const (
	FileBrowserMode AppMode = iota
	SearchInputMode
	SearchResultsMode
	SearchProgressMode
	ConfigMode
	AnalysisMode
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

// SearchProgress tracks search progress for large operations
type SearchProgress struct {
	TotalFiles     int64
	ProcessedFiles int64
	CurrentFile    string
	TotalSize      int64
	ProcessedSize  int64
	StartTime      time.Time
	Errors         []string
	Cancelled      bool
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
	Progress    SearchProgress
	Truncated   bool // True if results were truncated due to memory limits
}

// FolderAnalysis holds statistics about a directory
type FolderAnalysis struct {
	TotalFiles      int
	TotalSize       int64
	LargestFile     int64
	AverageFileSize int64
	BinaryFiles     int
	TextFiles       int
	HiddenFiles     int
	LargeFiles      int // Files larger than current threshold
	Recommendations SearchConfig
}

// SearchConfig holds configuration for search operations
type SearchConfig struct {
	MaxFileSize     int64
	MaxResults      int
	IncludePatterns []string
	ExcludePatterns []string
	CaseSensitive   bool
	MaxConcurrency  int
	AutoConfigured  bool // Whether this was auto-configured
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
	searchConfig  SearchConfig
	viewport      struct {
		width  int
		height int
		offset int
	}
	showHelp     bool
	quitting     bool
	statusMsg    string
	searching    bool
	searchCancel context.CancelFunc
	progress     SearchProgress
	analysis     FolderAnalysis // Store current analysis
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

	progressStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#50FA7B")).
			Bold(true)

	warningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F1FA8C")).
			Bold(true)
)

func initialModel() model {
	currentDir, _ := os.Getwd()
	m := model{
		mode:       FileBrowserMode,
		currentDir: currentDir,
		searchConfig: SearchConfig{
			MaxFileSize:    MaxFileSize,
			MaxResults:     MaxResultsInMemory,
			MaxConcurrency: MaxConcurrentFiles,
			CaseSensitive:  false,
		},
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
	return tea.Tick(time.Millisecond*ProgressUpdateMs, func(t time.Time) tea.Msg {
		return progressTickMsg{}
	})
}

type progressTickMsg struct{}

type searchCompleteMsg struct {
	results       SearchResults
	selectedCount int
	fileCount     int
	dirCount      int
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.viewport.width = msg.Width
		m.viewport.height = msg.Height - 8 // Reserve space for header, input, and footer
		return m, nil

	case progressTickMsg:
		if m.searching {
			return m, tea.Tick(time.Millisecond*ProgressUpdateMs, func(t time.Time) tea.Msg {
				return progressTickMsg{}
			})
		}
		return m, nil

	case searchCompleteMsg:
		m.handleSearchComplete(msg)
		return m, nil

	case tea.KeyMsg:
		switch m.mode {
		case FileBrowserMode:
			return m.updateFileBrowser(msg)
		case SearchInputMode:
			return m.updateSearchInput(msg)
		case SearchResultsMode:
			return m.updateSearchResults(msg)
		case SearchProgressMode:
			return m.updateSearchProgress(msg)
		case ConfigMode:
			return m.updateConfigMode(msg)
		case AnalysisMode:
			return m.updateAnalysisMode(msg)
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
			if selected.IsDir && selected.Name != ".." {
				// For directories (except parent), toggle selection or enter
				// If Shift+Enter or Ctrl+Enter, toggle selection
				// If just Enter, navigate into directory
				m.currentDir = selected.Path
				m.loadDirectory()
			} else if selected.IsDir && selected.Name == ".." {
				// Parent directory - always navigate
				m.currentDir = selected.Path
				m.loadDirectory()
			} else {
				// Toggle file selection
				m.files[m.selectedFile].Selected = !m.files[m.selectedFile].Selected
				m.statusMsg = fmt.Sprintf("Toggled selection: %s", selected.Name)
			}
		}

	case "space":
		if len(m.files) > 0 {
			selected := m.files[m.selectedFile]
			if selected.Name != ".." {
				// Toggle selection for both files and directories (except parent)
				m.files[m.selectedFile].Selected = !m.files[m.selectedFile].Selected
				if selected.IsDir {
					m.statusMsg = fmt.Sprintf("Toggled directory selection: %s", selected.Name)
				} else {
					m.statusMsg = fmt.Sprintf("Toggled file selection: %s", selected.Name)
				}
			}
		}

	case "ctrl+enter":
		// Alternative way to toggle directory selection without entering
		if len(m.files) > 0 {
			selected := m.files[m.selectedFile]
			if selected.IsDir && selected.Name != ".." {
				m.files[m.selectedFile].Selected = !m.files[m.selectedFile].Selected
				m.statusMsg = fmt.Sprintf("Toggled directory selection: %s", selected.Name)
			}
		}

	case "s", "/":
		m.mode = SearchInputMode
		m.searchInput = ""
		m.statusMsg = "Enter search pattern..."

	case "a":
		// Select all files and directories (except parent)
		count := 0
		for i := range m.files {
			if m.files[i].Name != ".." {
				m.files[i].Selected = true
				count++
			}
		}
		m.statusMsg = fmt.Sprintf("Selected %d items", count)

	case "f":
		// Select all files only
		count := 0
		for i := range m.files {
			if !m.files[i].IsDir {
				m.files[i].Selected = true
				count++
			}
		}
		m.statusMsg = fmt.Sprintf("Selected %d files", count)

	case "d":
		// Toggle directory selection (multiple allowed)
		if len(m.files) > 0 {
			selected := m.files[m.selectedFile]
			if selected.IsDir && selected.Name != ".." {
				m.files[m.selectedFile].Selected = !m.files[m.selectedFile].Selected
				if m.files[m.selectedFile].Selected {
					m.statusMsg = fmt.Sprintf("Selected directory: %s", selected.Name)
				} else {
					m.statusMsg = fmt.Sprintf("Deselected directory: %s", selected.Name)
				}
			} else {
				m.statusMsg = "Can only select directories with 'd'"
			}
		}

	case "A":
		// Deselect all
		for i := range m.files {
			m.files[i].Selected = false
		}
		m.statusMsg = "Deselected all files"

	case "c":
		// Configuration mode
		m.mode = ConfigMode
		m.statusMsg = "Configuration mode - adjust settings for large datasets"

	case "i":
		// Analyze folder
		targets := []string{m.currentDir}
		analysis := m.analyzeFolderStructure(targets)
		m.showFolderAnalysis(analysis)

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

	case "ctrl+d":
		// Select all directories only (except parent)
		count := 0
		for i := range m.files {
			if m.files[i].IsDir && m.files[i].Name != ".." {
				m.files[i].Selected = true
				count++
			}
		}
		m.statusMsg = fmt.Sprintf("Selected %d directories", count)
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
			return m, m.performSearch()
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

func (m model) updateSearchProgress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q", "esc":
		if m.searchCancel != nil {
			m.searchCancel()
		}
		m.mode = FileBrowserMode
		m.searching = false
		m.statusMsg = "Search cancelled"
	}
	return m, nil
}

func (m model) updateConfigMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q", "esc":
		m.mode = FileBrowserMode
		m.statusMsg = "Returned to file browser"

	case "1":
		// Toggle max file size
		if m.searchConfig.MaxFileSize == MaxFileSize {
			m.searchConfig.MaxFileSize = 1 << 30 // 1GB
			m.statusMsg = "Max file size set to 1GB"
		} else {
			m.searchConfig.MaxFileSize = MaxFileSize
			m.statusMsg = "Max file size set to 100MB"
		}

	case "2":
		// Adjust max results
		if m.searchConfig.MaxResults == MaxResultsInMemory {
			m.searchConfig.MaxResults = 50000
			m.statusMsg = "Max results set to 50,000"
		} else {
			m.searchConfig.MaxResults = MaxResultsInMemory
			m.statusMsg = "Max results set to 10,000"
		}

	case "3":
		// Adjust concurrency
		maxCPU := runtime.NumCPU()
		if m.searchConfig.MaxConcurrency == MaxConcurrentFiles {
			m.searchConfig.MaxConcurrency = maxCPU * 2
			m.statusMsg = fmt.Sprintf("Concurrency set to %d (2x CPU cores)", maxCPU*2)
		} else {
			m.searchConfig.MaxConcurrency = MaxConcurrentFiles
			m.statusMsg = fmt.Sprintf("Concurrency set to %d (default)", MaxConcurrentFiles)
		}

	case "h", "?":
		m.showHelp = !m.showHelp
	}
	return m, nil
}

func (m model) updateAnalysisMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q", "esc":
		m.mode = FileBrowserMode
		m.statusMsg = "Returned to file browser"

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

func (m *model) performSearch() tea.Cmd {
	m.searching = true
	m.mode = SearchProgressMode
	m.statusMsg = "Analyzing folder structure..."

	// Get selected files and directories
	var targets []string
	selectedCount := 0
	fileCount := 0
	dirCount := 0

	for _, file := range m.files {
		if file.Selected && file.Name != ".." {
			targets = append(targets, file.Path)
			selectedCount++
			if file.IsDir {
				dirCount++
			} else {
				fileCount++
			}
		}
	}

	// If no files or directories selected, search current directory
	if selectedCount == 0 {
		targets = append(targets, m.currentDir)
	}

	// Analyze folder structure and apply dynamic configuration
	analysis := m.analyzeFolderStructure(targets)
	m.applyDynamicConfig(analysis)

	// Create context for cancellation
	ctx, cancel := context.WithCancel(context.Background())
	m.searchCancel = cancel

	// Return command that will perform search and send completion message
	return func() tea.Msg {
		results := m.performLargeSearchSync(ctx, targets, fileCount, dirCount, selectedCount, analysis)
		return searchCompleteMsg{
			results:       results,
			selectedCount: selectedCount,
			fileCount:     fileCount,
			dirCount:      dirCount,
		}
	}
}

func (m *model) performLargeSearchSync(ctx context.Context, targets []string, fileCount, dirCount, selectedCount int, analysis FolderAnalysis) SearchResults {
	startTime := time.Now()

	results := SearchResults{
		Pattern: m.searchInput,
		Target:  strings.Join(targets, ", "),
		Progress: SearchProgress{
			StartTime: startTime,
		},
	}

	// Validate pattern
	re, err := regexp.Compile(m.searchInput)
	if err != nil {
		results.Errors = append(results.Errors, fmt.Sprintf("Invalid regex pattern: %s", err))
		results.SearchTime = time.Since(startTime)
		return results
	}

	// Collect all files to search
	var allFiles []string
	var totalSize int64

	for _, target := range targets {
		if fileInfo, err := os.Stat(target); err == nil {
			if fileInfo.IsDir() {
				files, size := m.collectFilesFromDir(ctx, target)
				allFiles = append(allFiles, files...)
				totalSize += size
			} else {
				if m.shouldSearchFile(target, fileInfo) {
					allFiles = append(allFiles, target)
					totalSize += fileInfo.Size()
				}
			}
		}
	}

	results.Progress.TotalFiles = int64(len(allFiles))
	results.Progress.TotalSize = totalSize
	results.TotalFiles = len(allFiles)

	// If no files to search, return early
	if len(allFiles) == 0 {
		results.Errors = append(results.Errors, "No searchable files found (all files may be binary, hidden, or too large)")
		results.SearchTime = time.Since(startTime)
		return results
	}

	// Parallel search with worker pool
	resultsChan := make(chan SearchResult, 1000)
	errorsChan := make(chan string, 100)

	// Worker pool
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, m.searchConfig.MaxConcurrency)

	// Progress tracking
	var processedFiles int64
	var processedSize int64

	// Start workers
	for _, filePath := range allFiles {
		select {
		case <-ctx.Done():
			results.Progress.Cancelled = true
			break
		default:
		}

		wg.Add(1)
		go func(path string) {
			defer wg.Done()

			semaphore <- struct{}{}        // Acquire
			defer func() { <-semaphore }() // Release

			// Update progress
			atomic.AddInt64(&processedFiles, 1)
			results.Progress.ProcessedFiles = atomic.LoadInt64(&processedFiles)
			results.Progress.CurrentFile = filepath.Base(path)

			// Search file
			fileResults, fileSize, err := m.searchFileOptimized(ctx, re, path)
			if err != nil {
				select {
				case errorsChan <- err.Error():
				default:
				}
				return
			}

			atomic.AddInt64(&processedSize, fileSize)
			results.Progress.ProcessedSize = atomic.LoadInt64(&processedSize)

			// Send results
			for _, result := range fileResults {
				select {
				case resultsChan <- result:
				case <-ctx.Done():
					return
				}
			}
		}(filePath)
	}

	// Close channels when done
	go func() {
		wg.Wait()
		close(resultsChan)
		close(errorsChan)
	}()

	// Collect results
	var allResults []SearchResult

	// Collect results with memory limit
	for result := range resultsChan {
		if len(allResults) < m.searchConfig.MaxResults {
			allResults = append(allResults, result)
		} else {
			results.Truncated = true
			// Continue draining the channel to prevent goroutine leaks
			go func() {
				for range resultsChan {
					// Drain remaining results
				}
			}()
			break
		}
	}

	// Collect errors
	for err := range errorsChan {
		results.Errors = append(results.Errors, err)
	}

	// Sort results
	sort.Slice(allResults, func(i, j int) bool {
		if allResults[i].FilePath == allResults[j].FilePath {
			return allResults[i].LineNumber < allResults[j].LineNumber
		}
		return allResults[i].FilePath < allResults[j].FilePath
	})

	results.Results = allResults
	results.SearchTime = time.Since(startTime)

	return results
}

func (m *model) collectFilesFromDir(ctx context.Context, dirPath string) ([]string, int64) {
	var files []string
	var totalSize int64

	filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		select {
		case <-ctx.Done():
			return filepath.SkipDir
		default:
		}

		if err != nil {
			return nil
		}

		if !info.IsDir() && m.shouldSearchFile(path, info) {
			files = append(files, path)
			totalSize += info.Size()
		}

		return nil
	})

	return files, totalSize
}

func (m *model) shouldSearchFile(filePath string, info os.FileInfo) bool {
	// Skip hidden files
	if strings.HasPrefix(filepath.Base(filePath), ".") {
		return false
	}

	// Skip large files
	if info.Size() > m.searchConfig.MaxFileSize {
		return false
	}

	// Skip binary files (basic check) - but be more permissive
	if m.isBinaryFile(filePath) {
		return false
	}

	// For debugging - let's be more permissive with text files
	ext := strings.ToLower(filepath.Ext(filePath))

	// Allow common text file extensions and files without extensions
	textExts := []string{
		"", ".txt", ".md", ".go", ".js", ".ts", ".py", ".java", ".c", ".cpp", ".h", ".hpp",
		".rs", ".rb", ".php", ".html", ".css", ".json", ".xml", ".yaml", ".yml", ".toml",
		".sh", ".bash", ".zsh", ".fish", ".ps1", ".bat", ".cmd", ".sql", ".log", ".conf",
		".cfg", ".ini", ".env", ".gitignore", ".dockerfile", ".makefile", ".cmake",
	}

	for _, textExt := range textExts {
		if ext == textExt {
			return true
		}
	}

	// If no extension or unknown extension, try to detect if it's text
	// For now, allow it and let the search handle it
	return true
}

func (m *model) isBinaryFile(filePath string) bool {
	// Simple binary file detection
	ext := strings.ToLower(filepath.Ext(filePath))
	binaryExts := []string{
		".exe", ".bin", ".so", ".dll", ".dylib", ".a", ".o",
		".jpg", ".jpeg", ".png", ".gif", ".bmp", ".ico",
		".mp3", ".mp4", ".avi", ".mov", ".wav", ".flac",
		".zip", ".tar", ".gz", ".bz2", ".xz", ".7z",
		".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx",
	}

	for _, binaryExt := range binaryExts {
		if ext == binaryExt {
			return true
		}
	}

	return false
}

func (m *model) searchFileOptimized(ctx context.Context, re *regexp.Regexp, filePath string) ([]SearchResult, int64, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, 0, fmt.Errorf("unable to open file %s: %v", filePath, err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return nil, 0, fmt.Errorf("unable to get file info %s: %v", filePath, err)
	}

	var results []SearchResult
	scanner := bufio.NewScanner(file)

	// Use larger buffer for better performance
	buf := make([]byte, 0, BufferSize)
	scanner.Buffer(buf, BufferSize)

	lineNum := 1

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return results, fileInfo.Size(), nil
		default:
		}

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
				results = append(results, result)
			}
		}
		lineNum++
	}

	if err := scanner.Err(); err != nil {
		return results, fileInfo.Size(), fmt.Errorf("error reading file %s: %v", filePath, err)
	}

	return results, fileInfo.Size(), nil
}

func (m *model) finishSearch(results SearchResults, selectedCount, fileCount, dirCount int) {
	// Update the model with results - this needs to be thread-safe
	m.searchResults = results
	m.resultIndex = 0
	m.searching = false
	m.mode = SearchResultsMode
	m.searchCancel = nil

	// Enhanced status message
	statusParts := []string{
		fmt.Sprintf("Found %d matches", len(results.Results)),
	}

	if results.Truncated {
		statusParts = append(statusParts, fmt.Sprintf("(truncated at %d)", m.searchConfig.MaxResults))
	}

	statusParts = append(statusParts, fmt.Sprintf("in %d files", results.TotalFiles))

	if selectedCount > 0 {
		var targetDesc string
		if fileCount > 0 && dirCount > 0 {
			targetDesc = fmt.Sprintf("(searched %d files and %d directories)", fileCount, dirCount)
		} else if fileCount > 0 {
			targetDesc = fmt.Sprintf("(searched %d files)", fileCount)
		} else {
			targetDesc = fmt.Sprintf("(searched %d directories)", dirCount)
		}
		statusParts = append(statusParts, targetDesc)
	} else {
		statusParts = append(statusParts, "(searched current directory)")
	}

	statusParts = append(statusParts, fmt.Sprintf("in %v", results.SearchTime))

	// Add error information if any
	if len(results.Errors) > 0 {
		statusParts = append(statusParts, fmt.Sprintf("(%d errors)", len(results.Errors)))
	}

	m.statusMsg = strings.Join(statusParts, " ")
}

func (m model) View() string {
	if m.quitting {
		return "Thanks for using zx! 👋\n"
	}

	var b strings.Builder

	// Header
	switch m.mode {
	case FileBrowserMode:
		title := fmt.Sprintf(" ZX - %s ", m.currentDir)
		b.WriteString(titleStyle.Render(title))
	case SearchInputMode:
		title := " ZX Search Input "
		b.WriteString(titleStyle.Render(title))
	case SearchResultsMode:
		title := fmt.Sprintf(" ZX Search Results - '%s' ", m.searchResults.Pattern)
		b.WriteString(titleStyle.Render(title))
	case SearchProgressMode:
		title := " ZX Search Progress "
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
	case SearchProgressMode:
		b.WriteString(m.renderSearchProgress())
	case ConfigMode:
		b.WriteString(m.renderConfig())
	case AnalysisMode:
		b.WriteString(m.renderAnalysis())
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

	// Selected files and directories info
	selectedFiles := 0
	selectedDirs := 0
	for _, file := range m.files {
		if file.Selected && file.Name != ".." {
			if file.IsDir {
				selectedDirs++
			} else {
				selectedFiles++
			}
		}
	}

	if selectedFiles > 0 || selectedDirs > 0 {
		var targetInfo string
		if selectedFiles > 0 && selectedDirs > 0 {
			targetInfo = fmt.Sprintf("Will search in %d selected files and %d selected directories", selectedFiles, selectedDirs)
		} else if selectedFiles > 0 {
			targetInfo = fmt.Sprintf("Will search in %d selected files", selectedFiles)
		} else {
			targetInfo = fmt.Sprintf("Will search in %d selected directories", selectedDirs)
		}
		b.WriteString(headerStyle.Render(targetInfo))
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

func (m model) renderSearchProgress() string {
	var b strings.Builder

	progress := m.searchResults.Progress

	// Progress summary
	b.WriteString(headerStyle.Render("Search in Progress"))
	b.WriteString("\n\n")

	// Current file being processed
	if progress.CurrentFile != "" {
		b.WriteString(fmt.Sprintf("Processing: %s", progress.CurrentFile))
		b.WriteString("\n\n")
	}

	// Progress bars
	if progress.TotalFiles > 0 {
		fileProgress := float64(progress.ProcessedFiles) / float64(progress.TotalFiles) * 100
		b.WriteString(fmt.Sprintf("Files: %d/%d (%.1f%%)",
			progress.ProcessedFiles, progress.TotalFiles, fileProgress))
		b.WriteString("\n")
		b.WriteString(m.renderProgressBar(fileProgress, 50))
		b.WriteString("\n\n")
	}

	if progress.TotalSize > 0 {
		sizeProgress := float64(progress.ProcessedSize) / float64(progress.TotalSize) * 100
		b.WriteString(fmt.Sprintf("Data: %s/%s (%.1f%%)",
			formatSize(progress.ProcessedSize), formatSize(progress.TotalSize), sizeProgress))
		b.WriteString("\n")
		b.WriteString(m.renderProgressBar(sizeProgress, 50))
		b.WriteString("\n\n")
	}

	// Time elapsed
	elapsed := time.Since(progress.StartTime)
	b.WriteString(fmt.Sprintf("Elapsed: %v", elapsed.Round(time.Second)))
	b.WriteString("\n")

	// ETA calculation
	if progress.ProcessedFiles > 0 && progress.TotalFiles > 0 {
		rate := float64(progress.ProcessedFiles) / elapsed.Seconds()
		remaining := float64(progress.TotalFiles - progress.ProcessedFiles)
		eta := time.Duration(remaining/rate) * time.Second
		b.WriteString(fmt.Sprintf("ETA: %v", eta.Round(time.Second)))
		b.WriteString("\n")
	}

	// Current results count
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("Matches found so far: %d", len(m.searchResults.Results)))

	// Errors
	if len(progress.Errors) > 0 {
		b.WriteString("\n\n")
		b.WriteString(errorStyle.Render(fmt.Sprintf("Errors: %d", len(progress.Errors))))
	}

	return b.String()
}

func (m model) renderProgressBar(percentage float64, width int) string {
	filled := int(percentage / 100 * float64(width))
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	return progressStyle.Render(fmt.Sprintf("[%s] %.1f%%", bar, percentage))
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
  Space         Toggle file/directory selection
  Ctrl+Enter    Toggle directory selection (without entering)
  d             Toggle directory selection (multiple allowed)
  s//           Start search
  a             Select all files and directories
  f             Select all files only
  Ctrl+D        Select all directories only
  A             Deselect all files and directories
  c             Configuration (performance settings)
  i             Analyze folder (show statistics)
  r             Refresh directory
  g/Home        Go to first item
  G/End         Go to last item
  h/?           Toggle this help
  q/Ctrl+C      Quit

Navigation: Use arrow keys or vim-style keys (j/k)
Selection: Select files and/or directories to search within
Directory Selection: Use Space to select, Enter to navigate, Ctrl+Enter to select without entering
Multiple Directories: Use 'd' to toggle directory selection (allows multiple)
All Directories: Use Ctrl+D to select all directories
Configuration: Press 'c' to adjust settings for large datasets
Analysis: Press 'i' to see why searches might fail
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
	case SearchProgressMode:
		help = `
Search Progress Mode:
  Shows progress of ongoing search
`
	case ConfigMode:
		help = `
Configuration Mode:
  1             Toggle max file size (100MB ↔ 1GB)
  2             Toggle max results (10K ↔ 50K)
  3             Toggle concurrency (50 ↔ 2x CPU cores)
  h/?           Toggle this help
  Esc/q         Return to file browser

Adjust these settings based on your dataset size:
• Large files: Increase max file size
• Many matches: Increase max results  
• Fast search: Increase concurrency
`
	case AnalysisMode:
		help = `
Analysis Mode:
  Shows folder analysis and recommendations
`
	}

	return helpStyle.Render(help)
}

func (m model) renderFooter() string {
	var shortcuts string

	switch m.mode {
	case FileBrowserMode:
		shortcuts = "s:search | Enter:navigate/select | Space:toggle | d:multiple dirs | a:all | f:files | Ctrl+D:all dirs | A:none | c:config | i:analyze | h:help | q:quit"
	case SearchInputMode:
		shortcuts = "Enter:search | Esc:cancel"
	case SearchResultsMode:
		shortcuts = "↑↓:navigate | s:new search | Esc:back | h:help"
	case SearchProgressMode:
		shortcuts = "Esc:cancel"
	case ConfigMode:
		shortcuts = "1:file size | 2:max results | 3:concurrency | h:help | Esc:back"
	case AnalysisMode:
		shortcuts = "h:help | Esc:back"
	}

	return helpStyle.Render(shortcuts)
}

func (m model) renderConfig() string {
	var b strings.Builder

	b.WriteString(headerStyle.Render("Performance Configuration"))
	b.WriteString("\n\n")

	// Current settings
	b.WriteString("Current Settings:\n\n")

	// Max file size
	b.WriteString(fmt.Sprintf("1. Max File Size: %s\n", formatSize(m.searchConfig.MaxFileSize)))
	b.WriteString("   Files larger than this will be skipped\n\n")

	// Max results
	b.WriteString(fmt.Sprintf("2. Max Results: %d\n", m.searchConfig.MaxResults))
	b.WriteString("   Maximum search results to keep in memory\n\n")

	// Concurrency
	b.WriteString(fmt.Sprintf("3. Concurrency: %d workers\n", m.searchConfig.MaxConcurrency))
	b.WriteString(fmt.Sprintf("   CPU cores available: %d\n\n", runtime.NumCPU()))

	// Performance tips
	b.WriteString(warningStyle.Render("Performance Tips for Large Datasets:"))
	b.WriteString("\n\n")
	b.WriteString("• Increase max file size for large codebases\n")
	b.WriteString("• Increase max results if you need more matches\n")
	b.WriteString("• Increase concurrency for faster searching\n")
	b.WriteString("• Use file/directory selection to limit scope\n")
	b.WriteString("• Binary files are automatically skipped\n")

	return b.String()
}

func (m model) renderAnalysis() string {
	var b strings.Builder

	b.WriteString(headerStyle.Render("Folder Analysis"))
	b.WriteString("\n\n")

	analysis := m.analysis

	// File statistics
	b.WriteString(headerStyle.Render("File Statistics:"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("Total Files: %d\n", analysis.TotalFiles))
	b.WriteString(fmt.Sprintf("Text Files: %d\n", analysis.TextFiles))
	b.WriteString(fmt.Sprintf("Binary Files: %d (skipped)\n", analysis.BinaryFiles))
	b.WriteString(fmt.Sprintf("Hidden Files: %d (skipped)\n", analysis.HiddenFiles))
	b.WriteString(fmt.Sprintf("Large Files: %d (may be skipped)\n", analysis.LargeFiles))
	b.WriteString("\n")

	// Size statistics
	b.WriteString(headerStyle.Render("Size Statistics:"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("Total Size: %s\n", formatSize(analysis.TotalSize)))
	b.WriteString(fmt.Sprintf("Largest File: %s\n", formatSize(analysis.LargestFile)))
	b.WriteString(fmt.Sprintf("Average File Size: %s\n", formatSize(analysis.AverageFileSize)))
	b.WriteString("\n")

	// Current configuration
	b.WriteString(headerStyle.Render("Current Configuration:"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("Max File Size: %s\n", formatSize(m.searchConfig.MaxFileSize)))
	b.WriteString(fmt.Sprintf("Max Results: %d\n", m.searchConfig.MaxResults))
	b.WriteString(fmt.Sprintf("Concurrency: %d workers\n", m.searchConfig.MaxConcurrency))
	if m.searchConfig.AutoConfigured {
		b.WriteString(statusStyle.Render("(Auto-configured)"))
	} else {
		b.WriteString(statusStyle.Render("(Manual configuration)"))
	}
	b.WriteString("\n\n")

	// Recommendations
	if analysis.LargeFiles > 0 {
		b.WriteString(warningStyle.Render("⚠️  Potential Issues:"))
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("• %d files are larger than the current limit (%s)\n",
			analysis.LargeFiles, formatSize(m.searchConfig.MaxFileSize)))
		b.WriteString("• These files will be skipped during search\n")
		b.WriteString("• Consider increasing max file size in configuration\n\n")
	}

	// Search scope
	searchableFiles := analysis.TextFiles - analysis.LargeFiles
	if searchableFiles <= 0 {
		b.WriteString(errorStyle.Render("❌ No files will be searched!"))
		b.WriteString("\n")
		b.WriteString("All text files are either hidden or too large.\n")
		b.WriteString("Adjust configuration to include more files.\n")
	} else {
		b.WriteString(progressStyle.Render(fmt.Sprintf("✅ %d files will be searched", searchableFiles)))
		b.WriteString("\n")
	}

	return b.String()
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
	re, err := regexp.Compile(pattern)
	if err != nil {
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

	// Create a temporary model for search methods
	m := &model{
		searchConfig: SearchConfig{
			MaxFileSize:    MaxFileSize,
			MaxResults:     MaxResultsInMemory,
			MaxConcurrency: 1, // Single-threaded for legacy mode
		},
	}

	ctx := context.Background()

	if fileInfo.IsDir() {
		files, _ := m.collectFilesFromDir(ctx, target)
		results.TotalFiles = len(files)

		for _, filePath := range files {
			fileResults, _, err := m.searchFileOptimized(ctx, re, filePath)
			if err != nil {
				results.Errors = append(results.Errors, err.Error())
				continue
			}
			results.Results = append(results.Results, fileResults...)
		}
	} else {
		results.TotalFiles = 1
		fileResults, _, err := m.searchFileOptimized(ctx, re, target)
		if err != nil {
			results.Errors = append(results.Errors, err.Error())
		} else {
			results.Results = fileResults
		}
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

func (m *model) analyzeFolderStructure(targets []string) FolderAnalysis {
	analysis := FolderAnalysis{}

	for _, target := range targets {
		if fileInfo, err := os.Stat(target); err == nil {
			if fileInfo.IsDir() {
				m.analyzeDirectory(target, &analysis)
			} else {
				m.analyzeFile(target, fileInfo, &analysis)
			}
		}
	}

	// Calculate averages
	if analysis.TotalFiles > 0 {
		analysis.AverageFileSize = analysis.TotalSize / int64(analysis.TotalFiles)
	}

	// Generate recommendations
	analysis.Recommendations = m.generateRecommendations(analysis)

	return analysis
}

func (m *model) analyzeDirectory(dirPath string, analysis *FolderAnalysis) {
	filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if !info.IsDir() {
			m.analyzeFile(path, info, analysis)
		}
		return nil
	})
}

func (m *model) analyzeFile(filePath string, info os.FileInfo, analysis *FolderAnalysis) {
	analysis.TotalFiles++
	analysis.TotalSize += info.Size()

	if info.Size() > analysis.LargestFile {
		analysis.LargestFile = info.Size()
	}

	// Check if hidden
	if strings.HasPrefix(filepath.Base(filePath), ".") {
		analysis.HiddenFiles++
		return // Don't count hidden files in other categories
	}

	// Check if binary
	if m.isBinaryFile(filePath) {
		analysis.BinaryFiles++
	} else {
		analysis.TextFiles++
	}

	// Check if larger than current threshold
	if info.Size() > m.searchConfig.MaxFileSize {
		analysis.LargeFiles++
	}
}

func (m *model) generateRecommendations(analysis FolderAnalysis) SearchConfig {
	config := SearchConfig{
		MaxConcurrency: runtime.NumCPU(),
		AutoConfigured: true,
	}

	// Dynamic max file size based on largest files
	if analysis.LargestFile > 0 {
		if analysis.LargestFile <= 1<<20 { // 1MB
			config.MaxFileSize = 10 << 20 // 10MB
		} else if analysis.LargestFile <= 10<<20 { // 10MB
			config.MaxFileSize = 50 << 20 // 50MB
		} else if analysis.LargestFile <= 100<<20 { // 100MB
			config.MaxFileSize = 500 << 20 // 500MB
		} else {
			config.MaxFileSize = 2 << 30 // 2GB
		}
	} else {
		config.MaxFileSize = MaxFileSize // Default 100MB
	}

	// Dynamic max results based on total files
	if analysis.TotalFiles <= 1000 {
		config.MaxResults = 5000
	} else if analysis.TotalFiles <= 10000 {
		config.MaxResults = 15000
	} else if analysis.TotalFiles <= 50000 {
		config.MaxResults = 30000
	} else {
		config.MaxResults = 50000
	}

	// Dynamic concurrency based on file count and system
	cpuCount := runtime.NumCPU()
	if analysis.TotalFiles <= 100 {
		config.MaxConcurrency = min(cpuCount, 10)
	} else if analysis.TotalFiles <= 1000 {
		config.MaxConcurrency = min(cpuCount*2, 25)
	} else {
		config.MaxConcurrency = min(cpuCount*3, 100)
	}

	return config
}

func (m *model) applyDynamicConfig(analysis FolderAnalysis) {
	oldConfig := m.searchConfig
	m.searchConfig = analysis.Recommendations

	// Show what changed
	var changes []string
	if oldConfig.MaxFileSize != m.searchConfig.MaxFileSize {
		changes = append(changes, fmt.Sprintf("Max file size: %s → %s",
			formatSize(oldConfig.MaxFileSize), formatSize(m.searchConfig.MaxFileSize)))
	}
	if oldConfig.MaxResults != m.searchConfig.MaxResults {
		changes = append(changes, fmt.Sprintf("Max results: %d → %d",
			oldConfig.MaxResults, m.searchConfig.MaxResults))
	}
	if oldConfig.MaxConcurrency != m.searchConfig.MaxConcurrency {
		changes = append(changes, fmt.Sprintf("Concurrency: %d → %d workers",
			oldConfig.MaxConcurrency, m.searchConfig.MaxConcurrency))
	}

	if len(changes) > 0 {
		m.statusMsg = fmt.Sprintf("Auto-configured: %s", strings.Join(changes, ", "))
	} else {
		m.statusMsg = "Configuration already optimal"
	}
}

func (m *model) showFolderAnalysis(analysis FolderAnalysis) {
	m.analysis = analysis
	m.mode = AnalysisMode
	m.statusMsg = fmt.Sprintf("Analysis complete: %d files, %s total", analysis.TotalFiles, formatSize(analysis.TotalSize))
}

func (m *model) handleSearchComplete(msg searchCompleteMsg) {
	// Update the model with results
	m.searchResults = msg.results
	m.resultIndex = 0
	m.searching = false
	m.mode = SearchResultsMode
	m.searchCancel = nil

	// Enhanced status message
	statusParts := []string{
		fmt.Sprintf("Found %d matches", len(msg.results.Results)),
	}

	if msg.results.Truncated {
		statusParts = append(statusParts, fmt.Sprintf("(truncated at %d)", m.searchConfig.MaxResults))
	}

	statusParts = append(statusParts, fmt.Sprintf("in %d files", msg.results.TotalFiles))

	if msg.selectedCount > 0 {
		var targetDesc string
		if msg.fileCount > 0 && msg.dirCount > 0 {
			targetDesc = fmt.Sprintf("(searched %d files and %d directories)", msg.fileCount, msg.dirCount)
		} else if msg.fileCount > 0 {
			targetDesc = fmt.Sprintf("(searched %d files)", msg.fileCount)
		} else {
			targetDesc = fmt.Sprintf("(searched %d directories)", msg.dirCount)
		}
		statusParts = append(statusParts, targetDesc)
	} else {
		statusParts = append(statusParts, "(searched current directory)")
	}

	statusParts = append(statusParts, fmt.Sprintf("in %v", msg.results.SearchTime))

	// Add error information if any
	if len(msg.results.Errors) > 0 {
		statusParts = append(statusParts, fmt.Sprintf("(%d errors)", len(msg.results.Errors)))
	}

	m.statusMsg = strings.Join(statusParts, " ")
}
