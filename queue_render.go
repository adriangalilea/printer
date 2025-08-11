package main

import (
	"fmt"
	"strings"
)

func (m *model) renderQueueContent(width, height int) string {
	if height <= 0 {
		return ""
	}

	var result strings.Builder
	
	// Active print jobs header (not scrollable)
	activeHeader := "🖨  PRINTING"
	if len(m.jobs) > 0 {
		activeHeader = fmt.Sprintf("🖨  PRINTING (%d)", len(m.jobs))
	}
	result.WriteString(activeHeaderStyle.Render(activeHeader))
	result.WriteString("\n")
	
	// Calculate available height
	// Fixed overhead: PRINTING header (1) + separator+spacing (2) + STAGED header (1) = 4 lines
	// Remaining height is split between active and staged scrollable areas
	
	fixedOverhead := 4
	availableHeight := height - fixedOverhead
	
	if availableHeight <= 0 {
		return result.String()
	}
	
	// Dynamic height allocation based on content
	var activeScrollHeight, stagedScrollHeight int
	
	if len(m.jobs) == 0 {
		// No active jobs - give minimal space to active, rest to staged
		activeScrollHeight = 1  // Just 1 line for "No active jobs"
		stagedScrollHeight = availableHeight - activeScrollHeight
		
		if stagedScrollHeight < 1 {
			stagedScrollHeight = 1
		}
	} else {
		// Have active jobs - split space reasonably
		activeJobLines := len(m.jobs)
		
		// Give active jobs what they need, up to 30% of available space
		maxActiveHeight := (availableHeight * 3) / 10
		if maxActiveHeight < 3 {
			maxActiveHeight = 3  // Minimum useful height for active jobs
		}
		
		if activeJobLines <= maxActiveHeight {
			activeScrollHeight = activeJobLines
		} else {
			activeScrollHeight = maxActiveHeight
		}
		
		// Give rest to staged files
		stagedScrollHeight = availableHeight - activeScrollHeight
		
		// Ensure minimum heights
		if activeScrollHeight < 1 {
			activeScrollHeight = 1
		}
		if stagedScrollHeight < 1 {
			stagedScrollHeight = 1
		}
	}
	
	// Build active jobs content for scrollable area
	var activeContent strings.Builder
	if len(m.jobs) == 0 {
		activeContent.WriteString(dimStyle.Render("  No active jobs"))
	} else {
		for i, job := range m.jobs {
			cursor := "  "
			if i == m.activeCursor && m.activePane == PaneQueue && m.queueSection == SectionActive {
				cursor = "▶ "
			}
			
			statusSymbol := "● "
			fileName := job.FileName
			if len(fileName) > width-10 && width > 10 {
				fileName = fileName[:width-13] + "..."
			}
			
			line := fmt.Sprintf("%s%s%s", cursor, statusSymbol, fileName)
			
			if i == m.activeCursor && m.activePane == PaneQueue && m.queueSection == SectionActive {
				activeContent.WriteString(selectedStyle.Render(line))
			} else {
				activeContent.WriteString(normalStyle.Render(line))
			}
			
			if i < len(m.jobs)-1 {
				activeContent.WriteString("\n")
			}
		}
	}
	
	// Create scrollable area for active jobs
	activeScroll := NewScrollableArea(width, activeScrollHeight)
	activeScroll.SetContent(activeContent.String())
	
	// Ensure active cursor is visible
	if m.queueSection == SectionActive && len(m.jobs) > 0 {
		activeScroll.ScrollToLine(m.activeCursor)
	}
	
	// Add active jobs scrollable area
	result.WriteString(activeScroll.Render())
	
	// Separator (not scrollable)
	result.WriteString(dimStyle.Render(strings.Repeat("─", width-2)))
	result.WriteString("\n\n")
	
	// Staged files header (not scrollable)
	relativeStagedFiles := m.getRelativeStagedFiles()
	stagedHeader := fmt.Sprintf("📄 STAGED (%d)", len(relativeStagedFiles))
	result.WriteString(stagedHeaderStyle.Render(stagedHeader))
	result.WriteString("\n")
	
	// Build staged files content for scrollable area
	var stagedContent strings.Builder
	if len(relativeStagedFiles) == 0 {
		stagedContent.WriteString(dimStyle.Render("  No staged files"))
	} else {
		for i, file := range relativeStagedFiles {
			cursor := "  "
			if i == m.stagedCursor && m.activePane == PaneQueue && m.queueSection == SectionStaged {
				cursor = "▶ "
			}
			
			statusSymbol := "◉ "
			fileName := m.formatStagedFileName(file)
			if len(fileName) > width-10 && width > 10 {
				fileName = fileName[:width-13] + "..."
			}
			
			line := fmt.Sprintf("%s%s%s", cursor, statusSymbol, fileName)
			
			if i == m.stagedCursor && m.activePane == PaneQueue && m.queueSection == SectionStaged {
				stagedContent.WriteString(selectedStyle.Render(line))
			} else {
				stagedContent.WriteString(printableStyle.Render(line))
			}
			
			if i < len(relativeStagedFiles)-1 {
				stagedContent.WriteString("\n")
			}
		}
	}
	
	// Create scrollable area for staged files
	stagedScroll := NewScrollableArea(width, stagedScrollHeight)
	stagedScroll.SetContent(stagedContent.String())
	
	// Ensure staged cursor is visible
	if m.queueSection == SectionStaged && len(relativeStagedFiles) > 0 {
		stagedScroll.ScrollToLine(m.stagedCursor)
	}
	
	// Add staged files scrollable area
	result.WriteString(stagedScroll.Render())
	
	return result.String()
}