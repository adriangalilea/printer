package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const version = "0.0.3"

var (
	// Base styles
	baseStyle = lipgloss.NewStyle()

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 2)

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#01FAC6")).
			Bold(true)

	normalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#DDDDDD"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262")).
			Padding(1, 0)

	// File browser styles
	fileStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#DDDDDD"))

	dirStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7D56F4")).
			Bold(true)

	printableStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#01FAC6"))

	selectedFileStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#000000")).
				Background(lipgloss.Color("#01FAC6")).
				Bold(true)

	markedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFD700")).
			Bold(true)

	matchedStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#3C3C3C")).
			Foreground(lipgloss.Color("#DDDDDD"))

	stagedHeaderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#01FAC6")).
				Bold(true)

	activeHeaderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FF6B6B")).
				Bold(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF6B6B")).
			Bold(true)

	// Border styles
	activeBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#01FAC6")).
				Padding(0, 1)

	inactiveBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#626262")).
				Padding(0, 1)
)

type LayoutMode int

const (
	LayoutSingle LayoutMode = iota
	LayoutHorizontal
	LayoutVertical
)

type ActivePane int

const (
	PaneQueue ActivePane = iota
	PaneFiles
)

type QueueSection int

const (
	SectionActive QueueSection = iota
	SectionStaged
)

type FileFocus int

const (
	FocusInput FileFocus = iota
	FocusFileList
)

type FileItem struct {
	Name        string
	Path        string
	IsDir       bool
	IsPrintable bool
	Size        int64
}

type PrintJob struct {
	ID       string
	FileName string
	FilePath string
	Size     int64
	Status   string
}

type StagedFile struct {
	Name    string
	Path    string
	Size    int64
	AddedAt time.Time
}

type model struct {
	layoutMode LayoutMode
	activePane ActivePane
	fileFocus  FileFocus

	// Queue state
	jobs         []PrintJob
	stagedFiles  []StagedFile
	queueSection QueueSection
	activeCursor int
	stagedCursor int
	selected     map[int]bool

	// Dimensions
	width  int
	height int

	// File browser state
	textInput    textinput.Model
	currentDir   string
	files        []FileItem
	fileCursor   int
	fileOffset   int
	markedFiles  map[string]bool // Files checked for staging
	matchedFiles map[string]bool // Files matching pattern (visual only)

	errorMsg string
	args     []string
}

func initialModel(args []string) model {
	ti := textinput.New()
	ti.Placeholder = "Type path or glob pattern (e.g., *.pdf)"
	ti.CharLimit = 256
	ti.Width = 50

	currentDir, _ := os.Getwd()

	m := model{
		layoutMode:   LayoutSingle,
		activePane:   PaneQueue,
		fileFocus:    FocusInput,
		queueSection: SectionActive,
		selected:     make(map[int]bool),
		markedFiles:  make(map[string]bool),
		matchedFiles: make(map[string]bool),
		stagedFiles:  []StagedFile{},
		textInput:    ti,
		currentDir:   currentDir,
		args:         args,
	}

	// If args provided, start with files pane focused
	if len(args) > 0 && args[0] == "add" && len(args) > 1 {
		m.activePane = PaneFiles
		m.fileFocus = FocusInput
		m.textInput.SetValue(strings.Join(args[1:], " "))
		m.textInput.Focus()
	}

	// Always load directory for split view
	m.loadDirectory()

	m.refreshJobs()
	return m
}

func (m *model) refreshJobs() {
	m.jobs = getSystemPrintJobs()
}

