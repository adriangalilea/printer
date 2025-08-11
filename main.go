package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const version = "0.0.1"

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

	errorStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF6B6B")).
		Bold(true)
	
	// Border styles
	activeBorderStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#01FAC6"))
	
	inactiveBorderStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#626262"))
)

type Mode int

const (
	ModeQueue Mode = iota
	ModeAddFiles
)

type FocusPane int

const (
	FocusInput FocusPane = iota
	FocusFiles
)

type FileItem struct {
	Name       string
	Path       string
	IsDir      bool
	IsPrintable bool
	Size       int64
}

type PrintJob struct {
	ID       string
	FileName string
	FilePath string
	Size     int64
	Status   string
}

type model struct {
	mode          Mode
	focusPane     FocusPane
	jobs          []PrintJob
	cursor        int
	selected      map[int]bool
	width         int
	height        int
	
	// File browser state
	textInput     textinput.Model
	currentDir    string
	files         []FileItem
	fileCursor    int
	fileOffset    int
	markedFiles   map[string]bool
	
	errorMsg      string
	args          []string
}

func initialModel(args []string) model {
	ti := textinput.New()
	ti.Placeholder = "Type path or glob pattern (e.g., *.pdf)"
	ti.CharLimit = 256
	ti.Width = 50
	
	currentDir, _ := os.Getwd()
	
	m := model{
		mode:        ModeQueue,
		selected:    make(map[int]bool),
		markedFiles: make(map[string]bool),
		textInput:   ti,
		currentDir:  currentDir,
		args:        args,
	}

	// If args provided, start in add mode
	if len(args) > 0 && args[0] == "add" && len(args) > 1 {
		m.mode = ModeAddFiles
		m.focusPane = FocusInput
		m.textInput.SetValue(strings.Join(args[1:], " "))
		m.textInput.Focus()
		m.loadDirectory()
	}

	m.refreshJobs()
	return m
}

func (m *model) refreshJobs() {
	m.jobs = getSystemPrintJobs()
}

func (m *model) loadDirectory() {
	m.files = []FileItem{}
	m.errorMsg = ""
	
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
			Name:  fmt.Sprintf("[Select/Deselect All %d Printable Files]", printableCount),
			Path:  "TOGGLE_ALL",
			IsDir: false,
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
		
		// Check if it matches pattern
		if pattern != "" {
			if matched, _ := filepath.Match(pattern, name); matched {
				if isPrintable {
					m.markedFiles[path] = true
				}
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

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.mode {
		case ModeQueue:
			return m.updateQueueMode(msg)
		case ModeAddFiles:
			return m.updateAddMode(msg)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Update text input width
		m.textInput.Width = m.width - 4
		if m.textInput.Width > 100 {
			m.textInput.Width = 100
		}

	case tickMsg:
		if m.mode == ModeQueue {
			m.refreshJobs()
		}
		return m, tickCmd()
	}

	return m, tea.Batch(cmds...)
}

func (m model) updateQueueMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}

	case "down", "j":
		if m.cursor < len(m.jobs)-1 {
			m.cursor++
		}

	case "a":
		m.mode = ModeAddFiles
		m.focusPane = FocusInput
		m.textInput.Focus()
		m.textInput.SetValue("")
		m.fileCursor = 0
		m.fileOffset = 0
		m.markedFiles = make(map[string]bool)
		m.loadDirectory()
		return m, textinput.Blink

	case "x":
		if m.cursor < len(m.jobs) {
			cancelPrintJob(m.jobs[m.cursor].ID)
			m.refreshJobs()
		}

	case "o":
		if m.cursor < len(m.jobs) {
			openFile(m.jobs[m.cursor].FilePath)
		}

	case "O":
		if m.cursor < len(m.jobs) {
			openFolder(m.jobs[m.cursor].FilePath)
		}

	case "r":
		m.refreshJobs()

	case " ":
		if m.cursor < len(m.jobs) {
			if m.selected[m.cursor] {
				delete(m.selected, m.cursor)
			} else {
				m.selected[m.cursor] = true
			}
		}

	case "X":
		for i := range m.selected {
			if i < len(m.jobs) {
				cancelPrintJob(m.jobs[i].ID)
			}
		}
		m.selected = make(map[int]bool)
		m.refreshJobs()
	}

	return m, nil
}

