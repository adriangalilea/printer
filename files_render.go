package main

import (
	"fmt"
	"strings"
	"github.com/charmbracelet/lipgloss"
)

func (m *model) renderFilesContent(width, height int) string {
	if height <= 0 {
		return ""
	}

	var result strings.Builder
	
	// Header (not scrollable)
	header := "📁 File Browser"
	result.WriteString(header)
	result.WriteString("\n\n")
	
	// Input field (not scrollable)
	result.WriteString(m.textInput.View())
	result.WriteString("\n\n")
	
	// Calculate remaining height for scrollable file list
	// Account for: header (1 line), spacing after header (1 line), 
	// input (1 line), spacing after input (1 line) = 4 lines total
	scrollableHeight := height - 4
	if scrollableHeight <= 0 {
		return result.String()
	}
	
	// Build file list content for scrollable area
	var fileListContent strings.Builder
	
	if len(m.files) == 0 {
		fileListContent.WriteString(dimStyle.Render("No files"))
	} else {
		for i, file := range m.files {
			// Cursor indicator
			cursor := "  "
			if i == m.fileCursor && m.activePane == PaneFiles && m.fileFocus == FocusFileList {
				cursor = "▶ "
			}
			
			// Get selection symbol based on file state
			selectionSymbol := m.getSelectionSymbol(file)
			
			// Format file name
			displayName := file.Name
			maxNameLen := width - 10
			if len(displayName) > maxNameLen && maxNameLen > 0 {
				displayName = displayName[:maxNameLen-3] + "..."
			}
			
			// Special handling for toggle all item
			if file.Path == "TOGGLE_ALL" {
				// Check if all are selected to show appropriate symbol
				allMarked := true
				for _, f := range m.files {
					if f.IsPrintable && !m.markedFiles[f.Path] {
						allMarked = false
						break
					}
				}
				selectAllSymbol := "○ "
				if allMarked {
					selectAllSymbol = "◉ "
				}
				line := fmt.Sprintf("%s%s %s", cursor, selectAllSymbol, displayName)
				
				if i == m.fileCursor && m.activePane == PaneFiles && m.fileFocus == FocusFileList {
					fileListContent.WriteString(selectedFileStyle.Render(line))
				} else {
					fileListContent.WriteString(selectedStyle.Render(line))
				}
				
				if i < len(m.files)-1 {
					fileListContent.WriteString("\n")
				}
				continue
			}
			
			// Add type indicator
			typeIndicator := ""
			if file.IsDir {
				typeIndicator = "📁 "
			} else if file.IsPrintable {
				typeIndicator = "📄 "
			} else {
				typeIndicator = "   "
			}
			
			line := fmt.Sprintf("%s%s%s%s", cursor, selectionSymbol, typeIndicator, displayName)
			
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
			
			fileListContent.WriteString(styledLine)
			if i < len(m.files)-1 {
				fileListContent.WriteString("\n")
			}
		}
	}
	
	// Create scrollable area for just the file list
	filesScroll := NewScrollableArea(width, scrollableHeight)
	filesScroll.SetContent(fileListContent.String())
	
	// Ensure cursor is visible
	if len(m.files) > 0 && m.fileFocus == FocusFileList {
		filesScroll.ScrollToLine(m.fileCursor)
	}
	
	// Add the scrollable file list to the result
	result.WriteString(filesScroll.Render())
	
	return result.String()
}