func (m *model) loadDirectory() {
	m.files = []FileItem{}
	m.errorMsg = ""

	// Clear matched files, keep marked files
	m.matchedFiles = make(map[string]bool)

	// Read directory contents
	entries, err := ioutil.ReadDir(m.currentDir)
	if err != nil {
		m.errorMsg = fmt.Sprintf("Cannot read directory: %v", err)
		return
	}

	// Add select/deselect all option at the top
	printableCount := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			printableExts := []string{".pdf", ".txt", ".doc", ".docx", ".jpg", ".jpeg", ".png", ".gif"}
			for _, pExt := range printableExts {
				if ext == pExt {
					printableCount++
					break
				}
			}
		}
	}

	if printableCount > 0 {
		m.files = append(m.files, FileItem{
			Name:        fmt.Sprintf("[Select/Deselect All %d Printable Files]", printableCount),
			Path:        "TOGGLE_ALL",
			IsDir:       false,
			IsPrintable: false, // Special item
		})
	}

	// Process entries
	pattern := m.textInput.Value()
	for _, entry := range entries {
		name := entry.Name()
		path := filepath.Join(m.currentDir, name)

		// Check if it's printable
		isPrintable := false
		if !entry.IsDir() {
			ext := strings.ToLower(filepath.Ext(name))
			printableExts := []string{".pdf", ".txt", ".doc", ".docx", ".jpg", ".jpeg", ".png", ".gif"}
			for _, pExt := range printableExts {
				if ext == pExt {
					isPrintable = true
					break
				}
			}
		}

		// Check if it matches pattern (visual highlight only)
		if pattern != "" && isPrintable {
			if matched, _ := filepath.Match(pattern, name); matched {
				m.matchedFiles[path] = true
			}
		}

		item := FileItem{
			Name:        name,
			Path:        path,
			IsDir:       entry.IsDir(),
			IsPrintable: isPrintable,
			Size:        entry.Size(),
		}

		m.files = append(m.files, item)
	}

	// Sort: special items first, then directories, then files
	sort.Slice(m.files, func(i, j int) bool {
		// Keep toggle all at the top
		if m.files[i].Path == "TOGGLE_ALL" {
			return true
		}
		if m.files[j].Path == "TOGGLE_ALL" {
			return false
		}
		if m.files[i].IsDir != m.files[j].IsDir {
			return m.files[i].IsDir
		}
		return m.files[i].Name < m.files[j].Name
	})

	// Reset cursor if out of bounds
	if m.fileCursor >= len(m.files) {
		m.fileCursor = 0
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		textinput.Blink,
		tickCmd(),
	)
}

func (m *model) updateLayoutMode() {
	const (
		minQueueWidth = 60
		minFilesWidth = 50
		minPaneHeight = 12 // Minimum height for a useful pane
		helpBarHeight = 1  // Space needed for help bar
	)

	// Account for help bar in height calculations
	availableHeight := m.height - helpBarHeight - 1

	// Check for horizontal split (side by side)
	// Need enough width for both panes and enough height for one pane
	if m.width >= (minQueueWidth+minFilesWidth) && availableHeight >= minPaneHeight {
		m.layoutMode = LayoutHorizontal
	} else if m.width >= minQueueWidth && availableHeight >= (minPaneHeight*2) {
		// Check for vertical split (stacked)
		// Need enough height for two panes plus help bar
		m.layoutMode = LayoutVertical
	} else {
		// Single pane mode for small terminals
		m.layoutMode = LayoutSingle
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle global shortcuts first
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit

		case "tab":
			// Move to next pane
			if m.layoutMode != LayoutSingle {
				if m.activePane == PaneQueue {
					m.activePane = PaneFiles
				} else {
					m.activePane = PaneQueue
				}
			}
			return m, nil

		case "shift+tab":
			// Move to previous pane (same as tab in 2-pane layout)
			if m.layoutMode != LayoutSingle {
				if m.activePane == PaneQueue {
					m.activePane = PaneFiles
				} else {
					m.activePane = PaneQueue
				}
			}
			return m, nil

		case "P":
			// Send all staged files to printer from any context
			if len(m.stagedFiles) > 0 {
				for _, file := range m.stagedFiles {
					addToPrintQueue(file.Path)
					delete(m.markedFiles, file.Path)
				}
				m.stagedFiles = []StagedFile{}
				m.stagedCursor = 0
				if m.activePane == PaneQueue {
					m.queueSection = SectionActive
				}
				m.refreshJobs()
			}
			return m, nil

		case "X":
			// Clear all staged files from any context
			for _, file := range m.stagedFiles {
				delete(m.markedFiles, file.Path)
			}
			m.stagedFiles = []StagedFile{}
			m.stagedCursor = 0
			return m, nil
		}

		// Route to appropriate handler based on active pane
		if m.activePane == PaneQueue {
			return m.updateQueuePane(msg)
		} else {
			return m.updateFilesPane(msg)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Determine layout mode based on terminal size
		m.updateLayoutMode()

		// Update text input width based on layout
		if m.layoutMode == LayoutHorizontal {
			m.textInput.Width = (m.width / 2) - 6
		} else {
			m.textInput.Width = m.width - 6
		}
		if m.textInput.Width > 100 {
			m.textInput.Width = 100
		}

	case tickMsg:
		// Always refresh jobs for split view
		m.refreshJobs()
		return m, tickCmd()
	}

	return m, tea.Batch(cmds...)
}