func (m model) updateAddMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit

	case "esc":
		m.mode = ModeQueue
		m.textInput.Blur()
		m.refreshJobs()
		return m, nil

	case "tab":
		// Switch focus between input and file list
		if m.focusPane == FocusInput {
			m.focusPane = FocusFiles
			m.textInput.Blur()
			if len(m.files) > 0 && m.fileCursor == 0 {
				m.fileCursor = 0
			}
		} else {
			m.focusPane = FocusInput
			m.textInput.Focus()
			return m, textinput.Blink
		}
		return m, nil

	case "enter":
		if m.focusPane == FocusFiles && m.fileCursor < len(m.files) {
			file := m.files[m.fileCursor]
			if file.IsDir {
				// Navigate into directory
				m.currentDir = file.Path
				m.fileCursor = 0
				m.fileOffset = 0
				m.markedFiles = make(map[string]bool)
				m.loadDirectory()
			} else if file.IsPrintable {
				// Add single file
				addToPrintQueue(file.Path)
				m.mode = ModeQueue
				m.textInput.Blur()
				m.refreshJobs()
			}
		} else {
			// Add all marked files
			for path := range m.markedFiles {
				addToPrintQueue(path)
			}
			if len(m.markedFiles) > 0 {
				m.mode = ModeQueue
				m.textInput.Blur()
				m.refreshJobs()
			}
		}
		return m, nil

	case "up", "k":
		if m.focusPane == FocusFiles {
			if m.fileCursor > 0 {
				m.fileCursor--
			} else {
				// Move to input when at top
				m.focusPane = FocusInput
				m.textInput.Focus()
				return m, textinput.Blink
			}
		}
		return m, nil

	case "down", "j":
		if m.focusPane == FocusInput {
			// Move to file list
			m.focusPane = FocusFiles
			m.textInput.Blur()
			if len(m.files) > 0 {
				m.fileCursor = 0
			}
		} else if m.focusPane == FocusFiles {
			if m.fileCursor < len(m.files)-1 {
				m.fileCursor++
			}
		}
		return m, nil
	
	case "left", "h", "backspace":
		if m.focusPane == FocusFiles && m.currentDir != "/" {
			// Go to parent directory
			m.currentDir = filepath.Dir(m.currentDir)
			m.fileCursor = 0
			m.fileOffset = 0
			m.loadDirectory()
		}
		return m, nil
	
	case "right", "l":
		if m.focusPane == FocusFiles && m.fileCursor < len(m.files) {
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
		if m.focusPane == FocusFiles && m.fileCursor < len(m.files) {
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
					// Deselect all
					m.markedFiles = make(map[string]bool)
				} else {
					// Select all printable files
					for _, f := range m.files {
						if f.IsPrintable {
							m.markedFiles[f.Path] = true
						}
					}
				}
			} else if file.IsPrintable {
				if m.markedFiles[file.Path] {
					delete(m.markedFiles, file.Path)
				} else {
					m.markedFiles[file.Path] = true
				}
			}
		}
		return m, nil
	}

	// Handle text input when focused
	if m.focusPane == FocusInput {
		oldValue := m.textInput.Value()
		m.textInput, cmd = m.textInput.Update(msg)
		if m.textInput.Value() != oldValue {
			m.loadDirectory()
		}
		return m, cmd
	}

	return m, nil
}

func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Initializing..."
	}
	
	switch m.mode {
	case ModeAddFiles:
		return m.viewAddMode()
	default:
		return m.viewQueueMode()
	}
}

func (m model) viewQueueMode() string {
	// Calculate dimensions
	contentHeight := m.height - 4 // Title, spacing, help
	contentWidth := m.width - 4   // Margins
	
	// Title bar
	title := titleStyle.Copy().
		Width(contentWidth).
		Align(lipgloss.Center).
		Render("🖨  Printer Queue Manager")
	
	// Content area
	var content strings.Builder
	
	if len(m.jobs) == 0 {
		emptyMsg := dimStyle.Copy().
			Width(contentWidth).
			Height(contentHeight - 6).
			Align(lipgloss.Center, lipgloss.Center).
			Render("No print jobs in queue\n\nPress 'a' to add files")
		content.WriteString(emptyMsg)
	} else {
		// Calculate visible jobs
		visibleJobs := contentHeight - 6
		start := 0
		if m.cursor >= visibleJobs {
			start = m.cursor - visibleJobs + 1
		}
		end := start + visibleJobs
		if end > len(m.jobs) {
			end = len(m.jobs)
		}
		
		for i := start; i < end; i++ {
			job := m.jobs[i]
			cursor := "  "
			if i == m.cursor {
				cursor = "▶ "
			}
			
			selected := " "
			if m.selected[i] {
				selected = "✓"
			}
			
			// Format job line
			jobID := job.ID
			if len(jobID) > 20 {
				jobID = jobID[:17] + "..."
			}
			
			fileName := job.FileName
			maxFileLen := contentWidth - 40
			if len(fileName) > maxFileLen && maxFileLen > 0 {
				fileName = "..." + fileName[len(fileName)-maxFileLen+3:]
			}
			
			line := fmt.Sprintf("%s[%s] %-20s %s %s",
				cursor, selected, jobID, fileName, formatSize(job.Size))
			
			if i == m.cursor {
				content.WriteString(selectedStyle.Render(line))
			} else {
				content.WriteString(normalStyle.Render(line))
			}
			
			if i < end-1 {
				content.WriteString("\n")
			}
		}
		
		// Show scroll indicator
		if len(m.jobs) > visibleJobs {
			scrollInfo := fmt.Sprintf("\n\n%s (%d/%d)", 
				dimStyle.Render("Showing"), 
				m.cursor+1, len(m.jobs))
			content.WriteString(scrollInfo)
		}
	}
	
	// Help bar
	help := helpStyle.Copy().
		Width(contentWidth).
		Render("a: add • o: open • O: folder • x: cancel • X: cancel selected • space: select • q: quit")
	
	// Combine all parts
	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		content.String(),
		"",
		help,
	)
}

