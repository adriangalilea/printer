package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// PrintOperation tracks a print operation initiated by the user
type PrintOperation struct {
	ID        string
	FilePath  string
	FileName  string
	Status    PrintStatus
	Error     error
	StartedAt time.Time
	UpdatedAt time.Time
	SystemJobID string // The actual system print job ID if successfully submitted
}

func (m *model) renderPrintContent(width, height int) string {
	if height <= 0 {
		return ""
	}

	var result strings.Builder
	
	// Header
	header := "🖨  PRINT OPERATIONS"
	if len(m.printOps) > 0 {
		activeCount := 0
		for _, op := range m.printOps {
			if op.Status == StatusSending {
				activeCount++
			}
		}
		if activeCount > 0 {
			header = fmt.Sprintf("🖨  PRINT OPERATIONS (%d active)", activeCount)
		} else {
			header = fmt.Sprintf("🖨  PRINT OPERATIONS (%d)", len(m.printOps))
		}
	}
	result.WriteString(activeHeaderStyle.Render(header))
	result.WriteString("\n")
	
	// Calculate available height for scrollable content
	contentHeight := height - 1 // Subtract header
	if contentHeight <= 0 {
		return result.String()
	}
	
	// Build content for scrollable area
	var content strings.Builder
	if len(m.printOps) == 0 {
		content.WriteString(dimStyle.Render("  No print operations"))
	} else {
		for i, op := range m.printOps {
			cursor := "  "
			if i == m.printCursor && m.activePane == PanePrint {
				cursor = "▶ "
			}
			
			// Status symbol
			var statusSymbol string
			var statusStyle = normalStyle
			switch op.Status {
			case StatusPending:
				statusSymbol = "⏳"
				statusStyle = dimStyle
			case StatusSending:
				statusSymbol = "📤"
				statusStyle = selectedStyle
			case StatusSent:
				statusSymbol = "✓ "
				statusStyle = printableStyle
			case StatusFailed:
				statusSymbol = "✗ "
				statusStyle = errorStyle
			case StatusCanceled:
				statusSymbol = "⊘ "
				statusStyle = dimStyle
			}
			
			// Format filename with relative path
			fileName := m.formatPrintFileName(op)
			if len(fileName) > width-15 && width > 15 {
				fileName = fileName[:width-18] + "..."
			}
			
			// Build the line
			line := fmt.Sprintf("%s%s %s", cursor, statusSymbol, fileName)
			
			// Add status text for failed operations
			if op.Status == StatusFailed && op.Error != nil {
				errorText := fmt.Sprintf(" - %s", op.Error.Error())
				if len(line+errorText) > width-2 && width > 2 {
					errorText = errorText[:width-len(line)-5] + "..."
				}
				line += errorText
			}
			
			// Add time info
			timeAgo := m.formatTimeAgo(op.UpdatedAt)
			if len(line) + len(timeAgo) + 3 < width {
				padding := width - len(line) - len(timeAgo) - 2
				if padding > 0 {
					line += strings.Repeat(" ", padding) + timeAgo
				}
			}
			
			// Apply style
			if i == m.printCursor && m.activePane == PanePrint {
				content.WriteString(selectedStyle.Render(line))
			} else {
				content.WriteString(statusStyle.Render(line))
			}
			
			if i < len(m.printOps)-1 {
				content.WriteString("\n")
			}
		}
	}
	
	// Create scrollable area
	scroll := NewScrollableArea(width, contentHeight)
	scroll.SetContent(content.String())
	
	// Ensure cursor is visible
	if m.activePane == PanePrint && len(m.printOps) > 0 {
		scroll.ScrollToLine(m.printCursor)
	}
	
	result.WriteString(scroll.Render())
	
	return result.String()
}

func (m *model) formatPrintFileName(op PrintOperation) string {
	// Show relative path from current directory
	relPath, err := filepath.Rel(m.currentDir, op.FilePath)
	if err != nil {
		// If we can't get relative path, just show the filename
		return op.FileName
	}
	
	// If file is in current directory, just show the name
	if !strings.Contains(relPath, string(filepath.Separator)) && !strings.HasPrefix(relPath, "..") {
		return relPath
	}
	
	// Otherwise show the relative path
	return relPath
}

func (m *model) formatTimeAgo(t time.Time) string {
	duration := time.Since(t)
	
	if duration < time.Second {
		return "now"
	} else if duration < time.Minute {
		return fmt.Sprintf("%ds ago", int(duration.Seconds()))
	} else if duration < time.Hour {
		return fmt.Sprintf("%dm ago", int(duration.Minutes()))
	} else if duration < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(duration.Hours()))
	} else {
		return fmt.Sprintf("%dd ago", int(duration.Hours()/24))
	}
}

func (m *model) updatePrintPane(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.printCursor > 0 {
			m.printCursor--
		}
		
	case "down", "j":
		if m.printCursor < len(m.printOps)-1 {
			m.printCursor++
		}
		
	case "pgup", "ctrl+u":
		newCursor := m.printCursor - 5
		if newCursor < 0 {
			newCursor = 0
		}
		m.printCursor = newCursor
		
	case "pgdown", "ctrl+d":
		newCursor := m.printCursor + 5
		if newCursor >= len(m.printOps) {
			newCursor = len(m.printOps) - 1
		}
		if newCursor < 0 {
			newCursor = 0
		}
		m.printCursor = newCursor
		
	case "r":
		// Retry failed print operation
		if m.printCursor < len(m.printOps) {
			op := &m.printOps[m.printCursor]
			if op.Status == StatusFailed || op.Status == StatusCanceled {
				// Reset status and resubmit
				op.Status = StatusPending
				op.Error = nil
				op.UpdatedAt = time.Now()
				
				// Return command to retry the print job
				return m, submitPrintJobCmd(op.ID, op.FilePath)
			}
		}
		
	case "x":
		// Cancel or remove print operation
		if m.printCursor < len(m.printOps) {
			op := &m.printOps[m.printCursor]
			if op.Status == StatusSending || op.Status == StatusPending {
				// Cancel active operation
				op.Status = StatusCanceled
				op.UpdatedAt = time.Now()
			} else if op.Status == StatusFailed || op.Status == StatusCanceled || op.Status == StatusSent {
				// Remove completed/failed/canceled operation
				m.printOps = append(m.printOps[:m.printCursor], m.printOps[m.printCursor+1:]...)
				if m.printCursor >= len(m.printOps) && m.printCursor > 0 {
					m.printCursor--
				}
			}
		}
		
	case "X":
		// Clear all completed/failed/canceled operations
		var remaining []PrintOperation
		for _, op := range m.printOps {
			if op.Status == StatusPending || op.Status == StatusSending {
				remaining = append(remaining, op)
			}
		}
		m.printOps = remaining
		if m.printCursor >= len(m.printOps) {
			m.printCursor = len(m.printOps) - 1
			if m.printCursor < 0 {
				m.printCursor = 0
			}
		}
		
	case "o":
		// Open file
		if m.printCursor < len(m.printOps) {
			openFile(m.printOps[m.printCursor].FilePath)
		}
		
	case "O":
		// Open containing folder
		if m.printCursor < len(m.printOps) {
			openFolder(m.printOps[m.printCursor].FilePath)
		}
		
	case "a", "f":
		// Switch to files pane
		m.activePane = PaneFiles
		if m.fileFocus == FocusInput {
			m.textInput.Focus()
			return m, textinput.Blink
		}
		
	case "q":
		// Switch to queue pane
		m.activePane = PaneQueue
	}
	
	return m, nil
}