func (m model) updateQueuePane(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		// In split view, q switches to queue pane
		if m.layoutMode != LayoutSingle && m.activePane == PaneFiles {
			m.activePane = PaneQueue
			m.textInput.Blur()
			return m, nil
		}
		return m, tea.Quit

	case "up", "k":
		if m.queueSection == SectionActive {
			if m.activeCursor > 0 {
				m.activeCursor--
			}
		} else {
			if m.stagedCursor > 0 {
				m.stagedCursor--
			} else {
				// Move to active section
				m.queueSection = SectionActive
				if len(m.jobs) > 0 {
					m.activeCursor = len(m.jobs) - 1
				}
			}
		}

	case "down", "j":
		if m.queueSection == SectionActive {
			if m.activeCursor < len(m.jobs)-1 {
				m.activeCursor++
			} else if len(m.stagedFiles) > 0 {
				// Move to staged section
				m.queueSection = SectionStaged
				m.stagedCursor = 0
			} else if m.layoutMode != LayoutSingle {
				// No staged files, go to files pane
				m.activePane = PaneFiles
				m.fileFocus = FocusInput
				m.textInput.Focus()
				return m, textinput.Blink
			}
		} else {
			if m.stagedCursor < len(m.stagedFiles)-1 {
				m.stagedCursor++
			} else if m.layoutMode != LayoutSingle {
				// At bottom of staged, move to files pane
				m.activePane = PaneFiles
				m.fileFocus = FocusInput
				m.textInput.Focus()
				return m, textinput.Blink
			}
		}

	case "a", "f":
		// Switch to files pane
		if m.layoutMode == LayoutSingle {
			m.activePane = PaneFiles
			m.fileFocus = FocusInput
			m.textInput.Focus()
			return m, textinput.Blink
		}
		// In split view, just switch focus
		m.activePane = PaneFiles
		if m.fileFocus == FocusInput {
			m.textInput.Focus()
			return m, textinput.Blink
		}

	case "x":
		if m.queueSection == SectionActive && m.activeCursor < len(m.jobs) {
			cancelPrintJob(m.jobs[m.activeCursor].ID)
			m.refreshJobs()
		} else if m.queueSection == SectionStaged && m.stagedCursor < len(m.stagedFiles) {
			// Remove from staged and unmark
			if m.stagedCursor < len(m.stagedFiles) {
				filePath := m.stagedFiles[m.stagedCursor].Path
				delete(m.markedFiles, filePath)
				m.stagedFiles = append(m.stagedFiles[:m.stagedCursor], m.stagedFiles[m.stagedCursor+1:]...)
				if m.stagedCursor >= len(m.stagedFiles) && m.stagedCursor > 0 {
					m.stagedCursor--
				}
			}
		}

	case "o":
		if m.queueSection == SectionActive && m.activeCursor < len(m.jobs) {
			openFile(m.jobs[m.activeCursor].FilePath)
		} else if m.queueSection == SectionStaged && m.stagedCursor < len(m.stagedFiles) {
			openFile(m.stagedFiles[m.stagedCursor].Path)
		}

	case "O":
		if m.queueSection == SectionActive && m.activeCursor < len(m.jobs) {
			openFolder(m.jobs[m.activeCursor].FilePath)
		} else if m.queueSection == SectionStaged && m.stagedCursor < len(m.stagedFiles) {
			openFolder(m.stagedFiles[m.stagedCursor].Path)
		}

	case "r":
		m.refreshJobs()

	case " ":
		if m.queueSection == SectionActive && m.activeCursor < len(m.jobs) {
			if m.selected[m.activeCursor] {
				delete(m.selected, m.activeCursor)
			} else {
				m.selected[m.activeCursor] = true
			}
		}

	}

	return m, nil
}

