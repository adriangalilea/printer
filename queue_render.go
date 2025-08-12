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
	// Count deduplicated jobs
	totalJobs := len(m.jobs)
	// Add print operations that don't have corresponding system jobs
	for _, op := range m.printOps {
		hasSystemJob := false
		for _, job := range m.jobs {
			if job.FilePath == op.FilePath && job.FilePath != "" {
				hasSystemJob = true
				break
			}
		}
		// Count operations that are not in system queue and not successfully sent
		if !hasSystemJob && op.Status != StatusSent {
			totalJobs++
		}
	}
	
	activeHeader := "🖨  PRINTING"
	if totalJobs > 0 {
		activeHeader = fmt.Sprintf("🖨  PRINTING (%d)", totalJobs)
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
	
	if totalJobs == 0 {
		// No active jobs - give minimal space to active, rest to staged
		activeScrollHeight = 1  // Just 1 line for "No active jobs"
		stagedScrollHeight = availableHeight - activeScrollHeight
		
		if stagedScrollHeight < 1 {
			stagedScrollHeight = 1
		}
	} else {
		// Have active jobs - split space reasonably
		activeJobLines := totalJobs  // Use total jobs count (system + print ops)
		
		// Give active jobs what they need, up to 50% of available space (increased from 30%)
		maxActiveHeight := availableHeight / 2
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
	
	// FIXME: MASSIVE DEDUPLICATION PROBLEM!
	// The deduplication logic is duplicated in multiple places:
	// 1. Here in queue_render.go for display
	// 2. In main.go getActualJobCount() 
	// 3. In main.go for cursor navigation
	// 4. In main.go for handling actions (cancel, open, etc.)
	// 
	// This needs to be centralized into a single source of truth!
	// We should have ONE function that builds the deduplicated list
	// and everything else should use that list.
	//
	// ALSO: Jobs without filenames (system jobs we didn't track) can't be canceled
	// because we're matching by FilePath which is empty for those jobs.
	// This breaks the cancel functionality for any job not initiated through our app.
	//
	// Build active jobs content for scrollable area
	// Merge system jobs and our print operations, deduplicating by file path
	
	var activeContent strings.Builder
	if totalJobs == 0 {
		activeContent.WriteString(dimStyle.Render("  No active jobs"))
	} else {
		itemIndex := 0
		
		// Create a map of our print operations by file path for quick lookup
		printOpsByPath := make(map[string]*PrintOperation)
		for i := range m.printOps {
			op := &m.printOps[i]
			printOpsByPath[op.FilePath] = op
		}
		
		// Track which print operations we've already shown (matched with system jobs)
		shownOps := make(map[string]bool)
		
		// First, show system print jobs (enriched with our tracking data if available)
		for _, job := range m.jobs {
			cursor := "  "
			if itemIndex == m.activeCursor && m.activePane == PaneQueue && m.queueSection == SectionActive {
				cursor = "▶ "
			}
			
			// Check if we have tracking data for this job
			var statusSymbol string
			var statusStyle = normalStyle
			fileName := job.FileName
			
			if op, exists := printOpsByPath[job.FilePath]; exists && job.FilePath != "" {
				// We have tracking data for this job - use our enhanced status
				shownOps[job.FilePath] = true
				
				switch op.Status {
				case StatusPending:
					statusSymbol = "⏳"
					statusStyle = dimStyle
				case StatusSending:
					statusSymbol = "📤"
					statusStyle = selectedStyle
				case StatusSent: // If sent but still in system queue
					statusSymbol = "● "
					statusStyle = normalStyle
				case StatusFailed:
					statusSymbol = "✗ "
					statusStyle = errorStyle
				case StatusCanceled:
					statusSymbol = "⊘ "
					statusStyle = dimStyle
				}
				
				// Use our filename if available
				if op.FileName != "" {
					fileName = op.FileName
				}
			} else {
				// No tracking data - just show as active system job
				statusSymbol = "● "
			}
			
			if len(fileName) > width-10 && width > 10 {
				fileName = fileName[:width-13] + "..."
			}
			
			line := fmt.Sprintf("%s%s%s", cursor, statusSymbol, fileName)
			
			if itemIndex == m.activeCursor && m.activePane == PaneQueue && m.queueSection == SectionActive {
				activeContent.WriteString(selectedStyle.Render(line))
			} else {
				activeContent.WriteString(statusStyle.Render(line))
			}
			
			if itemIndex < totalJobs-1 {
				activeContent.WriteString("\n")
			}
			itemIndex++
		}
		
		// Then, show print operations that don't have corresponding system jobs
		// (these are either completed, failed, or not yet in system queue)
		for _, op := range m.printOps {
			// Skip if we already showed this with a system job
			if shownOps[op.FilePath] {
				continue
			}
			
			// Don't show successfully sent jobs that are no longer in system queue
			if op.Status == StatusSent {
				continue
			}
			
			cursor := "  "
			if itemIndex == m.activeCursor && m.activePane == PaneQueue && m.queueSection == SectionActive {
				cursor = "▶ "
			}
			
			// Status symbol based on operation status
			var statusSymbol string
			var statusStyle = normalStyle
			switch op.Status {
			case StatusPending:
				statusSymbol = "⏳"
				statusStyle = dimStyle
			case StatusSending:
				statusSymbol = "📤"
				statusStyle = selectedStyle
			case StatusFailed:
				statusSymbol = "✗ "
				statusStyle = errorStyle
			case StatusCanceled:
				statusSymbol = "⊘ "
				statusStyle = dimStyle
			}
			
			fileName := op.FileName
			if len(fileName) > width-15 && width > 15 {
				fileName = fileName[:width-18] + "..."
			}
			
			line := fmt.Sprintf("%s%s %s", cursor, statusSymbol, fileName)
			
			// Add error message for failed operations
			if op.Status == StatusFailed && op.Error != nil {
				errorText := fmt.Sprintf(" - %s", op.Error.Error())
				if len(line+errorText) > width-2 && width > 2 {
					errorText = errorText[:width-len(line)-5] + "..."
				}
				line += errorText
			}
			
			if itemIndex == m.activeCursor && m.activePane == PaneQueue && m.queueSection == SectionActive {
				activeContent.WriteString(selectedStyle.Render(line))
			} else {
				activeContent.WriteString(statusStyle.Render(line))
			}
			
			activeContent.WriteString("\n")
			itemIndex++
		}
	}
	
	// Create scrollable area for active jobs
	activeScroll := NewScrollableArea(width, activeScrollHeight)
	activeScroll.SetContent(activeContent.String())
	
	// Ensure active cursor is visible
	if m.queueSection == SectionActive && totalJobs > 0 {
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