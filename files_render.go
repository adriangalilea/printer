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
	result.WriteString("\n")

	// Input field with visual box
	inputLine := fmt.Sprintf("┌%s┐", strings.Repeat("─", width-4))
	result.WriteString(dimStyle.Render(inputLine))
	result.WriteString("\n")
	result.WriteString(dimStyle.Render("│ ") + m.textInput.View())
	result.WriteString("\n")
	inputBottom := fmt.Sprintf("└%s┘", strings.Repeat("─", width-4))
	result.WriteString(dimStyle.Render(inputBottom))
	result.WriteString("\n")
	
	// Calculate remaining height for scrollable file list
	// Account for: header (1), input box (3 lines: top border, input, bottom border), spacing (1) = 5 lines
	scrollableHeight := height - 5
	if scrollableHeight <= 0 {
		return result.String()
	}
	
	// Build file list content for scrollable area
	var fileListContent strings.Builder
	
	if len(m.files) == 0 {
		fileListContent.WriteString(dimStyle.Render("  No files"))
	} else {
		for i, file := range m.files {
			isCursor := i == m.fileCursor && m.activePane == PaneFiles && m.fileFocus == FocusFileList
			selectionSymbol := m.getSelectionSymbol(file)

			displayName := file.Name
			maxNameLen := width - 10
			if maxNameLen > 0 && len(displayName) > maxNameLen {
				displayName = displayName[:maxNameLen-3] + "..."
			}

			// Special handling for toggle all item
			if file.Path == "TOGGLE_ALL" {
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
				content := fmt.Sprintf("%s%s", selectAllSymbol, displayName)
				fileListContent.WriteString(renderSelectable(isCursor, 2, content, selectedFileStyle, selectedStyle))
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

			content := fmt.Sprintf("%s%s%s", selectionSymbol, typeIndicator, displayName)
			isMarked := m.markedFiles[file.Path]
			isMatched := m.matchedFiles[file.Path]

			// Determine styles based on state
			var selStyle, normStyle lipgloss.Style
			if isMarked {
				selStyle = selectedFileStyle.Copy().Background(theme.Yellow)
				normStyle = markedStyle
			} else if isMatched {
				selStyle = selectedFileStyle
				normStyle = matchedStyle
			} else if file.IsDir {
				selStyle = selectedFileStyle
				normStyle = dirStyle
			} else if file.IsPrintable {
				selStyle = selectedFileStyle
				normStyle = printableStyle
			} else {
				selStyle = selectedFileStyle
				normStyle = dimStyle
			}

			fileListContent.WriteString(renderSelectable(isCursor, 2, content, selStyle, normStyle))
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