func (m model) updateFilesPane(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	// If text input is focused, let it handle most keys first
	if m.fileFocus == FocusInput {
		// Only intercept special navigation keys
		switch msg.String() {
		case "esc":
			// Switch back to queue pane
			if m.layoutMode == LayoutSingle {
				m.activePane = PaneQueue
				m.textInput.Blur()
				return m, nil
			}
			// In split view, just switch focus
			m.activePane = PaneQueue
			m.textInput.Blur()
			return m, nil

		case "enter":
			// Enter stages all matched files
			if m.textInput.Value() != "" {
				for path := range m.matchedFiles {
					if !m.markedFiles[path] {
						// Find the file in the list and stage it
						for _, f := range m.files {
							if f.Path == path && f.IsPrintable {
								m.markedFiles[path] = true
								m.stagedFiles = append(m.stagedFiles, StagedFile{
									Name:    f.Name,
									Path:    f.Path,
									Size:    f.Size,
									AddedAt: time.Now(),
								})
								break
							}
						}
					}
				}
			}
			// Move focus to file list
			m.fileFocus = FocusFileList
			m.textInput.Blur()
			if len(m.files) > 0 {
				m.fileCursor = 0
			}
			return m, nil

		case "down", "j":
			// Down arrow moves to file list
			m.fileFocus = FocusFileList
			m.textInput.Blur()
			if len(m.files) > 0 {
				m.fileCursor = 0
			}
			return m, nil

		case "up", "k":
			// Up arrow moves to queue pane if in split view
			if m.layoutMode != LayoutSingle {
				m.activePane = PaneQueue
				m.textInput.Blur()
				// Go to bottom of staged or active section
				if len(m.stagedFiles) > 0 {
					m.queueSection = SectionStaged
					m.stagedCursor = len(m.stagedFiles) - 1
				} else if len(m.jobs) > 0 {
					m.queueSection = SectionActive
					m.activeCursor = len(m.jobs) - 1
				}
				return m, nil
			}
			// In single pane, can't go up from input
			return m, nil

		default:
			// Let text input handle all other keys (including left/right arrows)
			oldValue := m.textInput.Value()
			m.textInput, cmd = m.textInput.Update(msg)
			if m.textInput.Value() != oldValue {
				m.loadDirectory()
			}
			return m, cmd
		}
	}

	// Handle keys when file list is focused
	switch msg.String() {
	case "x":
		// Unmark file at cursor
		if m.fileFocus == FocusFileList && m.fileCursor < len(m.files) {
			file := m.files[m.fileCursor]
			if file.IsPrintable && m.markedFiles[file.Path] {
				// Unmark file and remove from staged
				delete(m.markedFiles, file.Path)
				for i := len(m.stagedFiles) - 1; i >= 0; i-- {
					if m.stagedFiles[i].Path == file.Path {
						m.stagedFiles = append(m.stagedFiles[:i], m.stagedFiles[i+1:]...)
						break
					}
				}
			}
		}
		return m, nil

	case "esc", "q":
		// Switch back to queue pane
		if m.layoutMode == LayoutSingle {
			m.activePane = PaneQueue
			m.textInput.Blur()
			return m, nil
		}
		// In split view, just switch focus
		m.activePane = PaneQueue
		m.textInput.Blur()
		return m, nil

	case "enter":
		if m.fileFocus == FocusFileList && m.fileCursor < len(m.files) {
			file := m.files[m.fileCursor]
			if file.IsDir {
				// Navigate into directory
				m.currentDir = file.Path
				m.fileCursor = 0
				m.fileOffset = 0
				m.loadDirectory()
			} else if file.IsPrintable {
				// Stage single file if not already staged
				if !m.markedFiles[file.Path] {
					m.markedFiles[file.Path] = true
					// Add to staged files
					m.stagedFiles = append(m.stagedFiles, StagedFile{
						Name:    file.Name,
						Path:    file.Path,
						Size:    file.Size,
						AddedAt: time.Now(),
					})
				}
			}
		}
		return m, nil

	case "up", "k":
		if m.fileFocus == FocusFileList {
			if m.fileCursor > 0 {
				m.fileCursor--
			} else {
				// At top of file list, move to input
				m.fileFocus = FocusInput
				m.textInput.Focus()
				return m, textinput.Blink
			}
		}
		return m, nil

	case "down", "j":
		if m.fileFocus == FocusFileList {
			if m.fileCursor < len(m.files)-1 {
				m.fileCursor++
			}
		}
		return m, nil

	case "left", "h", "backspace":
		if m.fileFocus == FocusFileList && m.currentDir != "/" {
			// Go to parent directory
			m.currentDir = filepath.Dir(m.currentDir)
			m.fileCursor = 0
			m.fileOffset = 0
			m.loadDirectory()
		}
		return m, nil

	case "right", "l":
		if m.fileFocus == FocusFileList && m.fileCursor < len(m.files) {
			file := m.files[m.fileCursor]
			if file.IsDir {
				// Navigate into directory
				m.currentDir = file.Path
				m.fileCursor = 0
				m.fileOffset = 0
				m.loadDirectory()
			}
		}
		return m, nil

	case " ":
		if m.fileFocus == FocusFileList && m.fileCursor < len(m.files) {
			file := m.files[m.fileCursor]
			if file.Path == "TOGGLE_ALL" {
				// Toggle all printable files
				allMarked := true
				for _, f := range m.files {
					if f.IsPrintable && !m.markedFiles[f.Path] {
						allMarked = false
						break
					}
				}

				if allMarked {
					// Deselect all and remove from staged
					for path := range m.markedFiles {
						// Remove from staged files
						for i := len(m.stagedFiles) - 1; i >= 0; i-- {
							if m.stagedFiles[i].Path == path {
								m.stagedFiles = append(m.stagedFiles[:i], m.stagedFiles[i+1:]...)
							}
						}
					}
					m.markedFiles = make(map[string]bool)
				} else {
					// Select all printable files and add to staged
					for _, f := range m.files {
						if f.IsPrintable && !m.markedFiles[f.Path] {
							m.markedFiles[f.Path] = true
							m.stagedFiles = append(m.stagedFiles, StagedFile{
								Name:    f.Name,
								Path:    f.Path,
								Size:    f.Size,
								AddedAt: time.Now(),
							})
						}
					}
				}
			} else if file.IsPrintable {
				if m.markedFiles[file.Path] {
					// Unmark and remove from staged
					delete(m.markedFiles, file.Path)
					for i := len(m.stagedFiles) - 1; i >= 0; i-- {
						if m.stagedFiles[i].Path == file.Path {
							m.stagedFiles = append(m.stagedFiles[:i], m.stagedFiles[i+1:]...)
							break
						}
					}
				} else {
					// Mark and add to staged
					m.markedFiles[file.Path] = true
					m.stagedFiles = append(m.stagedFiles, StagedFile{
						Name:    file.Name,
						Path:    file.Path,
						Size:    file.Size,
						AddedAt: time.Now(),
					})
				}
			}
		}
		return m, nil
	}

	return m, nil
}

