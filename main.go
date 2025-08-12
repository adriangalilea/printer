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

const version = "0.1.2"

var (
	// Base styles
	baseStyle = lipgloss.NewStyle()

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(theme.Text).
			Background(theme.Mauve).
			Padding(0, 2)

	selectedStyle = lipgloss.NewStyle().
			Foreground(theme.Green).
			Bold(true)

	normalStyle = lipgloss.NewStyle().
			Foreground(theme.Text)

	dimStyle = lipgloss.NewStyle().
			Foreground(theme.Surface2)

	helpStyle = lipgloss.NewStyle().
			Foreground(theme.Overlay0)

	// File browser styles
	fileStyle = lipgloss.NewStyle().
			Foreground(theme.Text)

	dirStyle = lipgloss.NewStyle().
			Foreground(theme.Blue).
			Bold(true)

	printableStyle = lipgloss.NewStyle().
			Foreground(theme.Teal)

	selectedFileStyle = lipgloss.NewStyle().
				Foreground(theme.Base).
				Background(theme.Green).
				Bold(true)

	markedStyle = lipgloss.NewStyle().
			Foreground(theme.Yellow).
			Bold(true)

	matchedStyle = lipgloss.NewStyle().
			Background(theme.Surface1).
			Foreground(theme.Text)

	stagedHeaderStyle = lipgloss.NewStyle().
				Foreground(theme.Green).
				Bold(true)

	activeHeaderStyle = lipgloss.NewStyle().
				Foreground(theme.Red).
				Bold(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(theme.Red).
			Bold(true)

	// Border styles
	activeBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(theme.Green).
				Padding(0, 1)

	inactiveBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(theme.Surface2).
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
	Name       string
	Path       string
	StagedFrom string // Directory this was staged from
	Size       int64
	AddedAt    time.Time
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
	textInput       textinput.Model
	currentDir      string
	files           []FileItem
	fileCursor      int
	markedFiles     map[string]bool // Files checked for staging
	matchedFiles    map[string]bool // Files matching pattern (visual only)
	dirCursorMemory map[string]int  // Remember cursor position for each directory

	// Print operations state
	printOps     []PrintOperation

	// Help bar component
	helpBar *HelpBar

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
		layoutMode:      LayoutSingle,
		activePane:      PaneQueue,
		fileFocus:       FocusInput,
		queueSection:    SectionActive,
		selected:        make(map[int]bool),
		markedFiles:     make(map[string]bool),
		matchedFiles:    make(map[string]bool),
		dirCursorMemory: make(map[string]int),
		stagedFiles:     []StagedFile{},
		textInput:       ti,
		currentDir:      currentDir,
		printOps:        []PrintOperation{},
		helpBar:         NewHelpBar(80), // Initial width, will be updated
		args:            args,
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

	// Don't refresh jobs synchronously anymore
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
		refreshJobsCmd(),  // Initial job refresh
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
		// Let help bar handle key if it wants to
		if m.helpBar.HandleKey(msg.String()) {
			return m, nil
		}

		// Handle global shortcuts first
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit

		case "tab":
			// Move to next pane
			if m.layoutMode != LayoutSingle {
				switch m.activePane {
				case PaneQueue:
					m.activePane = PaneFiles
				case PaneFiles:
					m.activePane = PaneQueue
				}
			}
			return m, nil

		case "shift+tab":
			// Move to previous pane
			if m.layoutMode != LayoutSingle {
				switch m.activePane {
				case PaneQueue:
					m.activePane = PaneFiles
				case PaneFiles:
					m.activePane = PaneQueue
				}
			}
			return m, nil

		case "P":
			// Send all staged files to printer from any context
			if len(m.stagedFiles) > 0 {
				// Store start index before adding new operations
				startIndex := len(m.printOps)

				// Create print operations and commands for each staged file
				var printCmds []tea.Cmd
				for _, file := range m.stagedFiles {
					opID := fmt.Sprintf("%s-%d", file.Path, time.Now().UnixNano())
					op := PrintOperation{
						ID:        opID,
						FilePath:  file.Path,
						FileName:  file.Name,
						Status:    StatusSending,  // Start as sending since we submit immediately
						StartedAt: time.Now(),
						UpdatedAt: time.Now(),
					}
					m.printOps = append(m.printOps, op)

					// Submit the print job - it runs async in its own goroutine
					printCmds = append(printCmds, submitPrintJobCmd(opID, file.Path))

					delete(m.markedFiles, file.Path)
				}
				
				// Clear staged files
				m.stagedFiles = []StagedFile{}
				m.stagedCursor = 0

				// Switch to queue pane to show progress
				m.activePane = PaneQueue
				m.queueSection = SectionActive  // Focus on the active jobs section
				// Position cursor at the first newly added operation
				m.activeCursor = len(m.jobs) + startIndex  // Position after system jobs
				
				// Use Batch to run all commands concurrently
				return m, tea.Batch(printCmds...)
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
		switch m.activePane {
		case PaneQueue:
			return m.updateQueuePane(msg)
		case PaneFiles:
			return m.updateFilesPane(msg)
		default:
			return m, nil
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
		// Always continue ticking and refresh
		// The timeout in getSystemPrintJobs prevents hanging
		return m, tea.Batch(
			tickCmd(),          // Continue ticking
			refreshJobsCmd(),   // Refresh jobs in background
		)

	case jobsRefreshedMsg:
		// Update jobs from async refresh
		m.jobs = msg.jobs
		return m, nil

	case PrintStatusMsg:
		// Update print operation status
		for i := range m.printOps {
			if m.printOps[i].ID == msg.FileID {
				m.printOps[i].Status = msg.Status
				m.printOps[i].Error = msg.Error
				m.printOps[i].UpdatedAt = time.Now()
				break
			}
		}
		return m, nil
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
			} else {
				relativeStagedFiles := m.getRelativeStagedFiles()
				if len(relativeStagedFiles) > 0 {
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
			}
		} else {
			relativeStagedFiles := m.getRelativeStagedFiles()
			if m.stagedCursor < len(relativeStagedFiles)-1 {
				m.stagedCursor++
			} else if m.layoutMode != LayoutSingle {
				// At bottom of staged, move to files pane
				m.activePane = PaneFiles
				m.fileFocus = FocusInput
				m.textInput.Focus()
				return m, textinput.Blink
			}
		}
	
	case "pgup", "ctrl+u":
		// Page up in queue
		if m.queueSection == SectionActive {
			newCursor := m.activeCursor - 5
			if newCursor < 0 {
				newCursor = 0
			}
			m.activeCursor = newCursor
		} else {
			newCursor := m.stagedCursor - 5
			if newCursor < 0 {
				// Switch to active section if we have jobs
				if len(m.jobs) > 0 {
					m.queueSection = SectionActive
					m.activeCursor = 0
				} else {
					m.stagedCursor = 0
				}
			} else {
				m.stagedCursor = newCursor
			}
		}
	
	case "pgdown", "ctrl+d":
		// Page down in queue
		if m.queueSection == SectionActive {
			newCursor := m.activeCursor + 5
			if newCursor >= len(m.jobs) {
				// Switch to staged section if we have staged files
				relativeStagedFiles := m.getRelativeStagedFiles()
				if len(relativeStagedFiles) > 0 {
					m.queueSection = SectionStaged
					m.stagedCursor = 0
				} else {
					m.activeCursor = len(m.jobs) - 1
				}
			} else {
				m.activeCursor = newCursor
			}
		} else {
			relativeStagedFiles := m.getRelativeStagedFiles()
			newCursor := m.stagedCursor + 5
			if newCursor >= len(relativeStagedFiles) {
				newCursor = len(relativeStagedFiles) - 1
			}
			if newCursor < 0 {
				newCursor = 0
			}
			m.stagedCursor = newCursor
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

	case "p":
		// Focus on queue pane to see print operations
		m.activePane = PaneQueue
		m.queueSection = SectionActive

	case "x":
		if m.queueSection == SectionActive {
			totalJobs := len(m.jobs) + len(m.printOps)
			if m.activeCursor < totalJobs {
				if m.activeCursor < len(m.jobs) {
					// Cancel system job
					cancelPrintJob(m.jobs[m.activeCursor].ID)
					// Return command to refresh jobs async
					return m, refreshJobsCmd()
				} else {
					// Handle print operation
					opIndex := m.activeCursor - len(m.jobs)
					op := &m.printOps[opIndex]
					if op.Status == StatusSending || op.Status == StatusPending {
						// Cancel active operation
						op.Status = StatusCanceled
						op.UpdatedAt = time.Now()
					} else if op.Status == StatusFailed || op.Status == StatusCanceled || op.Status == StatusSent {
						// Remove completed/failed/canceled operation
						m.printOps = append(m.printOps[:opIndex], m.printOps[opIndex+1:]...)
						if m.activeCursor >= len(m.jobs)+len(m.printOps) && m.activeCursor > 0 {
							m.activeCursor--
						}
					}
				}
			}
		} else if m.queueSection == SectionStaged {
			// Get relative files for current directory
			relativeStagedFiles := m.getRelativeStagedFiles()
			if m.stagedCursor < len(relativeStagedFiles) {
				// Remove from staged and unmark
				filePath := relativeStagedFiles[m.stagedCursor].Path
				delete(m.markedFiles, filePath)
				
				// Remove from the main staged files list
				for i := len(m.stagedFiles) - 1; i >= 0; i-- {
					if m.stagedFiles[i].Path == filePath {
						m.stagedFiles = append(m.stagedFiles[:i], m.stagedFiles[i+1:]...)
						break
					}
				}
				
				// Adjust cursor for relative list
				newRelativeFiles := m.getRelativeStagedFiles()
				if m.stagedCursor >= len(newRelativeFiles) && m.stagedCursor > 0 {
					m.stagedCursor--
				}
			}
		}

	case "o":
		if m.queueSection == SectionActive {
			totalJobs := len(m.jobs) + len(m.printOps)
			if m.activeCursor < totalJobs {
				if m.activeCursor < len(m.jobs) {
					openFile(m.jobs[m.activeCursor].FilePath)
				} else {
					opIndex := m.activeCursor - len(m.jobs)
					openFile(m.printOps[opIndex].FilePath)
				}
			}
		} else if m.queueSection == SectionStaged {
			relativeStagedFiles := m.getRelativeStagedFiles()
			if m.stagedCursor < len(relativeStagedFiles) {
				openFile(relativeStagedFiles[m.stagedCursor].Path)
			}
		}

	case "O":
		if m.queueSection == SectionActive {
			totalJobs := len(m.jobs) + len(m.printOps)
			if m.activeCursor < totalJobs {
				if m.activeCursor < len(m.jobs) {
					openFolder(m.jobs[m.activeCursor].FilePath)
				} else {
					opIndex := m.activeCursor - len(m.jobs)
					openFolder(m.printOps[opIndex].FilePath)
				}
			}
		} else if m.queueSection == SectionStaged && m.stagedCursor < len(m.stagedFiles) {
			openFolder(m.stagedFiles[m.stagedCursor].Path)
		}

	case "r":
		// Refresh jobs asynchronously
		return m, refreshJobsCmd()

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
									Name:       f.Name,
									Path:       f.Path,
									StagedFrom: m.currentDir,
									Size:       f.Size,
									AddedAt:    time.Now(),
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
				relativeStagedFiles := m.getRelativeStagedFiles()
				if len(relativeStagedFiles) > 0 {
					m.queueSection = SectionStaged
					m.stagedCursor = len(relativeStagedFiles) - 1
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
				m.loadDirectory()
			} else if file.IsPrintable {
				// Stage single file if not already staged
				if !m.markedFiles[file.Path] {
					m.markedFiles[file.Path] = true
					// Add to staged files
					m.stagedFiles = append(m.stagedFiles, StagedFile{
						Name:       file.Name,
						Path:       file.Path,
						StagedFrom: m.currentDir,
						Size:       file.Size,
						AddedAt:    time.Now(),
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
	
	case "pgup", "ctrl+u":
		// Page up
		if m.fileFocus == FocusFileList {
			// Move cursor up by 5 items
			newCursor := m.fileCursor - 5
			if newCursor < 0 {
				newCursor = 0
			}
			m.fileCursor = newCursor
		}
		return m, nil
	
	case "pgdown", "ctrl+d":
		// Page down
		if m.fileFocus == FocusFileList {
			// Move cursor down by 5 items
			newCursor := m.fileCursor + 5
			if newCursor >= len(m.files) {
				newCursor = len(m.files) - 1
			}
			if newCursor < 0 {
				newCursor = 0
			}
			m.fileCursor = newCursor
		}
		return m, nil

	case "left", "h", "backspace":
		if m.fileFocus == FocusFileList && m.currentDir != "/" {
			// Save current cursor position for this directory
			m.dirCursorMemory[m.currentDir] = m.fileCursor
			
			// Remember which directory we're leaving (to position cursor on it)
			previousDir := m.currentDir
			previousDirName := filepath.Base(previousDir)
			
			// Go to parent directory
			parentDir := filepath.Dir(m.currentDir)
			m.currentDir = parentDir
			
			m.loadDirectory()
			
			// Try to position cursor on the directory we just left
			foundPrevDir := false
			for i, file := range m.files {
				if file.IsDir && file.Name == previousDirName {
					m.fileCursor = i
					foundPrevDir = true
					break
				}
			}
			
			// If we didn't find the previous directory, restore saved position or default to 0
			if !foundPrevDir {
				if savedCursor, exists := m.dirCursorMemory[parentDir]; exists {
					m.fileCursor = savedCursor
				} else {
					m.fileCursor = 0
				}
			}
			
			// Ensure cursor is within bounds
			if m.fileCursor >= len(m.files) {
				m.fileCursor = len(m.files) - 1
			}
			if m.fileCursor < 0 {
				m.fileCursor = 0
			}
		}
		return m, nil

	case "right", "l":
		if m.fileFocus == FocusFileList && m.fileCursor < len(m.files) {
			file := m.files[m.fileCursor]
			if file.IsDir {
				// Save current cursor position for this directory
				m.dirCursorMemory[m.currentDir] = m.fileCursor
				
				// Navigate into directory
				m.currentDir = file.Path
				
				// Restore cursor position if we've been to this directory before
				if savedCursor, exists := m.dirCursorMemory[file.Path]; exists {
					m.fileCursor = savedCursor
				} else {
					m.fileCursor = 0
				}
				
				m.loadDirectory()
				
				// Ensure cursor is within bounds after loading
				if m.fileCursor >= len(m.files) {
					m.fileCursor = len(m.files) - 1
				}
				if m.fileCursor < 0 {
					m.fileCursor = 0
				}
			} else if file.IsPrintable {
				// For printable files, right arrow acts like space (mark/unmark)
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
						Name:       file.Name,
						Path:       file.Path,
						StagedFrom: m.currentDir,
						Size:       file.Size,
						AddedAt:    time.Now(),
					})
				}
			}
		}
		return m, nil

	case "p":
		// Focus on queue pane to see print operations
		m.activePane = PaneQueue
		m.queueSection = SectionActive
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
								Name:       f.Name,
								Path:       f.Path,
								StagedFrom: m.currentDir,
								Size:       f.Size,
								AddedAt:    time.Now(),
							})
						}
					}
				}
			} else if file.IsDir {
				// Toggle all printable files in directory
				total, staged, _ := m.getDirectoryStatus(file.Path)
				if total > 0 {
					entries, _ := ioutil.ReadDir(file.Path)
					if staged == total {
						// Unmark all files in directory
						for _, entry := range entries {
							if !entry.IsDir() {
								fullPath := filepath.Join(file.Path, entry.Name())
								delete(m.markedFiles, fullPath)
								// Remove from staged
								for i := len(m.stagedFiles) - 1; i >= 0; i-- {
									if m.stagedFiles[i].Path == fullPath {
										m.stagedFiles = append(m.stagedFiles[:i], m.stagedFiles[i+1:]...)
									}
								}
							}
						}
					} else {
						// Mark all printable files in directory
						for _, entry := range entries {
							if !entry.IsDir() {
								ext := strings.ToLower(filepath.Ext(entry.Name()))
								printableExts := []string{".pdf", ".txt", ".doc", ".docx", ".jpg", ".jpeg", ".png", ".gif"}
								isPrintable := false
								for _, pExt := range printableExts {
									if ext == pExt {
										isPrintable = true
										break
									}
								}
								if isPrintable {
									fullPath := filepath.Join(file.Path, entry.Name())
									if !m.markedFiles[fullPath] {
										m.markedFiles[fullPath] = true
										m.stagedFiles = append(m.stagedFiles, StagedFile{
											Name:       entry.Name(),
											Path:       fullPath,
											StagedFrom: file.Path,
											Size:       entry.Size(),
											AddedAt:    time.Now(),
										})
									}
								}
							}
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
						Name:       file.Name,
						Path:       file.Path,
						StagedFrom: m.currentDir,
						Size:       file.Size,
						AddedAt:    time.Now(),
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

	var mainView string
	switch m.layoutMode {
	case LayoutHorizontal:
		mainView = m.viewSplitHorizontal()
	case LayoutVertical:
		mainView = m.viewSplitVertical()
	default:
		mainView = m.viewSinglePane()
	}

	// If help overlay is showing, render it instead of main view
	if m.helpBar.IsShowingFullHelp() {
		overlay := m.helpBar.RenderFullHelp()
		// Center the overlay on the screen
		return lipgloss.Place(m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			overlay)
	}

	return mainView
}

func (m model) viewSinglePane() string {
	switch m.activePane {
	case PaneFiles:
		return m.viewFilesPane()
	default:
		return m.viewQueuePane()
	}
}

func (m model) viewSplitHorizontal() string {
	const helpHeight = 2 // Help bar needs more space
	const pathHeight = 1 // Height for the path display

	// Calculate pane dimensions
	leftWidth := m.width / 2
	rightWidth := m.width - leftWidth
	paneHeight := m.height - helpHeight - pathHeight

	// Queue pane (left side)
	queueBorder := inactiveBorderStyle
	if m.activePane == PaneQueue {
		queueBorder = activeBorderStyle
	}
	queueContent := m.renderQueueContent(leftWidth-6, paneHeight-4)
	queuePane := queueBorder.Copy().
		Width(leftWidth - 2).
		Height(paneHeight - 2).
		Render(queueContent)

	// Files pane (right side)
	filesBorder := inactiveBorderStyle
	if m.activePane == PaneFiles {
		filesBorder = activeBorderStyle
	}
	filesContent := m.renderFilesContent(rightWidth-6, paneHeight-4) // Account for border + padding
	filesPane := filesBorder.Copy().
		Width(rightWidth - 2).
		Height(paneHeight - 2).
		Render(filesContent)

	// Join panes horizontally
	panes := lipgloss.JoinHorizontal(lipgloss.Top, queuePane, filesPane)
	
	// Add current path at the top
	path := m.renderCurrentPath(m.width)
	
	// Add help bar
	help := m.renderHelpBar()

	return path + "\n" + panes + help
}

func (m model) viewSplitVertical() string {
	const helpHeight = 1 // Help bar is 1 line
	const pathHeight = 1 // Path display is 1 line

	// Calculate pane heights
	availableHeight := m.height - helpHeight - pathHeight
	topHeight := availableHeight / 2
	bottomHeight := availableHeight - topHeight

	// Queue pane (top)
	queueBorder := inactiveBorderStyle
	if m.activePane == PaneQueue {
		queueBorder = activeBorderStyle
	}
	queueContent := m.renderQueueContent(m.width-6, topHeight-4)
	queuePane := queueBorder.Copy().
		Width(m.width - 2).
		Height(topHeight - 2).
		Render(queueContent)

	// Current path display (between panes)
	path := m.renderCurrentPath(m.width)

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

	// Help bar
	help := m.renderHelpBar()

	// Join everything vertically
	return lipgloss.JoinVertical(lipgloss.Left, queuePane, path, filesPane, help)
}

func (m *model) renderHelpBar() string {
	// Update help bar context and width
	m.helpBar.Update(m.width - 2, m.activePane, m.layoutMode, m.fileFocus, m.queueSection)
	return m.helpBar.Render()
}

func (m model) renderCurrentPath(width int) string {
	displayDir := m.currentDir
	if home, _ := os.UserHomeDir(); strings.HasPrefix(displayDir, home) {
		displayDir = "~" + strings.TrimPrefix(displayDir, home)
	}
	
	// Truncate if too long
	if len(displayDir) > width-6 && width > 6 {
		displayDir = "..." + displayDir[len(displayDir)-(width-9):]
	}
	
	pathStyle := lipgloss.NewStyle().
		Foreground(theme.Lavender).
		Bold(true).
		Padding(0, 1). // Add padding instead of centering
		Width(width)
		
	return pathStyle.Render("📂 " + displayDir)
}

func (m model) getRelativeStagedFiles() []StagedFile {
	// Return ALL staged files - they'll be shown with relative paths
	return m.stagedFiles
}

func (m model) formatStagedFileName(file StagedFile) string {
	// Always show relative path from current directory
	relPath, err := filepath.Rel(m.currentDir, file.Path)
	if err != nil {
		// If we can't get relative path, show full path
		return file.Path
	}
	
	// If file is in current directory, just show the name
	if !strings.Contains(relPath, string(filepath.Separator)) && !strings.HasPrefix(relPath, "..") {
		return relPath
	}
	
	// Otherwise show the relative path (could be ../, ../../, subdirs/, etc.)
	return relPath
}

func (m model) getDirectoryStatus(dirPath string) (totalPrintable int, stagedCount int, printingCount int) {
	// Don't do file I/O! Use the existing state from the model
	// Count files based on path prefix matching
	
	dirPrefix := dirPath + string(filepath.Separator)
	
	// First pass: count all printable files we know about from staged and jobs
	// We'll assume if a file is staged or printing, it's printable
	processedFiles := make(map[string]bool)
	
	// Count staged files in this directory
	for path := range m.markedFiles {
		// Check if file is direct child of dirPath (not in subdirectories)
		if strings.HasPrefix(path, dirPrefix) {
			relPath := strings.TrimPrefix(path, dirPrefix)
			if !strings.Contains(relPath, string(filepath.Separator)) {
				// It's a direct child
				processedFiles[path] = true
				totalPrintable++
				stagedCount++
			}
		}
	}
	
	// Count printing files in this directory
	for _, job := range m.jobs {
		if strings.HasPrefix(job.FilePath, dirPrefix) {
			relPath := strings.TrimPrefix(job.FilePath, dirPrefix)
			if !strings.Contains(relPath, string(filepath.Separator)) {
				// It's a direct child
				if !processedFiles[job.FilePath] {
					totalPrintable++
					processedFiles[job.FilePath] = true
				}
				printingCount++
			}
		}
	}
	
	// For a more accurate count, we'd need to check m.files if this is the current directory
	// or maintain a cache of directory contents. But for status indicators, the above is sufficient.
	// If no files are staged or printing, we show the directory as having no status.
	
	return totalPrintable, stagedCount, printingCount
}


func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (m model) getSelectionSymbol(file FileItem) string {
	if file.IsDir {
		_, staged, printing := m.getDirectoryStatus(file.Path)
		// Simplified logic - we only show status if files are actually staged or printing
		// We don't scan directories just to count printable files
		if printing > 0 && staged > 0 {
			return "◑ " // Some printing, some staged
		}
		if printing > 0 {
			return "● " // Has printing files
		}
		if staged > 0 {
			return "◉ " // Has staged files
		}
		return "  " // No special status
	}
	
	if !file.IsPrintable {
		return "  " // Not printable
	}
	
	// Check if in print queue
	for _, job := range m.jobs {
		if job.FilePath == file.Path {
			return "● " // Printing
		}
	}
	
	// Check if staged
	if m.markedFiles[file.Path] {
		return "◉ " // Staged
	}
	
	// Check if matches pattern
	if m.matchedFiles[file.Path] {
		return "◎ " // Matches pattern
	}
	
	return "○ " // Available
}

func (m model) viewQueuePane() string {
	// Calculate dimensions
	contentWidth := m.width - 4   // Border margins

	// Title bar
	title := titleStyle.Copy().
		Width(contentWidth).
		Align(lipgloss.Center).
		Render("🖨  Printer Queue Manager")

	// Content area
	// Height available = m.height - 4 (for borders/padding)
	// Height for content = available - 4 (title, 2 spacers, help)
	contentHeight := m.height - 8
	queueContent := m.renderQueueContent(contentWidth, contentHeight)

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
	contentWidth := m.width - 4   // Border margins

	// Title
	title := titleStyle.Copy().
		Width(contentWidth).
		Align(lipgloss.Center).
		Render("📁 Add Files to Print Queue")

	// Files content
	// Height available = m.height - 4 (for borders/padding)
	// Height for content = available - 4 (title, 2 spacers, help)
	contentHeight := m.height - 8
	filesContent := m.renderFilesContent(contentWidth, contentHeight)

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
