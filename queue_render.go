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

	// Printer info header
	printer := getDefaultPrinter()
	printerLine := fmt.Sprintf("🖨  %s", printer.Name)
	if printer.Status != "" {
		printerLine += fmt.Sprintf(" (%s)", printer.Status)
	}
	result.WriteString(dimStyle.Render(printerLine))
	result.WriteString("\n")

	// Active print jobs header (not scrollable)
	// Count deduplicated jobs
	totalJobs := len(m.jobs)
	// Add print operations that don't have corresponding system jobs (match by job ID)
	for _, op := range m.printOps {
		hasSystemJob := false
		if op.SystemJobID != "" {
			for _, job := range m.jobs {
				if job.ID == op.SystemJobID {
					hasSystemJob = true
					break
				}
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
	// Overhead: printer line (1) + PRINTING header (1) + separator+spacing (2) + STAGED header (1) = 5
	fixedOverhead := 5
	availableHeight := height - fixedOverhead

	if availableHeight <= 0 {
		return result.String()
	}

	// Dynamic height allocation based on content
	var activeScrollHeight, stagedScrollHeight int

	if totalJobs == 0 {
		activeScrollHeight = 1
		stagedScrollHeight = availableHeight - activeScrollHeight
		if stagedScrollHeight < 1 {
			stagedScrollHeight = 1
		}
	} else {
		activeJobLines := totalJobs
		maxActiveHeight := availableHeight / 2
		if maxActiveHeight < 3 {
			maxActiveHeight = 3
		}
		if activeJobLines <= maxActiveHeight {
			activeScrollHeight = activeJobLines
		} else {
			activeScrollHeight = maxActiveHeight
		}
		stagedScrollHeight = availableHeight - activeScrollHeight
		if activeScrollHeight < 1 {
			activeScrollHeight = 1
		}
		if stagedScrollHeight < 1 {
			stagedScrollHeight = 1
		}
	}

	// Build active jobs content for scrollable area
	// Create a map of our print operations by job ID for quick lookup
	printOpsByJobID := make(map[string]*PrintOperation)
	for i := range m.printOps {
		op := &m.printOps[i]
		if op.SystemJobID != "" {
			printOpsByJobID[op.SystemJobID] = op
		}
	}

	// Track which print operations we've already shown (matched with system jobs)
	shownOpIDs := make(map[string]bool)

	var activeContent strings.Builder
	if totalJobs == 0 {
		activeContent.WriteString(dimStyle.Render("  No active jobs"))
	} else {
		itemIndex := 0

		// First, show system print jobs (enriched with our tracking data if available)
		for _, job := range m.jobs {
			cursor := "  "
			if itemIndex == m.activeCursor && m.activePane == PaneQueue && m.queueSection == SectionActive {
				cursor = "▶ "
			}

			// Check if we have tracking data for this job (match by job ID)
			var statusSymbol string
			var statusStyle = normalStyle
			fileName := job.FileName

			if op, exists := printOpsByJobID[job.ID]; exists {
				// We have tracking data for this job - use our enhanced status
				shownOpIDs[op.ID] = true

				switch op.Status {
				case StatusPending:
					statusSymbol = "⏳"
					statusStyle = dimStyle
				case StatusSending:
					statusSymbol = "📤"
					statusStyle = selectedStyle
				case StatusSent:
					statusSymbol = "● "
					statusStyle = normalStyle
				case StatusFailed:
					statusSymbol = "✗ "
					statusStyle = errorStyle
				case StatusCanceled:
					statusSymbol = "⊘ "
					statusStyle = dimStyle
				}

				// Use our filename if available (more complete)
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
		for _, op := range m.printOps {
			// Skip if we already showed this with a system job
			if shownOpIDs[op.ID] {
				continue
			}

			// Skip if it has a system job ID that matches a current job
			if op.SystemJobID != "" {
				hasSystemJob := false
				for _, job := range m.jobs {
					if job.ID == op.SystemJobID {
						hasSystemJob = true
						break
					}
				}
				if hasSystemJob {
					continue
				}
			}

			// Don't show successfully sent jobs that are no longer in system queue
			if op.Status == StatusSent {
				continue
			}

			cursor := "  "
			if itemIndex == m.activeCursor && m.activePane == PaneQueue && m.queueSection == SectionActive {
				cursor = "▶ "
			}

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

	if m.queueSection == SectionActive && totalJobs > 0 {
		activeScroll.ScrollToLine(m.activeCursor)
	}

	result.WriteString(activeScroll.Render())

	// Separator
	result.WriteString(dimStyle.Render(strings.Repeat("─", width-2)))
	result.WriteString("\n\n")

	// Staged files header
	relativeStagedFiles := m.getRelativeStagedFiles()
	stagedHeader := fmt.Sprintf("📄 STAGED (%d)", len(relativeStagedFiles))
	result.WriteString(stagedHeaderStyle.Render(stagedHeader))
	result.WriteString("\n")

	// Build staged files content
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

	stagedScroll := NewScrollableArea(width, stagedScrollHeight)
	stagedScroll.SetContent(stagedContent.String())

	if m.queueSection == SectionStaged && len(relativeStagedFiles) > 0 {
		stagedScroll.ScrollToLine(m.stagedCursor)
	}

	result.WriteString(stagedScroll.Render())

	return result.String()
}