func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Initializing..."
	}

	switch m.layoutMode {
	case LayoutHorizontal:
		return m.viewSplitHorizontal()
	case LayoutVertical:
		return m.viewSplitVertical()
	default:
		return m.viewSinglePane()
	}
}

func (m model) viewSinglePane() string {
	if m.activePane == PaneFiles {
		return m.viewFilesPane()
	}
	return m.viewQueuePane()
}

func (m model) viewSplitHorizontal() string {
	const helpHeight = 2 // Help bar needs more space

	// Calculate pane dimensions
	leftWidth := m.width / 2
	rightWidth := m.width - leftWidth
	paneHeight := m.height - helpHeight

	// Queue pane
	queueBorder := inactiveBorderStyle
	if m.activePane == PaneQueue {
		queueBorder = activeBorderStyle
	}
	queueContent := m.renderQueueContent(leftWidth-6, paneHeight-4) // Account for border + padding
	queuePane := queueBorder.Copy().
		Width(leftWidth - 2).
		Height(paneHeight - 2).
		Render(queueContent)

	// Files pane
	filesBorder := inactiveBorderStyle
	if m.activePane == PaneFiles {
		filesBorder = activeBorderStyle
	}
	filesContent := m.renderFilesContent(rightWidth-6, paneHeight-4) // Account for border + padding
	filesPane := filesBorder.Copy().
		Width(rightWidth - 2).
		Height(paneHeight - 2).
		Render(filesContent)

	// Join and add help
	content := lipgloss.JoinHorizontal(lipgloss.Top, queuePane, filesPane)
	help := m.renderHelpBar()

	return content + help
}