func (m model) viewAddMode() string {
	contentWidth := m.width - 4
	contentHeight := m.height - 8 // Title, input, help, spacing
	
	// Title
	title := titleStyle.Copy().
		Width(contentWidth).
		Align(lipgloss.Center).
		Render("📁 Add Files to Print Queue")
	
	// Input field with border
	inputBorder := inactiveBorderStyle
	if m.focusPane == FocusInput {
		inputBorder = activeBorderStyle
	}
	inputView := inputBorder.Copy().
		Width(contentWidth - 2).
		Render(m.textInput.View())
	
	// File browser with current path in title
	displayDir := m.currentDir
	if home, _ := os.UserHomeDir(); strings.HasPrefix(displayDir, home) {
		displayDir = "~" + strings.TrimPrefix(displayDir, home)
	}
	
	fileBorder := inactiveBorderStyle
	if m.focusPane == FocusFiles {
		fileBorder = activeBorderStyle
	}
	
	// Calculate visible files
	browserHeight := contentHeight - 6
	if browserHeight < 5 {
		browserHeight = 5
	}
	
	var fileList strings.Builder
	
	if len(m.files) == 0 {
		fileList.WriteString(dimStyle.Render("No files in directory"))
	} else {
		// Calculate scroll window
		visibleFiles := browserHeight - 2 // Account for border
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
			if i == m.fileCursor {
				cursor = "▶ "
			}
			
			marked := " "
			if m.markedFiles[file.Path] {
				marked = "✓"
			}
			
			// Format file name
			displayName := file.Name
			maxNameLen := contentWidth - 15
			if len(displayName) > maxNameLen && maxNameLen > 0 {
				displayName = displayName[:maxNameLen-3] + "..."
			}
			
			// Special handling for toggle all item
			if file.Path == "TOGGLE_ALL" {
				line := fmt.Sprintf("%s    %s", cursor, displayName)
				fileList.WriteString(selectedStyle.Render(line))
				if i < end-1 {
					fileList.WriteString("\n")
				}
				continue
			}
			
			// Add type indicator
			typeIndicator := "   "
			if file.IsDir {
				typeIndicator = "📁 "
			} else if file.IsPrintable {
				typeIndicator = "📄 "
			} else {
				typeIndicator = "   "
			}
			
			line := fmt.Sprintf("%s[%s] %s%s", cursor, marked, typeIndicator, displayName)
			
			// Apply styles
			var styledLine string
			if i == m.fileCursor && m.focusPane == FocusFiles {
				styledLine = selectedFileStyle.Render(line)
			} else if m.markedFiles[file.Path] {
				styledLine = markedStyle.Render(line)
			} else if file.IsDir {
				styledLine = dirStyle.Render(line)
			} else if file.IsPrintable {
				styledLine = printableStyle.Render(line)
			} else {
				styledLine = dimStyle.Render(line)
			}
			
			fileList.WriteString(styledLine)
			if i < end-1 {
				fileList.WriteString("\n")
			}
		}
		
		// Scroll indicator
		if len(m.files) > visibleFiles {
			scrollInfo := fmt.Sprintf("\n%s (%d/%d files, %d marked)",
				dimStyle.Render("Scroll"),
				m.fileCursor+1, len(m.files), len(m.markedFiles))
			fileList.WriteString(scrollInfo)
		}
	}
	
	// Add path header above the file browser
	pathHeader := fmt.Sprintf("📂 %s", displayDir)
	maxPathLen := contentWidth - 5
	if len(pathHeader) > maxPathLen && maxPathLen > 0 {
		pathHeader = "..." + pathHeader[len(pathHeader)-(maxPathLen-3):]
	}
	
	pathHeaderStyle := dimStyle
	if m.focusPane == FocusFiles {
		pathHeaderStyle = selectedStyle
	}
	pathHeaderView := pathHeaderStyle.Copy().
		Width(contentWidth - 2).
		Render(pathHeader)
	
	fileBrowserView := fileBorder.Copy().
		Width(contentWidth - 2).
		Height(browserHeight - 1).
		Render(fileList.String())
	
	// Error message
	errorView := ""
	if m.errorMsg != "" {
		errorView = errorStyle.Render(m.errorMsg) + "\n"
	}
	
	// Help
	help := helpStyle.Copy().
		Width(contentWidth).
		Render("tab: switch panes • ←→: navigate dirs • space: mark • enter: add/open • esc: cancel")
	
	// Combine all parts
	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		inputView,
		"",
		pathHeaderView,
		fileBrowserView,
		errorView,
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