func (m model) viewSplitVertical() string {
	const helpHeight = 2 // Help bar needs more space

	// Calculate pane heights
	availableHeight := m.height - helpHeight
	topHeight := availableHeight / 2
	bottomHeight := availableHeight - topHeight

	// Queue pane
	queueBorder := inactiveBorderStyle
	if m.activePane == PaneQueue {
		queueBorder = activeBorderStyle
	}
	queueContent := m.renderQueueContent(m.width-6, topHeight-4) // Account for border + padding
	queuePane := queueBorder.Copy().
		Width(m.width - 2).
		Height(topHeight - 2).
		Render(queueContent)

	// Files pane
	filesBorder := inactiveBorderStyle
	if m.activePane == PaneFiles {
		filesBorder = activeBorderStyle
	}
	filesContent := m.renderFilesContent(m.width-6, bottomHeight-4) // Account for border + padding
	filesPane := filesBorder.Copy().
		Width(m.width - 2).
		Height(bottomHeight - 2).
		Render(filesContent)

	// Join and add help
	content := lipgloss.JoinVertical(lipgloss.Left, queuePane, filesPane)
	help := m.renderHelpBar()

	return content + help
}

func (m model) renderHelpBar() string {
	var helpText string

	// Global shortcuts
	helpText = "P: print staged • X: clear staged • "

	if m.layoutMode != LayoutSingle {
		helpText += "tab: switch pane • "
	}

	if m.activePane == PaneQueue {
		helpText += "↑↓: nav • x: cancel • o: open • q: quit"
	} else {
		if m.fileFocus == FocusInput {
			helpText += "↓: list • esc: back"
		} else {
			helpText += "←→: dirs • space: mark • ↑: input"
		}
	}

	return helpStyle.Copy().
		Width(m.width - 2).
		Render(helpText)
}

func (m model) viewQueuePane() string {
	// Calculate dimensions
	contentHeight := m.height - 4 // Title, spacing, help
	contentWidth := m.width - 4   // Margins

	// Title bar
	title := titleStyle.Copy().
		Width(contentWidth).
		Align(lipgloss.Center).
		Render("🖨  Printer Queue Manager")

	// Content area
	queueContent := m.renderQueueContent(contentWidth, contentHeight-6)

	// Help bar
	help := m.renderHelpBar()

	// Combine all parts
	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		queueContent,
		"",
		help,
	)
}

func (m model) viewFilesPane() string {
	contentWidth := m.width - 4
	contentHeight := m.height - 4 // Title, help, spacing

	// Title
	title := titleStyle.Copy().
		Width(contentWidth).
		Align(lipgloss.Center).
		Render("📁 Add Files to Print Queue")

	// Files content
	filesContent := m.renderFilesContent(contentWidth, contentHeight-4)

	// Help
	help := m.renderHelpBar()

	// Combine all parts
	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		filesContent,
		"",
		help,
	)
}

func (m model) renderQueueContent(width, height int) string {
	if height <= 0 {
		return ""
	}

	var content strings.Builder

	// Calculate space for each section (split evenly)
	// Reserve space for headers and separator
	availableHeight := height - 5 // 2 headers, separator, spacing
	activeHeight := availableHeight / 2
	stagedHeight := availableHeight - activeHeight

	// Ensure minimum heights
	if activeHeight < 2 {
		activeHeight = 2
	}
	if stagedHeight < 2 {
		stagedHeight = 2
	}

	// Active print jobs section
	content.WriteString(activeHeaderStyle.Render("🖨  PRINTING"))
	content.WriteString("\n")

	if len(m.jobs) == 0 {
		content.WriteString(dimStyle.Render("  No active jobs"))
	} else {
		for i, job := range m.jobs {
			if i >= activeHeight-2 {
				break
			}

			cursor := "  "
			if i == m.activeCursor && m.activePane == PaneQueue && m.queueSection == SectionActive {
				cursor = "▶ "
			}

			selected := " "
			if m.selected[i] {
				selected = "✓"
			}

			fileName := job.FileName
			if len(fileName) > width-20 && width > 20 {
				fileName = fileName[:width-23] + "..."
			}

			line := fmt.Sprintf("%s[%s] %s", cursor, selected, fileName)

			if i == m.activeCursor && m.activePane == PaneQueue && m.queueSection == SectionActive {
				content.WriteString(selectedStyle.Render(line))
			} else {
				content.WriteString(normalStyle.Render(line))
			}
			content.WriteString("\n")
		}
	}

	// Separator
	content.WriteString("\n")
	content.WriteString(dimStyle.Render(strings.Repeat("─", width)))
	content.WriteString("\n\n")

	// Staged files section
	content.WriteString(stagedHeaderStyle.Render("📄 STAGED"))
	content.WriteString("\n")

	if len(m.stagedFiles) == 0 {
		content.WriteString(dimStyle.Render("  No staged files (use space to mark)"))
	} else {
		// Show all staged files as an interactive list
		stagedStart := 0
		stagedVisible := stagedHeight - 3

		// Handle scrolling for staged section
		if m.queueSection == SectionStaged && m.stagedCursor >= stagedStart+stagedVisible {
			stagedStart = m.stagedCursor - stagedVisible + 1
		}

		stagedEnd := stagedStart + stagedVisible
		if stagedEnd > len(m.stagedFiles) {
			stagedEnd = len(m.stagedFiles)
		}

		for i := stagedStart; i < stagedEnd; i++ {
			file := m.stagedFiles[i]

			cursor := "  "
			if i == m.stagedCursor && m.activePane == PaneQueue && m.queueSection == SectionStaged {
				cursor = "▶ "
			}

			fileName := file.Name
			if len(fileName) > width-10 && width > 10 {
				fileName = fileName[:width-13] + "..."
			}

			line := fmt.Sprintf("%s%s", cursor, fileName)

			if i == m.stagedCursor && m.activePane == PaneQueue && m.queueSection == SectionStaged {
				content.WriteString(selectedStyle.Render(line))
			} else {
				content.WriteString(printableStyle.Render(line))
			}

			if i < stagedEnd-1 {
				content.WriteString("\n")
			}
		}

		// Show count if there are more items than visible
		if len(m.stagedFiles) > stagedVisible {
			content.WriteString("\n")
			content.WriteString(dimStyle.Render(fmt.Sprintf("  (%d/%d)", m.stagedCursor+1, len(m.stagedFiles))))
		}
	}

	return content.String()
}

func (m model) renderFilesContent(width, height int) string {
	if height <= 0 {
		return ""
	}

	var content strings.Builder
	content.WriteString("📁 File Browser\n\n")

	// Input field
	content.WriteString(m.textInput.View())
	content.WriteString("\n")

	// Current directory
	displayDir := m.currentDir
	if home, _ := os.UserHomeDir(); strings.HasPrefix(displayDir, home) {
		displayDir = "~" + strings.TrimPrefix(displayDir, home)
	}
	if len(displayDir) > width-5 && width > 5 {
		displayDir = "..." + displayDir[len(displayDir)-(width-8):]
	}
	content.WriteString(dimStyle.Render("📂 " + displayDir))
	content.WriteString("\n")

	// File list
	if len(m.files) == 0 {
		content.WriteString(dimStyle.Render("No files"))
	} else {
		// Calculate visible files (account for title, input, path)
		visibleFiles := height - 5
		if visibleFiles < 1 {
			visibleFiles = 1
		}
		if m.fileCursor >= m.fileOffset+visibleFiles {
			m.fileOffset = m.fileCursor - visibleFiles + 1
		} else if m.fileCursor < m.fileOffset {
			m.fileOffset = m.fileCursor
		}

		end := m.fileOffset + visibleFiles
		if end > len(m.files) {
			end = len(m.files)
		}

		for i := m.fileOffset; i < end; i++ {
			file := m.files[i]

			// Cursor and selection indicators
			cursor := "  "
			if i == m.fileCursor && m.activePane == PaneFiles {
				cursor = "▶ "
			}

			marked := " "
			if m.markedFiles[file.Path] {
				marked = "✓"
			}

			// Format file name
			displayName := file.Name
			maxNameLen := width - 10
			if len(displayName) > maxNameLen && maxNameLen > 0 {
				displayName = displayName[:maxNameLen-3] + "..."
			}

			// Special handling for toggle all item
			if file.Path == "TOGGLE_ALL" {
				line := fmt.Sprintf("%s    %s", cursor, displayName)
				content.WriteString(selectedStyle.Render(line))
				if i < end-1 {
					content.WriteString("\n")
				}
				continue
			}

			// Add type indicator
			typeIndicator := "   "
			if file.IsDir {
				typeIndicator = "📁 "
			} else if file.IsPrintable {
				typeIndicator = "📄 "
			}

			line := fmt.Sprintf("%s[%s] %s%s", cursor, marked, typeIndicator, displayName)

			// Apply styles based on state combinations
			var styledLine string
			isCursor := i == m.fileCursor && m.activePane == PaneFiles && m.fileFocus == FocusFileList
			isMarked := m.markedFiles[file.Path]
			isMatched := m.matchedFiles[file.Path]

			if isCursor {
				// Cursor position - highest priority
				if isMarked {
					// Cursor + marked
					styledLine = selectedFileStyle.Copy().Background(lipgloss.Color("#FFD700")).Render(line)
				} else if isMatched {
					// Cursor + matched
					styledLine = selectedFileStyle.Copy().Background(lipgloss.Color("#3C3C3C")).Render(line)
				} else {
					// Just cursor
					styledLine = selectedFileStyle.Render(line)
				}
			} else if isMarked {
				// Marked file
				styledLine = markedStyle.Render(line)
			} else if isMatched {
				// Matched by pattern
				styledLine = matchedStyle.Render(line)
			} else if file.IsDir {
				styledLine = dirStyle.Render(line)
			} else if file.IsPrintable {
				styledLine = printableStyle.Render(line)
			} else {
				styledLine = dimStyle.Render(line)
			}

			content.WriteString(styledLine)
			if i < end-1 {
				content.WriteString("\n")
			}
		}
	}

	return content.String()
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
	var versionFlag bool
	flag.BoolVar(&versionFlag, "version", false, "Print version information")
	flag.BoolVar(&versionFlag, "v", false, "Print version information")
	flag.Parse()

	if versionFlag {
		fmt.Printf("printer version %s\n", version)
		os.Exit(0)
	}

	args := flag.Args()

	p := tea.NewProgram(initialModel(args))
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
